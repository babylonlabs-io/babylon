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

// TestHookStakingSlashedValidator_PostSlashDelegationUnbond tests the fix for the bug where
// unbonding a delegation made AFTER validator slashing incorrectly subtracted from ActiveBaby.
// This delegation was never added to ActiveBaby (because validator was slashed), so it should
// not be subtracted when unbonded.
func TestHookStakingSlashedValidator_PostSlashDelegationUnbond(t *testing.T) {
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, nil)
	ctx = ctx.WithBlockHeight(100)

	delAddr, valAddr := datagen.GenRandomAddress(), datagen.GenRandomValidatorAddress()
	mockStkK := k.stkK.(*types.MockStakingKeeper)
	hooks := k.HookStaking()

	// Step 1: Delegate 1000 tokens to validator (before slashing)
	preSlashShares := math.LegacyNewDec(1000)
	preSlashTokens := math.NewInt(1000)

	delegation := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           preSlashShares,
	}

	val, err := tmocks.CreateValidator(valAddr, preSlashTokens)
	require.NoError(t, err)

	// Setup validator as active - updateValidatorSet needs to get the unslashed validator
	mockStkK.EXPECT().GetValidator(gomock.Any(), valAddr).Return(val, nil).Times(1) // For updateValidatorSet
	err = k.updateValidatorSet(ctx, []sdk.ValAddress{valAddr})
	require.NoError(t, err)

	// Now set up for initial delegation
	mockStkK.EXPECT().GetDelegation(gomock.Any(), delAddr, valAddr).Return(delegation, nil).Times(1)
	mockStkK.EXPECT().GetValidator(gomock.Any(), valAddr).Return(val, nil).Times(1) // For AfterDelegationModified

	// Trigger AfterDelegationModified - adds 1000 to ActiveBaby
	err = hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	tracker, err := k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.Equal(t, "1000", tracker.ActiveBaby.String())

	// Step 2: Simulate validator slashing (10% slash)
	// Tokens reduced from 1000 to 900, but shares remain 1000
	slashedTokens := math.NewInt(900)
	slashedVal := val
	slashedVal.Tokens = slashedTokens
	// Shares remain the same, breaking 1:1 ratio

	// Clear cache to simulate new block, but DON'T update validator set
	// The validator set still has the original (pre-slash) tokens, which allows detection of slashing
	k.stkCache.Clear()
	// From now on, GetValidator returns slashed validator
	mockStkK.EXPECT().GetValidator(gomock.Any(), valAddr).Return(slashedVal, nil).AnyTimes()

	// Step 3: Delegate 200 more tokens AFTER slashing
	// With broken ratio (900 tokens / 1000 shares = 0.9), 200 tokens = ~222.22 shares
	postSlashShares := math.LegacyMustNewDecFromStr("222.222222222222222222")
	totalShares := preSlashShares.Add(postSlashShares) // 1000 + 222.22 = 1222.22

	// Before delegation, call BeforeDelegationSharesModified to cache current state
	delegationBeforePostSlash := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           preSlashShares, // Still has original 1000 shares
	}
	mockStkK.EXPECT().GetDelegation(gomock.Any(), delAddr, valAddr).Return(delegationBeforePostSlash, nil).Times(1)
	err = hooks.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Now delegate 200 more tokens
	delegationAfterPostSlash := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           totalShares,
	}

	mockStkK.EXPECT().GetDelegation(gomock.Any(), delAddr, valAddr).Return(delegationAfterPostSlash, nil).Times(1)

	// Trigger AfterDelegationModified - should NOT add to ActiveBaby (validator is slashed)
	// But should cache deltaShares = 222.22
	err = hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Verify ActiveBaby unchanged (still 1000)
	tracker, err = k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.Equal(t, "1000", tracker.ActiveBaby.String(), "ActiveBaby should not increase for slashed validator")

	// Verify deltaShares were cached
	deltaShares := k.stkCache.GetDeltaShares(valAddr, delAddr)
	require.Len(t, deltaShares, 1)
	require.True(t, postSlashShares.Equal(deltaShares[0]))

	// Step 4: Unbond the post-slash delegation (222.22 shares / 200 tokens)
	// First call BeforeDelegationSharesModified to cache current state
	delegationBeforeUnbond := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           postSlashShares, // Only the post-slash shares remain
	}
	mockStkK.EXPECT().GetDelegation(gomock.Any(), delAddr, valAddr).Return(delegationBeforeUnbond, nil).Times(1)
	err = hooks.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Trigger BeforeDelegationRemoved - should NOT subtract from ActiveBaby
	// because all shares are delta shares (added after slashing)
	err = hooks.BeforeDelegationRemoved(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Verify ActiveBaby unchanged (still 1000)
	tracker, err = k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.Equal(t, "1000", tracker.ActiveBaby.String(), "ActiveBaby should not decrease when removing post-slash delegation")
}

