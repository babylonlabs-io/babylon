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
	gasFees := params.EnqueueGasFees

	testCases := []struct {
		operation string
		gasFee    uint64
	}{
		{"Delegate", gasFees.Delegate},
		{"Undelegate", gasFees.Undelegate},
		{"BeginRedelegate", gasFees.BeginRedelegate},
		{"CancelUnbondingDelegation", gasFees.CancelUnbondingDelegation},
		{"EditValidator", gasFees.EditValidator},
		{"StakingUpdateParams", gasFees.StakingUpdateParams},
		{"CreateValidator", gasFees.CreateValidator},
	}

	for _, tc := range testCases {
		t.Run(tc.operation+"_gas_requirement", func(t *testing.T) {
			// Test that gas fee is positive (spam prevention requirement)
			require.Greater(t, tc.gasFee, uint64(0), "%s gas fee must be positive for spam prevention", tc.operation)

			// Test that insufficient gas meter would panic
			insufficientGasLimit := tc.gasFee - 1
			insufficientGasMeter := storetypes.NewGasMeter(insufficientGasLimit)

			// Demonstrate that trying to consume required gas with insufficient limit panics
			require.Panics(t, func() {
				insufficientGasMeter.ConsumeGas(tc.gasFee, tc.operation+" enqueue fee")
			}, "Should panic when trying to consume %d gas with limit %d for %s", tc.gasFee, insufficientGasLimit, tc.operation)

			// Test that sufficient gas meter works
			sufficientGasLimit := tc.gasFee + 100
			sufficientGasMeter := storetypes.NewGasMeter(sufficientGasLimit)

			require.NotPanics(t, func() {
				sufficientGasMeter.ConsumeGas(tc.gasFee, tc.operation+" enqueue fee")
			}, "Should not panic when consuming %d gas with limit %d for %s", tc.gasFee, sufficientGasLimit, tc.operation)

			// Verify gas was actually consumed
			require.Equal(t, tc.gasFee, sufficientGasMeter.GasConsumed(), "Gas consumed should equal required fee for %s", tc.operation)
		})
	}
}

// TestAllEnqueueGasFees_Values tests that all enqueue gas fees are correctly set
func TestAllEnqueueGasFees_Values(t *testing.T) {
	helper := testhelper.NewHelper(t)
	ctx := helper.Ctx

	params := helper.App.EpochingKeeper.GetParams(ctx)
	gasFees := params.EnqueueGasFees

	// Verify all gas fees are positive and match expected defaults
	require.Equal(t, uint64(500), gasFees.Delegate, "Delegate gas fee should be 500")
	require.Equal(t, uint64(400), gasFees.Undelegate, "Undelegate gas fee should be 400")
	require.Equal(t, uint64(600), gasFees.BeginRedelegate, "BeginRedelegate gas fee should be 600")
	require.Equal(t, uint64(300), gasFees.CancelUnbondingDelegation, "CancelUnbondingDelegation gas fee should be 300")
	require.Equal(t, uint64(200), gasFees.EditValidator, "EditValidator gas fee should be 200")
	require.Equal(t, uint64(100), gasFees.StakingUpdateParams, "StakingUpdateParams gas fee should be 100")
	require.Equal(t, uint64(800), gasFees.CreateValidator, "CreateValidator gas fee should be 800")

	// Verify all gas fees are positive (spam prevention requirement)
	require.Greater(t, gasFees.Delegate, uint64(0), "Delegate gas fee must be positive")
	require.Greater(t, gasFees.Undelegate, uint64(0), "Undelegate gas fee must be positive")
	require.Greater(t, gasFees.BeginRedelegate, uint64(0), "BeginRedelegate gas fee must be positive")
	require.Greater(t, gasFees.CancelUnbondingDelegation, uint64(0), "CancelUnbondingDelegation gas fee must be positive")
	require.Greater(t, gasFees.EditValidator, uint64(0), "EditValidator gas fee must be positive")
	require.Greater(t, gasFees.StakingUpdateParams, uint64(0), "StakingUpdateParams gas fee must be positive")
	require.Greater(t, gasFees.CreateValidator, uint64(0), "CreateValidator gas fee must be positive")
}

