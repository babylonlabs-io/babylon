package keeper_test

import (
	"math/rand"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/testutil/coins"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	testkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
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

		k, ctx := testkeeper.IncentiveKeeper(t, bankKeeper, nil, nil, nil)
		height := datagen.RandomInt(r, 1000)
		ctx = datagen.WithCtxHeight(ctx, height)

		var btcStkGauge *types.Gauge
		var fpDirectGauge *types.Gauge
		if height%2 == 0 {
			// Case 1: use a randomly generated gauge and store it via keeper
			btcStkGauge = datagen.GenRandomGauge(r)
			k.SetBTCStakingGauge(ctx, height, btcStkGauge)
		} else {
			// Case 2: no gauge is stored - no fees to intercept
			btcStkGauge = types.NewGauge(sdk.NewCoins()...)
			g := k.GetBTCStakingGauge(ctx, height)
			require.Nil(t, g)
		}

		// Sometimes add FP direct rewards gauge (30% chance)
		if r.Int()%10 < 3 {
			fpDirectGauge = datagen.GenRandomGauge(r)
			k.SetFPDirectGauge(ctx, height, fpDirectGauge)
		} else {
			fpDirectGauge = types.NewGauge(sdk.NewCoins()...)
		}

		// generate a random voting power distribution cache
		dc, btcTotalSatByDelAddressByFpAddress, err := datagen.GenRandomVotingPowerDistCache(r, 100)
		require.NoError(t, err)

		// randomly select some FPs as voters
		voters := make(map[string]struct{})
		totalVotingPowerOfVoters := uint64(0)
		for i, fp := range dc.FinalityProviders {
			if i >= int(dc.NumActiveFps) {
				break
			}
			// 50% chance of being a voter
			if r.Int()%2 == 0 {
				voters[fp.BtcPk.MarshalHex()] = struct{}{}
				totalVotingPowerOfVoters += fp.TotalBondedSat
			}
		}
		// ensure at least one voter
		if len(voters) == 0 && dc.NumActiveFps > 0 {
			fp := dc.FinalityProviders[0]
			voters[fp.BtcPk.MarshalHex()] = struct{}{}
			totalVotingPowerOfVoters = fp.TotalBondedSat
		}

		// expected values
		distributedCoins := sdk.NewCoins()
		fpRewardMap := map[string]sdk.Coins{}     // key: address, value: reward
		btcDelRewardMap := map[string]sdk.Coins{} // key: address, value: reward

		sumCoinsForDels := sdk.NewCoins()
		for i, fp := range dc.FinalityProviders {
			if i >= int(dc.NumActiveFps) {
				break
			}

			// Skip non-voters
			if _, isVoter := voters[fp.BtcPk.MarshalHex()]; !isVoter {
				continue
			}

			// Calculate portion based on total voting power of voters only
			fpPortion := sdkmath.LegacyNewDec(int64(fp.TotalBondedSat)).
				QuoTruncate(sdkmath.LegacyNewDec(int64(totalVotingPowerOfVoters)))
			coinsForFpsAndDels := btcStkGauge.GetCoinsPortion(fpPortion)
			coinsForCommission := types.GetCoinsPortion(coinsForFpsAndDels, *fp.Commission)

			// Add FP direct rewards from fee collector (goes entirely to FP)
			coinsForFpDirect := fpDirectGauge.GetCoinsPortion(fpPortion)
			totalCoinsForFp := coinsForCommission.Add(coinsForFpDirect...)

			if totalCoinsForFp.IsAllPositive() {
				fpRewardMap[fp.GetAddress().String()] = totalCoinsForFp
				distributedCoins.Add(totalCoinsForFp...)
			}

			coinsForBTCDels := coinsForFpsAndDels.Sub(coinsForCommission...)
			sumCoinsForDels = sumCoinsForDels.Add(coinsForBTCDels...)
			fpAddr := fp.GetAddress()

			for delAddrStr, delSat := range btcTotalSatByDelAddressByFpAddress[fpAddr.String()] {
				btcDelAddr := sdk.MustAccAddressFromBech32(delAddrStr)

				err := k.BtcDelegationActivated(ctx, fpAddr, btcDelAddr, sdkmath.NewIntFromUint64(delSat))
				require.NoError(t, err)

				btcDelPortion := fp.GetBTCDelPortion(delSat)
				coinsForDel := types.GetCoinsPortion(coinsForBTCDels, btcDelPortion)
				if coinsForDel.IsAllPositive() {
					btcDelRewardMap[delAddrStr] = coinsForDel
					distributedCoins.Add(coinsForDel...)
				}
			}
		}

		// distribute rewards in the gauge to finality providers/delegations
		k.RewardBTCStaking(ctx, height, dc, voters)

		for _, fp := range dc.FinalityProviders {
			_, exists := voters[fp.BtcPk.MarshalHex()]
			if exists {
				fpAddr := fp.GetAddress()
				for delAddrStr, delSat := range btcTotalSatByDelAddressByFpAddress[fpAddr.String()] {
					delAddr := sdk.MustAccAddressFromBech32(delAddrStr)
					delRwd, err := k.GetBTCDelegationRewardsTracker(ctx, fpAddr, delAddr)
					require.NoError(t, err)
					require.Equal(t, delRwd.TotalActiveSat.Uint64(), delSat)

					// makes sure the rewards added reach the delegation gauge
					err = k.BtcDelegationActivated(ctx, fpAddr, delAddr, sdkmath.ZeroInt())
					require.NoError(t, err)
					fpCurrentRwd, err := k.GetFinalityProviderCurrentRewards(ctx, fpAddr)
					require.NoError(t, err)
					require.Equal(t, fpCurrentRwd.TotalActiveSat.Uint64(), fp.TotalBondedSat)
				}
			}
		}

		// assert consistency between reward map and reward gauge
		for addrStr, reward := range fpRewardMap {
			addr, err := sdk.AccAddressFromBech32(addrStr)
			require.NoError(t, err)
			rg := k.GetRewardGauge(ctx, types.FINALITY_PROVIDER, addr)
			require.NotNil(t, rg)
			require.Equal(t, reward, rg.Coins)
		}

		sumRewards := sdk.NewCoins()
		for addrStr, reward := range btcDelRewardMap {
			addr, err := sdk.AccAddressFromBech32(addrStr)
			require.NoError(t, err)
			rg := k.GetRewardGauge(ctx, types.BTC_STAKER, addr)
			require.NotNil(t, rg)

			// A little bit of rewards could be lost in the process due to precision points
			// so 0.1% difference can be considered okay
			require.Truef(t, coins.CoinsDiffInPointOnePercentMargin(reward, rg.Coins),
				"BTC delegation failed within the margin of error 0.1%\nRewards: %s\nGauge: %s",
				reward.String(), rg.Coins.String(),
			)

			sumRewards = sumRewards.Add(reward...)
		}

		require.Truef(t, coins.CoinsDiffInPointOnePercentMargin(sumCoinsForDels, sumRewards),
			"Sum of total rewards failed within the margin of error 0.1%\nRewards: %s\nGauge: %s",
			sumCoinsForDels.String(), sumRewards.String(),
		)

		// assert distributedCoins is a subset of coins in both gauges combined
		totalGaugeCoins := btcStkGauge.Coins.Add(fpDirectGauge.Coins...)
		require.True(t, totalGaugeCoins.IsAllGTE(distributedCoins))

		// Additional assertions for non-voters
		for i, fp := range dc.FinalityProviders {
			if i >= int(dc.NumActiveFps) {
				break
			}

			fpAddr := fp.GetAddress()
			if _, isVoter := voters[fp.BtcPk.MarshalHex()]; !isVoter {
				// Check that non-voters received no rewards
				rg := k.GetRewardGauge(ctx, types.FINALITY_PROVIDER, fpAddr)
				require.Nil(t, rg)

				// Check their delegators received no rewards
				for delAddrStr := range btcTotalSatByDelAddressByFpAddress[fpAddr.String()] {
					delAddr := sdk.MustAccAddressFromBech32(delAddrStr)
					rg := k.GetRewardGauge(ctx, types.BTC_STAKER, delAddr)
					require.Nil(t, rg)
				}
			}
		}
	})
}