// TestHookStakingSlashedValidator_PreSlashDelegationUnbond tests that unbonding
// a delegation made BEFORE slashing correctly subtracts from ActiveBaby using the
// original (pre-slash) ratio.
func TestHookStakingSlashedValidator_PreSlashDelegationUnbond(t *testing.T) {
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, nil)
	ctx = ctx.WithBlockHeight(100)

	delAddr, valAddr := datagen.GenRandomAddress(), datagen.GenRandomValidatorAddress()
	mockStkK := k.stkK.(*types.MockStakingKeeper)
	hooks := k.HookStaking()

	// Delegate 1000 tokens before slashing
	preSlashShares := math.LegacyNewDec(1000)
	preSlashTokens := math.NewInt(1000)

	delegation := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           preSlashShares,
	}

	val, err := tmocks.CreateValidator(valAddr, preSlashTokens)
	require.NoError(t, err)

	mockStkK.EXPECT().GetDelegation(gomock.Any(), delAddr, valAddr).Return(delegation, nil).Times(1)
	mockStkK.EXPECT().GetValidator(gomock.Any(), valAddr).Return(val, nil).AnyTimes()
	err = k.updateValidatorSet(ctx, []sdk.ValAddress{valAddr})
	require.NoError(t, err)

	err = hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	tracker, err := k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.Equal(t, "1000", tracker.ActiveBaby.String())

	// Simulate validator slashing
	slashedTokens := math.NewInt(900)
	slashedVal := val
	slashedVal.Tokens = slashedTokens

	// Clear cache but don't update validator set (keeps original tokens for slashing detection)
	k.stkCache.Clear()
	mockStkK.EXPECT().GetValidator(gomock.Any(), valAddr).Return(slashedVal, nil).AnyTimes()

	// Unbond the pre-slash delegation
	// Current state: 900 tokens, 1000 shares
	// First call BeforeDelegationSharesModified to cache current state
	delegationBeforeUnbond := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           preSlashShares,
	}
	mockStkK.EXPECT().GetDelegation(gomock.Any(), delAddr, valAddr).Return(delegationBeforeUnbond, nil).Times(1)
	err = hooks.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	err = hooks.BeforeDelegationRemoved(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Should subtract using ORIGINAL ratio: 1000 shares * 1000 tokens / 1000 shares = 1000 tokens
	tracker, err = k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.Equal(t, "0", tracker.ActiveBaby.String(), "ActiveBaby should be reduced by original amount (1000)")
}

// TestHookStakingSlashedValidator_MixedPreAndPostSlashUnbond tests unbonding when
// the delegation contains both pre-slash and post-slash shares. Only the pre-slash
// portion should be subtracted from ActiveBaby.
func TestHookStakingSlashedValidator_MixedPreAndPostSlashUnbond(t *testing.T) {
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, nil)
	ctx = ctx.WithBlockHeight(100)

	delAddr, valAddr := datagen.GenRandomAddress(), datagen.GenRandomValidatorAddress()
	mockStkK := k.stkK.(*types.MockStakingKeeper)
	hooks := k.HookStaking()

	// Step 1: Delegate 1000 tokens before slashing
	preSlashShares := math.LegacyNewDec(1000)
	preSlashTokens := math.NewInt(1000)

	delegation := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           preSlashShares,
	}

	val, err := tmocks.CreateValidator(valAddr, preSlashTokens)
	require.NoError(t, err)

	mockStkK.EXPECT().GetDelegation(gomock.Any(), delAddr, valAddr).Return(delegation, nil).Times(1)
	mockStkK.EXPECT().GetValidator(gomock.Any(), valAddr).Return(val, nil).AnyTimes()
	err = k.updateValidatorSet(ctx, []sdk.ValAddress{valAddr})
	require.NoError(t, err)

	err = hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Step 2: Slash validator
	slashedTokens := math.NewInt(900)
	slashedVal := val
	slashedVal.Tokens = slashedTokens

	// Clear cache but don't update validator set (keeps original tokens for slashing detection)
	k.stkCache.Clear()
	mockStkK.EXPECT().GetValidator(gomock.Any(), valAddr).Return(slashedVal, nil).AnyTimes()

	// Step 3: Delegate 200 more tokens after slashing
	postSlashShares := math.LegacyMustNewDecFromStr("222.222222222222222222")
	totalShares := preSlashShares.Add(postSlashShares)

	// BeforeDelegationSharesModified to cache state before delegation
	delegationBeforePostSlash := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           preSlashShares,
	}
	mockStkK.EXPECT().GetDelegation(gomock.Any(), delAddr, valAddr).Return(delegationBeforePostSlash, nil).Times(1)
	err = hooks.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	delegationAfterPostSlash := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           totalShares,
	}
	mockStkK.EXPECT().GetDelegation(gomock.Any(), delAddr, valAddr).Return(delegationAfterPostSlash, nil).Times(1)

	err = hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Step 4: Unbond ALL shares (both pre-slash 1000 and post-slash 222.22)
	// Total current tokens: 900 + 200 = 1100
	// BeforeDelegationSharesModified to cache state before unbonding
	delegationBeforeUnbond := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           totalShares,
	}
	mockStkK.EXPECT().GetDelegation(gomock.Any(), delAddr, valAddr).Return(delegationBeforeUnbond, nil).Times(1)
	err = hooks.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	err = hooks.BeforeDelegationRemoved(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Should only subtract pre-slash shares using original ratio:
	// originalShares = totalShares(1222.22) - deltaShares(222.22) = 1000
	// delTokenChange = 1000 shares * 1000 tokens / 1000 shares = 1000 tokens
	tracker, err := k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.Equal(t, "0", tracker.ActiveBaby.String(), "ActiveBaby should only be reduced by pre-slash amount (1000)")
}