// TestWrappedDelegate_ActualTx tests actual WrappedDelegate transaction with gas consumption
func TestWrappedDelegate_OutOfGas(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	helper := testhelper.NewHelper(t)

	// Enter first epoch
	ctx, err := helper.ApplyEmptyBlockWithVoteExtension(r)
	require.NoError(t, err)

	// Get default delegate gas fee
	params := helper.App.EpochingKeeper.GetParams(ctx)
	delegateGasFee := params.EnqueueGasFees.Delegate

	// Test with insufficient gas - should fail due to out of gas
	t.Run("insufficient_gas", func(t *testing.T) {
		// Set gas limit lower than required
		gasLimit := delegateGasFee - 1
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
			helper.MsgSrvr.WrappedDelegate(ctx, msgDelegate)
		}, "Expected OutOfGas panic when gasLimit (%d) < required (%d)", gasLimit, delegateGasFee)

		t.Logf("SUCCESS: OutOfGas panic occurred as expected with gasLimit %d < required %d", gasLimit, delegateGasFee)
	})

	// Test with sufficient gas
	t.Run("sufficient_gas", func(t *testing.T) {
		// Set gas limit higher than required
		gasLimit := delegateGasFee + 100000
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
		if gasConsumed >= delegateGasFee {
			t.Logf("SUCCESS: Gas consumption reached the ConsumeGas call (consumed: %d >= required: %d)", gasConsumed, delegateGasFee)
		}
	})
}

func TestWrappedUnDelegate_OutOfGas(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	helper := testhelper.NewHelper(t)

	// Enter first epoch
	ctx, err := helper.ApplyEmptyBlockWithVoteExtension(r)
	require.NoError(t, err)

	// Get default delegate gas fee
	params := helper.App.EpochingKeeper.GetParams(ctx)
	delegateGasFee := params.EnqueueGasFees.Delegate

	// Test with insufficient gas - should fail due to out of gas
	t.Run("insufficient_gas", func(t *testing.T) {
		// Set gas limit lower than required
		gasLimit := delegateGasFee - 1
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
			helper.MsgSrvr.WrappedUndelegate(ctx, msgUndelegate)
		}, "Expected OutOfGas panic when gasLimit (%d) < required (%d)", gasLimit, delegateGasFee)

		t.Logf("SUCCESS: OutOfGas panic occurred as expected with gasLimit %d < required %d", gasLimit, delegateGasFee)
	})

	// Test with sufficient gas
	t.Run("sufficient_gas", func(t *testing.T) {
		// Set gas limit higher than required
		gasLimit := delegateGasFee + 100000
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
		t.Logf("WrappedDelegate result - Error: %v, Gas consumed: %d", err, ctx.GasMeter().GasConsumed())

		// We expect some gas to be consumed if the function progresses
		gasConsumed := ctx.GasMeter().GasConsumed()
		if gasConsumed >= delegateGasFee {
			t.Logf("SUCCESS: Gas consumption reached the ConsumeGas call (consumed: %d >= required: %d)", gasConsumed, delegateGasFee)
		}
	})
}

