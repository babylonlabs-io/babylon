package keeper

import (
	"math/rand"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/testutil/store"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

func FuzzCheckCurrentRewards(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)

		_, err := k.GetCurrentRewards(ctx)
		require.Error(t, err)

		currentRwd, found, err := k.GetCurrentRewardsCheckFound(ctx)
		require.NoError(t, err)
		require.False(t, found)
		require.Nil(t, currentRwd)

		expCurRwd := datagen.GenRandomCurrentRewards(r)
		err = k.SetCurrentRewards(ctx, expCurRwd)
		require.NoError(t, err)

		err = k.setHistoricalRewards(ctx, expCurRwd.Period-1, datagen.GenRandomHistoricalRewards(r))
		require.NoError(t, err)

		currentRwd, err = k.GetCurrentRewardsInitialized(ctx)
		require.NoError(t, err)
		require.Equal(t, expCurRwd.Rewards.String(), currentRwd.Rewards.String())
		require.Equal(t, expCurRwd.Period, currentRwd.Period)
		require.Equal(t, expCurRwd.TotalScore.String(), currentRwd.TotalScore.String())

		currentRwd, found, err = k.GetCurrentRewardsCheckFound(ctx)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, expCurRwd.Rewards.String(), currentRwd.Rewards.String())
		require.Equal(t, expCurRwd.Period, currentRwd.Period)
		require.Equal(t, expCurRwd.TotalScore.String(), currentRwd.TotalScore.String())

		newTotalScore := datagen.RandomMathInt(r, 50000)
		err = k.UpdateCurrentRewardsTotalScore(ctx, newTotalScore)
		require.NoError(t, err)

		updatedCurrentRwd, err := k.GetCurrentRewards(ctx)
		require.NoError(t, err)
		require.Equal(t, sdk.NewCoins().String(), updatedCurrentRwd.Rewards.String())
		require.Equal(t, expCurRwd.Period+1, updatedCurrentRwd.Period)
		require.Equal(t, newTotalScore.String(), updatedCurrentRwd.TotalScore.String())
	})
}

func FuzzCheckHistoricalRewards(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)

		period1 := datagen.RandomInt(r, 10)
		period2 := datagen.RandomInt(r, 10) + 11

		_, err := k.GetHistoricalRewards(ctx, period1)
		require.Error(t, err)

		expectedHistRwd1 := datagen.GenRandomHistoricalRewards(r)
		expectedHistRwd2 := datagen.GenRandomHistoricalRewards(r)

		err = k.setHistoricalRewards(ctx, period1, expectedHistRwd1)
		require.NoError(t, err)
		err = k.setHistoricalRewards(ctx, period2, expectedHistRwd2)
		require.NoError(t, err)

		histRwd1, err := k.GetHistoricalRewards(ctx, period1)
		require.NoError(t, err)
		require.Equal(t, expectedHistRwd1.CumulativeRewardsPerScore.String(), histRwd1.CumulativeRewardsPerScore.String())

		histRwd2, err := k.GetHistoricalRewards(ctx, period2)
		require.NoError(t, err)
		require.Equal(t, expectedHistRwd2.CumulativeRewardsPerScore.String(), histRwd2.CumulativeRewardsPerScore.String())
	})
}

func FuzzCheckCostakerRewardsTracker(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)

		costaker1 := datagen.GenRandomAddress()
		costaker2 := datagen.GenRandomAddress()

		_, err := k.GetCostakerRewards(ctx, costaker1)
		require.Error(t, err)

		costakrRwd, found, err := k.GetCostakerRewardsTrackerCheckFound(ctx, costaker1)
		require.NoError(t, err)
		require.False(t, found)
		require.Nil(t, costakrRwd)

		expectedCostakerRwd1 := datagen.GenRandomCostakerRewardsTracker(r)
		expectedCostakerRwd2 := datagen.GenRandomCostakerRewardsTracker(r)

		err = k.setCostakerRewardsTracker(ctx, costaker1, expectedCostakerRwd1)
		require.NoError(t, err)
		err = k.setCostakerRewardsTracker(ctx, costaker2, expectedCostakerRwd2)
		require.NoError(t, err)

		costakrRwd1, err := k.GetCostakerRewards(ctx, costaker1)
		require.NoError(t, err)
		require.Equal(t, expectedCostakerRwd1.StartPeriodCumulativeReward, costakrRwd1.StartPeriodCumulativeReward)
		require.Equal(t, expectedCostakerRwd1.TotalScore.String(), costakrRwd1.TotalScore.String())

		costakrRwd2, err := k.GetCostakerRewards(ctx, costaker2)
		require.NoError(t, err)
		require.Equal(t, expectedCostakerRwd2.StartPeriodCumulativeReward, costakrRwd2.StartPeriodCumulativeReward)
		require.Equal(t, expectedCostakerRwd2.TotalScore.String(), costakrRwd2.TotalScore.String())

		costakrRwd, found, err = k.GetCostakerRewardsTrackerCheckFound(ctx, costaker1)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, expectedCostakerRwd1.StartPeriodCumulativeReward, costakrRwd.StartPeriodCumulativeReward)
		require.Equal(t, expectedCostakerRwd1.TotalScore.String(), costakrRwd.TotalScore.String())

		newCostakerRwd1 := datagen.GenRandomCostakerRewardsTracker(r)
		err = k.setCostakerRewardsTracker(ctx, costaker1, newCostakerRwd1)
		require.NoError(t, err)

		updatedCostakerRwd1, err := k.GetCostakerRewards(ctx, costaker1)
		require.NoError(t, err)
		require.Equal(t, newCostakerRwd1.StartPeriodCumulativeReward, updatedCostakerRwd1.StartPeriodCumulativeReward)
		require.Equal(t, newCostakerRwd1.TotalScore.String(), updatedCostakerRwd1.TotalScore.String())
	})
}

func TestCurrentRewardsAddRewards(t *testing.T) {
	r := rand.New(rand.NewSource(42))

	currentRwd := datagen.GenRandomCurrentRewards(r)
	originalRewards := currentRwd.Rewards
	coinsToAdd := datagen.GenRandomCoins(r)

	err := currentRwd.AddRewards(coinsToAdd)
	require.NoError(t, err)

	expectedCoins := originalRewards.Add(coinsToAdd.MulInt(ictvtypes.DecimalRewards)...)
	require.Equal(t, expectedCoins.String(), currentRwd.Rewards.String())

	zeroCoins := sdk.NewCoins()
	err = currentRwd.AddRewards(zeroCoins)
	require.NoError(t, err)
	require.Equal(t, expectedCoins.String(), currentRwd.Rewards.String())
}

func NewKeeperWithCtx(t *testing.T) (*Keeper, sdk.Context) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	encConf := appparams.DefaultEncodingConfig()
	ctx, kvStore := store.NewStoreWithCtx(t, types.ModuleName)

	accK := types.NewMockAccountKeeper(ctrl)
	accK.EXPECT().GetModuleAddress(gomock.Any()).Return(authtypes.NewModuleAddress(types.ModuleName)).AnyTimes()

	k := NewKeeper(encConf.Codec, kvStore, nil, accK, nil, nil, nil, appparams.AccGov.String(), appparams.AccFeeCollector.String())
	return &k, ctx
}