// TestHookStakingSlashedValidator_MultipleDeltaShares tests the scenario where
// multiple delegations are made after slashing, accumulating multiple delta shares.
func TestHookStakingSlashedValidator_MultipleDeltaShares(t *testing.T) {
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, nil)
	ctx = ctx.WithBlockHeight(100)

	delAddr, valAddr := datagen.GenRandomAddress(), datagen.GenRandomValidatorAddress()
	mockStkK := k.stkK.(*types.MockStakingKeeper)
	hooks := k.HookStaking()

	// Delegate before slashing
	preSlashShares := math.LegacyNewDec(1000)
	val, err := tmocks.CreateValidator(valAddr, preSlashShares.TruncateInt())
	require.NoError(t, err)

	// 5 times, buildCurrEpochValSetMap, updateValidatorSet, TokensFromShares, BeforeValidatorSlashed
	mockStkK.EXPECT().GetValidator(gomock.Any(), valAddr).Return(val, nil).Times(4)
	err = k.updateValidatorSet(ctx, []sdk.ValAddress{valAddr})
	require.NoError(t, err)

	delegation := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           preSlashShares,
	}
	mockStkK.EXPECT().GetDelegation(gomock.Any(), delAddr, valAddr).Return(delegation, nil).Times(1)
	err = hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// slash the validator current shares by 1/10
	mockStkK.EXPECT().GetValidatorDelegations(gomock.Any(), valAddr).Return([]stakingtypes.Delegation{delegation}, nil).Times(1)
	slashRatio := math.LegacyMustNewDecFromStr("0.1")
	hooks.BeforeValidatorSlashed(ctx, valAddr, slashRatio)

	// Slash validator
	slashedVal := val
	slashedVal.Tokens = math.NewInt(900)
	k.stkCache.Clear()

	// slash should reduce the active baby staked
	tracker, err := k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.Equal(t, "900", tracker.ActiveBaby.String())

	// First post-slash delegation: 100 tokens

	// buildCurrEpochValSetMap, TokensFromShares
	mockStkK.EXPECT().GetValidator(gomock.Any(), valAddr).Return(slashedVal, nil).Times(2)

	// BeforeDelegationSharesModified before first post-slash delegation
	delegationBeforeFirst := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           preSlashShares,
	}
	mockStkK.EXPECT().GetDelegation(gomock.Any(), delAddr, valAddr).Return(delegationBeforeFirst, nil).Times(1)
	err = hooks.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// added 100 tokens to slashed val
	amtTokensToAddFirstDel := math.NewInt(100)

	var addedShares math.LegacyDec
	slashedVal, addedShares = slashedVal.AddTokensFromDel(amtTokensToAddFirstDel)

	// TokensFromShares, TokensFromShares
	mockStkK.EXPECT().GetValidator(gomock.Any(), valAddr).Return(slashedVal, nil).Times(2)

	sharesAfterFirst := preSlashShares.Add(addedShares)
	require.Equal(t, sharesAfterFirst.String(), "1111.111111111111111111")

	delegationAfterFirst := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           sharesAfterFirst,
	}
	mockStkK.EXPECT().GetDelegation(gomock.Any(), delAddr, valAddr).Return(delegationAfterFirst, nil).Times(1)
	err = hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// First stake should add 100 tokens
	tracker, err = k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.Equal(t, "1000", tracker.ActiveBaby.String())

	mockStkK.EXPECT().GetDelegation(gomock.Any(), delAddr, valAddr).Return(delegationAfterFirst, nil).Times(1)
	err = hooks.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Second post-slash delegation: 50 tokens
	amtTokensToAddSecondDel := math.NewInt(50)

	slashedVal, addedShares = slashedVal.AddTokensFromDel(amtTokensToAddSecondDel)
	totalShares := sharesAfterFirst.Add(addedShares)

	// TokensFromShares (AfterDelegationModified), TokensFromShares (BeforeDelegationSharesModified of ubd), TokensFromShares (BeforeDelegationRemoved)
	mockStkK.EXPECT().GetValidator(gomock.Any(), valAddr).Return(slashedVal, nil).Times(3)

	delegationAfterSecond := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           totalShares,
	}
	mockStkK.EXPECT().GetDelegation(gomock.Any(), delAddr, valAddr).Return(delegationAfterSecond, nil).Times(1)
	err = hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Second stake should add 50 tokens
	tracker, err = k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.Equal(t, "1050", tracker.ActiveBaby.String())

	// Unbond all shares
	// BeforeDelegationSharesModified before unbonding
	mockStkK.EXPECT().GetDelegation(gomock.Any(), delAddr, valAddr).Return(delegationAfterSecond, nil).Times(2)
	err = hooks.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	err = hooks.BeforeDelegationRemoved(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Should unbond all from the costaking tracker
	tracker, err = k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.Equal(t, "0", tracker.ActiveBaby.String())
}

