package keeper

import (
	"bytes"
	"context"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/x/feegrant"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/gogoproto/proto"

	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

// RefundTx refunds the transaction fee to the appropriate party.
// If a fee grant was used, it restores the granted allowance.
// Otherwise, it sends the fee back to the original fee payer.
func (k Keeper) RefundTx(ctx context.Context, tx sdk.FeeTx) error {
	txFee := tx.GetFee()
	if txFee.IsZero() {
		return nil
	}

	feeGranter := tx.FeeGranter()
	feePayer := tx.FeePayer()

	if feeGranter != nil && !bytes.Equal(feeGranter, feePayer) {
		return k.refundToFeeGranter(ctx, feeGranter, feePayer, txFee)
	}

	// refund to fee payer
	return k.bankKeeper.SendCoinsFromModuleToAccount(ctx, k.feeCollectorName, feePayer, txFee)
}

// refundToFeeGranter restores the fee grant allowance by increasing the spend limit or restoring the deleted grant
// and sends the refund to the fee granter
func (k Keeper) refundToFeeGranter(ctx context.Context, feeGranter, feePayer sdk.AccAddress, refund sdk.Coins) error {
	allowance, err := k.feegrantKeeper.GetAllowance(ctx, feeGranter, feePayer)

	if err != nil {
		if errorsmod.IsOf(err, sdkerrors.ErrNotFound) || errorsmod.IsOf(err, feegrant.ErrNoAllowance) {
			// Allowance was totally depleted and deleted
			// The allowance will not be restored in this case
			// Only send refund to fee granter
			return k.bankKeeper.SendCoinsFromModuleToAccount(ctx, k.feeCollectorName, feeGranter, refund)
		}
		// got another error
		return err
	}

	// Existing allowance still present — just increase spend limit
	updatedAllowance, err := restoreAllowanceSpent(allowance, refund)
	if err != nil {
		return err
	}

	if err := k.feegrantKeeper.UpdateAllowance(ctx, feeGranter, feePayer, updatedAllowance); err != nil {
		return fmt.Errorf("failed to update fee allowance: %w", err)
	}

	return k.bankKeeper.SendCoinsFromModuleToAccount(ctx, k.feeCollectorName, feeGranter, refund)
}

// IndexRefundableMsg indexes the given refundable message by its hash.
func (k Keeper) IndexRefundableMsg(ctx context.Context, msg sdk.Msg) {
	msgHash := types.HashMsg(msg)
	err := k.RefundableMsgKeySet.Set(ctx, msgHash)
	if err != nil {
		panic(err) // encoding issue; this can only be a programming error
	}
}

// HasRefundableMsg checks if the message with a given hash is refundable.
func (k Keeper) HasRefundableMsg(ctx context.Context, msgHash []byte) bool {
	has, err := k.RefundableMsgKeySet.Has(ctx, msgHash)
	if err != nil {
		panic(err) // encoding issue; this can only be a programming error
	}
	return has
}

// RemoveRefundableMsg removes the given refundable message from the index.
func (k Keeper) RemoveRefundableMsg(ctx context.Context, msgHash []byte) {
	err := k.RefundableMsgKeySet.Remove(ctx, msgHash)
	if err != nil {
		panic(err) // encoding issue; this can only be a programming error
	}
}

// restoreAllowanceSpent increases the spend limit of a fee grant allowance.
func restoreAllowanceSpent(original feegrant.FeeAllowanceI, refund sdk.Coins) (feegrant.FeeAllowanceI, error) {
	var restoredAllowance feegrant.FeeAllowanceI
	switch a := original.(type) {
	case *feegrant.BasicAllowance:
		newLimit := a.SpendLimit.Add(refund...)
		restoredAllowance = &feegrant.BasicAllowance{
			SpendLimit: newLimit,
			Expiration: a.Expiration,
		}

	case *feegrant.PeriodicAllowance:
		// Initialize new SpendLimit only if the original was not nil
		var newSpendLimit sdk.Coins
		if a.Basic.SpendLimit != nil {
			newSpendLimit = a.Basic.SpendLimit.Add(refund...)
		} else {
			newSpendLimit = refund
		}

		// Always add refund to PeriodCanSpend
		newCanSpend := a.PeriodCanSpend.Add(refund...)

		// Create a new BasicAllowance with copied expiration
		newBasic := feegrant.BasicAllowance{
			SpendLimit: newSpendLimit,
			Expiration: a.Basic.Expiration,
		}

		restoredAllowance = &feegrant.PeriodicAllowance{
			Basic:            newBasic,
			Period:           a.Period,
			PeriodSpendLimit: a.PeriodSpendLimit,
			PeriodCanSpend:   newCanSpend,
			PeriodReset:      a.PeriodReset,
		}
	case *feegrant.AllowedMsgAllowance:
		// Unpack the inner allowance
		inner, err := a.GetAllowance()
		if err != nil {
			return nil, fmt.Errorf("failed to get inner allowance: %w", err)
		}

		// Recursively restore it
		restoredInner, err := restoreAllowanceSpent(inner, refund)
		if err != nil {
			return nil, err
		}

		// Repack into Any
		msg, ok := restoredInner.(proto.Message)
		if !ok {
			return nil, errorsmod.Wrapf(sdkerrors.ErrPackAny, "cannot proto marshal %T", msg)
		}
		any, err := codectypes.NewAnyWithValue(msg)
		if err != nil {
			return nil, fmt.Errorf("failed to rewrap restored allowance: %w", err)
		}

		restoredAllowance = &feegrant.AllowedMsgAllowance{
			Allowance:       any,
			AllowedMessages: a.AllowedMessages,
		}

	default:
		return nil, fmt.Errorf("unsupported fee grant type: %T", original)
	}

	if err := restoredAllowance.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("error on restored allowance ValidateBasic: %w", err)
	}

	return restoredAllowance, nil
}
