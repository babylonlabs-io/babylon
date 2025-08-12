package keeper

import (
	"context"
	"encoding/hex"
	"fmt"

	"cosmossdk.io/store/prefix"
	"github.com/babylonlabs-io/babylon/v3/x/incentive/types"
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
		// check that fp address exists
		fpAddr := sdk.MustAccAddressFromBech32(entry.Address)
		acc := k.accountKeeper.GetAccount(ctx, fpAddr)
		if acc == nil {
			return fmt.Errorf("finality provider account with address %s does not exist", entry.Address)
		}
		if err := k.SetFinalityProviderCurrentRewards(ctx, fpAddr, *entry.Rewards); err != nil {
			return err
		}
	}

	for _, entry := range gs.FinalityProvidersHistoricalRewards {
		// check that fp address exists
		fpAddr := sdk.MustAccAddressFromBech32(entry.Address)
		acc := k.accountKeeper.GetAccount(ctx, fpAddr)
		if acc == nil {
			return fmt.Errorf("finality provider account with address %s does not exist", entry.Address)
		}
		if err := k.setFinalityProviderHistoricalRewards(ctx, fpAddr, entry.Period, *entry.Rewards); err != nil {
			return err
		}
	}

	for _, entry := range gs.BtcDelegationRewardsTrackers {
		// check that fp and delegator accounts exist
		fpAddr := sdk.MustAccAddressFromBech32(entry.FinalityProviderAddress)
		acc := k.accountKeeper.GetAccount(ctx, fpAddr)
		if acc == nil {
			return fmt.Errorf("finality provider account with address %s does not exist", entry.FinalityProviderAddress)
		}
		delAddr := sdk.MustAccAddressFromBech32(entry.DelegatorAddress)
		acc = k.accountKeeper.GetAccount(ctx, delAddr)
		if acc == nil {
			return fmt.Errorf("delegator account with address %s does not exist", entry.DelegatorAddress)
		}
		// This function also calls setBTCDelegatorToFP() which stores the BTC delegator to FPs mapping.
		// Additionally, the GenesisState.Validate() function checks that the delegator <> FP relationships
		// match between the entries in BtcDelegationRewardsTrackers and BtcDelegatorsToFps.
		// So this function call, stores all the corresponding BTCDelegationRewardsTrackers and the BTCDelegatorToFPs
		if err := k.setBTCDelegationRewardsTracker(ctx, fpAddr, delAddr, *entry.Tracker); err != nil {
			return err
		}
	}

	for _, entry := range gs.EventRewardTracker {
		if err := k.SetRewardTrackerEvent(ctx, entry.Height, entry.Events); err != nil {
			return fmt.Errorf("failed to set the reward tracker events to height: %d: %w", entry.Height, err)
		}
	}

	// make sure LastProcessedHeightEventRewardTracker <= current height
	currentHeight := sdk.UnwrapSDKContext(ctx).BlockHeader().Height
	if gs.LastProcessedHeightEventRewardTracker > uint64(currentHeight) {
		return fmt.Errorf("invalid latest processed block height. Value %d is higher than current block height %d", gs.LastProcessedHeightEventRewardTracker, currentHeight)
	}

	if err := k.SetRewardTrackerEventLastProcessedHeight(ctx, gs.LastProcessedHeightEventRewardTracker); err != nil {
		return fmt.Errorf("failed to set the latest processed block height: %d: %w", gs.LastProcessedHeightEventRewardTracker, err)
	}

	// NOTE: no need to store the entries on gs.BtcDelegatorsToFps because these are stored with the setBTCDelegationRewardsTracker
	// call in the lines above

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

	rmh, err := k.refundableMsgHashes(ctx)
	if err != nil {
		return nil, err
	}

	fpCurrentRwd, err := k.finalityProvidersCurrentRewards(ctx)
	if err != nil {
		return nil, err
	}

	fpHistRwd, err := k.finalityProvidersHistoricalRewards(ctx)
	if err != nil {
		return nil, err
	}

	bdrt, err := k.btcDelegationRewardsTrackers(ctx)
	if err != nil {
		return nil, err
	}

	d2fp, err := k.btcDelegatorsToFps(ctx)
	if err != nil {
		return nil, err
	}

	evtsRwdTracker, err := k.rewardTrackerEventsEntry(ctx)
	if err != nil {
		return nil, err
	}

	lastProcessedBlkHeightEvtsRwdTracker, err := k.GetRewardTrackerEventLastProcessedHeight(ctx)
	if err != nil {
		return nil, err
	}

	return &types.GenesisState{
		Params:                                k.GetParams(ctx),
		BtcStakingGauges:                      bsg,
		RewardGauges:                          rg,
		WithdrawAddresses:                     wa,
		RefundableMsgHashes:                   rmh,
		FinalityProvidersCurrentRewards:       fpCurrentRwd,
		FinalityProvidersHistoricalRewards:    fpHistRwd,
		BtcDelegationRewardsTrackers:          bdrt,
		BtcDelegatorsToFps:                    d2fp,
		EventRewardTracker:                    evtsRwdTracker,
		LastProcessedHeightEventRewardTracker: lastProcessedBlkHeightEvtsRwdTracker,
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

// refundableMsgHashes loads all refundable msg hashes stored.
// It encodes the hashes as hex strings to be human readable on exporting the genesis
// This function has high resource consumption and should be only used on export genesis.
func (k Keeper) refundableMsgHashes(ctx context.Context) ([]string, error) {
	hashes := make([]string, 0)
	iterator, err := k.RefundableMsgKeySet.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		key, err := iterator.Key()
		if err != nil {
			return nil, err
		}

		// encode hash as a hex string
		hashStr := hex.EncodeToString(key)
		hashes = append(hashes, hashStr)
	}

	return hashes, nil
}