// TestHookStakingSlashedValidator_OnlyPostSlashDelegationExists tests the edge case
// where a delegator only has post-slash delegation (no pre-slash delegation existed).
// If the validator is active in the active set (even if he is slashed) the new delegations
// should count. Once the validator leaves the active set it should reduce it.
func TestHookStakingSlashedValidator_OnlyPostSlashDelegationExists(t *testing.T) {
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, nil)
	ctx = ctx.WithBlockHeight(100)

	delAddr, valAddr := datagen.GenRandomAddress(), datagen.GenRandomValidatorAddress()
	mockStkK := k.stkK.(*types.MockStakingKeeper)
	hooks := k.HookStaking()

	// Validator exists and is already slashed (someone else staked before)
	val, err := tmocks.CreateValidator(valAddr, math.NewInt(900))
	require.NoError(t, err)
	val.DelegatorShares = math.LegacyNewDec(1000) // Broken ratio indicates slashing

	mockStkK.EXPECT().GetValidator(gomock.Any(), valAddr).Return(val, nil).AnyTimes()
	err = k.updateValidatorSet(ctx, []sdk.ValAddress{valAddr})
	require.NoError(t, err)

	// New delegator stakes 100 tokens to already-slashed validator
	postSlashShares := math.LegacyMustNewDecFromStr("111.111111111111111111")

	delegation := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           postSlashShares,
	}

	// BeforeDelegationSharesModified - no pre-existing delegation
	mockStkK.EXPECT().GetDelegation(gomock.Any(), delAddr, valAddr).Return(stakingtypes.Delegation{}, stakingtypes.ErrNoDelegation).Times(1)
	err = hooks.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
	require.NoError(t, err) // Should not error even though delegation doesn't exist yet

	// AfterDelegationModified - delegate after validator is slashed
	mockStkK.EXPECT().GetDelegation(gomock.Any(), delAddr, valAddr).Return(delegation, nil).Times(1)
	err = hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Should not add to ActiveBaby
	tracker, err := k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.Equal(t, "100", tracker.ActiveBaby.String())

	// Unbond the post-slash delegation
	// BeforeDelegationSharesModified before unbonding
	delegationBeforeUnbond := stakingtypes.Delegation{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
		Shares:           postSlashShares,
	}
	mockStkK.EXPECT().GetDelegation(gomock.Any(), delAddr, valAddr).Return(delegationBeforeUnbond, nil).Times(1)
	err = hooks.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
	require.NoError(t, err)

	err = hooks.BeforeDelegationRemoved(ctx, delAddr, valAddr)
	require.NoError(t, err)

	// Should subtract from ActiveBaby
	tracker, err = k.GetCostakerRewards(ctx, delAddr)
	require.NoError(t, err)
	require.Equal(t, "0", tracker.ActiveBaby.String())
}
