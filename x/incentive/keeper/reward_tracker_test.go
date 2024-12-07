package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	appparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	"github.com/babylonlabs-io/babylon/x/incentive/keeper"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestAddSubDelegationSat(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	k, ctx := keepertest.IncentiveKeeper(t, nil, nil, nil)

	fp1, del1 := datagen.GenRandomAddress(), datagen.GenRandomAddress()
	fp2, del2 := datagen.GenRandomAddress(), datagen.GenRandomAddress()
	amtFp1Del1, amtFp1Del2, amtFp2Del2, amtFp2Del1 := math.NewInt(2000), math.NewInt(4000), math.NewInt(500), math.NewInt(700)

	// adds 2000 for fp1, del1
	// fp1       => 2000
	// fp1, del1 => 2000
	err := k.AddDelegationSat(ctx, fp1, del1, amtFp1Del1)
	require.NoError(t, err)
	checkFpTotalSat(t, ctx, k, fp1, amtFp1Del1)
	checkFpDelTotalSat(t, ctx, k, fp1, del1, amtFp1Del1)

	// adds 4000 for fp1, del2
	// fp1       => 6000
	// fp1, del1 => 2000
	// fp1, del2 => 4000
	err = k.AddDelegationSat(ctx, fp1, del2, amtFp1Del2)
	require.NoError(t, err)

	checkFpTotalSat(t, ctx, k, fp1, amtFp1Del1.Add(amtFp1Del2))
	checkFpDelTotalSat(t, ctx, k, fp1, del2, amtFp1Del2)
	checkFpDelTotalSat(t, ctx, k, fp1, del1, amtFp1Del1)

	// adds 500 for fp2, del2
	// fp1       => 6000
	// fp2       =>  500
	// fp1, del1 => 2000
	// fp1, del2 => 4000
	// fp2, del2 =>  500
	err = k.AddDelegationSat(ctx, fp2, del2, amtFp2Del2)
	require.NoError(t, err)
	checkFpTotalSat(t, ctx, k, fp1, amtFp1Del1.Add(amtFp1Del2))
	checkFpTotalSat(t, ctx, k, fp2, amtFp2Del2)
	checkFpDelTotalSat(t, ctx, k, fp1, del1, amtFp1Del1)
	checkFpDelTotalSat(t, ctx, k, fp1, del2, amtFp1Del2)
	checkFpDelTotalSat(t, ctx, k, fp2, del2, amtFp2Del2)

	// adds 700 for fp2, del1
	// fp1       => 6000
	// fp2       => 1200
	// fp1, del1 => 2000
	// fp1, del2 => 4000
	// fp2, del1 =>  700
	// fp2, del2 =>  500
	err = k.AddDelegationSat(ctx, fp2, del1, amtFp2Del1)
	require.NoError(t, err)
	checkFpTotalSat(t, ctx, k, fp1, amtFp1Del1.Add(amtFp1Del2))
	checkFpTotalSat(t, ctx, k, fp2, amtFp2Del2.Add(amtFp2Del1))
	checkFpDelTotalSat(t, ctx, k, fp1, del1, amtFp1Del1)
	checkFpDelTotalSat(t, ctx, k, fp1, del2, amtFp1Del2)
	checkFpDelTotalSat(t, ctx, k, fp2, del1, amtFp2Del1)
	checkFpDelTotalSat(t, ctx, k, fp2, del2, amtFp2Del2)

	lastAmtFp1Del2 := math.NewInt(2000)
	// adds 2000 for fp1, del2
	// fp1       => 8000
	// fp2       => 1200
	// fp1, del1 => 2000
	// fp1, del2 => 6000
	// fp2, del1 =>  700
	// fp2, del2 =>  500
	err = k.AddDelegationSat(ctx, fp1, del2, lastAmtFp1Del2)
	require.NoError(t, err)
	checkFpTotalSat(t, ctx, k, fp1, amtFp1Del1.Add(amtFp1Del2).Add(lastAmtFp1Del2))
	checkFpTotalSat(t, ctx, k, fp2, amtFp2Del2.Add(amtFp2Del1))
	checkFpDelTotalSat(t, ctx, k, fp1, del1, amtFp1Del1)
	checkFpDelTotalSat(t, ctx, k, fp1, del2, amtFp1Del2.Add(lastAmtFp1Del2))
	checkFpDelTotalSat(t, ctx, k, fp2, del1, amtFp2Del1)
	checkFpDelTotalSat(t, ctx, k, fp2, del2, amtFp2Del2)

	subAmtFp2Del2 := math.NewInt(350)
	// subtract 350 for fp2, del2
	// fp1       => 8000
	// fp2       =>  850
	// fp1, del1 => 2000
	// fp1, del2 => 6000
	// fp2, del1 =>  700
	// fp2, del2 =>  150
	err = k.SubDelegationSat(ctx, fp2, del2, subAmtFp2Del2)
	require.NoError(t, err)
	checkFpTotalSat(t, ctx, k, fp1, amtFp1Del1.Add(amtFp1Del2).Add(lastAmtFp1Del2))
	checkFpTotalSat(t, ctx, k, fp2, amtFp2Del2.Add(amtFp2Del1).Sub(subAmtFp2Del2))
	checkFpDelTotalSat(t, ctx, k, fp1, del1, amtFp1Del1)
	checkFpDelTotalSat(t, ctx, k, fp1, del2, amtFp1Del2.Add(lastAmtFp1Del2))
	checkFpDelTotalSat(t, ctx, k, fp2, del1, amtFp2Del1)
	checkFpDelTotalSat(t, ctx, k, fp2, del2, amtFp2Del2.Sub(subAmtFp2Del2))
}

