package keeper

import (
	"context"
	"testing"

	"cosmossdk.io/math"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	tmocks "github.com/babylonlabs-io/babylon/v4/testutil/mocks"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func TestHookEpochingAfterEpochEnds_ValidatorBecomesActive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)
	ctx = ctx.WithBlockHeight(100)

	// Setup: create a delegator and validator
	delAddr := datagen.GenRandomAddress()
	valAddr := datagen.GenRandomValidatorAddress()
	shares := math.LegacyNewDec(1000)

	delegation := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           shares,
	}

	val, err := tmocks.CreateValidator(valAddr, shares.RoundInt())
	require.NoError(t, err)

	mockStkK := k.stkK.(*types.MockStakingKeeper)

	hooks := k.HookEpoching()

	// Store an initial empty validator set to simulate previous epoch
	err = k.updateValidatorSet(ctx, []sdk.ValAddress{})
	require.NoError(t, err)

	// Now validator becomes active (new epoch) - IterateLastValidatorPowers is called to build new validator set
	mockStkK.EXPECT().IterateLastValidatorPowers(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(sdk.ValAddress, int64) bool) error {
			fn(valAddr, 1000) // Validator is now active
			return nil
		},
	).Times(1)

	// Mock getting validator for updateValidatorSet (to store validator set for next epoch)
	mockStkK.EXPECT().GetValidator(gomock.Any(), valAddr).Return(val, nil).AnyTimes()

	// Mock getting delegations for the newly active validator
	mockStkK.EXPECT().GetValidatorDelegations(gomock.Any(), valAddr).Return([]stakingtypes.Delegation{delegation}, nil).Times(1)

	// Call AfterEpochEnds - should add baby tokens for the newly active validator
	hooks.AfterEpochEnds(ctx, 1)

	// Verify the costaker tracker was created/updated with the delegation amount
	tracker, err := k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.Equal(t, shares.TruncateInt().String(), tracker.ActiveBaby.String())
}

func TestHookEpochingAfterEpochEnds_ValidatorBecomesInactive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)
	ctx = ctx.WithBlockHeight(100)

	delAddr := datagen.GenRandomAddress()
	valAddr := datagen.GenRandomValidatorAddress()
	shares := math.LegacyNewDec(1000)

	delegation := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           shares,
	}

	val, err := tmocks.CreateValidator(valAddr, shares.RoundInt())
	require.NoError(t, err)

	// Setup: create a costaker tracker with existing ActiveBaby
	err = k.setCostakerRewardsTracker(ctx, delAddr, types.NewCostakerRewardsTracker(0, math.ZeroInt(), shares.TruncateInt(), math.ZeroInt()))
	require.NoError(t, err)

	mockStkK := k.stkK.(*types.MockStakingKeeper)

	hooks := k.HookEpoching()

	// Store validator set with validator as active (simulating previous epoch state)
	mockStkK.EXPECT().GetValidator(gomock.Any(), valAddr).Return(val, nil).Times(1)
	err = k.updateValidatorSet(ctx, []sdk.ValAddress{valAddr})
	require.NoError(t, err)

	// Now validator becomes inactive (new epoch)
	mockStkK.EXPECT().IterateLastValidatorPowers(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(sdk.ValAddress, int64) bool) error {
			// Empty - validator is no longer active
			return nil
		},
	).Times(1)

	// Mock getting delegations for the newly inactive validator
	mockStkK.EXPECT().GetValidatorDelegations(gomock.Any(), valAddr).Return([]stakingtypes.Delegation{delegation}, nil).Times(1)

	// Call AfterEpochEnds - should remove baby tokens for the newly inactive validator
	mockStkK.EXPECT().GetValidator(gomock.Any(), valAddr).Return(val, nil).Times(1)
	hooks.AfterEpochEnds(ctx, 1)

	// Verify the costaker tracker was updated (ActiveBaby should be zero)
	tracker, err := k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.True(t, tracker.ActiveBaby.IsZero(), "ActiveBaby should be zero after validator becomes inactive", "active baby", tracker.ActiveBaby.String())
}

