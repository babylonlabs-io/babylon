package keeper

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	tmocks "github.com/babylonlabs-io/babylon/v4/testutil/mocks"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func TestHookStakingBeforeDelegationSharesModifiedUpdateCache(t *testing.T) {
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, nil)

	delAddr, valAddr := datagen.GenRandomAddress(), datagen.GenRandomValidatorAddress()
	shares := math.LegacyNewDec(1000)

	delegation := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           shares,
	}

	val, err := tmocks.CreateValidator(valAddr, shares.RoundInt())
	require.NoError(t, err)

	mockStkK := k.stkK.(*types.MockStakingKeeper)

	mockStkK.EXPECT().GetDelegation(ctx, delAddr, valAddr).Return(delegation, nil).Times(1)
	mockStkK.EXPECT().GetValidator(ctx, valAddr).Return(val, nil).Times(1)

	hooks := k.HookStaking()

	err = hooks.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Verify the amount was cached by retrieving
	info := k.stkCache.GetStakedInfo(delAddr, valAddr)
	require.True(t, shares.Equal(info.Amount))
	require.True(t, shares.Equal(info.Shares))
	// get again and make sure it is not deleted
	info = k.stkCache.GetStakedInfo(delAddr, valAddr)
	require.True(t, shares.Equal(info.Amount))
	require.True(t, shares.Equal(info.Shares))

	mockStkK.EXPECT().GetDelegation(ctx, delAddr, valAddr).Return(stakingtypes.Delegation{}, stakingtypes.ErrNoDelegation).Times(1)
	// Call BeforeDelegationSharesModified - should not return error even though the get del returned err
	err = hooks.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	info = k.stkCache.GetStakedInfo(delAddr, valAddr)
	require.True(t, shares.Equal(info.Amount))
	require.True(t, shares.Equal(info.Shares))
}

func TestHookStakingAfterDelegationModified(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)
	p := k.GetParams(ctx)
	p.ScoreRatioBtcByBaby = math.NewInt(50)
	err := k.SetParams(ctx, p)
	require.NoError(t, err)

	delAddr, valAddr := datagen.GenRandomAddress(), datagen.GenRandomValidatorAddress()

	// simulates as if the user had staked 100 baby at genesis
	expShares := math.LegacyNewDec(100)
	delegation := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           expShares,
	}

	val, err := tmocks.CreateValidator(valAddr, math.NewInt(100))
	require.NoError(t, err)

	mockStkK := k.stkK.(*types.MockStakingKeeper)

	mockStkK.EXPECT().GetDelegation(ctx, delAddr, valAddr).Return(delegation, nil).Times(1)
	mockStkK.EXPECT().GetValidator(gomock.Any(), valAddr).Return(val, nil).Times(2)
	// Store an initial validator set
	err = k.updateValidatorSet(ctx, []sdk.ValAddress{valAddr})
	require.NoError(t, err)

	hooks := k.HookStaking()

	// Call AfterDelegationModified only
	err = hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Verify the costaker tracker was updated with the delta
	tracker, err := k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.Equal(t, expShares.TruncateInt().String(), tracker.ActiveBaby.String())

	// simulates that the user staked a bit of BTC
	err = k.costakerModified(ctx, delAddr, func(rwdTracker *types.CostakerRewardsTracker) {
		rwdTracker.ActiveSatoshis = math.NewInt(1000)
	})
	require.NoError(t, err)

	// add a few rewards
	expRwd := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, math.NewInt(1000000000000)))
	err = k.AddRewardsForCostakers(ctx, expRwd)
	require.NoError(t, err)

	mockStkK.EXPECT().GetDelegation(ctx, delAddr, valAddr).Return(delegation, nil).Times(1)
	mockStkK.EXPECT().GetValidator(gomock.Any(), gomock.Eq(valAddr)).Return(val, nil).Times(1)
	err = hooks.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	delegationAfter := delegation
	delegationAfter.Shares = delegation.Shares.MulInt64(2)

	mockBankK := k.bankK.(*types.MockBankKeeper)

	mockStkK.EXPECT().GetDelegation(ctx, delAddr, valAddr).Return(delegationAfter, nil).Times(1)
	mockBankK.EXPECT().SendCoinsFromModuleToModule(ctx, types.ModuleName, ictvtypes.ModuleName, expRwd).Return(nil).Times(1)
	mockIctvK.EXPECT().AccumulateRewardGaugeForCostaker(gomock.Any(), gomock.Eq(delAddr), expRwd).Times(1)

	mockStkK.EXPECT().GetValidator(gomock.Any(), gomock.Eq(valAddr)).Return(val, nil).Times(1)
	err = hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	require.NoError(t, err)
}