func checkFpTotalSat(t *testing.T, ctx sdk.Context, k *keeper.Keeper, fp sdk.AccAddress, expectedSat math.Int) {
	rwd, err := k.GetFinalityProviderCurrentRewards(ctx, fp)
	require.NoError(t, err)
	require.Equal(t, expectedSat.String(), rwd.TotalActiveSat.String())
}

func checkFpDelTotalSat(t *testing.T, ctx sdk.Context, k *keeper.Keeper, fp, del sdk.AccAddress, expectedSat math.Int) {
	rwd, err := k.GetBTCDelegationRewardsTracker(ctx, fp, del)
	require.NoError(t, err)
	require.Equal(t, expectedSat.String(), rwd.TotalActiveSat.String())
}

func TestIncrementFinalityProviderPeriod(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	k, ctx := keepertest.IncentiveKeeper(t, nil, nil, nil)

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
	checkFpHistoricalRwd(t, ctx, k, fp1, fp1EndedPeriod, newBaseCoins(4000).MulInt(keeper.DecimalAccumulatedRewards))
	checkFpCurrentRwd(t, ctx, k, fp1, fp1EndedPeriod+1, sdk.NewCoins(), satsDelegated)

	fp2EndedPeriod, err := k.IncrementFinalityProviderPeriod(ctx, fp2)
	require.NoError(t, err)
	require.Equal(t, fp2EndedPeriod, uint64(1))
}

func checkFpHistoricalRwd(t *testing.T, ctx sdk.Context, k *keeper.Keeper, fp sdk.AccAddress, period uint64, expectedRwd sdk.Coins) {
	historical, err := k.GetFinalityProviderHistoricalRewards(ctx, fp, period)
	require.NoError(t, err)
	require.Equal(t, historical.CumulativeRewardsPerSat.String(), expectedRwd.String())
}

func checkFpCurrentRwd(t *testing.T, ctx sdk.Context, k *keeper.Keeper, fp sdk.AccAddress, expectedPeriod uint64, expectedRwd sdk.Coins, totalActiveSat math.Int) {
	fp1CurrentRwd, err := k.GetFinalityProviderCurrentRewards(ctx, fp)
	require.NoError(t, err)
	require.Equal(t, fp1CurrentRwd.CurrentRewards.String(), expectedRwd.String())
	require.Equal(t, fp1CurrentRwd.Period, expectedPeriod)
	require.Equal(t, fp1CurrentRwd.TotalActiveSat.String(), totalActiveSat.String())
}

func newBaseCoins(amt uint64) sdk.Coins {
	return sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, math.NewIntFromUint64(amt)))
}
