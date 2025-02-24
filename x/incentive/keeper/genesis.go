package keeper

import (
	"context"
	"encoding/hex"
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/store/prefix"
	"github.com/babylonlabs-io/babylon/x/incentive/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis performs stateful validations and initializes the keeper state from a provided initial genesis state.
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := sdkCtx.BlockHeight()

	for _, entry := range gs.BtcStakingGauges {
		// check that height is less than current height
		if entry.Height > uint64(height) {
			return fmt.Errorf("BTC staking gauge height (%d) is higher than current block height (%d)", entry.Height, height)
		}

		k.SetBTCStakingGauge(ctx, entry.Height, entry.Gauge)
	}

	for _, entry := range gs.RewardGauges {
		// check that the address exists
		// we can use MustAccAddressFromBech32 safely here because it is validated before
		accAddr := sdk.MustAccAddressFromBech32(entry.Address)
		acc := k.accountKeeper.GetAccount(ctx, accAddr)
		if acc == nil {
			return fmt.Errorf("account in rewards gauge with address %s does not exist", entry.Address)
		}

		k.SetRewardGauge(ctx, entry.StakeholderType, accAddr, entry.RewardGauge)
	}

	for _, entry := range gs.WithdrawAddresses {
		// check that delegator address exists
		delAddr := sdk.MustAccAddressFromBech32(entry.DelegatorAddress)
		acc := k.accountKeeper.GetAccount(ctx, delAddr)
		if acc == nil {
			return fmt.Errorf("delegator account with address %s does not exist", entry.DelegatorAddress)
		}
		withdrawAddr := sdk.MustAccAddressFromBech32(entry.WithdrawAddress)
		if err := k.SetWithdrawAddr(ctx, delAddr, withdrawAddr); err != nil {
			return err
		}
	}

	for _, entry := range gs.RefundableMsgHashes {
		// hashes are hex encoded for better readability
		bz, err := hex.DecodeString(entry)
		if err != nil {
			return fmt.Errorf("error decoding msg hash: %w", err)
		}
		if err := k.RefundableMsgKeySet.Set(ctx, bz); err != nil {
			return fmt.Errorf("error storing msg hash: %w", err)
		}
	}

	for _, entry := range gs.FinalityProvidersCurrentRewards {
		if err := k.FinalityProviderCurrentRewards.Set(
			ctx,
			sdk.MustAccAddressFromBech32(entry.Address).Bytes(),
			*entry.Rewards,
		); err != nil {
			return err
		}
	}

	for _, entry := range gs.FinalityProvidersHistoricalRewards {
		if err := k.FinalityProviderHistoricalRewards.Set(
			ctx,
			collections.Join(
				sdk.MustAccAddressFromBech32(entry.Address).Bytes(),
				entry.Period,
			),
			*entry.Rewards,
		); err != nil {
			return err
		}
	}

	for _, entry := range gs.BtcDelegationRewardsTrackers {
		if err := k.BTCDelegationRewardsTracker.Set(
			ctx,
			collections.Join(
				sdk.MustAccAddressFromBech32(entry.FinalityProviderAddress).Bytes(),
				sdk.MustAccAddressFromBech32(entry.DelegatorAddress).Bytes(),
			),
			*entry.Tracker,
		); err != nil {
			return err
		}
	}

	for _, entry := range gs.BtcDelegatorsToFps {
		k.setBTCDelegatorToFP(
			ctx,
			sdk.MustAccAddressFromBech32(entry.DelegatorAddress),
			sdk.MustAccAddressFromBech32(entry.FinalityProviderAddress),
		)
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

	wa, err := k.withdrawAddresses(ctx)
	if err != nil {
		return nil, err
	}
	return &types.GenesisState{
		Params:            k.GetParams(ctx),
		BtcStakingGauges:  bsg,
		RewardGauges:      rg,
		WithdrawAddresses: wa,
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

// withdrawAddresses loads all withdraw addresses stored.
// This function has high resource consumption and should be only used on export genesis.
func (k Keeper) withdrawAddresses(ctx context.Context) ([]types.WithdrawAddressEntry, error) {
	entries := make([]types.WithdrawAddressEntry, 0)

	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, types.DelegatorWithdrawAddrPrefix)
	iterator := store.Iterator(nil, nil)

	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		key := iterator.Key()
		value := iterator.Value()

		// Extract delegator address from key (strip prefix)
		delAddr := sdk.AccAddress(key[len(types.DelegatorWithdrawAddrPrefix):])

		// Convert stored withdraw address from bytes to sdk.AccAddress
		withdrawAddr := sdk.AccAddress(value)

		entries = append(entries, types.WithdrawAddressEntry{
			DelegatorAddress: delAddr.String(),
			WithdrawAddress:  withdrawAddr.String(),
		})
	}

	return entries, nil
}