func TestHookStakingAfterDelegationModifiedErrorDelegationNotFound(t *testing.T) {
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, nil)
	delAddr, valAddr := datagen.GenRandomAddress(), datagen.GenRandomValidatorAddress()

	mockStkK := k.stkK.(*types.MockStakingKeeper)

	mockStkK.EXPECT().GetDelegation(ctx, delAddr, valAddr).Return(stakingtypes.Delegation{}, stakingtypes.ErrNoDelegation).Times(1)

	hooks := k.HookStaking()
	err := hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	require.EqualError(t, err, stakingtypes.ErrNoDelegation.Error())
}

func TestHookStakingAfterDelegationModifiedReducingAmountStaked(t *testing.T) {
	mockIctvK := types.NewMockIncentiveKeeper(gomock.NewController(t))
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

	delAddr, valAddr := datagen.GenRandomAddress(), datagen.GenRandomValidatorAddress()

	initShares := math.NewInt(2000)
	err := k.setCostakerRewardsTracker(ctx, delAddr, types.NewCostakerRewardsTracker(0, math.ZeroInt(), initShares, math.ZeroInt()))
	require.NoError(t, err)

	k.stkCache.SetStakedInfo(delAddr, valAddr, initShares.ToLegacyDec(), initShares.ToLegacyDec())

	// reduces it by 500
	afterShares := math.LegacyNewDec(1500)
	delegation := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           afterShares,
	}

	mockStkK := k.stkK.(*types.MockStakingKeeper)

	mockStkK.EXPECT().GetDelegation(ctx, delAddr, valAddr).Return(delegation, nil).Times(1)

	val, err := tmocks.CreateValidator(valAddr, math.NewInt(100))
	require.NoError(t, err)
	mockStkK.EXPECT().GetValidator(gomock.Any(), gomock.Any()).Return(val, nil).Times(2)
	// Store an initial validator set
	err = k.updateValidatorSet(ctx, []sdk.ValAddress{valAddr})
	require.NoError(t, err)

	hooks := k.HookStaking()

	err = hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Verify the costaker tracker was updated with the negative delta
	tracker, err := k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.Equal(t, afterShares.TruncateInt().String(), tracker.ActiveBaby.String())
}

func TestHookStakingAfterDelegationModified_InactiveValidator(t *testing.T) {
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, nil)
	// Set block height > 0 to avoid genesis special case
	ctx = ctx.WithBlockHeight(100)

	delAddr, valAddr := datagen.GenRandomAddress(), datagen.GenRandomValidatorAddress()

	hooks := k.HookStaking()
	// Store an initial validator set
	err := k.updateValidatorSet(ctx, []sdk.ValAddress{})
	require.NoError(t, err)

	// Call AfterDelegationModified - should skip processing because validator is not active
	err = hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Verify no costaker tracker was created (validator was not active)
	_, err = k.GetCostakerRewards(ctx, delAddr)
	require.Error(t, err) // Should return error because no tracker exists
}

