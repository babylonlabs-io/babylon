package keeper_test

import (
	"math/rand"
	"testing"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	testkeeper "github.com/babylonlabs-io/babylon/testutil/keeper"
	"github.com/babylonlabs-io/babylon/x/incentive/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func FuzzRewardGaugesQuery(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		keeper, ctx := testkeeper.IncentiveKeeper(t, nil, nil, nil)

		// generate a list of random RewardGauge map and insert them to KVStore
		// where in each map, key is stakeholder type and address is the reward gauge
		rgMaps := []map[string]*types.RewardGaugesResponse{}
		sAddrList := []sdk.AccAddress{}
		numRgMaps := datagen.RandomInt(r, 100)
		for i := uint64(0); i < numRgMaps; i++ {
			rgMap := map[string]*types.RewardGaugesResponse{}
			sAddr := datagen.GenRandomAccount().GetAddress()
			sAddrList = append(sAddrList, sAddr)
			for i := uint64(0); i <= datagen.RandomInt(r, 4); i++ {
				sType := datagen.GenRandomStakeholderType(r)
				rg := datagen.GenRandomRewardGauge(r)
				rgMap[sType.String()] = &types.RewardGaugesResponse{
					Coins:          rg.Coins,
					WithdrawnCoins: rg.WithdrawnCoins,
				}

				keeper.SetRewardGauge(ctx, sType, sAddr, rg)
			}
			rgMaps = append(rgMaps, rgMap)
		}

		// query existence and assert consistency
		for i := range rgMaps {
			req := &types.QueryRewardGaugesRequest{
				Address: sAddrList[i].String(),
			}
			resp, err := keeper.RewardGauges(ctx, req)
			require.NoError(t, err)
			for sTypeStr, rg := range rgMaps[i] {
				require.Equal(t, rg.Coins, resp.RewardGauges[sTypeStr].Coins)
			}
		}
	})
}

func FuzzBTCStakingGaugeQuery(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		keeper, ctx := testkeeper.IncentiveKeeper(t, nil, nil, nil)

		// generate a list of random Gauges at random heights, then insert them to KVStore
		heightList := []uint64{datagen.RandomInt(r, 1000) + 1}
		gaugeList := []*types.Gauge{datagen.GenRandomGauge(r)}
		keeper.SetBTCStakingGauge(ctx, heightList[0], gaugeList[0])

		numGauges := datagen.RandomInt(r, 100) + 1
		for i := uint64(1); i < numGauges; i++ {
			height := heightList[i-1] + datagen.RandomInt(r, 100) + 1
			heightList = append(heightList, height)
			gauge := datagen.GenRandomGauge(r)
			gaugeList = append(gaugeList, gauge)
			keeper.SetBTCStakingGauge(ctx, height, gauge)
		}

		// query existence and assert consistency
		for i := range gaugeList {
			req := &types.QueryBTCStakingGaugeRequest{
				Height: heightList[i],
			}
			resp, err := keeper.BTCStakingGauge(ctx, req)
			require.NoError(t, err)
			require.True(t, resp.Gauge.Coins.Equal(gaugeList[i].Coins))
		}
	})
}

func FuzzDelegationRewardsQuery(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		var (
			r            = rand.New(rand.NewSource(seed))
			storeKey     = storetypes.NewKVStoreKey(types.StoreKey)
			k, ctx       = testkeeper.IncentiveKeeperWithStoreKey(t, storeKey, nil, nil, nil)
			fp, del      = datagen.GenRandomAddress(), datagen.GenRandomAddress()
			storeService = runtime.NewKVStoreService(storeKey)
			store        = storeService.OpenKVStore(ctx)
			storeAdaptor = runtime.KVStoreAdapter(store)
			btcRwd       = datagen.GenRandomBTCDelegationRewardsTracker(r)
		)

		// Setup a BTCDelegationRewardsTracker for the delegator
		require.NoError(t, k.BTCDelegationRewardsTracker.Set(ctx, collections.Join(fp.Bytes(), del.Bytes()), btcRwd))
		st := prefix.NewStore(storeAdaptor, types.BTCDelegatorToFPKey)
		delStore := prefix.NewStore(st, del.Bytes())
		delStore.Set(fp.Bytes(), []byte{0x00})

		// Setup FP current rewards with the corresponding period
		// and same TotalActiveSat than the BTCDelegationRewardsTracker
		fpCurrentRwd := datagen.GenRandomFinalityProviderCurrentRewards(r)
		fpCurrentRwd.Period = btcRwd.StartPeriodCumulativeReward + 2
		fpCurrentRwd.TotalActiveSat = btcRwd.TotalActiveSat
		err := k.FinalityProviderCurrentRewards.Set(ctx, fp.Bytes(), fpCurrentRwd)
		require.NoError(t, err)

		// set start historical rewards corresponding to btcRwd.StartPeriodCumulativeReward
		amtRwdInHistStart := fpCurrentRwd.CurrentRewards.MulInt(types.DecimalAccumulatedRewards).QuoInt(math.NewInt(2))
		startHist := types.NewFinalityProviderHistoricalRewards(amtRwdInHistStart)
		err = k.FinalityProviderHistoricalRewards.Set(ctx, collections.Join(fp.Bytes(), btcRwd.StartPeriodCumulativeReward), startHist)
		require.NoError(t, err)

		// set end period historical rewards
		// end period for calculation is fpCurrentRwd.Period-1
		amtRwdInHistEnd := amtRwdInHistStart.Add(fpCurrentRwd.CurrentRewards.MulInt(types.DecimalAccumulatedRewards).QuoInt(fpCurrentRwd.TotalActiveSat)...)
		endHist := types.NewFinalityProviderHistoricalRewards(amtRwdInHistEnd)
		err = k.FinalityProviderHistoricalRewards.Set(ctx, collections.Join(fp.Bytes(), fpCurrentRwd.Period-1), endHist)
		require.NoError(t, err)

		// Calculate expected rewards
		expectedRwd := endHist.CumulativeRewardsPerSat.Sub(startHist.CumulativeRewardsPerSat...)
		expectedRwd = expectedRwd.MulInt(btcRwd.TotalActiveSat.Add(fpCurrentRwd.TotalActiveSat))
		expectedRwd = expectedRwd.QuoInt(types.DecimalAccumulatedRewards)

		// Call the DelegationRewards query
		res, err := k.DelegationRewards(
			ctx,
			&types.QueryDelegationRewardsRequest{
				FinalityProviderAddress: fp.String(),
				DelegatorAddress:        del.String(),
			},
		)
		require.NoError(t, err)
		require.Equal(t, expectedRwd, res.Rewards)
	})
}
