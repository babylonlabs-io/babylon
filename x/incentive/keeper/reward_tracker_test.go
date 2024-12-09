package keeper

import (
	"testing"

	"cosmossdk.io/math"
	appparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestIncrementFinalityProviderPeriod(t *testing.T) {
	k, ctx := NewKeeperWithCtx(t)

	fp1, fp2 := datagen.GenRandomAddress(), datagen.GenRandomAddress()
	del1 := datagen.GenRandomAddress()

	fp1EndedPeriod, err := k.IncrementFinalityProviderPeriod(ctx, fp1)
	require.NoError(t, err)
	require.Equal(t, fp1EndedPeriod, uint64(1))

	checkFpCurrentRwd(t, ctx, k, fp1, fp1EndedPeriod, sdk.NewCoins(), math.NewInt(0))
	checkFpHistoricalRwd(t, ctx, k, fp1, 0, sdk.NewCoins())

	rwdAddedToPeriod1 := newBaseCoins(2_000000) // 2bbn
	err = k.AddFinalityProviderRewardsForDelegationsBTC(ctx, fp1, rwdAddedToPeriod1)
	require.NoError(t, err)

	// historical should not modify the rewards for the period already created
	checkFpHistoricalRwd(t, ctx, k, fp1, 0, sdk.NewCoins())
	checkFpCurrentRwd(t, ctx, k, fp1, fp1EndedPeriod, rwdAddedToPeriod1, math.NewInt(0))

	// needs to add some voting power so it can calculate the amount of rewards per share
	satsDelegated := math.NewInt(500)
	err = k.AddDelegationSat(ctx, fp1, del1, satsDelegated)
	require.NoError(t, err)

	fp1EndedPeriod, err = k.IncrementFinalityProviderPeriod(ctx, fp1)
	require.NoError(t, err)
	require.Equal(t, fp1EndedPeriod, uint64(1))

	// now the historical that just ended should have as cumulative rewards 4000ubbn 2_000000ubbn/500sats
	checkFpHistoricalRwd(t, ctx, k, fp1, fp1EndedPeriod, newBaseCoins(4000).MulInt(DecimalAccumulatedRewards))
	checkFpCurrentRwd(t, ctx, k, fp1, fp1EndedPeriod+1, sdk.NewCoins(), satsDelegated)

	fp2EndedPeriod, err := k.IncrementFinalityProviderPeriod(ctx, fp2)
	require.NoError(t, err)
	require.Equal(t, fp2EndedPeriod, uint64(1))
}

func checkFpHistoricalRwd(t *testing.T, ctx sdk.Context, k *Keeper, fp sdk.AccAddress, period uint64, expectedRwd sdk.Coins) {
	historical, err := k.GetFinalityProviderHistoricalRewards(ctx, fp, period)
	require.NoError(t, err)
	require.Equal(t, historical.CumulativeRewardsPerSat.String(), expectedRwd.String())
}

func checkFpCurrentRwd(t *testing.T, ctx sdk.Context, k *Keeper, fp sdk.AccAddress, expectedPeriod uint64, expectedRwd sdk.Coins, totalActiveSat math.Int) {
	fp1CurrentRwd, err := k.GetFinalityProviderCurrentRewards(ctx, fp)
	require.NoError(t, err)
	require.Equal(t, fp1CurrentRwd.CurrentRewards.String(), expectedRwd.String())
	require.Equal(t, fp1CurrentRwd.Period, expectedPeriod)
	require.Equal(t, fp1CurrentRwd.TotalActiveSat.String(), totalActiveSat.String())
}

func newBaseCoins(amt uint64) sdk.Coins {
	return sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, math.NewIntFromUint64(amt)))
}
