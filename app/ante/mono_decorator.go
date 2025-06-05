package ante

import (
	"math/big"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	evmante "github.com/cosmos/evm/ante/evm"
	anteinterfaces "github.com/cosmos/evm/ante/interfaces"
	precisebankkeeper "github.com/cosmos/evm/x/precisebank/keeper"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/cosmos/evm/x/vm/wrappers"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// MonoDecorator is a single decorator that handles all the prechecks for
// ethereum transactions.
type MonoDecorator struct {
	accountKeeper   anteinterfaces.AccountKeeper
	feeMarketKeeper anteinterfaces.FeeMarketKeeper
	evmKeeper       anteinterfaces.EVMKeeper
	maxGasWanted    uint64
	bankWrapper     wrappers.BankWrapper
}

// NewEVMMonoDecorator creates the 'mono' decorator, that is used to run the ante handle logic
// for EVM transactions on the chain.
//
// This runs all the default checks for EVM transactions enable through Cosmos EVM.
// Any partner chains can use this in their ante handler logic and build additional EVM
// decorators using the returned DecoratorUtils
func NewEVMMonoDecorator(
	accountKeeper anteinterfaces.AccountKeeper,
	feeMarketKeeper anteinterfaces.FeeMarketKeeper,
	evmKeeper anteinterfaces.EVMKeeper,
	maxGasWanted uint64,
	preciseBankKeeper precisebankkeeper.Keeper,
) MonoDecorator {
	bankWrapper := wrappers.NewBankWrapper(preciseBankKeeper)
	return MonoDecorator{
		accountKeeper:   accountKeeper,
		feeMarketKeeper: feeMarketKeeper,
		evmKeeper:       evmKeeper,
		maxGasWanted:    maxGasWanted,
		bankWrapper:     *bankWrapper,
	}
}

// AnteHandle handles the entire decorator chain using a mono decorator.
func (md MonoDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	// 0. Basic validation of the transaction
	var txFeeInfo *txtypes.Fee
	if !ctx.IsReCheckTx() {
		// NOTE: txFeeInfo is associated with the Cosmos stack, not the EVM. For
		// this reason, the fee is represented in the original decimals and
		// should be converted later when used.
		txFeeInfo, err = evmante.ValidateTx(tx)
		if err != nil {
			return ctx, err
		}
	}

	evmDenom := evmtypes.GetEVMCoinDenom()

	// 1. setup ctx
	ctx, err = evmante.SetupContextAndResetTransientGas(ctx, tx, md.evmKeeper)
	if err != nil {
		return ctx, err
	}

	// 2. get utils
	decUtils, err := evmante.NewMonoDecoratorUtils(ctx, md.evmKeeper)
	if err != nil {
		return ctx, err
	}

	// NOTE: the protocol does not support multiple EVM messages currently so
	// this loop will complete after the first message.
	for i, msg := range tx.GetMsgs() {
		ethMsg, txData, err := evmtypes.UnpackEthMsg(msg)
		if err != nil {
			return ctx, err
		}

		feeAmt := txData.Fee()
		gas := txData.GetGas()
		fee := sdkmath.LegacyNewDecFromBigInt(feeAmt)
		gasLimit := sdkmath.LegacyNewDecFromBigInt(new(big.Int).SetUint64(gas))

		// TODO: computation for mempool and global fee can be made using only
		// the price instead of the fee. This would save some computation.
		//
		// 2. mempool inclusion fee
		if ctx.IsCheckTx() && !simulate {
			// FIX: Mempool dec should be converted
			if err := evmante.CheckMempoolFee(fee, decUtils.MempoolMinGasPrice, gasLimit, decUtils.Rules.IsLondon); err != nil {
				return ctx, err
			}
		}

		if txData.TxType() == ethtypes.DynamicFeeTxType && decUtils.BaseFee != nil {
			// If the base fee is not empty, we compute the effective gas price
			// according to current base fee price. The gas limit is specified
			// by the user, while the price is given by the minimum between the
			// max price paid for the entire tx, and the sum between the price
			// for the tip and the base fee.
			feeAmt = txData.EffectiveFee(decUtils.BaseFee)
			fee = sdkmath.LegacyNewDecFromBigInt(feeAmt)
		}

		// 3. min gas price (global min fee)
		if err := evmante.CheckGlobalFee(fee, decUtils.GlobalMinGasPrice, gasLimit); err != nil {
			return ctx, err
		}

		// 4. validate msg contents
		if err := evmante.ValidateMsg(
			decUtils.EvmParams,
			txData,
			ethMsg.GetFrom(),
		); err != nil {
			return ctx, err
		}

		// 5. signature verification
		if err := evmante.SignatureVerification(
			ethMsg,
			decUtils.Signer,
			decUtils.EvmParams.AllowUnprotectedTxs,
		); err != nil {
			return ctx, err
		}

		from := ethMsg.GetFrom()
		fromAddr := common.BytesToAddress(from)

		// 6. account balance verification
		// We get the account with the balance from the EVM keeper because it is
		// using a wrapper of the bank keeper as a dependency to scale all
		// balances to 18 decimals.
		account := md.evmKeeper.GetAccount(ctx, fromAddr)
		if err := evmante.VerifyAccountBalance(
			ctx,
			md.accountKeeper,
			account,
			fromAddr,
			txData,
		); err != nil {
			return ctx, err
		}

		// 7. can transfer
		coreMsg, err := ethMsg.AsMessage(decUtils.Signer, decUtils.BaseFee)
		if err != nil {
			return ctx, errorsmod.Wrapf(
				err,
				"failed to create an ethereum core.Message from signer %T", decUtils.Signer,
			)
		}

		if err := evmante.CanTransfer(
			ctx,
			md.evmKeeper,
			*coreMsg,
			decUtils.BaseFee,
			decUtils.EvmParams,
			decUtils.Rules.IsLondon,
		); err != nil {
			return ctx, err
		}

		// 8. gas consumption
		msgFees, err := evmkeeper.VerifyFee(
			txData,
			evmDenom,
			decUtils.BaseFee,
			decUtils.Rules.IsHomestead,
			decUtils.Rules.IsIstanbul,
			decUtils.Rules.IsShanghai,
			ctx.IsCheckTx(),
		)
		if err != nil {
			return ctx, err
		}

		err = ConsumeFeesAndEmitEvent(
			ctx,
			md.evmKeeper,
			msgFees,
			from,
			md.accountKeeper,
			md.bankWrapper,
		)
		if err != nil {
			return ctx, err
		}

		gasWanted := evmante.UpdateCumulativeGasWanted(
			ctx,
			gas,
			md.maxGasWanted,
			decUtils.GasWanted,
		)
		decUtils.GasWanted = gasWanted

		minPriority := evmante.GetMsgPriority(
			txData,
			decUtils.MinPriority,
			decUtils.BaseFee,
		)
		decUtils.MinPriority = minPriority

		// Update the fee to be paid for the tx adding the fee specified for the
		// current message.
		decUtils.TxFee.Add(decUtils.TxFee, txData.Fee())

		// Update the transaction gas limit adding the gas specified in the
		// current message.
		decUtils.TxGasLimit += gas

		// 9. increment sequence
		acc := md.accountKeeper.GetAccount(ctx, from)
		if acc == nil {
			// safety check: shouldn't happen
			return ctx, errorsmod.Wrapf(
				errortypes.ErrUnknownAddress,
				"account %s does not exist",
				from,
			)
		}

		if err := evmante.IncrementNonce(ctx, md.accountKeeper, acc, txData.GetNonce()); err != nil {
			return ctx, err
		}

		// 10. gas wanted
		if err := evmante.CheckGasWanted(ctx, md.feeMarketKeeper, tx, decUtils.Rules.IsLondon); err != nil {
			return ctx, err
		}

		// 11. emit events
		txIdx := uint64(i) //nolint:gosec // G115
		evmante.EmitTxHashEvent(ctx, ethMsg, decUtils.BlockTxIndex, txIdx)
	}

	if err := evmante.CheckTxFee(txFeeInfo, decUtils.TxFee, decUtils.TxGasLimit); err != nil {
		return ctx, err
	}

	ctx, err = evmante.CheckBlockGasLimit(ctx, decUtils.GasWanted, decUtils.MinPriority)
	if err != nil {
		return ctx, err
	}

	return next(ctx, tx, simulate)
}