// finalityProvidersCurrentRewards loads all finality providers current rewards stored.
// This function has high resource consumption and should be only used on export genesis.
func (k Keeper) finalityProvidersCurrentRewards(ctx context.Context) ([]types.FinalityProviderCurrentRewardsEntry, error) {
	entries := make([]types.FinalityProviderCurrentRewardsEntry, 0)

	iter, err := k.finalityProviderCurrentRewards.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		key, err := iter.Key()
		if err != nil {
			return nil, err
		}
		addr := sdk.AccAddress(key)
		currRwd, err := iter.Value()
		if err != nil {
			return nil, err
		}
		entry := types.FinalityProviderCurrentRewardsEntry{
			Address: addr.String(),
			Rewards: &currRwd,
		}
		if err := entry.Validate(); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// finalityProvidersHistoricalRewards loads all finality providers historical rewards stored.
// This function has high resource consumption and should be only used on export genesis.
func (k Keeper) finalityProvidersHistoricalRewards(ctx context.Context) ([]types.FinalityProviderHistoricalRewardsEntry, error) {
	entries := make([]types.FinalityProviderHistoricalRewardsEntry, 0)

	iter, err := k.finalityProviderHistoricalRewards.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		key, err := iter.Key()
		if err != nil {
			return nil, err
		}
		addr := sdk.AccAddress(key.K1())
		period := key.K2()

		histRwd, err := iter.Value()
		if err != nil {
			return nil, err
		}
		entry := types.FinalityProviderHistoricalRewardsEntry{
			Address: addr.String(),
			Period:  period,
			Rewards: &histRwd,
		}
		if err := entry.Validate(); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// btcDelegationRewardsTrackers loads all BTC delegation rewards trackers stored.
// This function has high resource consumption and should be only used on export genesis.
func (k Keeper) btcDelegationRewardsTrackers(ctx context.Context) ([]types.BTCDelegationRewardsTrackerEntry, error) {
	entries := make([]types.BTCDelegationRewardsTrackerEntry, 0)

	iter, err := k.btcDelegationRewardsTracker.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		key, err := iter.Key()
		if err != nil {
			return nil, err
		}
		fpAddr := sdk.AccAddress(key.K1())
		delAddr := sdk.AccAddress(key.K2())
		tracker, err := iter.Value()
		if err != nil {
			return nil, err
		}
		entry := types.BTCDelegationRewardsTrackerEntry{
			FinalityProviderAddress: fpAddr.String(),
			DelegatorAddress:        delAddr.String(),
			Tracker:                 &tracker,
		}
		if err := entry.Validate(); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// btcDelegatorsToFps loads all BTC delegators to finality providers relationships stored.
// This function has high resource consumption and should be only used on export genesis.
func (k Keeper) btcDelegatorsToFps(ctx context.Context) ([]types.BTCDelegatorToFpEntry, error) {
	// address length is 20 bytes
	addrLen := 20
	entries := make([]types.BTCDelegatorToFpEntry, 0)
	storeAdaptor := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdaptor, types.BTCDelegatorToFPKey)
	iter := store.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) < addrLen*2 { // key is composed of delAddr + fpAddr
			return nil, fmt.Errorf("invalid key length: %d", len(key))
		}

		delAddr := sdk.AccAddress(key[:addrLen]) // First 20 bytes for delegator
		fpAddr := sdk.AccAddress(key[addrLen:])  // Remaining 20 bytes for finality provider

		entry := types.BTCDelegatorToFpEntry{
			DelegatorAddress:        delAddr.String(),
			FinalityProviderAddress: fpAddr.String(),
		}

		if err := entry.Validate(); err != nil {
			return nil, err
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

func (k Keeper) rewardTrackerEventsEntry(ctx context.Context) ([]types.EventsPowerUpdateAtHeightEntry, error) {
	entries := make([]types.EventsPowerUpdateAtHeightEntry, 0)

	iter, err := k.rewardTrackerEvents.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		height, err := iter.Key()
		if err != nil {
			return nil, err
		}
		v, err := iter.Value()
		if err != nil {
			return nil, err
		}
		entry := types.EventsPowerUpdateAtHeightEntry{
			Height: height,
			Events: &v,
		}
		if err := entry.Validate(); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}
