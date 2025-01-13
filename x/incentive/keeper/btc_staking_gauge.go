package keeper

import (
	"context"
	"fmt"

	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	ftypes "github.com/babylonlabs-io/babylon/x/finality/types"
	"github.com/babylonlabs-io/babylon/x/incentive/types"
)

// RewardBTCStaking distributes rewards to finality providers/delegations at a given height according
// to the filtered reward distribution cache (that only contains voted finality providers)
// (adapted from https://github.com/cosmos/cosmos-sdk/blob/release/v0.47.x/x/distribution/keeper/allocation.go#L12-L64)
func (k Keeper) RewardBTCStaking(ctx context.Context, height uint64, dc *ftypes.VotingPowerDistCache, voters map[string]struct{}) {
	gauge := k.GetBTCStakingGauge(ctx, height)
	if gauge == nil {
		// failing to get a reward gauge at previous height is a programming error
		panic("failed to get a reward gauge at previous height")
	}

	// calculate total voting power of voters
	var totalVotingPowerOfVoters uint64
	for i, fp := range dc.FinalityProviders {
		if i >= int(dc.NumActiveFps) {
			break
		}
		if _, ok := voters[fp.BtcPk.MarshalHex()]; ok {
			totalVotingPowerOfVoters += fp.TotalBondedSat
		}
	}

	// distribute rewards according to voting power portions for voters
	for i, fp := range dc.FinalityProviders {
		if i >= int(dc.NumActiveFps) {
			break
		}

		if _, ok := voters[fp.BtcPk.MarshalHex()]; !ok {
			continue
		}

		// calculate the portion of a finality provider's voting power out of the total voting power of the voters
		fpPortion := sdkmath.LegacyNewDec(int64(fp.TotalBondedSat)).
			QuoTruncate(sdkmath.LegacyNewDec(int64(totalVotingPowerOfVoters)))
		coinsForFpsAndDels := gauge.GetCoinsPortion(fpPortion)

		// reward the finality provider with commission
		coinsForCommission := types.GetCoinsPortion(coinsForFpsAndDels, *fp.Commission)
		k.accumulateRewardGauge(ctx, types.FinalityProviderType, fp.GetAddress(), coinsForCommission)

		// reward the rest of coins to each BTC delegation proportional to its voting power portion
		coinsForBTCDels := coinsForFpsAndDels.Sub(coinsForCommission...)
		if err := k.AddFinalityProviderRewardsForBtcDelegations(ctx, fp.GetAddress(), coinsForBTCDels); err != nil {
			panic(fmt.Errorf("failed to add fp rewards for btc delegation %s at height %d: %w", fp.GetAddress().String(), height, err))
		}
	}
	// TODO: prune unnecessary state (delete BTCStakingGauge after the amount is used)
}

func (k Keeper) accumulateBTCStakingReward(ctx context.Context, btcStakingReward sdk.Coins) {
	// update BTC staking gauge
	height := uint64(sdk.UnwrapSDKContext(ctx).HeaderInfo().Height)
	gauge := types.NewGauge(btcStakingReward...)
	k.SetBTCStakingGauge(ctx, height, gauge)

	// transfer the BTC staking reward from fee collector account to incentive module account
	err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, k.feeCollectorName, types.ModuleName, btcStakingReward)
	if err != nil {
		// this can only be programming error and is unrecoverable
		panic(err)
	}
}

func (k Keeper) SetBTCStakingGauge(ctx context.Context, height uint64, gauge *types.Gauge) {
	store := k.btcStakingGaugeStore(ctx)
	gaugeBytes := k.cdc.MustMarshal(gauge)
	store.Set(sdk.Uint64ToBigEndian(height), gaugeBytes)
}

func (k Keeper) GetBTCStakingGauge(ctx context.Context, height uint64) *types.Gauge {
	store := k.btcStakingGaugeStore(ctx)
	gaugeBytes := store.Get(sdk.Uint64ToBigEndian(height))
	if gaugeBytes == nil {
		return nil
	}

	var gauge types.Gauge
	k.cdc.MustUnmarshal(gaugeBytes, &gauge)
	return &gauge
}

// btcStakingGaugeStore returns the KVStore of the gauge of total reward for
// BTC staking at each height
// prefix: BTCStakingGaugeKey
// key: gauge height
// value: gauge of rewards for BTC staking at this height
func (k Keeper) btcStakingGaugeStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.BTCStakingGaugeKey)
}
