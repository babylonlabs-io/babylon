package ante

import (
	"math/big"

	errorsmod "cosmossdk.io/errors"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	anteinterfaces "github.com/cosmos/evm/ante/interfaces"
	"github.com/cosmos/evm/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/cosmos/evm/x/vm/wrappers"
	"github.com/ethereum/go-ethereum/common"
)

// UpdateCumulativeGasWanted updates the cumulative gas wanted
func UpdateCumulativeGasWanted(
	ctx sdktypes.Context,
	msgGasWanted uint64,
	maxTxGasWanted uint64,
	cumulativeGasWanted uint64,
) uint64 {
	if ctx.IsCheckTx() && maxTxGasWanted != 0 {
		// We can't trust the tx gas limit, because we'll refund the unused gas.
		if msgGasWanted > maxTxGasWanted {
			cumulativeGasWanted += maxTxGasWanted
		} else {
			cumulativeGasWanted += msgGasWanted
		}
	} else {
		cumulativeGasWanted += msgGasWanted
	}
	return cumulativeGasWanted
}

// ConsumeFeesAndEmitEvent deduces fees from sender and emits the event
func ConsumeFeesAndEmitEvent(
	ctx sdktypes.Context,
	evmKeeper anteinterfaces.EVMKeeper,
	fees sdktypes.Coins,
	from sdktypes.AccAddress,
	accountKeeper anteinterfaces.AccountKeeper,
	bankWrapper wrappers.BankWrapper,
) error {
	if err := deductFees(
		ctx,
		evmKeeper,
		fees,
		from,
		accountKeeper,
		bankWrapper,
	); err != nil {
		return err
	}

	ctx.EventManager().EmitEvent(
		sdktypes.NewEvent(
			sdktypes.EventTypeTx,
			sdktypes.NewAttribute(sdktypes.AttributeKeyFee, fees.String()),
		),
	)
	return nil
}

// deductFee checks if the fee payer has enough funds to pay for the fees and deducts them.
func deductFees(
	ctx sdktypes.Context,
	_ anteinterfaces.EVMKeeper,
	fees sdktypes.Coins,
	feePayer sdktypes.AccAddress,
	accountKeeper anteinterfaces.AccountKeeper,
	bankWrapper wrappers.BankWrapper,
) error {
	if fees.IsZero() {
		return nil
	}

	if err := DeductTxCostsFromUserBalance(
		ctx,
		fees,
		common.BytesToAddress(feePayer),
		accountKeeper,
		bankWrapper,
	); err != nil {
		return errorsmod.Wrapf(err, "failed to deduct transaction costs from user balance")
	}

	return nil
}

// DeductTxCostsFromUserBalance deducts the fees from the user balance.
func DeductTxCostsFromUserBalance(
	ctx sdktypes.Context,
	fees sdktypes.Coins,
	from common.Address,
	accountKeeper anteinterfaces.AccountKeeper,
	bankWrapper wrappers.BankWrapper,
) error {
	// fetch sender account
	signerAcc, err := authante.GetSignerAcc(ctx, accountKeeper, from.Bytes())
	if err != nil {
		return errorsmod.Wrapf(err, "account not found for sender %s", from)
	}

	// Deduct fees from the user balance. Notice that it is used
	// the bankWrapper to properly convert fees from the 18 decimals
	// representation to the original one before calling into the bank keeper.
	if err := EVMDeductFees(bankWrapper, ctx, signerAcc, fees); err != nil {
		return errorsmod.Wrapf(err, "failed to deduct full gas cost %s from the user %s balance", fees, from)
	}

	return nil
}

// DeductFees deducts fees from the given account.
func EVMDeductFees(bankKeeper wrappers.BankWrapper, ctx sdktypes.Context, acc sdktypes.AccountI, fees sdktypes.Coins) error {
	if !fees.IsValid() {
		return errorsmod.Wrapf(sdkerrors.ErrInsufficientFee, "invalid fee amount: %s", fees)
	}

	// burn the evm coin denom from the account
	feesAfterBurn := sdktypes.Coins{}
	for _, coin := range fees {
		if coin.Denom == evmtypes.GetEVMCoinDenom() {
			err := bankKeeper.BurnAmountFromAccount(ctx, acc.GetAddress(), coin.Amount.BigInt())
			if err != nil {
				return errorsmod.Wrapf(sdkerrors.ErrInsufficientFunds, err.Error())
			}
		} else {
			feesAfterBurn = feesAfterBurn.Add(coin)
		}
	}

	// send the remaining fees of other denoms to the fee collector
	err := bankKeeper.SendCoinsFromAccountToModule(ctx, acc.GetAddress(), authtypes.FeeCollectorName, feesAfterBurn)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInsufficientFunds, err.Error())
	}

	return nil
}

// GetMsgPriority returns the priority of an Eth Tx capped by the minimum priority
func GetMsgPriority(
	txData evmtypes.TxData,
	minPriority int64,
	baseFee *big.Int,
) int64 {
	priority := evmtypes.GetTxPriority(txData, baseFee)

	if priority < minPriority {
		minPriority = priority
	}
	return minPriority
}

// TODO: (@fedekunze) Why is this necessary? This seems to be a duplicate from the CheckGasWanted function.
func CheckBlockGasLimit(ctx sdktypes.Context, gasWanted uint64, minPriority int64) (sdktypes.Context, error) {
	blockGasLimit := types.BlockGasLimit(ctx)

	// return error if the tx gas is greater than the block limit (max gas)

	// NOTE: it's important here to use the gas wanted instead of the gas consumed
	// from the tx gas pool. The latter only has the value so far since the
	// EthSetupContextDecorator, so it will never exceed the block gas limit.
	if gasWanted > blockGasLimit {
		return ctx, errorsmod.Wrapf(
			sdkerrors.ErrOutOfGas,
			"tx gas (%d) exceeds block gas limit (%d)",
			gasWanted,
			blockGasLimit,
		)
	}

	// Set tx GasMeter with a limit of GasWanted (i.e. gas limit from the Ethereum tx).
	// The gas consumed will be then reset to the gas used by the state transition
	// in the EVM.

	// FIXME: use a custom gas configuration that doesn't add any additional gas and only
	// takes into account the gas consumed at the end of the EVM transaction.
	ctx = ctx.
		WithGasMeter(types.NewInfiniteGasMeterWithLimit(gasWanted)).
		WithPriority(minPriority)

	return ctx, nil
}
