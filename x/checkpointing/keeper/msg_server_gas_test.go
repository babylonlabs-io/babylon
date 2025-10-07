package keeper_test

import (
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/app"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	testhelper "github.com/babylonlabs-io/babylon/v4/testutil/helper"
	checkpointingkeeper "github.com/babylonlabs-io/babylon/v4/x/checkpointing/keeper"
)

// TestWrappedCreateValidator_OutOfGas tests actual WrappedCreateValidator transaction with gas consumption
func TestWrappedCreateValidator_OutOfGas(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	helper := testhelper.NewHelper(t)

	// Enter first epoch
	ctx, err := helper.ApplyEmptyBlockWithVoteExtension(r)
	require.NoError(t, err)

	// Get default create validator gas fee from epoching params
	epochingParams := helper.App.EpochingKeeper.GetParams(ctx)
	createValidatorGas := epochingParams.ExecuteGas.CreateValidator

	msgServer := checkpointingkeeper.NewMsgServerImpl(helper.App.CheckpointingKeeper)

	// Test with insufficient gas
	t.Run("insufficient_gas", func(t *testing.T) {
		gasLimit := createValidatorGas - 1
		ctx = ctx.WithGasMeter(storetypes.NewGasMeter(gasLimit))

		// Add test addresses with sufficient balance
		addrs, err := app.AddTestAddrs(helper.App, ctx, 1, math.NewInt(100000000))
		require.NoError(t, err)

		// Use datagen to create properly constructed MsgWrappedCreateValidator
		msgCreateValidator, err := datagen.BuildMsgWrappedCreateValidatorWithAmount(addrs[0], math.NewInt(10000000))
		require.NoError(t, err)

		// Should panic due to insufficient gas
		require.Panics(t, func() {
			_, _ = msgServer.WrappedCreateValidator(ctx, msgCreateValidator)
		}, "Expected OutOfGas panic when gasLimit (%d) < required (%d)", gasLimit, createValidatorGas)

		t.Logf("SUCCESS: OutOfGas panic occurred as expected with gasLimit %d < required %d", gasLimit, createValidatorGas)
	})

	// Test with sufficient gas
	t.Run("sufficient_gas", func(t *testing.T) {
		gasLimit := createValidatorGas + 200000
		ctx = ctx.WithGasMeter(storetypes.NewGasMeter(gasLimit))

		// Add test addresses with sufficient balance for validator creation
		addrs, err := app.AddTestAddrs(helper.App, ctx, 1, math.NewInt(100000000))
		require.NoError(t, err)

		initialGas := ctx.GasMeter().GasConsumed()

		// Use datagen to create properly constructed MsgWrappedCreateValidator
		msgCreateValidator, err := datagen.BuildMsgWrappedCreateValidatorWithAmount(addrs[0], math.NewInt(10000000))
		require.NoError(t, err)

		_, err = msgServer.WrappedCreateValidator(ctx, msgCreateValidator)
		finalGas := ctx.GasMeter().GasConsumed()
		actualGasUsed := finalGas - initialGas
		t.Logf("WrappedCreateValidator result - Error: %v, initialGas: %d", err, initialGas)
		t.Logf("WrappedCreateValidator result - Error: %v, Gas consumed: %d", err, actualGasUsed)

		gasConsumed := ctx.GasMeter().GasConsumed()
		require.GreaterOrEqual(t, gasLimit, gasConsumed, "GasLimit should be >= gasConsumed")
	})
}