func TestHookStakingBeforeDelegationSharesModified_InactiveValidator(t *testing.T) {
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, nil)
	// Set block height > 0 to avoid genesis special case
	ctx = ctx.WithBlockHeight(100)

	delAddr, valAddr := datagen.GenRandomAddress(), datagen.GenRandomValidatorAddress()

	hooks := k.HookStaking()
	// Store an initial validator set
	err := k.updateValidatorSet(ctx, []sdk.ValAddress{})
	require.NoError(t, err)

	// Call BeforeDelegationSharesModified - should skip processing
	err = hooks.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Verify nothing was cached (validator was not active)
	info := k.stkCache.GetStakedInfo(delAddr, valAddr)
	require.True(t, info.Amount.IsZero())
}

func TestHookStakingMultipleValidators_MixedActiveInactive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)
	// Set block height > 0 to avoid genesis special case
	ctx = ctx.WithBlockHeight(100)

	delAddr := datagen.GenRandomAddress()
	activeValAddr := datagen.GenRandomValidatorAddress()
	inactiveValAddr := datagen.GenRandomValidatorAddress()

	activeShares := math.LegacyNewDec(1000)

	activeDelegation := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: activeValAddr.String(),
		Shares:           activeShares,
	}

	activeVal, err := tmocks.CreateValidator(activeValAddr, activeShares.RoundInt())
	require.NoError(t, err)

	mockStkK := k.stkK.(*types.MockStakingKeeper)

	// Store an initial validator set
	mockStkK.EXPECT().GetValidator(gomock.Any(), activeValAddr).Return(activeVal, nil).AnyTimes()
	err = k.updateValidatorSet(ctx, []sdk.ValAddress{activeValAddr})
	require.NoError(t, err)
	mockStkK.EXPECT().GetDelegation(ctx, delAddr, activeValAddr).Return(activeDelegation, nil).Times(1)

	hooks := k.HookStaking()

	// Delegate to active validator - should be tracked
	err = hooks.AfterDelegationModified(ctx, delAddr, activeValAddr)
	require.NoError(t, err)

	// Verify tracker was created with active validator's amount
	tracker, err := k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.Equal(t, activeShares.TruncateInt().String(), tracker.ActiveBaby.String())

	// Second call: try to delegate to inactive validator
	// Note: cache is already populated, so no IterateLastValidatorPowers call
	err = hooks.AfterDelegationModified(ctx, delAddr, inactiveValAddr)
	require.NoError(t, err)

	// Verify tracker amount didn't change (inactive validator was skipped)
	tracker, err = k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.Equal(t, activeShares.TruncateInt().String(), tracker.ActiveBaby.String())
}

func TestHookStakingValidatorBecomesInactive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)
	// Set block height > 0 to avoid genesis special case
	ctx = ctx.WithBlockHeight(100)

	delAddr, valAddr := datagen.GenRandomAddress(), datagen.GenRandomValidatorAddress()
	shares := math.LegacyNewDec(1000)

	delegation := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           shares,
	}

	val, err := tmocks.CreateValidator(valAddr, shares.RoundInt())
	require.NoError(t, err)

	mockStkK := k.stkK.(*types.MockStakingKeeper)

	hooks := k.HookStaking()

	// First: validator is active

	mockStkK.EXPECT().GetDelegation(gomock.Any(), delAddr, valAddr).Return(delegation, nil).Times(1)
	mockStkK.EXPECT().GetValidator(gomock.Any(), gomock.Eq(valAddr)).Return(val, nil).AnyTimes()
	err = k.updateValidatorSet(ctx, []sdk.ValAddress{valAddr})
	require.NoError(t, err)

	// Delegate while validator is active
	err = hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	tracker, err := k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.Equal(t, shares.TruncateInt().String(), tracker.ActiveBaby.String())

	// Clear the cache to simulate new block/epoch
	k.stkCache.Clear()

	err = k.updateValidatorSet(ctx, []sdk.ValAddress{})
	require.NoError(t, err)

	// Try to modify delegation while validator is inactive - should be skipped
	err = hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Tracker should still have the old amount (no change because validator is inactive)
	tracker, err = k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.Equal(t, shares.TruncateInt().String(), tracker.ActiveBaby.String())
}
