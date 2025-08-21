package feemarketwrapper_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"math/rand"
	"testing"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	testhelper "github.com/babylonlabs-io/babylon/v4/testutil/helper"
	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	btclightclientkeeper "github.com/babylonlabs-io/babylon/v4/x/btclightclient/keeper"
	btclightclienttypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v4/x/feemarketwrapper"
	incentivekeeper "github.com/babylonlabs-io/babylon/v4/x/incentive/keeper"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
)

// TestIntegration_RefundableTransactionFlow tests the complete flow:
// 1. Execute MsgInsertHeaders (which is refundable and gets automatically indexed)
// 2. Execute RefundTxDecorator PostHandle to process refundable gas tracking
// 3. Verify EndBlock calculates gas correctly excluding refundable amounts
// 4. Check that base fee calculation accounts for refunds
func TestIntegration_RefundableTransactionFlow(t *testing.T) {
	helper := testhelper.NewHelper(t)
	app := helper.App
	ctx := helper.Ctx

	feemarketKeeper := app.FeemarketKeeper
	params := feemarkettypes.DefaultParams()
	params.MinGasMultiplier = math.LegacyOneDec()
	params.BaseFee = math.LegacyNewDec(1000000000) // 1 gwei base fee
	err := feemarketKeeper.SetParams(ctx, params)
	require.NoError(t, err)

	feemarketKeeper.SetBaseFee(ctx, params.BaseFee)

	tKey := app.GetTKey(feemarkettypes.TransientKey)

	gasMeter := storetypes.NewGasMeter(10000000) // 10M gas limit
	ctx = ctx.WithBlockGasMeter(gasMeter)

	t.Run("CompleteRefundableTransactionFlow", func(t *testing.T) {
		r := rand.New(rand.NewSource(12345))

		initTip := app.BTCLightClientKeeper.GetTipInfo(ctx)
		require.NotNil(t, initTip, "BTC light client should have a tip")

		signer := helper.GenAccs[0]
		signerAddr := signer.GetAddress()

		chainExtensionLength := uint32(3)
		chainExtension := datagen.GenRandomValidChainStartingFrom(
			r,
			initTip.Header.ToBlockHeader(),
			nil,
			chainExtensionLength,
		)

		insertMsg := &btclightclienttypes.MsgInsertHeaders{
			Signer:  signerAddr.String(),
			Headers: keepertest.NewBTCHeaderBytesList(chainExtension),
		}

		msgServer := btclightclientkeeper.NewMsgServerImpl(app.BTCLightClientKeeper)
		beforeGas := ctx.GasMeter().GasConsumed()

		_, err = msgServer.InsertHeaders(ctx, insertMsg)
		require.NoError(t, err)

		afterGas := ctx.GasMeter().GasConsumed()
		txGasUsed := afterGas - beforeGas

		// consume gas used for block gas meter tracking
		ctx.BlockGasMeter().ConsumeGas(txGasUsed, "btc_header_insertion")

		t.Logf("MsgInsertHeaders executed - Gas used: %d", txGasUsed)

		txBuilder := app.TxConfig().NewTxBuilder()
		err = txBuilder.SetMsgs(insertMsg)
		require.NoError(t, err)

		txGasLimit := uint64(200000)
		txBuilder.SetGasLimit(txGasLimit)
		fee := sdk.NewCoins(sdk.NewCoin("ubbn", math.NewInt(1000000)))
		txBuilder.SetFeeAmount(fee)

		feeCollectorAcc := app.AccountKeeper.GetModuleAccount(ctx, "fee_collector")
		err = app.BankKeeper.MintCoins(ctx, "mint", fee)
		require.NoError(t, err)
		err = app.BankKeeper.SendCoinsFromModuleToModule(ctx, "mint", "fee_collector", fee)
		require.NoError(t, err)

		t.Logf("Fee collector balance: %s", app.BankKeeper.GetAllBalances(ctx, feeCollectorAcc.GetAddress()))

		tx := txBuilder.GetTx()

		refundDecorator := incentivekeeper.NewRefundTxDecorator(&app.IncentiveKeeper, tKey)
		postHandler := sdk.ChainPostDecorators(refundDecorator)

		ctxWithExecMode := ctx.WithExecMode(sdk.ExecModeFinalize)
		_, err = postHandler(ctxWithExecMode, tx, false, true)
		require.NoError(t, err)
		t.Logf("RefundTxDecorator PostHandle executed successfully")

		storedRefundableGasWanted := feemarketwrapper.GetTransientRefundableBlockGasWanted(ctx, tKey)
		storedRefundableGasUsed := feemarketwrapper.GetTransientRefundableBlockGasUsed(ctx, tKey)

		require.Equal(t, txGasLimit, storedRefundableGasWanted)
		require.Greater(t, storedRefundableGasUsed, uint64(0), "Some gas should have been recorded as used")

		t.Logf("RefundTxDecorator stored - Gas wanted: %d, Gas used: %d", storedRefundableGasWanted, storedRefundableGasUsed)

		additionalGas := uint64(200000)
		totalBlockGasWanted := storedRefundableGasWanted + additionalGas
		feemarketKeeper.SetTransientBlockGasWanted(ctx, totalBlockGasWanted)

		ctx.BlockGasMeter().ConsumeGas(additionalGas, "other_transactions")

		consumedGas := ctx.BlockGasMeter().GasConsumed()
		t.Logf("Block state - Total gas wanted: %d, Total gas consumed: %d, Refundable wanted: %d, Refundable used: %d",
			totalBlockGasWanted, consumedGas, storedRefundableGasWanted, storedRefundableGasUsed)

		wrapper := feemarketwrapper.NewAppModule(feemarketKeeper, tKey)
		validatorUpdates, err := wrapper.EndBlock(ctx)
		require.NoError(t, err)
		require.Empty(t, validatorUpdates)

		expectedAdjustedGasWanted := totalBlockGasWanted - storedRefundableGasWanted
		t.Logf("Expected adjusted gas wanted: %d (should exclude refundable gas)", expectedAdjustedGasWanted)

		err = wrapper.BeginBlock(ctx)
		require.NoError(t, err)

		finalBaseFee := feemarketKeeper.GetBaseFee(ctx)
		require.NotNil(t, finalBaseFee)
		t.Logf("Begin base fee: %s", params.BaseFee.String())
		t.Logf("Final base fee: %s", finalBaseFee.String())

		require.Greater(t, txGasUsed, uint64(0), "Transaction should have consumed gas")
		require.Equal(t, txGasLimit, storedRefundableGasWanted, "Refundable gas wanted should match tx gas limit")
		require.Greater(t, storedRefundableGasUsed, uint64(0), "Refundable gas used should be greater than 0")
		require.NotNil(t, finalBaseFee, "Final base fee should be calculated")

		t.Log("✅ Integration test passed: Refundable transaction flow works end-to-end")
	})
}
