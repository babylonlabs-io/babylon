package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis initializes the keeper state from a provided initial genesis state.
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	for _, entry := range gs.BtcStakingGauges {
		if err := entry.Validate(); err != nil {
			return err
		}
		// TODO check that height is less than current height

		k.SetBTCStakingGauge(ctx, entry.Height, entry.Gauge)
	}

	for _, entry := range gs.RewardGauges {
		if err := entry.Validate(); err != nil {
			return err
		}
		// TODO check that the address exists

		// we can use MustAccAddressFromBech32 safely here because it is validated before
		k.SetRewardGauge(ctx, entry.StakeholderType, sdk.MustAccAddressFromBech32(entry.Address), entry.RewardGauge)
	}

	return k.SetParams(ctx, gs.Params)
}

// ExportGenesis returns the keeper state into a exported genesis state.
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	bsg, err := k.btcStakingGauges(ctx)
	if err != nil {
		return nil, err
	}
	rg, err := k.rewardGauges(ctx)
	if err != nil {
		return nil, err
	}
	return &types.GenesisState{
		Params:           k.GetParams(ctx),
		BtcStakingGauges: bsg,
		RewardGauges:     rg,
	}, nil
}

// btcStakingGauges loads all BTC staking gauges stored.
// This function has high resource consumption and should be only used on export genesis.
func (k Keeper) btcStakingGauges(ctx context.Context) ([]types.BTCStakingGaugeEntry, error) {
	entries := make([]types.BTCStakingGaugeEntry, 0)

	iter := k.btcStakingGaugeStore(ctx).Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var gauge types.Gauge
		if err := k.cdc.Unmarshal(iter.Value(), &gauge); err != nil {
			return nil, err
		}
		height := sdk.BigEndianToUint64(iter.Key())
		entry := types.BTCStakingGaugeEntry{
			Height: height,
			Gauge:  &gauge,
		}
		if err := entry.Validate(); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// rewardGauges loads all reward gauges stored.
// This function has high resource consumption and should be only used on export genesis.
func (k Keeper) rewardGauges(ctx context.Context) ([]types.RewardGaugeEntry, error) {
	entries := make([]types.RewardGaugeEntry, 0)

	for _, st := range types.GetAllStakeholderTypes() {
		iter := k.rewardGaugeStore(ctx, st).Iterator(nil, nil)
		defer iter.Close()

		for ; iter.Valid(); iter.Next() {
			var gauge types.RewardGauge
			if err := k.cdc.Unmarshal(iter.Value(), &gauge); err != nil {
				return nil, err
			}
			if gauge.WithdrawnCoins == nil {
				gauge.WithdrawnCoins = sdk.NewCoins()
			}
			addr := sdk.AccAddress(iter.Key())
			entry := types.RewardGaugeEntry{
				StakeholderType: st,
				Address:         addr.String(),
				RewardGauge:     &gauge,
			}
			if err := entry.Validate(); err != nil {
				return nil, err
			}
			entries = append(entries, entry)
		}
	}

	return entries, nil
}
