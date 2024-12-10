package keeper

import (
	"context"
	"encoding/binary"
	"errors"

	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	"github.com/babylonlabs-io/babylon/x/incentive/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// storeBTCDelegatorToFp returns the KVStore of the mapping del => fp
// note: it stores the finality provider as key and sets a one byte as value
// so each BTC delegator address can have multiple finality providers.
// Useful to iterate over all the pairs (fp,del) by filtering the
// delegator address.
// prefix: BTCDelegatorToFPKey
// key: (DelAddr, FpAddr)
// value: 0x00
func (k Keeper) storeBTCDelegatorToFp(ctx context.Context, del sdk.AccAddress) prefix.Store {
	storeAdaptor := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	st := prefix.NewStore(storeAdaptor, types.BTCDelegatorToFPKey)
	return prefix.NewStore(st, del.Bytes())
}

// storeBTCDelegationRewardsTracker returns the KVStore of the FP current rewards
// prefix: BTCDelegationRewardsTrackerKey
// key: (FpAddr, DelAddr)
// value: BTCDelegationRewardsTracker
func (k Keeper) storeBTCDelegationRewardsTracker(ctx context.Context, fp sdk.AccAddress) prefix.Store {
	storeAdaptor := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	st := prefix.NewStore(storeAdaptor, types.BTCDelegationRewardsTrackerKey)
	return prefix.NewStore(st, fp.Bytes())
}

// storeFpCurrentRewards returns the KVStore of the FP current rewards
// prefix: FinalityProviderCurrentRewardsKey
// key: (finality provider cosmos address)
// value: FinalityProviderCurrentRewards
func (k Keeper) storeFpCurrentRewards(ctx context.Context) prefix.Store {
	storeAdaptor := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdaptor, types.FinalityProviderCurrentRewardsKey)
}

// storeFpHistoricalRewards returns the KVStore of the FP historical rewards
// prefix: FinalityProviderHistoricalRewardsKey
// key: (finality provider cosmos address, period)
// value: FinalityProviderHistoricalRewards
func (k Keeper) storeFpHistoricalRewards(ctx context.Context, fp sdk.AccAddress) prefix.Store {
	storeAdaptor := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	st := prefix.NewStore(storeAdaptor, types.FinalityProviderHistoricalRewardsKey)
	return prefix.NewStore(st, fp.Bytes())
}

// setBTCDelegatorToFP sets a new delegator to finality provider record.
func (k Keeper) setBTCDelegatorToFP(ctx context.Context, del, fp sdk.AccAddress) {
	st := k.storeBTCDelegatorToFp(ctx, del)
	st.Set(fp.Bytes(), []byte{0x00})
}

// iterBtcDelegationsByDelegator iterates over all the possible BTC delegations
// filtering by the delegator address (uses the BTCDelegatorToFPKey keystore)
// It stops if the `it` function returns an error
func (k Keeper) iterBtcDelegationsByDelegator(ctx context.Context, del sdk.AccAddress, it func(del, fp sdk.AccAddress) error) error {
	st := k.storeBTCDelegatorToFp(ctx, del)

	iter := st.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		fp := sdk.AccAddress(iter.Key())
		if err := it(del, fp); err != nil {
			return err
		}
	}
	return nil
}

// deleteBTCDelegatorToFP deletes one key (del, fp) from the store
// without checking if it exists.
func (k Keeper) deleteBTCDelegatorToFP(ctx context.Context, del, fp sdk.AccAddress) {
	st := k.storeBTCDelegatorToFp(ctx, del)
	st.Delete(fp.Bytes())
}

// GetFinalityProviderCurrentRewards returns the Finality Provider current rewards
// based on the FP address key
func (k Keeper) GetFinalityProviderCurrentRewards(ctx context.Context, fp sdk.AccAddress) (types.FinalityProviderCurrentRewards, error) {
	key := fp.Bytes()
	bz := k.storeFpCurrentRewards(ctx).Get(key)
	if bz == nil {
		return types.FinalityProviderCurrentRewards{}, types.ErrFPCurrentRewardsNotFound
	}

	var value types.FinalityProviderCurrentRewards
	if err := k.cdc.Unmarshal(bz, &value); err != nil {
		return types.FinalityProviderCurrentRewards{}, err
	}
	return value, nil
}

