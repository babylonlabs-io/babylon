package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	cmtprotocrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/babylonlabs-io/babylon/v3/x/epoching/types"
)

// CheckMsgCreateValidator performs checks on a given `MsgCreateValidator` message
// The checkpointing module will use this function to verify the `MsgCreateValidator` message
// inside a `MsgWrappedCreateValidator` message.
// (adapted from https://github.com/cosmos/cosmos-sdk/blob/v0.46.10/x/staking/keeper/msg_server.go#L34-L108)
func (k Keeper) CheckMsgCreateValidator(ctx context.Context, msg *stakingtypes.MsgCreateValidator) (uint64, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	// ensure validator address is correctly encoded
	valAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		return 0, err
	}

	// get parameters of the staking module
	sParams, err := k.stk.GetParams(ctx)
	if err != nil {
		return 0, err
	}

	// check commission rate
	if msg.Commission.Rate.LT(sParams.MinCommissionRate) {
		return 0, errorsmod.Wrapf(stakingtypes.ErrCommissionLTMinRate, "cannot set validator commission to less than minimum rate of %s", sParams.MinCommissionRate)
	}

	// ensure the validator operator was not registered before
	if _, err := k.stk.GetValidator(ctx, valAddr); err == nil {
		return 0, stakingtypes.ErrValidatorOwnerExists
	}

	// check if the pubkey is correctly encoded
	pk, ok := msg.Pubkey.GetCachedValue().(cryptotypes.PubKey)
	if !ok {
		return 0, errorsmod.Wrapf(sdkerrors.ErrInvalidType, "Expecting cryptotypes.PubKey, got %T", pk)
	}

	// ensure the validator was not registered before
	if _, err := k.stk.GetValidatorByConsAddr(ctx, sdk.GetConsAddress(pk)); err == nil {
		return 0, stakingtypes.ErrValidatorPubKeyExists
	}

	// ensure BondDemon is correct
	if msg.Value.Denom != sParams.BondDenom {
		return 0, errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest, "invalid coin denomination: got %s, expected %s", msg.Value.Denom, sParams.BondDenom,
		)
	}

	// ensure description's length is valid
	if _, err := msg.Description.EnsureLength(); err != nil {
		return 0, err
	}

	// ensure public key type is supported
	cp := sdkCtx.ConsensusParams()
	if cp.Validator != nil {
		pkType := pk.Type()
		hasKeyType := false
		for _, keyType := range cp.Validator.PubKeyTypes {
			if pkType == keyType {
				hasKeyType = true
				break
			}
		}
		if !hasKeyType {
			return 0, errorsmod.Wrapf(
				stakingtypes.ErrValidatorPubKeyTypeNotSupported,
				"got: %s, expected: %s", pk.Type(), cp.Validator.PubKeyTypes,
			)
		}
	}

	// check validator
	validator, err := stakingtypes.NewValidator(valAddr.String(), pk, msg.Description)
	if err != nil {
		return 0, err
	}

	// check if SetInitialCommission fails or not
	commission := stakingtypes.NewCommissionWithTime(
		msg.Commission.Rate, msg.Commission.MaxRate,
		msg.Commission.MaxChangeRate, sdkCtx.HeaderInfo().Time,
	)
	if _, err := validator.SetInitialCommission(commission); err != nil {
		return 0, err
	}

	// sanity check on delegator address -- delegator is the same as validator
	delegatorAddr := sdk.AccAddress(valAddr)
	if err != nil {
		return 0, err
	}

	balance := k.bk.GetBalance(ctx, delegatorAddr, msg.Value.GetDenom())
	if msg.Value.IsGTE(balance) {
		return 0, types.ErrInsufficientBalance
	}

	params := k.GetParams(ctx)

	// check if the delegation amount is above the minimum required amount
	if msg.Value.Amount.LT(math.NewIntFromUint64(params.MinAmount)) {
		return 0, errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"delegation amount %s is below minimum required amount %d",
			msg.Value.Amount.String(),
			params.MinAmount,
		)
	}

	return params.ExecuteGas.CreateValidator, nil
}

// StkMsgCreateValidator calls the staking msg server
func (k Keeper) StkMsgCreateValidator(ctx context.Context, msg *stakingtypes.MsgCreateValidator) error {
	_, err := k.stkMsgServer.CreateValidator(ctx, msg)
	return err
}

func (k Keeper) GetPubKeyByConsAddr(ctx context.Context, consAddr sdk.ConsAddress) (cmtprotocrypto.PublicKey, error) {
	return k.stk.GetPubKeyByConsAddr(ctx, consAddr)
}

func (k Keeper) GetValidator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error) {
	return k.stk.GetValidator(ctx, addr)
}