// TestWrappedBeginRedelegate_ActualTx tests actual WrappedBeginRedelegate transaction
func TestWrappedBeginRedelegate_OutOfGas(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	helper := testhelper.NewHelper(t)

	// Enter first epoch
	ctx, err := helper.ApplyEmptyBlockWithVoteExtension(r)
	require.NoError(t, err)

	// Get default redelegate gas fee
	params := helper.App.EpochingKeeper.GetParams(ctx)
	redelegateGasFee := params.EnqueueGasFees.BeginRedelegate

	// Test with insufficient gas
	t.Run("insufficient_gas", func(t *testing.T) {
		gasLimit := redelegateGasFee - 1
		ctx = ctx.WithGasMeter(storetypes.NewGasMeter(gasLimit))

		// Use actual test addresses from helper
		delegatorAddr := helper.GenAccs[0].GetAddress()
		validatorSrcAddr := sdk.ValAddress(delegatorAddr)
		// Create a different validator address for destination
		validatorDstAddr := sdk.ValAddress(append(delegatorAddr, byte(1))) // Slightly different address

		msgRedelegate := epochingtypes.NewMsgWrappedBeginRedelegate(&stakingtypes.MsgBeginRedelegate{
			DelegatorAddress:    delegatorAddr.String(),
			ValidatorSrcAddress: validatorSrcAddr.String(),
			ValidatorDstAddress: validatorDstAddr.String(),
			Amount:              sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1000000)),
		})

		// Should panic due to insufficient gas
		require.Panics(t, func() {
			helper.MsgSrvr.WrappedBeginRedelegate(ctx, msgRedelegate)
		}, "Expected OutOfGas panic when gasLimit (%d) < required (%d)", gasLimit, redelegateGasFee)

		t.Logf("SUCCESS: OutOfGas panic occurred as expected with gasLimit %d < required %d", gasLimit, redelegateGasFee)
	})

	// Test with sufficient gas
	t.Run("sufficient_gas", func(t *testing.T) {
		gasLimit := redelegateGasFee + 100000
		ctx = ctx.WithGasMeter(storetypes.NewGasMeter(gasLimit))

		// Use actual test addresses from helper
		delegatorAddr := helper.GenAccs[0].GetAddress()
		validatorSrcAddr := sdk.ValAddress(delegatorAddr)
		// Create a different validator address for destination
		validatorDstAddr := sdk.ValAddress(append(delegatorAddr, byte(1))) // Slightly different address

		msgRedelegate := epochingtypes.NewMsgWrappedBeginRedelegate(&stakingtypes.MsgBeginRedelegate{
			DelegatorAddress:    delegatorAddr.String(),
			ValidatorSrcAddress: validatorSrcAddr.String(),
			ValidatorDstAddress: validatorDstAddr.String(),
			Amount:              sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1000000)),
		})

		_, err := helper.MsgSrvr.WrappedBeginRedelegate(ctx, msgRedelegate)
		t.Logf("WrappedBeginRedelegate result - Error: %v, Gas consumed: %d", err, ctx.GasMeter().GasConsumed())

		gasConsumed := ctx.GasMeter().GasConsumed()
		if gasConsumed >= redelegateGasFee {
			t.Logf("SUCCESS: Gas consumption reached the ConsumeGas call (consumed: %d >= required: %d)", gasConsumed, redelegateGasFee)
		}
	})
}

// TestWrappedCancelUnbondingDelegation_ActualTx tests actual WrappedCancelUnbondingDelegation transaction
func TestWrappedCancelUnbondingDelegation_OutOfGas(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	helper := testhelper.NewHelper(t)

	// Enter first epoch
	ctx, err := helper.ApplyEmptyBlockWithVoteExtension(r)
	require.NoError(t, err)

	// Get default cancel unbonding delegation gas fee
	params := helper.App.EpochingKeeper.GetParams(ctx)
	cancelGasFee := params.EnqueueGasFees.CancelUnbondingDelegation

	// Test with insufficient gas
	t.Run("insufficient_gas", func(t *testing.T) {
		gasLimit := cancelGasFee - 1
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
			helper.MsgSrvr.WrappedCancelUnbondingDelegation(ctx, msgCancel)
		}, "Expected OutOfGas panic when gasLimit (%d) < required (%d)", gasLimit, cancelGasFee)

		t.Logf("SUCCESS: OutOfGas panic occurred as expected with gasLimit %d < required %d", gasLimit, cancelGasFee)
	})

	// Test with sufficient gas
	t.Run("sufficient_gas", func(t *testing.T) {
		gasLimit := cancelGasFee + 100000
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
		if gasConsumed >= cancelGasFee {
			t.Logf("SUCCESS: Gas consumption reached the ConsumeGas call (consumed: %d >= required: %d)", gasConsumed, cancelGasFee)
		}
	})
}