// IterateBTCDelegationRewardsTracker iterates over all the delegation rewards tracker by the finality provider.
// It stops if the function `it` returns an error.
func (k Keeper) IterateBTCDelegationRewardsTracker(ctx context.Context, fp sdk.AccAddress, it func(fp, del sdk.AccAddress) error) error {
	st := k.storeBTCDelegationRewardsTracker(ctx, fp)

	iter := st.Iterator(nil, nil)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		del := sdk.AccAddress(iter.Key())
		if err := it(fp, del); err != nil {
			return err
		}
	}

	return nil
}

// deleteKeysFromBTCDelegationRewardsTracker iterates over all the BTC delegation rewards tracker by the finality provider and deletes it.
func (k Keeper) deleteKeysFromBTCDelegationRewardsTracker(ctx context.Context, fp sdk.AccAddress, delKeys [][]byte) {
	stDelRwdTracker := k.storeBTCDelegationRewardsTracker(ctx, fp)
	for _, delKey := range delKeys {
		stDelRwdTracker.Delete(delKey)
		k.deleteBTCDelegatorToFP(ctx, sdk.AccAddress(delKey), fp)
	}
}

// GetBTCDelegationRewardsTracker returns the BTCDelegationRewardsTracker based on the delegation key (fp, del)
// It returns an error in case the key is not found.
func (k Keeper) GetBTCDelegationRewardsTracker(ctx context.Context, fp, del sdk.AccAddress) (types.BTCDelegationRewardsTracker, error) {
	key := del.Bytes()
	bz := k.storeBTCDelegationRewardsTracker(ctx, fp).Get(key)
	if bz == nil {
		return types.BTCDelegationRewardsTracker{}, types.ErrBTCDelegationRewardsTrackerNotFound
	}

	var value types.BTCDelegationRewardsTracker
	if err := k.cdc.Unmarshal(bz, &value); err != nil {
		return types.BTCDelegationRewardsTracker{}, err
	}
	return value, nil
}

// setBTCDelegationRewardsTracker sets a new structure in the store, it fails and returns an error if the rwd fails to marshal.
func (k Keeper) setBTCDelegationRewardsTracker(ctx context.Context, fp, del sdk.AccAddress, rwd types.BTCDelegationRewardsTracker) error {
	key := del.Bytes()
	bz, err := rwd.Marshal()
	if err != nil {
		return err
	}

	k.setBTCDelegatorToFP(ctx, del, fp)
	k.storeBTCDelegationRewardsTracker(ctx, fp).Set(key, bz)
	return nil
}

// setFinalityProviderCurrentRewards sets a new structure in the store, it fails and returns an error if the rwd fails to marshal.
func (k Keeper) setFinalityProviderCurrentRewards(ctx context.Context, fp sdk.AccAddress, rwd types.FinalityProviderCurrentRewards) error {
	key := fp.Bytes()
	bz, err := rwd.Marshal()
	if err != nil {
		return err
	}

	k.storeFpCurrentRewards(ctx).Set(key, bz)
	return nil
}

// deleteAllFromFinalityProviderRwd deletes all the data related to Finality Provider Rewards
// Historical and current from a fp address key.
func (k Keeper) deleteAllFromFinalityProviderRwd(ctx context.Context, fp sdk.AccAddress) {
	stHistoricalRwd := k.storeFpHistoricalRewards(ctx, fp)

	iter := stHistoricalRwd.Iterator(nil, nil)
	defer iter.Close()

	keys := make([][]byte, 0)
	for ; iter.Valid(); iter.Next() {
		keys = append(keys, iter.Key())
	}

	for _, key := range keys {
		stHistoricalRwd.Delete(key)
	}

	k.deleteFinalityProviderCurrentRewards(ctx, fp)
}

// deleteFinalityProviderCurrentRewards deletes the current FP reward based on the key received
func (k Keeper) deleteFinalityProviderCurrentRewards(ctx context.Context, fp sdk.AccAddress) {
	key := fp.Bytes()
	k.storeFpCurrentRewards(ctx).Delete(key)
}

// GetFinalityProviderHistoricalRewards returns the FinalityProviderHistoricalRewards based on the key (fp, period)
// It returns an error if the key is not found inside the store.
func (k Keeper) GetFinalityProviderHistoricalRewards(ctx context.Context, fp sdk.AccAddress, period uint64) (types.FinalityProviderHistoricalRewards, error) {
	key := make([]byte, 8)
	binary.LittleEndian.PutUint64(key, period)

	bz := k.storeFpHistoricalRewards(ctx, fp).Get(key)
	if bz == nil {
		return types.FinalityProviderHistoricalRewards{}, types.ErrFPHistoricalRewardsNotFound
	}

	var value types.FinalityProviderHistoricalRewards
	if err := k.cdc.Unmarshal(bz, &value); err != nil {
		return types.FinalityProviderHistoricalRewards{}, err
	}
	return value, nil
}

