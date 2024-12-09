package keeper_test

import (
	"math/rand"
	"testing"

	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	testkeeper "github.com/babylonlabs-io/babylon/testutil/keeper"
	"github.com/babylonlabs-io/babylon/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func FuzzRewardBTCStaking(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
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

		// expected values
		distributedCoins := sdk.NewCoins()
		fpRewardMap := map[string]sdk.Coins{}     // key: address, value: reward
		btcDelRewardMap := map[string]sdk.Coins{} // key: address, value: reward

		sumCoinsForDels := sdk.NewCoins()
		for _, fp := range dc.FinalityProviders {
			fpPortion := dc.GetFinalityProviderPortion(fp)
			coinsForFpsAndDels := gauge.GetCoinsPortion(fpPortion)
			coinsForCommission := types.GetCoinsPortion(coinsForFpsAndDels, *fp.Commission)
			if coinsForCommission.IsAllPositive() {
				fpRewardMap[fp.GetAddress().String()] = coinsForCommission
				distributedCoins.Add(coinsForCommission...)
			}

			coinsForBTCDels := coinsForFpsAndDels.Sub(coinsForCommission...)
			sumCoinsForDels = sumCoinsForDels.Add(coinsForBTCDels...)
			fpAddr := fp.GetAddress()

			for _, btcDel := range fp.BtcDels {
				err := keeper.BtcDelegationActivated(ctx, fpAddr, btcDel.GetAddress(), btcDel.TotalSat)
				require.NoError(t, err)

				btcDelPortion := fp.GetBTCDelPortion(btcDel)
				coinsForDel := types.GetCoinsPortion(coinsForBTCDels, btcDelPortion)
				if coinsForDel.IsAllPositive() {
					btcDelRewardMap[btcDel.GetAddress().String()] = coinsForDel
					distributedCoins.Add(coinsForDel...)
				}
			}
		}

		// distribute rewards in the gauge to finality providers/delegations
		keeper.RewardBTCStaking(ctx, height, dc)

		for _, fp := range dc.FinalityProviders {
			fpAddr := fp.GetAddress()
			for _, btcDel := range fp.BtcDels {
				delAddr := btcDel.GetAddress()
				delRwd, err := keeper.GetBTCDelegationRewardsTracker(ctx, fpAddr, delAddr)
				require.NoError(t, err)
				require.Equal(t, delRwd.TotalActiveSat.Uint64(), btcDel.TotalSat)

				// makes sure the rewards added reach the delegation gauge
				err = keeper.SendBtcDelegationRewardsToGauge(ctx, fpAddr, delAddr)
				require.NoError(t, err)
			}
			fpCurrentRwd, err := keeper.GetFinalityProviderCurrentRewards(ctx, fpAddr)
			require.NoError(t, err)
			require.Equal(t, fpCurrentRwd.TotalActiveSat.Uint64(), fp.TotalBondedSat)
		}

		// assert consistency between reward map and reward gauge
		for addrStr, reward := range fpRewardMap {
			addr, err := sdk.AccAddressFromBech32(addrStr)
			require.NoError(t, err)
			rg := keeper.GetRewardGauge(ctx, types.FinalityProviderType, addr)
			require.NotNil(t, rg)
			require.Equal(t, reward, rg.Coins)
		}

		sumRewards := sdk.NewCoins()
		for addrStr, reward := range btcDelRewardMap {
			addr, err := sdk.AccAddressFromBech32(addrStr)
			require.NoError(t, err)
			rg := keeper.GetRewardGauge(ctx, types.BTCDelegationType, addr)
			require.NotNil(t, rg)

			// A little bit of rewards could be lost in the process due to precision points
			// so 0.1% difference can be considered okay
			allowedMarginError := CalculatePointOnePercent(reward)
			require.Truef(t, reward.Sub(rg.Coins...).IsAllLT(allowedMarginError),
				"BTC delegation failed within the margin of error: %s\nRewards: %s\nGauge: %s",
				allowedMarginError.String(), reward.String(), rg.Coins.String(),
			)

			sumRewards = sumRewards.Add(reward...)
		}

		allowedMarginError := CalculatePointOnePercent(sumCoinsForDels)
		diff, _ := sumCoinsForDels.SafeSub(sumRewards...)
		require.Truef(t, diff.IsAllLT(allowedMarginError),
			"Sum of total rewards failed within the margin of error: %s\nRewards: %s\nGauge: %s",
			allowedMarginError.String(), sumCoinsForDels.String(), sumRewards.String(),
		)

		// assert distributedCoins is a subset of coins in gauge
		require.True(t, gauge.Coins.IsAllGTE(distributedCoins))
	})
}

func CalculatePointOnePercent(value sdk.Coins) sdk.Coins {
	numerator := math.NewInt(1)      // 0.1% as numerator
	denominator := math.NewInt(1000) // 0.1% denominator
	result := value.MulInt(numerator).QuoInt(denominator)
	return result
}
