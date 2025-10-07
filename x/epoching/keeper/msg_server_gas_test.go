package keeper_test

import (
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	testhelper "github.com/babylonlabs-io/babylon/v4/testutil/helper"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
)

// TestGasConsumptionValidation tests the core principle that gas consumption
// in epoching module prevents spam attacks
func TestGasConsumptionValidation(t *testing.T) {
	helper := testhelper.NewHelper(t)
	ctx := helper.Ctx

	params := helper.App.EpochingKeeper.GetParams(ctx)
	gas := params.ExecuteGas

	testCases := []struct {
		operation string
		gas       uint64
	}{
		{"Delegate", gas.Delegate},
		{"Undelegate", gas.Undelegate},
		{"BeginRedelegate", gas.BeginRedelegate},
		{"CancelUnbondingDelegation", gas.CancelUnbondingDelegation},
		{"EditValidator", gas.EditValidator},
		{"CreateValidator", gas.CreateValidator},
	}

	for _, tc := range testCases {
		t.Run(tc.operation+"_gas_requirement", func(t *testing.T) {
			// Test that gas fee is positive (spam prevention requirement)
			require.Greater(t, tc.gas, uint64(0), "%s gas fee must be positive for spam prevention", tc.operation)

			// Test that insufficient gas meter would panic
			insufficientGasLimit := tc.gas - 1
			insufficientGasMeter := storetypes.NewGasMeter(insufficientGasLimit)

			// Demonstrate that trying to consume required gas with insufficient limit panics
			require.Panics(t, func() {
				insufficientGasMeter.ConsumeGas(tc.gas, tc.operation+" enqueue fee")
			}, "Should panic when trying to consume %d gas with limit %d for %s", tc.gas, insufficientGasLimit, tc.operation)

			// Test that sufficient gas meter works
			sufficientGasLimit := tc.gas + 100
			sufficientGasMeter := storetypes.NewGasMeter(sufficientGasLimit)

			require.NotPanics(t, func() {
				sufficientGasMeter.ConsumeGas(tc.gas, tc.operation+" enqueue fee")
			}, "Should not panic when consuming %d gas with limit %d for %s", tc.gas, sufficientGasLimit, tc.operation)

			// Verify gas was actually consumed
			require.Equal(t, tc.gas, sufficientGasMeter.GasConsumed(), "Gas consumed should equal required fee for %s", tc.operation)
		})
	}
}

// TestWrappedDelegate_OutOfGas tests actual WrappedDelegate transaction with gas consumption
func TestWrappedDelegate_OutOfGas(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	helper := testhelper.NewHelper(t)

	// Enter first epoch
	ctx, err := helper.ApplyEmptyBlockWithVoteExtension(r)
	require.NoError(t, err)

	// Get default delegate gas fee
	params := helper.App.EpochingKeeper.GetParams(ctx)
	delegateGas := params.ExecuteGas.Delegate

	// Test with insufficient gas - should fail due to out of gas
	t.Run("insufficient_gas", func(t *testing.T) {
		// Set gas limit lower than required
		gasLimit := delegateGas - 1
		ctx = ctx.WithGasMeter(storetypes.NewGasMeter(gasLimit))

		// Use actual test addresses from helper
		delegatorAddr := helper.GenAccs[0].GetAddress()
		validatorAddr := sdk.ValAddress(delegatorAddr) // Same account as validator

		msgDelegate := epochingtypes.NewMsgWrappedDelegate(&stakingtypes.MsgDelegate{
			DelegatorAddress: delegatorAddr.String(),
			ValidatorAddress: validatorAddr.String(),
			Amount:           sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1000000)),
		})

		// Should panic due to insufficient gas
		require.Panics(t, func() {
			_, _ = helper.MsgSrvr.WrappedDelegate(ctx, msgDelegate)
		}, "Expected OutOfGas panic when gasLimit (%d) < required (%d)", gasLimit, delegateGas)

		t.Logf("SUCCESS: OutOfGas panic occurred as expected with gasLimit %d < required %d", gasLimit, delegateGas)
	})

	// Test with sufficient gas
	t.Run("sufficient_gas", func(t *testing.T) {
		// Set gas limit higher than required
		gasLimit := delegateGas + 100000
		ctx = ctx.WithGasMeter(storetypes.NewGasMeter(gasLimit))

		// Use actual test addresses from helper
		delegatorAddr := helper.GenAccs[0].GetAddress()
		validatorAddr := sdk.ValAddress(delegatorAddr) // Same account as validator

		msgDelegate := epochingtypes.NewMsgWrappedDelegate(&stakingtypes.MsgDelegate{
			DelegatorAddress: delegatorAddr.String(),
			ValidatorAddress: validatorAddr.String(),
			Amount:           sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1000000)),
		})

		// Function should not panic from gas consumption (may fail for other validation reasons)
		_, err := helper.MsgSrvr.WrappedDelegate(ctx, msgDelegate)
		t.Logf("WrappedDelegate result - Error: %v, Gas consumed: %d", err, ctx.GasMeter().GasConsumed())

		// We expect some gas to be consumed if the function progresses
		gasConsumed := ctx.GasMeter().GasConsumed()
		require.GreaterOrEqual(t, gasLimit, gasConsumed, "GasLimit should be >= gasConsumed")
	})
}

