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

	mockStkK := k.stkK.(*types.MockStakingKeeper)
	mockStkK.EXPECT().GetDelegation(ctx, delAddr, valAddr).Return(delegation, nil).Times(1)

	hooks := k.HookStaking()

	err := hooks.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Verify the amount was cached by retrieving
	cachedAmount := k.stkCache.GetStakedAmount(delAddr, valAddr)
	require.True(t, shares.Equal(cachedAmount))
	// get again and make sure it is not deleted
	cachedAmount = k.stkCache.GetStakedAmount(delAddr, valAddr)
	require.True(t, shares.Equal(cachedAmount))

	mockStkK.EXPECT().GetDelegation(ctx, delAddr, valAddr).Return(stakingtypes.Delegation{}, stakingtypes.ErrNoDelegation).Times(1)
	// Call BeforeDelegationSharesModified - should not return error even though the get del returned err
	err = hooks.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	cachedAmount = k.stkCache.GetStakedAmount(delAddr, valAddr)
	require.True(t, shares.Equal(cachedAmount))
}

func TestHookStakingAfterDelegationModified(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIctvK := types.NewMockIncentiveKeeper(ctrl)
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, mockIctvK)

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
	mockStkK.EXPECT().Validator(gomock.Any(), gomock.Eq(valAddr)).Return(&val, nil).Times(1)

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
	err = hooks.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	delegationAfter := delegation
	delegationAfter.Shares = delegation.Shares.MulInt64(2)

	mockBankK := k.bankK.(*types.MockBankKeeper)

	mockStkK.EXPECT().GetDelegation(ctx, delAddr, valAddr).Return(delegationAfter, nil).Times(1)
	mockBankK.EXPECT().SendCoinsFromModuleToModule(ctx, types.ModuleName, ictvtypes.ModuleName, expRwd).Return(nil).Times(1)
	mockIctvK.EXPECT().AccumulateRewardGaugeForCostaker(gomock.Any(), gomock.Eq(delAddr), expRwd).Times(1)

	mockStkK.EXPECT().Validator(gomock.Any(), gomock.Eq(valAddr)).Return(&val, nil).Times(1)
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

	k.stkCache.SetStakedAmount(delAddr, valAddr, initShares.ToLegacyDec())

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
	mockStkK.EXPECT().Validator(gomock.Any(), gomock.Any()).Return(&val, nil).Times(1)

	hooks := k.HookStaking()

	err = hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Verify the costaker tracker was updated with the negative delta
	tracker, err := k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.Equal(t, afterShares.TruncateInt().String(), tracker.ActiveBaby.String())
}
