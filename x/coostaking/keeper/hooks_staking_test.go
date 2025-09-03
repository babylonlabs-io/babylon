package keeper

import (
	"crypto/rand"
	"testing"

	"cosmossdk.io/math"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func genRandomAddress() sdk.AccAddress {
	addr := make([]byte, 20)
	rand.Read(addr)
	return sdk.AccAddress(addr)
}

func genRandomValidatorAddress() sdk.ValAddress {
	addr := make([]byte, 20)
	rand.Read(addr)
	return sdk.ValAddress(addr)
}

func TestCalculateDelegationDelta(t *testing.T) {
	tests := []struct {
		name          string
		beforeAmount  math.LegacyDec
		afterAmount   math.LegacyDec
		expectedDelta math.Int
	}{
		{
			name:          "positive delta - increase in delegation",
			beforeAmount:  math.LegacyNewDec(100),
			afterAmount:   math.LegacyNewDec(150),
			expectedDelta: math.NewInt(50),
		},
		{
			name:          "negative delta - decrease in delegation",
			beforeAmount:  math.LegacyNewDec(200),
			afterAmount:   math.LegacyNewDec(150),
			expectedDelta: math.NewInt(-50),
		},
		{
			name:          "zero delta - no change",
			beforeAmount:  math.LegacyNewDec(100),
			afterAmount:   math.LegacyNewDec(100),
			expectedDelta: math.NewInt(0),
		},
		{
			name:          "from zero - new delegation",
			beforeAmount:  math.LegacyZeroDec(),
			afterAmount:   math.LegacyNewDec(100),
			expectedDelta: math.NewInt(100),
		},
		{
			name:          "to zero - full undelegation",
			beforeAmount:  math.LegacyNewDec(100),
			afterAmount:   math.LegacyZeroDec(),
			expectedDelta: math.NewInt(-100),
		},
		{
			name:          "decimal values - truncated result",
			beforeAmount:  math.LegacyNewDecWithPrec(1005, 1), // 100.5
			afterAmount:   math.LegacyNewDecWithPrec(1505, 1), // 150.5
			expectedDelta: math.NewInt(50),                    // truncated from 50.0
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := calculateDelegationDelta(tc.beforeAmount, tc.afterAmount)
			require.Equal(t, tc.expectedDelta.String(), result.String())
		})
	}
}

func TestHookStaking_BeforeDelegationSharesModified(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

	delAddr := genRandomAddress()
	valAddr := genRandomValidatorAddress()
	shares := math.LegacyNewDec(1000)

	// Mock delegation
	delegation := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           shares,
	}

	// Mock staking keeper to return the delegation
	mockStkK := k.stkK.(*types.MockStakingKeeper)
	mockStkK.EXPECT().GetDelegation(ctx, delAddr, valAddr).Return(delegation, nil).Times(1)

	hooks := k.HookStaking()

	// Call BeforeDelegationSharesModified
	err := hooks.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Verify the amount was cached by retrieving and deleting it
	cachedAmount := k.stkCache.GetAndDeleteStakedAmount(delAddr, valAddr)
	require.True(t, shares.Equal(cachedAmount))
}

func TestHookStaking_BeforeDelegationSharesModified_DelegationNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

	delAddr := genRandomAddress()
	valAddr := genRandomValidatorAddress()

	// Mock staking keeper to return error (delegation not found)
	mockStkK := k.stkK.(*types.MockStakingKeeper)
	mockStkK.EXPECT().GetDelegation(ctx, delAddr, valAddr).Return(stakingtypes.Delegation{}, stakingtypes.ErrNoDelegation).Times(1)

	hooks := k.HookStaking()

	// Call BeforeDelegationSharesModified - should not return error
	err := hooks.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Verify nothing was cached (returns zero)
	cachedAmount := k.stkCache.GetAndDeleteStakedAmount(delAddr, valAddr)
	require.True(t, math.LegacyZeroDec().Equal(cachedAmount))
}

func TestHookStaking_AfterDelegationModified(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

	delAddr := genRandomAddress()
	valAddr := genRandomValidatorAddress()

	// Set up initial cached amount
	beforeShares := math.LegacyNewDec(1000)
	k.stkCache.SetStakedAmount(delAddr, valAddr, beforeShares)

	// Current delegation after modification
	afterShares := math.LegacyNewDec(1500)
	delegation := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           afterShares,
	}

	// Mock staking keeper to return the delegation
	mockStkK := k.stkK.(*types.MockStakingKeeper)
	mockStkK.EXPECT().GetDelegation(ctx, delAddr, valAddr).Return(delegation, nil).Times(1)

	// Mock coostakerModified call - expect delta of +500
	expectedDelta := math.NewInt(500)
	mockIctvK.EXPECT().AccumulateRewardGaugeForCoostaker(gomock.Any(), gomock.Eq(delAddr), gomock.Any()).Times(0) // not called in hook

	// Set default params
	err := k.SetParams(ctx, types.DefaultParams())
	require.NoError(t, err)

	// Initialize rewards system to avoid errors in coostakerModified
	_, err = k.GetCurrentRewardsInitialized(ctx)
	require.NoError(t, err)

	hooks := k.HookStaking()

	// Call AfterDelegationModified
	err = hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Verify the coostaker tracker was updated with the delta
	tracker, err := k.GetCoostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.Equal(t, expectedDelta.String(), tracker.ActiveBaby.String())
}