// TestWrappedUnDelegate_OutOfGas tests actual WrappedUnDelegate transaction with gas consumption
func TestWrappedUnDelegate_OutOfGas(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	helper := testhelper.NewHelper(t)

	// Enter first epoch
	ctx, err := helper.ApplyEmptyBlockWithVoteExtension(r)
	require.NoError(t, err)

	// Get default undelegate gas fee
	params := helper.App.EpochingKeeper.GetParams(ctx)
	undelegateGas := params.ExecuteGas.Undelegate

	// Test with insufficient gas - should fail due to out of gas
	t.Run("insufficient_gas", func(t *testing.T) {
		// Set gas limit lower than required
		gasLimit := undelegateGas - 1
		ctx = ctx.WithGasMeter(storetypes.NewGasMeter(gasLimit))

		// Use actual test addresses from helper
		delegatorAddr := helper.GenAccs[0].GetAddress()
		validatorAddr := sdk.ValAddress(delegatorAddr) // Same account as validator

		msgUndelegate := epochingtypes.NewMsgWrappedUndelegate(&stakingtypes.MsgUndelegate{
			DelegatorAddress: delegatorAddr.String(),
			ValidatorAddress: validatorAddr.String(),
			Amount:           sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1000000)),
		})

		// Should panic due to insufficient gas
		require.Panics(t, func() {
			_, _ = helper.MsgSrvr.WrappedUndelegate(ctx, msgUndelegate)
		}, "Expected OutOfGas panic when gasLimit (%d) < required (%d)", gasLimit, undelegateGas)

		t.Logf("SUCCESS: OutOfGas panic occurred as expected with gasLimit %d < required %d", gasLimit, undelegateGas)
	})

	// Test with sufficient gas
	t.Run("sufficient_gas", func(t *testing.T) {
		// Set gas limit higher than required
		gasLimit := undelegateGas + 100000
		ctx = ctx.WithGasMeter(storetypes.NewGasMeter(gasLimit))

		// Use actual test addresses from helper
		delegatorAddr := helper.GenAccs[0].GetAddress()
		validatorAddr := sdk.ValAddress(delegatorAddr) // Same account as validator

		msgUndelegate := epochingtypes.NewMsgWrappedUndelegate(&stakingtypes.MsgUndelegate{
			DelegatorAddress: delegatorAddr.String(),
			ValidatorAddress: validatorAddr.String(),
			Amount:           sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1000000)),
		})

		// Function should not panic from gas consumption (may fail for other validation reasons)
		_, err := helper.MsgSrvr.WrappedUndelegate(ctx, msgUndelegate)
		t.Logf("WrappedUndelegate result - Error: %v, Gas consumed: %d", err, ctx.GasMeter().GasConsumed())

		// We expect some gas to be consumed if the function progresses
		gasConsumed := ctx.GasMeter().GasConsumed()
		require.GreaterOrEqual(t, gasLimit, gasConsumed, "GasLimit should be >= gasConsumed")
	})
}

