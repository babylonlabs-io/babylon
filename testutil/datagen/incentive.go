package datagen

import (
	"math/rand"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	btcctypes "github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/types"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
	itypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

const (
	characters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	denomLen   = 5
)

func GenRandomDenom(r *rand.Rand) string {
	var result string
	// Generate the random string
	for i := 0; i < denomLen; i++ {
		// Generate a random index within the range of the character set
		index := r.Intn(len(characters))
		// Add the randomly selected character to the result
		result += string(characters[index])
	}
	return result
}

func GenRandomStakeholderType(r *rand.Rand) itypes.StakeholderType {
	stBytes := []byte{byte(RandomInt(r, 2))}
	st, err := itypes.NewStakeHolderType(stBytes)
	if err != nil {
		panic(err) // only programming error is possible
	}
	return st
}

func GenRandomCoins(r *rand.Rand) sdk.Coins {
	numCoins := r.Int31n(10) + 10
	coins := sdk.NewCoins()
	for i := int32(0); i < numCoins; i++ {
		demon := GenRandomDenom(r)
		amount := r.Int63n(10_000000) + 10_000000
		coin := sdk.NewInt64Coin(demon, amount)
		coins = coins.Add(coin)
	}
	return coins
}

func GenRandomRewardGauge(r *rand.Rand) *itypes.RewardGauge {
	coins := GenRandomCoins(r)
	return itypes.NewRewardGauge(coins...)
}

func GenRandomWithdrawnCoins(r *rand.Rand, coins sdk.Coins) sdk.Coins {
	withdrawnCoins := sdk.NewCoins()
	for _, coin := range coins {
		// skip this coin with some probability
		if OneInN(r, 3) {
			continue
		}
		// a subset of the coin has been withdrawn
		withdrawnAmount := coin.Amount.Uint64()
		if withdrawnAmount > 1 {
			withdrawnAmount = RandomInt(r, int(withdrawnAmount)-1) + 1
		}
		withdrawnCoin := sdk.NewCoin(coin.Denom, sdkmath.NewIntFromUint64(withdrawnAmount))
		withdrawnCoins = withdrawnCoins.Add(withdrawnCoin)
	}
	return withdrawnCoins
}

func GenRandomGauge(r *rand.Rand) *itypes.Gauge {
	coins := GenRandomCoins(r)
	return itypes.NewGauge(coins...)
}

func GenRandomAddrAndSat(r *rand.Rand) (string, uint64) {
	return GenRandomAccount().Address, RandomInt(r, 1000) + 1
}

func GenRandomFinalityProviderDistInfo(r *rand.Rand) (
	fpDistInfo *ftypes.FinalityProviderDistInfo,
	btcTotalSatByAddress map[string]uint64,
	err error,
) {
	// create finality provider with random commission
	fp, err := GenRandomFinalityProvider(r)
	if err != nil {
		return nil, nil, err
	}
	// create finality provider distribution info
	fpDistInfo = ftypes.NewFinalityProviderDistInfo(fp)
	// add a random number of BTC delegation distribution info
	numBTCDels := RandomInt(r, 100) + 1
	btcTotalSatByAddress = make(map[string]uint64, numBTCDels)
	for i := uint64(0); i < numBTCDels; i++ {
		btcAddr, totalSat := GenRandomAddrAndSat(r)
		btcTotalSatByAddress[btcAddr] += totalSat
		fpDistInfo.TotalBondedSat += totalSat
		fpDistInfo.IsTimestamped = true
	}
	return fpDistInfo, btcTotalSatByAddress, nil
}

func GenRandomVotingPowerDistCache(r *rand.Rand, maxFPs uint32) (
	dc *ftypes.VotingPowerDistCache,
	// fpAddr => delAddr => totalSat
	btcTotalSatByDelAddressByFpAddress map[string]map[string]uint64,
	err error,
) {
	dc = ftypes.NewVotingPowerDistCache()
	// a random number of finality providers
	numFps := RandomInt(r, 10) + 1

	btcTotalSatByDelAddressByFpAddress = make(map[string]map[string]uint64, numFps)
	for i := uint64(0); i < numFps; i++ {
		v, btcTotalSatByAddress, err := GenRandomFinalityProviderDistInfo(r)
		if err != nil {
			return nil, nil, err
		}
		btcTotalSatByDelAddressByFpAddress[v.GetAddress().String()] = btcTotalSatByAddress
		dc.AddFinalityProviderDistInfo(v)
	}
	dc.ApplyActiveFinalityProviders(maxFPs)
	return dc, btcTotalSatByDelAddressByFpAddress, nil
}

func GenRandomCheckpointAddressPair(r *rand.Rand) *btcctypes.CheckpointAddressPair {
	return &btcctypes.CheckpointAddressPair{
		Submitter: GenRandomAccount().GetAddress(),
		Reporter:  GenRandomAccount().GetAddress(),
	}
}

func GenRandomBTCTimestampingRewardDistInfo(r *rand.Rand) *btcctypes.RewardDistInfo {
	best := GenRandomCheckpointAddressPair(r)
	numOthers := RandomInt(r, 10)
	others := []*btcctypes.CheckpointAddressPair{}
	for i := uint64(0); i < numOthers; i++ {
		others = append(others, GenRandomCheckpointAddressPair(r))
	}
	return btcctypes.NewRewardDistInfo(best, others...)
}

func GenRandomFinalityProviderCurrentRewards(r *rand.Rand) itypes.FinalityProviderCurrentRewards {
	rwd := GenRandomCoins(r)
	period := RandomInt(r, 100) + 3
	activeSatoshi := RandomMathInt(r, 10000).AddRaw(10)
	return itypes.NewFinalityProviderCurrentRewards(rwd, period, activeSatoshi)
}

func GenRandomBTCDelegationRewardsTracker(r *rand.Rand) itypes.BTCDelegationRewardsTracker {
	period := RandomInt(r, 100) + 2
	activeSatoshi := RandomMathInt(r, 10000).Add(sdkmath.NewInt(100))
	return itypes.NewBTCDelegationRewardsTracker(period, activeSatoshi)
}

func GenRandomFPHistRwd(r *rand.Rand) itypes.FinalityProviderHistoricalRewards {
	rwd := GenRandomCoins(r)
	return itypes.NewFinalityProviderHistoricalRewards(rwd)
}

func GenRandomFPHistRwdWithDecimals(r *rand.Rand) itypes.FinalityProviderHistoricalRewards {
	rwd := GenRandomFPHistRwd(r)
	rwd.CumulativeRewardsPerSat = rwd.CumulativeRewardsPerSat.MulInt(itypes.DecimalRewards)
	return rwd
}

func GenRandomFPHistRwdStartAndEnd(r *rand.Rand) (start, end itypes.FinalityProviderHistoricalRewards) {
	start = GenRandomFPHistRwdWithDecimals(r)
	end = GenRandomFPHistRwdWithDecimals(r)
	end.CumulativeRewardsPerSat = end.CumulativeRewardsPerSat.Add(start.CumulativeRewardsPerSat...)
	return start, end
}
