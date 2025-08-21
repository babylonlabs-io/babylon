package feemarketwrapper_test

import (
	"testing"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/stretchr/testify/require"

	testhelper "github.com/babylonlabs-io/babylon/v4/testutil/helper"
	"github.com/babylonlabs-io/babylon/v4/x/feemarketwrapper"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
)

func TestModule_Setup(t *testing.T) {
	helper := testhelper.NewHelper(t)
	require.NotNil(t, helper.App)
	require.NotNil(t, helper.App.FeemarketKeeper)
	require.NotNil(t, helper.Ctx)

	tKey := helper.App.GetTKey(feemarkettypes.TransientKey)
	require.NotNil(t, tKey)

	wrapper := feemarketwrapper.NewAppModule(helper.App.FeemarketKeeper, tKey)
	require.NotNil(t, wrapper)
}

func TestModule_TransientStoreOperations(t *testing.T) {
	helper := testhelper.NewHelper(t)
	ctx := helper.Ctx
	tKey := helper.App.GetTKey(feemarkettypes.TransientKey)

	t.Run("TransientStoreLifecycle", func(t *testing.T) {
		gasWanted := uint64(500000)
		gasUsed := uint64(300000)

		feemarketwrapper.SetTransientRefundableBlockGasWanted(ctx, gasWanted, tKey)
		feemarketwrapper.SetTransientRefundableBlockGasUsed(ctx, gasUsed, tKey)

		retrievedGasWanted := feemarketwrapper.GetTransientRefundableBlockGasWanted(ctx, tKey)
		retrievedGasUsed := feemarketwrapper.GetTransientRefundableBlockGasUsed(ctx, tKey)

		require.Equal(t, gasWanted, retrievedGasWanted)
		require.Equal(t, gasUsed, retrievedGasUsed)
	})

	t.Run("EmptyTransientStore", func(t *testing.T) {
		freshHelper := testhelper.NewHelper(t)
		freshCtx := freshHelper.Ctx
		freshTKey := freshHelper.App.GetTKey(feemarkettypes.TransientKey)

		gasWanted := feemarketwrapper.GetTransientRefundableBlockGasWanted(freshCtx, freshTKey)
		gasUsed := feemarketwrapper.GetTransientRefundableBlockGasUsed(freshCtx, freshTKey)

		require.Equal(t, uint64(0), gasWanted)
		require.Equal(t, uint64(0), gasUsed)
	})
}

func TestModule_BeginBlock(t *testing.T) {
	helper := testhelper.NewHelper(t)
	feemarketKeeper := helper.App.FeemarketKeeper
	ctx := helper.Ctx
	tKey := helper.App.GetTKey(feemarkettypes.TransientKey)

	params := feemarkettypes.DefaultParams()
	params.MinGasMultiplier = math.LegacyOneDec()
	err := feemarketKeeper.SetParams(ctx, params)
	require.NoError(t, err)

	wrapper := feemarketwrapper.NewAppModule(feemarketKeeper, tKey)

	err = wrapper.BeginBlock(ctx)
	require.NoError(t, err)
}

func TestModule_EndBlock(t *testing.T) {
	helper := testhelper.NewHelper(t)
	feemarketKeeper := helper.App.FeemarketKeeper
	ctx := helper.Ctx
	tKey := helper.App.GetTKey(feemarkettypes.TransientKey)

	params := feemarkettypes.DefaultParams()
	params.MinGasMultiplier = math.LegacyOneDec()
	err := feemarketKeeper.SetParams(ctx, params)
	require.NoError(t, err)

	// create a block gas meter to avoid nil pointer error
	gasMeter := storetypes.NewGasMeter(2000000)
	gasMeter.ConsumeGas(100000, "test")
	ctx = ctx.WithBlockGasMeter(gasMeter)

	wrapper := feemarketwrapper.NewAppModule(feemarketKeeper, tKey)

	t.Run("EndBlockNoRefundableGas", func(t *testing.T) {
		feemarketKeeper.SetTransientBlockGasWanted(ctx, 1000000)

		validatorUpdates, err := wrapper.EndBlock(ctx)
		require.NoError(t, err)
		require.Empty(t, validatorUpdates)
	})

	t.Run("EndBlockWithRefundableGas", func(t *testing.T) {
		feemarketKeeper.SetTransientBlockGasWanted(ctx, 1000000)

		refundableGasWanted := uint64(50000)
		refundableGasUsed := uint64(30000)
		feemarketwrapper.SetTransientRefundableBlockGasWanted(ctx, refundableGasWanted, tKey)
		feemarketwrapper.SetTransientRefundableBlockGasUsed(ctx, refundableGasUsed, tKey)

		validatorUpdates, err := wrapper.EndBlock(ctx)
		require.NoError(t, err)
		require.Empty(t, validatorUpdates)

		retrievedWanted := feemarketwrapper.GetTransientRefundableBlockGasWanted(ctx, tKey)
		retrievedUsed := feemarketwrapper.GetTransientRefundableBlockGasUsed(ctx, tKey)
		require.Equal(t, refundableGasWanted, retrievedWanted)
		require.Equal(t, refundableGasUsed, retrievedUsed)
	})
}

func TestModule_FullBlockLifecycle(t *testing.T) {
	helper := testhelper.NewHelper(t)
	feemarketKeeper := helper.App.FeemarketKeeper
	ctx := helper.Ctx
	tKey := helper.App.GetTKey(feemarkettypes.TransientKey)

	params := feemarkettypes.DefaultParams()
	params.MinGasMultiplier = math.LegacyOneDec()
	err := feemarketKeeper.SetParams(ctx, params)
	require.NoError(t, err)

	wrapper := feemarketwrapper.NewAppModule(feemarketKeeper, tKey)

	// simulate full block lifecycle
	t.Run("CompleteBlockExecution", func(t *testing.T) {
		// Step 1: BeginBlock
		err := wrapper.BeginBlock(ctx)
		require.NoError(t, err)

		// Step 2: Initialize block gas wanted to a reasonable value
		feemarketKeeper.SetTransientBlockGasWanted(ctx, 1000000)

		// Step 3: Simulate transaction processing with refundable gas
		refundableGasWanted := uint64(100000)
		refundableGasUsed := uint64(80000)
		feemarketwrapper.SetTransientRefundableBlockGasWanted(ctx, refundableGasWanted, tKey)
		feemarketwrapper.SetTransientRefundableBlockGasUsed(ctx, refundableGasUsed, tKey)

		// Step 4: Set up block gas meter
		gasMeter := storetypes.NewGasMeter(2000000)
		gasMeter.ConsumeGas(500000, "block_execution")
		ctx = ctx.WithBlockGasMeter(gasMeter)

		// Step 5: EndBlock
		validatorUpdates, err := wrapper.EndBlock(ctx)
		require.NoError(t, err)
		require.Empty(t, validatorUpdates)

		// Step 6: Verify transient data persisted through the block
		finalWanted := feemarketwrapper.GetTransientRefundableBlockGasWanted(ctx, tKey)
		finalUsed := feemarketwrapper.GetTransientRefundableBlockGasUsed(ctx, tKey)
		require.Equal(t, refundableGasWanted, finalWanted)
		require.Equal(t, refundableGasUsed, finalUsed)
	})
}