func TestHookStaking_AfterDelegationModified_DelegationNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

	delAddr := genRandomAddress()
	valAddr := genRandomValidatorAddress()

	// Mock staking keeper to return error (delegation not found)
	mockStkK := k.stkK.(*types.MockStakingKeeper)
	mockStkK.EXPECT().GetDelegation(ctx, delAddr, valAddr).Return(stakingtypes.Delegation{}, stakingtypes.ErrNoDelegation).Times(1)

	hooks := k.HookStaking()

	// Call AfterDelegationModified - should return error since delegation must be found
	err := hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	require.Error(t, err)
	require.Equal(t, stakingtypes.ErrNoDelegation, err)
}

func TestHookStaking_AfterDelegationModified_WithNegativeDelta(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

	delAddr := genRandomAddress()
	valAddr := genRandomValidatorAddress()

	// Set up initial cached amount (higher than current)
	beforeShares := math.LegacyNewDec(2000)
	k.stkCache.SetStakedAmount(delAddr, valAddr, beforeShares)

	// Current delegation after modification (lower than before)
	afterShares := math.LegacyNewDec(1500)
	delegation := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           afterShares,
	}

	// Mock staking keeper to return the delegation
	mockStkK := k.stkK.(*types.MockStakingKeeper)
	mockStkK.EXPECT().GetDelegation(ctx, delAddr, valAddr).Return(delegation, nil).Times(1)

	// Set default params
	err := k.SetParams(ctx, types.DefaultParams())
	require.NoError(t, err)

	// Initialize rewards system
	_, err = k.GetCurrentRewardsInitialized(ctx)
	require.NoError(t, err)

	hooks := k.HookStaking()

	// Call AfterDelegationModified
	err = hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Verify the coostaker tracker was updated with the negative delta
	tracker, err := k.GetCoostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	expectedDelta := math.NewInt(-500)
	require.Equal(t, expectedDelta.String(), tracker.ActiveBaby.String())
}

func TestHookStaking_AfterDelegationModified_NoCachedAmount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

	delAddr := genRandomAddress()
	valAddr := genRandomValidatorAddress()

	// No cached amount (should default to zero)

	// Current delegation after modification
	afterShares := math.LegacyNewDec(1000)
	delegation := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           afterShares,
	}

	// Mock staking keeper to return the delegation
	mockStkK := k.stkK.(*types.MockStakingKeeper)
	mockStkK.EXPECT().GetDelegation(ctx, delAddr, valAddr).Return(delegation, nil).Times(1)

	// Set default params
	err := k.SetParams(ctx, types.DefaultParams())
	require.NoError(t, err)

	// Initialize rewards system
	_, err = k.GetCurrentRewardsInitialized(ctx)
	require.NoError(t, err)

	hooks := k.HookStaking()

	// Call AfterDelegationModified
	err = hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Verify the coostaker tracker was updated (delta should be +1000 since before was 0)
	tracker, err := k.GetCoostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	expectedDelta := math.NewInt(1000)
	require.Equal(t, expectedDelta.String(), tracker.ActiveBaby.String())
}

func TestHookStaking_FullDelegationFlow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

	delAddr := genRandomAddress()
	valAddr := genRandomValidatorAddress()
	hooks := k.HookStaking()

	// Set default params
	err := k.SetParams(ctx, types.DefaultParams())
	require.NoError(t, err)

	// Initialize rewards system
	_, err = k.GetCurrentRewardsInitialized(ctx)
	require.NoError(t, err)

	// Simulate initial delegation (1000 shares)
	initialShares := math.LegacyNewDec(1000)
	initialDelegation := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           initialShares,
	}

	// Mock the BeforeDelegationSharesModified call (no cached amount initially)
	mockStkK := k.stkK.(*types.MockStakingKeeper)
	mockStkK.EXPECT().GetDelegation(ctx, delAddr, valAddr).Return(initialDelegation, nil).Times(1)

	err = hooks.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Mock the AfterDelegationModified call with increased shares
	newShares := math.LegacyNewDec(1500)
	newDelegation := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           newShares,
	}
	mockStkK.EXPECT().GetDelegation(ctx, delAddr, valAddr).Return(newDelegation, nil).Times(1)

	err = hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Verify the delta was calculated correctly (+500)
	tracker, err := k.GetCoostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	expectedDelta := math.NewInt(500)
	require.Equal(t, expectedDelta.String(), tracker.ActiveBaby.String())
}
