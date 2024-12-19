package keeper_test

import (
	"math/rand"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	testkeeper "github.com/babylonlabs-io/babylon/testutil/keeper"
	"github.com/babylonlabs-io/babylon/x/incentive/types"
)

func FuzzRewardBTCStaking(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock bank keeper
		bankKeeper := types.NewMockBankKeeper(ctrl)

		// create incentive keeper
		keeper, ctx := testkeeper.IncentiveKeeper(t, bankKeeper, nil, nil)
		height := datagen.RandomInt(r, 1000)
		ctx = datagen.WithCtxHeight(ctx, height)

		// set a random gauge
		gauge := datagen.GenRandomGauge(r)
		keeper.SetBTCStakingGauge(ctx, height, gauge)

		// generate a random voting power distribution cache
		dc, err := datagen.GenRandomVotingPowerDistCache(r, 100)
		require.NoError(t, err)

		// create voter map from a random subset of finality providers
		voterMap := make(map[string]struct{})
		for i, fp := range dc.FinalityProviders {
			// randomly decide if this FP voted (skip some FPs)
			if i < int(dc.NumActiveFps) && datagen.OneInN(r, 2) {
				voterMap[fp.BtcPk.MarshalHex()] = struct{}{}
			}
		}

		// expected values
		distributedCoins := sdk.NewCoins()
		fpRewardMap := map[string]sdk.Coins{}     // key: address, value: reward
		btcDelRewardMap := map[string]sdk.Coins{} // key: address, value: reward

		// calculate expected rewards only for FPs who voted
		for _, fp := range dc.FinalityProviders {
			// skip if not active or didn't vote
			if _, ok := voterMap[fp.BtcPk.MarshalHex()]; !ok {
				continue
			}

			fpPortion := dc.GetFinalityProviderPortion(fp)
			coinsForFpsAndDels := gauge.GetCoinsPortion(fpPortion)
			coinsForCommission := types.GetCoinsPortion(coinsForFpsAndDels, *fp.Commission)
			if coinsForCommission.IsAllPositive() {
				fpRewardMap[fp.GetAddress().String()] = coinsForCommission
				distributedCoins = distributedCoins.Add(coinsForCommission...)
			}
			coinsForBTCDels := coinsForFpsAndDels.Sub(coinsForCommission...)
			for _, btcDel := range fp.BtcDels {
				btcDelPortion := fp.GetBTCDelPortion(btcDel)
				coinsForDel := types.GetCoinsPortion(coinsForBTCDels, btcDelPortion)
				if coinsForDel.IsAllPositive() {
					btcDelRewardMap[btcDel.GetAddress().String()] = coinsForDel
					distributedCoins = distributedCoins.Add(coinsForDel...)
				}
			}
		}

		// distribute rewards in the gauge to finality providers/delegations
		keeper.RewardBTCStaking(ctx, height, dc, voterMap)

		// assert consistency between reward map and reward gauge
		for addrStr, reward := range fpRewardMap {
			addr, err := sdk.AccAddressFromBech32(addrStr)
			require.NoError(t, err)
			rg := keeper.GetRewardGauge(ctx, types.FinalityProviderType, addr)
			require.NotNil(t, rg)
			require.Equal(t, reward, rg.Coins)
		}
		for addrStr, reward := range btcDelRewardMap {
			addr, err := sdk.AccAddressFromBech32(addrStr)
			require.NoError(t, err)
			rg := keeper.GetRewardGauge(ctx, types.BTCDelegationType, addr)
			require.NotNil(t, rg)
			require.Equal(t, reward, rg.Coins)
		}

		// assert distributedCoins is a subset of coins in gauge
		require.True(t, gauge.Coins.IsAllGTE(distributedCoins))

		// verify that FPs who didn't vote got no rewards
		for _, fp := range dc.FinalityProviders {
			if _, voted := voterMap[fp.BtcPk.MarshalHex()]; !voted {
				rg := keeper.GetRewardGauge(ctx, types.FinalityProviderType, fp.GetAddress())
				if rg != nil {
					require.True(t, rg.Coins.IsZero(),
						"non-voting FP %s should not receive rewards", fp.GetAddress())
				}

				// their delegators should also not receive rewards
				for _, btcDel := range fp.BtcDels {
					rg := keeper.GetRewardGauge(ctx, types.BTCDelegationType, btcDel.GetAddress())
					if rg != nil {
						require.True(t, rg.Coins.IsZero(),
							"delegator %s of non-voting FP should not receive rewards", btcDel.GetAddress())
					}
				}
			}
		}
	})
}