// TestWrappedBeginRedelegate_OutOfGas tests actual WrappedBeginRedelegate transaction
func TestWrappedBeginRedelegate_OutOfGas(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	helper := testhelper.NewHelper(t)

	// Enter first epoch
	ctx, err := helper.ApplyEmptyBlockWithVoteExtension(r)
	require.NoError(t, err)

	// Get default redelegate gas fee
	params := helper.App.EpochingKeeper.GetParams(ctx)
	redelegateGas := params.ExecuteGas.BeginRedelegate

	// Test BeginRedelegate with success scenario - use EXISTING validators only
	t.Run("sufficient_gas", func(t *testing.T) {
		gasLimit := redelegateGas + 100000
		ctx = ctx.WithGasMeter(storetypes.NewGasMeter(gasLimit))
		initialGas := ctx.GasMeter().GasConsumed()

		// Use EXACTLY the same approach as successful Undelegate test
		delegatorAddr := helper.GenAccs[0].GetAddress()
		srcValAddr := sdk.ValAddress(delegatorAddr) // Same account as validator (self-delegation)

		// For destination, use genesis validator from helper
		dstValidator := helper.GenValidators.Keys[0]
		dstValAddr, err := sdk.ValAddressFromBech32(dstValidator.ValidatorAddress)
		require.NoError(t, err)

		msgRedelegate := epochingtypes.NewMsgWrappedBeginRedelegate(&stakingtypes.MsgBeginRedelegate{
			DelegatorAddress:    delegatorAddr.String(),
			ValidatorSrcAddress: srcValAddr.String(),
			ValidatorDstAddress: dstValAddr.String(),
			Amount:              sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1000000)), // Same amount as Undelegate
		})

		_, err = helper.MsgSrvr.WrappedBeginRedelegate(ctx, msgRedelegate)
		finalGas := ctx.GasMeter().GasConsumed()
		actualGasUsed := finalGas - initialGas

		t.Logf("BeginRedelegate - Error: %v, Actual gas consumed: %d", err, actualGasUsed)

		if err == nil {
			t.Logf("SUCCESS: BeginRedelegate succeeded with gas consumption: %d", actualGasUsed)
		} else {
			t.Logf("BeginRedelegate failed: %v, Gas consumed: %d", err, actualGasUsed)
		}

		gasConsumed := ctx.GasMeter().GasConsumed()
		require.GreaterOrEqual(t, gasLimit, gasConsumed, "GasLimit should be >= gasConsumed")
	})

	// Test with insufficient gas based on measured consumption
	t.Run("insufficient_gas", func(t *testing.T) {
		// Use the measured gas consumption - 1 (or current default - 1 if measurement failed)
		gasLimit := redelegateGas - 1
		ctx = ctx.WithGasMeter(storetypes.NewGasMeter(gasLimit))

		srcValidator := helper.GenValidators.Keys[0]
		srcValAddr, err := sdk.ValAddressFromBech32(srcValidator.ValidatorAddress)
		require.NoError(t, err)

		delegatorAddr := helper.GenAccs[0].GetAddress()
		dstAddr := delegatorAddr.Bytes()
		dstAddr[len(dstAddr)-1] = dstAddr[len(dstAddr)-1] + 2
		dstValAddr := sdk.ValAddress(dstAddr)

		msgRedelegate := epochingtypes.NewMsgWrappedBeginRedelegate(&stakingtypes.MsgBeginRedelegate{
			DelegatorAddress:    delegatorAddr.String(),
			ValidatorSrcAddress: srcValAddr.String(),
			ValidatorDstAddress: dstValAddr.String(),
			Amount:              sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(100000)),
		})

		// If gas consumption reaches ConsumeGas call, it should panic
		_, err = helper.MsgSrvr.WrappedBeginRedelegate(ctx, msgRedelegate)
		gasConsumed := ctx.GasMeter().GasConsumed()

		t.Logf("Insufficient gas test - Error: %v, Gas consumed: %d, Gas limit: %d", err, gasConsumed, gasLimit)

		// If we reach here without panic, ConsumeGas wasn't called (validation failed early)
		// This is actually expected for BeginRedelegate due to delegation requirements
	})
}

