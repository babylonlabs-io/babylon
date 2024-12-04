package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestAddDelegationSat(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	k, ctx := keepertest.IncentiveKeeper(t, nil, nil, nil)

	fp1Addr, del1Addr := datagen.GenRandomAddress(), datagen.GenRandomAddress()
	fp2Addr, del2Addr := datagen.GenRandomAddress(), datagen.GenRandomAddress()
	amtFp1Del1, amtFp1Del2, amtFp2Del2, amtFp2Del1 := math.NewInt(2000), math.NewInt(4000), math.NewInt(500), math.NewInt(700)

	// adds 2000 for fp1, del1
	// fp1       => 2000
	// fp1, del1 => 2000
	err := k.AddDelegationSat(ctx, fp1Addr, del1Addr, amtFp1Del1)
	require.NoError(t, err)

	fp1Rwd, err := k.GetFinalityProviderCurrentRewards(ctx, fp1Addr)
	require.NoError(t, err)
	require.Equal(t, amtFp1Del1.String(), fp1Rwd.TotalActiveSat.String())

	fp1Del1Rwd, err := k.GetBTCDelegationRewardsTracker(ctx, fp1Addr, del1Addr)
	require.NoError(t, err)
	require.Equal(t, amtFp1Del1.String(), fp1Del1Rwd.TotalActiveSat.String())

	// adds 4000 for fp1, del2
	// fp1       => 6000
	// fp1, del1 => 2000
	// fp1, del2 => 4000
	err = k.AddDelegationSat(ctx, fp1Addr, del2Addr, amtFp1Del2)
	require.NoError(t, err)

	fp1Rwd, err = k.GetFinalityProviderCurrentRewards(ctx, fp1Addr)
	require.NoError(t, err)
	require.Equal(t, amtFp1Del1.Add(amtFp1Del2).String(), fp1Rwd.TotalActiveSat.String())

	fp1Del2Rwd, err := k.GetBTCDelegationRewardsTracker(ctx, fp1Addr, del2Addr)
	require.NoError(t, err)
	require.Equal(t, amtFp1Del2.String(), fp1Del2Rwd.TotalActiveSat.String())

	fp1Del1Rwd, err = k.GetBTCDelegationRewardsTracker(ctx, fp1Addr, del1Addr)
	require.NoError(t, err)
	require.Equal(t, amtFp1Del1.String(), fp1Del1Rwd.TotalActiveSat.String())

	// adds 500 for fp2, del2
	// fp1       => 6000
	// fp2       =>  500
	// fp1, del1 => 2000
	// fp1, del2 => 4000
	// fp2, del2 =>  500
	err = k.AddDelegationSat(ctx, fp2Addr, del2Addr, amtFp2Del2)
	require.NoError(t, err)

	fp1Rwd, err = k.GetFinalityProviderCurrentRewards(ctx, fp1Addr)
	require.NoError(t, err)
	require.Equal(t, amtFp1Del1.Add(amtFp1Del2).String(), fp1Rwd.TotalActiveSat.String())

	fp2Rwd, err := k.GetFinalityProviderCurrentRewards(ctx, fp2Addr)
	require.NoError(t, err)
	require.Equal(t, amtFp2Del2.String(), fp2Rwd.TotalActiveSat.String())

	fp1Del1Rwd, err = k.GetBTCDelegationRewardsTracker(ctx, fp1Addr, del1Addr)
	require.NoError(t, err)
	require.Equal(t, amtFp1Del1.String(), fp1Del1Rwd.TotalActiveSat.String())

	fp1Del2Rwd, err = k.GetBTCDelegationRewardsTracker(ctx, fp1Addr, del2Addr)
	require.NoError(t, err)
	require.Equal(t, amtFp1Del2.String(), fp1Del2Rwd.TotalActiveSat.String())

	fp2Del2Rwd, err := k.GetBTCDelegationRewardsTracker(ctx, fp2Addr, del2Addr)
	require.NoError(t, err)
	require.Equal(t, amtFp2Del2.String(), fp2Del2Rwd.TotalActiveSat.String())

	// adds 700 for fp2, del1
	// fp1       => 6000
	// fp2       => 1200
	// fp1, del1 => 2000
	// fp1, del2 => 4000
	// fp2, del1 =>  700
	// fp2, del2 =>  500
	err = k.AddDelegationSat(ctx, fp2Addr, del1Addr, amtFp2Del1)
	require.NoError(t, err)

	fp1Rwd, err = k.GetFinalityProviderCurrentRewards(ctx, fp1Addr)
	require.NoError(t, err)
	require.Equal(t, amtFp1Del1.Add(amtFp1Del2).String(), fp1Rwd.TotalActiveSat.String())

	fp2Rwd, err = k.GetFinalityProviderCurrentRewards(ctx, fp2Addr)
	require.NoError(t, err)
	require.Equal(t, amtFp2Del2.Add(amtFp2Del1).String(), fp2Rwd.TotalActiveSat.String())

	fp1Del1Rwd, err = k.GetBTCDelegationRewardsTracker(ctx, fp1Addr, del1Addr)
	require.NoError(t, err)
	require.Equal(t, amtFp1Del1.String(), fp1Del1Rwd.TotalActiveSat.String())

	fp1Del2Rwd, err = k.GetBTCDelegationRewardsTracker(ctx, fp1Addr, del2Addr)
	require.NoError(t, err)
	require.Equal(t, amtFp1Del2.String(), fp1Del2Rwd.TotalActiveSat.String())

	fp2Del1Rwd, err := k.GetBTCDelegationRewardsTracker(ctx, fp2Addr, del1Addr)
	require.NoError(t, err)
	require.Equal(t, amtFp2Del1.String(), fp2Del1Rwd.TotalActiveSat.String())

	fp2Del2Rwd, err = k.GetBTCDelegationRewardsTracker(ctx, fp2Addr, del2Addr)
	require.NoError(t, err)
	require.Equal(t, amtFp2Del2.String(), fp2Del2Rwd.TotalActiveSat.String())

	lastAmtFp1Del2 := math.NewInt(2000)
	// adds 2000 for fp1, del2
	// fp1       => 8000
	// fp2       => 1200
	// fp1, del1 => 2000
	// fp1, del2 => 6000
	// fp2, del1 =>  700
	// fp2, del2 =>  500
	err = k.AddDelegationSat(ctx, fp1Addr, del2Addr, lastAmtFp1Del2)
	require.NoError(t, err)

	fp1Rwd, err = k.GetFinalityProviderCurrentRewards(ctx, fp1Addr)
	require.NoError(t, err)
	require.Equal(t, amtFp1Del1.Add(amtFp1Del2).Add(lastAmtFp1Del2).String(), fp1Rwd.TotalActiveSat.String())

	fp2Rwd, err = k.GetFinalityProviderCurrentRewards(ctx, fp2Addr)
	require.NoError(t, err)
	require.Equal(t, amtFp2Del2.Add(amtFp2Del1).String(), fp2Rwd.TotalActiveSat.String())

	fp1Del1Rwd, err = k.GetBTCDelegationRewardsTracker(ctx, fp1Addr, del1Addr)
	require.NoError(t, err)
	require.Equal(t, amtFp1Del1.String(), fp1Del1Rwd.TotalActiveSat.String())

	fp1Del2Rwd, err = k.GetBTCDelegationRewardsTracker(ctx, fp1Addr, del2Addr)
	require.NoError(t, err)
	require.Equal(t, amtFp1Del2.Add(lastAmtFp1Del2).String(), fp1Del2Rwd.TotalActiveSat.String())

	fp2Del1Rwd, err = k.GetBTCDelegationRewardsTracker(ctx, fp2Addr, del1Addr)
	require.NoError(t, err)
	require.Equal(t, amtFp2Del1.String(), fp2Del1Rwd.TotalActiveSat.String())

	fp2Del2Rwd, err = k.GetBTCDelegationRewardsTracker(ctx, fp2Addr, del2Addr)
	require.NoError(t, err)
	require.Equal(t, amtFp2Del2.String(), fp2Del2Rwd.TotalActiveSat.String())
}
