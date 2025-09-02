package keeper

import (
	"math/rand"
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/testutil/store"
	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
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

		currentRwd, err = k.GetCurrentRewards(ctx)
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
		require.Equal(t, expCurRwd.Rewards.String(), updatedCurrentRwd.Rewards.String())
		require.Equal(t, expCurRwd.Period, updatedCurrentRwd.Period)
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

func FuzzCheckCoostakerRewardsTracker(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)

		coostaker1 := datagen.GenRandomAddress()
		coostaker2 := datagen.GenRandomAddress()

		_, err := k.GetCoostakerRewards(ctx, coostaker1)
		require.Error(t, err)

		coostakerRwd, found, err := k.GetCoostakerRewardsTrackerCheckFound(ctx, coostaker1)
		require.NoError(t, err)
		require.False(t, found)
		require.Nil(t, coostakerRwd)

		expectedCoostakerRwd1 := datagen.GenRandomCoostakerRewardsTracker(r)
		expectedCoostakerRwd2 := datagen.GenRandomCoostakerRewardsTracker(r)

		err = k.setCoostakerRewardsTracker(ctx, coostaker1, expectedCoostakerRwd1)
		require.NoError(t, err)
		err = k.setCoostakerRewardsTracker(ctx, coostaker2, expectedCoostakerRwd2)
		require.NoError(t, err)

		coostakerRwd1, err := k.GetCoostakerRewards(ctx, coostaker1)
		require.NoError(t, err)
		require.Equal(t, expectedCoostakerRwd1.StartPeriodCumulativeReward, coostakerRwd1.StartPeriodCumulativeReward)
		require.Equal(t, expectedCoostakerRwd1.TotalScore.String(), coostakerRwd1.TotalScore.String())

		coostakerRwd2, err := k.GetCoostakerRewards(ctx, coostaker2)
		require.NoError(t, err)
		require.Equal(t, expectedCoostakerRwd2.StartPeriodCumulativeReward, coostakerRwd2.StartPeriodCumulativeReward)
		require.Equal(t, expectedCoostakerRwd2.TotalScore.String(), coostakerRwd2.TotalScore.String())

		coostakerRwd, found, err = k.GetCoostakerRewardsTrackerCheckFound(ctx, coostaker1)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, expectedCoostakerRwd1.StartPeriodCumulativeReward, coostakerRwd.StartPeriodCumulativeReward)
		require.Equal(t, expectedCoostakerRwd1.TotalScore.String(), coostakerRwd.TotalScore.String())

		newCoostakerRwd1 := datagen.GenRandomCoostakerRewardsTracker(r)
		err = k.setCoostakerRewardsTracker(ctx, coostaker1, newCoostakerRwd1)
		require.NoError(t, err)

		updatedCoostakerRwd1, err := k.GetCoostakerRewards(ctx, coostaker1)
		require.NoError(t, err)
		require.Equal(t, newCoostakerRwd1.StartPeriodCumulativeReward, updatedCoostakerRwd1.StartPeriodCumulativeReward)
		require.Equal(t, newCoostakerRwd1.TotalScore.String(), updatedCoostakerRwd1.TotalScore.String())
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

func TestUpdateCurrentRewardsTotalScoreWhenNotFound(t *testing.T) {
	k, ctx := NewKeeperWithCtx(t)

	newTotalScore := math.NewInt(1000)
	err := k.UpdateCurrentRewardsTotalScore(ctx, newTotalScore)
	require.Error(t, err)
}

func NewKeeperWithCtx(t *testing.T) (*Keeper, sdk.Context) {
	encConf := appparams.DefaultEncodingConfig()
	ctx, kvStore := store.NewStoreWithCtx(t, types.ModuleName)
	k := NewKeeper(encConf.Codec, kvStore, nil, nil, nil, nil, nil, appparams.AccGov.String(), appparams.AccFeeCollector.String())
	return &k, ctx
}