// setFinalityProviderHistoricalRewards sets a new value inside the store, it returns an error
// if the marshal of the `rwd` fails.
func (k Keeper) setFinalityProviderHistoricalRewards(ctx context.Context, fp sdk.AccAddress, period uint64, rwd types.FinalityProviderHistoricalRewards) error {
	key := make([]byte, 8)
	binary.LittleEndian.PutUint64(key, period)

	bz, err := rwd.Marshal()
	if err != nil {
		return err
	}

	k.storeFpHistoricalRewards(ctx, fp).Set(key, bz)
	return nil
}

// subDelegationSat subtracts an amount of active stake from the BTCDelegationRewardsTracker
// and the FinalityProviderCurrentRewards.
// There is no need to check if the fp or delegation exists, because they should exist
// otherwise it is probably a programming error calling to subtract the amount of active sat without
// having any sat added in the first place that created the structures.
func (k Keeper) subDelegationSat(ctx context.Context, fp, del sdk.AccAddress, amt sdkmath.Int) error {
	btcDelRwdTracker, err := k.GetBTCDelegationRewardsTracker(ctx, fp, del)
	if err != nil {
		return err
	}

	btcDelRwdTracker.SubTotalActiveSat(amt)
	if err := k.setBTCDelegationRewardsTracker(ctx, fp, del, btcDelRwdTracker); err != nil {
		return err
	}

	return k.subFinalityProviderStaked(ctx, fp, amt)
}

// subFinalityProviderStaked subtracts an amount of active stake from the
// FinalityProviderCurrentRewards, it errors out if the finality provider does not exist.
func (k Keeper) subFinalityProviderStaked(ctx context.Context, fp sdk.AccAddress, amt sdkmath.Int) error {
	fpCurrentRwd, err := k.GetFinalityProviderCurrentRewards(ctx, fp)
	if err != nil {
		return err
	}

	fpCurrentRwd.SubTotalActiveSat(amt)
	return k.setFinalityProviderCurrentRewards(ctx, fp, fpCurrentRwd)
}

// addFinalityProviderStaked increases the total amount of active satoshi to a finality provider
// if it does not exist one current reward in the store, it initializes a new one
// The initialization of a finality provider also stores one historical fp reward.
func (k Keeper) addFinalityProviderStaked(ctx context.Context, fp sdk.AccAddress, amt sdkmath.Int) error {
	fpCurrentRwd, err := k.GetFinalityProviderCurrentRewards(ctx, fp)
	if err != nil {
		if !errors.Is(err, types.ErrFPCurrentRewardsNotFound) {
			return err
		}

		// this is needed as the amount of sats for the FP is inside the FpCurrentRewards
		fpCurrentRwd, err = k.initializeFinalityProvider(ctx, fp)
		if err != nil {
			return err
		}
	}

	fpCurrentRwd.AddTotalActiveSat(amt)
	return k.setFinalityProviderCurrentRewards(ctx, fp, fpCurrentRwd)
}

// addDelegationSat it increases the amount of satoshi staked for the delegation (fp, del)
// and for the finality provider as well, it initializes the finality provider and the
// BTC delegation rewards tracker if it does not exist.
func (k Keeper) addDelegationSat(ctx context.Context, fp, del sdk.AccAddress, amt sdkmath.Int) error {
	btcDelRwdTracker, err := k.GetBTCDelegationRewardsTracker(ctx, fp, del)
	if err != nil {
		if !errors.Is(err, types.ErrBTCDelegationRewardsTrackerNotFound) {
			return err
		}

		// first delegation to this pair (fp, del), can start as 0 previous period as it
		// it should be updated afterwards with initilize btc delegation
		btcDelRwdTracker = types.NewBTCDelegationRewardsTracker(0, sdkmath.ZeroInt())
	}

	btcDelRwdTracker.AddTotalActiveSat(amt)
	if err := k.setBTCDelegationRewardsTracker(ctx, fp, del, btcDelRwdTracker); err != nil {
		return err
	}

	return k.addFinalityProviderStaked(ctx, fp, amt)
}