func TestHookEpochingAfterEpochEnds_MultipleValidatorsTransition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)
	ctx = ctx.WithBlockHeight(100)

	// Setup: 3 validators, 1 delegator
	delAddr := datagen.GenRandomAddress()
	val1Addr := datagen.GenRandomValidatorAddress() // Stays active
	val2Addr := datagen.GenRandomValidatorAddress() // Becomes inactive
	val3Addr := datagen.GenRandomValidatorAddress() // Becomes active

	shares1 := math.LegacyNewDec(1000)
	shares2 := math.LegacyNewDec(500)
	shares3 := math.LegacyNewDec(750)

	delegation2 := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: val2Addr.String(),
		Shares:           shares2,
	}
	delegation3 := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: val3Addr.String(),
		Shares:           shares3,
	}

	val1, err := tmocks.CreateValidator(val1Addr, shares2.RoundInt())
	require.NoError(t, err)
	val2, err := tmocks.CreateValidator(val2Addr, shares2.RoundInt())
	require.NoError(t, err)
	val3, err := tmocks.CreateValidator(val3Addr, shares3.RoundInt())
	require.NoError(t, err)
	require.NotNil(t, val3)
	require.NotNil(t, delegation2)
	require.NotNil(t, delegation3)

	// Setup: create a costaker tracker with ActiveBaby from val1 and val2
	initialActiveBaby := shares1.Add(shares2).TruncateInt()
	err = k.setCostakerRewardsTracker(ctx, delAddr, types.NewCostakerRewardsTracker(0, math.ZeroInt(), initialActiveBaby, math.ZeroInt()))
	require.NoError(t, err)

	mockStkK := k.stkK.(*types.MockStakingKeeper)

	hooks := k.HookEpoching()

	// Store validator set with val1 and val2 active (simulating previous epoch state)
	mockStkK.EXPECT().GetValidator(gomock.Any(), val1Addr).Return(val1, nil).Times(2) // initial + updateValidatorSet
	mockStkK.EXPECT().GetValidator(gomock.Any(), val2Addr).Return(val2, nil).Times(2) // initial + removeBabyForDelegators
	err = k.updateValidatorSet(ctx, []sdk.ValAddress{val1Addr, val2Addr})
	require.NoError(t, err)

	// New epoch: val1 and val3 are active (val2 became inactive, val3 became active)
	mockStkK.EXPECT().IterateLastValidatorPowers(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(sdk.ValAddress, int64) bool) error {
			fn(val1Addr, 1000)
			fn(val3Addr, 750)
			return nil
		},
	).Times(1)

	// Newly active validator val3
	mockStkK.EXPECT().GetValidatorDelegations(gomock.Any(), val3Addr).Return([]stakingtypes.Delegation{delegation3}, nil).Times(1)
	mockStkK.EXPECT().GetValidator(gomock.Any(), val3Addr).Return(val3, nil).Times(2) // addBabyForDelegators + updateValidatorSet

	// Newly inactive validator val2
	mockStkK.EXPECT().GetValidatorDelegations(gomock.Any(), val2Addr).Return([]stakingtypes.Delegation{delegation2}, nil).Times(1)

	// Call AfterEpochEnds
	hooks.AfterEpochEnds(ctx, 1)

	// Verify the costaker tracker:
	// Should have: val1 (1000) + val3 (750) = 1750
	// Lost: val2 (500)
	tracker, err := k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	expectedActiveBaby := shares1.Add(shares3).TruncateInt() // 1000 + 750 = 1750
	require.Equal(t, expectedActiveBaby.String(), tracker.ActiveBaby.String())
}