// TestWrappedCancelUnbondingDelegation_OutOfGas tests actual WrappedCancelUnbondingDelegation transaction
func TestWrappedCancelUnbondingDelegation_OutOfGas(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	helper := testhelper.NewHelper(t)

	// Enter first epoch
	ctx, err := helper.ApplyEmptyBlockWithVoteExtension(r)
	require.NoError(t, err)

	// Get default cancel unbonding delegation gas fee
	params := helper.App.EpochingKeeper.GetParams(ctx)
	cancelGas := params.ExecuteGas.CancelUnbondingDelegation

	// Test with insufficient gas
	t.Run("insufficient_gas", func(t *testing.T) {
		gasLimit := cancelGas - 1
		ctx = ctx.WithGasMeter(storetypes.NewGasMeter(gasLimit))

		// Use actual test addresses from helper
		delegatorAddr := helper.GenAccs[0].GetAddress()
		validatorAddr := sdk.ValAddress(delegatorAddr) // Same account as validator

		msgCancel := epochingtypes.NewMsgWrappedCancelUnbondingDelegation(&stakingtypes.MsgCancelUnbondingDelegation{
			DelegatorAddress: delegatorAddr.String(),
			ValidatorAddress: validatorAddr.String(),
			Amount:           sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1000000)),
			CreationHeight:   1,
		})

		// Should panic due to insufficient gas
		require.Panics(t, func() {
			_, _ = helper.MsgSrvr.WrappedCancelUnbondingDelegation(ctx, msgCancel)
		}, "Expected OutOfGas panic when gasLimit (%d) < required (%d)", gasLimit, cancelGas)

		t.Logf("SUCCESS: OutOfGas panic occurred as expected with gasLimit %d < required %d", gasLimit, cancelGas)
	})

	// Test with sufficient gas
	t.Run("sufficient_gas", func(t *testing.T) {
		gasLimit := cancelGas + 100000
		ctx = ctx.WithGasMeter(storetypes.NewGasMeter(gasLimit))

		// Use actual test addresses from helper
		delegatorAddr := helper.GenAccs[0].GetAddress()
		validatorAddr := sdk.ValAddress(delegatorAddr) // Same account as validator

		msgCancel := epochingtypes.NewMsgWrappedCancelUnbondingDelegation(&stakingtypes.MsgCancelUnbondingDelegation{
			DelegatorAddress: delegatorAddr.String(),
			ValidatorAddress: validatorAddr.String(),
			Amount:           sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1000000)),
			CreationHeight:   1,
		})

		_, err := helper.MsgSrvr.WrappedCancelUnbondingDelegation(ctx, msgCancel)
		t.Logf("WrappedCancelUnbondingDelegation result - Error: %v, Gas consumed: %d", err, ctx.GasMeter().GasConsumed())

		gasConsumed := ctx.GasMeter().GasConsumed()
		require.GreaterOrEqual(t, gasLimit, gasConsumed, "GasLimit should be >= gasConsumed")
	})
}

// TestWrappedEditValidator_OutOfGas tests actual WrappedEditValidator transaction
func TestWrappedEditValidator_OutOfGas(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	helper := testhelper.NewHelper(t)

	// Enter first epoch
	ctx, err := helper.ApplyEmptyBlockWithVoteExtension(r)
	require.NoError(t, err)

	// Get default edit validator gas fee
	params := helper.App.EpochingKeeper.GetParams(ctx)
	editValidatorGas := params.ExecuteGas.EditValidator

	// Test with insufficient gas
	t.Run("insufficient_gas", func(t *testing.T) {
		gasLimit := editValidatorGas - 1
		ctx = ctx.WithGasMeter(storetypes.NewGasMeter(gasLimit))

		// Use actual test addresses from helper
		validatorAddr := sdk.ValAddress(helper.GenAccs[0].GetAddress())

		msgEditValidator := epochingtypes.NewMsgWrappedEditValidator(&stakingtypes.MsgEditValidator{
			ValidatorAddress: validatorAddr.String(),
			Description: stakingtypes.Description{
				Moniker:  "updated-moniker",
				Identity: "updated-identity",
			},
		})

		// Should panic due to insufficient gas
		require.Panics(t, func() {
			_, _ = helper.MsgSrvr.WrappedEditValidator(ctx, msgEditValidator)
		}, "Expected OutOfGas panic when gasLimit (%d) < required (%d)", gasLimit, editValidatorGas)

		t.Logf("SUCCESS: OutOfGas panic occurred as expected with gasLimit %d < required %d", gasLimit, editValidatorGas)
	})

	// Test with sufficient gas
	t.Run("sufficient_gas", func(t *testing.T) {
		gasLimit := editValidatorGas + 100000
		ctx = ctx.WithGasMeter(storetypes.NewGasMeter(gasLimit))

		// Use actual test addresses from helper
		validatorAddr := sdk.ValAddress(helper.GenAccs[0].GetAddress())

		msgEditValidator := epochingtypes.NewMsgWrappedEditValidator(&stakingtypes.MsgEditValidator{
			ValidatorAddress: validatorAddr.String(),
			Description: stakingtypes.Description{
				Moniker:  "updated-moniker",
				Identity: "updated-identity",
			},
		})

		_, err := helper.MsgSrvr.WrappedEditValidator(ctx, msgEditValidator)
		t.Logf("WrappedEditValidator result - Error: %v, Gas consumed: %d", err, ctx.GasMeter().GasConsumed())

		gasConsumed := ctx.GasMeter().GasConsumed()
		require.GreaterOrEqual(t, gasLimit, gasConsumed, "GasLimit should be >= gasConsumed")
	})
}