func TestHookEpochingAfterEpochEnds_NoValidatorChanges(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)
	ctx = ctx.WithBlockHeight(100)

	delAddr := datagen.GenRandomAddress()
	valAddr := datagen.GenRandomValidatorAddress()
	shares := math.LegacyNewDec(1000)

	// Setup: create a costaker tracker with existing ActiveBaby
	err := k.setCostakerRewardsTracker(ctx, delAddr, types.NewCostakerRewardsTracker(0, math.ZeroInt(), shares.TruncateInt(), math.ZeroInt()))
	require.NoError(t, err)

	mockStkK := k.stkK.(*types.MockStakingKeeper)

	val, err := tmocks.CreateValidator(valAddr, shares.RoundInt())
	require.NoError(t, err)

	// Mock GetValidator for the setup phase (when storing initial validator set)
	mockStkK.EXPECT().GetValidator(gomock.Any(), gomock.Any()).Return(val, nil).AnyTimes()

	hooks := k.HookEpoching()

	// Store validator set with validator as active (simulating previous epoch state)
	err = k.updateValidatorSet(ctx, []sdk.ValAddress{valAddr})
	require.NoError(t, err)

	// Mock GetValidator for buildCurrEpochValSetMap (reads previous epoch's validator set)
	mockStkK.EXPECT().GetValidator(gomock.Any(), gomock.Any()).Return(val, nil).AnyTimes()

	// New epoch: same validator is still active (no change)
	mockStkK.EXPECT().IterateLastValidatorPowers(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(sdk.ValAddress, int64) bool) error {
			fn(valAddr, 1000)
			return nil
		},
	).Times(1)

	// Mock getting validator for updateValidatorSet
	mockStkK.EXPECT().GetValidator(gomock.Any(), gomock.Any()).Return(val, nil).AnyTimes()

	// Call AfterEpochEnds - should not modify anything since no validator transitions
	hooks.AfterEpochEnds(ctx, 1)

	// Verify the costaker tracker is unchanged
	tracker, err := k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.Equal(t, shares.TruncateInt().String(), tracker.ActiveBaby.String())
}

func TestHookEpochingAfterEpochEnds_MultipleDelegators(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)
	ctx = ctx.WithBlockHeight(100)

	// Setup: 1 validator, 3 delegators
	del1Addr := datagen.GenRandomAddress()
	del2Addr := datagen.GenRandomAddress()
	del3Addr := datagen.GenRandomAddress()
	valAddr := datagen.GenRandomValidatorAddress()

	shares1 := math.LegacyNewDec(1000)
	shares2 := math.LegacyNewDec(500)
	shares3 := math.LegacyNewDec(750)

	delegations := []stakingtypes.Delegation{
		{
			DelegatorAddress: del1Addr.String(),
			ValidatorAddress: valAddr.String(),
			Shares:           shares1,
		},
		{
			DelegatorAddress: del2Addr.String(),
			ValidatorAddress: valAddr.String(),
			Shares:           shares2,
		},
		{
			DelegatorAddress: del3Addr.String(),
			ValidatorAddress: valAddr.String(),
			Shares:           shares3,
		},
	}

	val, err := tmocks.CreateValidator(valAddr, math.NewInt(2250))
	require.NoError(t, err)

	mockStkK := k.stkK.(*types.MockStakingKeeper)

	hooks := k.HookEpoching()

	// Store an initial empty validator set
	err = k.updateValidatorSet(ctx, []sdk.ValAddress{})
	require.NoError(t, err)

	// New epoch: validator becomes active
	mockStkK.EXPECT().IterateLastValidatorPowers(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(sdk.ValAddress, int64) bool) error {
			fn(valAddr, 2250)
			return nil
		},
	).Times(1)

	// Mock getting validator for every call
	mockStkK.EXPECT().GetValidator(gomock.Any(), valAddr).Return(val, nil).AnyTimes()

	// Mock getting all delegations for the validator
	mockStkK.EXPECT().GetValidatorDelegations(gomock.Any(), valAddr).Return(delegations, nil).Times(1)

	// Call AfterEpochEnds - should add baby tokens for all delegators
	hooks.AfterEpochEnds(ctx, 1)

	// Verify all delegators have their trackers updated
	tracker1, err := k.GetCostakerRewards(ctx, del1Addr)
	require.NoError(t, err)
	require.Equal(t, shares1.TruncateInt().String(), tracker1.ActiveBaby.String())

	tracker2, err := k.GetCostakerRewards(ctx, del2Addr)
	require.NoError(t, err)
	require.Equal(t, shares2.TruncateInt().String(), tracker2.ActiveBaby.String())

	tracker3, err := k.GetCostakerRewards(ctx, del3Addr)
	require.NoError(t, err)
	require.Equal(t, shares3.TruncateInt().String(), tracker3.ActiveBaby.String())
}
