package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
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
	value, err := k.finalityProviderCurrentRewards.Get(ctx, fp.Bytes())
	if err != nil {
		return types.FinalityProviderCurrentRewards{}, types.ErrFPCurrentRewardsNotFound
	}
	return value, nil
}

// IterateBTCDelegationRewardsTracker iterates over all the delegation rewards tracker by the finality provider.
// It stops if the function `it` returns an error.
func (k Keeper) IterateBTCDelegationRewardsTracker(
	ctx context.Context,
	fp sdk.AccAddress,
	it func(fp, del sdk.AccAddress, btcRwdTracker types.BTCDelegationRewardsTracker) error,
) error {
	rng := collections.NewPrefixedPairRange[[]byte, []byte](fp.Bytes())
	return k.btcDelegationRewardsTracker.Walk(ctx, rng, func(key collections.Pair[[]byte, []byte], value types.BTCDelegationRewardsTracker) (stop bool, err error) {
		del := sdk.AccAddress(key.K2())
		if err := it(fp, del, value); err != nil {
			return err != nil, err
		}
		return false, nil
	})
}

// IterateBTCDelegationSatsUpdated iterates over all the delegation active sats by the finality provider.
// It stops if the function `it` returns an error.
// Note: It takes into account the events that are queued to be processed until the current block height.
func (k Keeper) IterateBTCDelegationSatsUpdated(
	ctx context.Context,
	fp sdk.AccAddress,
	it func(del sdk.AccAddress, activeSats sdkmath.Int) error,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	fpAddrStr := fp.String()

	compiledEvents, err := k.GetRewardTrackerEventsCompiledByBtcDel(
		ctx,
		uint64(sdkCtx.BlockHeader().Height),
		func(fpAddr string) (include bool) {
			return fpAddr == fpAddrStr
		},
	)
	if err != nil {
		return err
	}

	rng := collections.NewPrefixedPairRange[[]byte, []byte](fp.Bytes())
	err = k.btcDelegationRewardsTracker.Walk(ctx, rng, func(key collections.Pair[[]byte, []byte], rwdTracker types.BTCDelegationRewardsTracker) (stop bool, err error) {
		del := sdk.AccAddress(key.K2())

		activeSats := rwdTracker.TotalActiveSat

		delStr := del.String()
		// Add any pending events for this delegation. If there are more unbonding
		// sats than activation on the events, the pendingSats can be negative
		if pendingSats, exists := compiledEvents[delStr]; exists {
			activeSats = activeSats.Add(pendingSats)
		}
		delete(compiledEvents, delStr)

		if err := it(del, activeSats); err != nil {
			return err != nil, err
		}
		return false, nil
	})
	if err != nil {
		return err
	}

	// iterates over the compiled events as some new btc delegation could be activated during the pending events
	for delStr, sats := range compiledEvents {
		delAddr, err := sdk.AccAddressFromBech32(delStr)
		if err != nil {
			return err
		}

		if err := it(delAddr, sats); err != nil {
			return err
		}
	}

	return nil
}

// deleteKeysFromBTCDelegationRewardsTracker iterates over all the BTC delegation rewards tracker by the finality provider and deletes it.
func (k Keeper) deleteKeysFromBTCDelegationRewardsTracker(ctx context.Context, fp sdk.AccAddress, delKeys [][]byte) {
	rng := collections.NewPrefixedPairRange[[]byte, []byte](fp.Bytes())
	err := k.btcDelegationRewardsTracker.Clear(ctx, rng)
	if err != nil {
		k.Logger(sdk.UnwrapSDKContext(ctx)).Error("error deleting BTCDelegationRewardsTracker", "error", err)
	}
	for _, delKey := range delKeys {
		k.deleteBTCDelegatorToFP(ctx, sdk.AccAddress(delKey), fp)
	}
}

// GetBTCDelegationRewardsTracker returns the BTCDelegationRewardsTracker based on the delegation key (fp, del)
// It returns an error in case the key is not found.
func (k Keeper) GetBTCDelegationRewardsTracker(ctx context.Context, fp, del sdk.AccAddress) (types.BTCDelegationRewardsTracker, error) {
	value, err := k.btcDelegationRewardsTracker.Get(ctx, collections.Join(fp.Bytes(), del.Bytes()))
	if err != nil {
		return types.BTCDelegationRewardsTracker{}, types.ErrBTCDelegationRewardsTrackerNotFound
	}
	return value, nil
}

// setBTCDelegationRewardsTracker sets a new structure in the store, it fails and returns an error if the rwd fails to marshal.
func (k Keeper) setBTCDelegationRewardsTracker(ctx context.Context, fp, del sdk.AccAddress, rwd types.BTCDelegationRewardsTracker) error {
	k.setBTCDelegatorToFP(ctx, del, fp)
	return k.btcDelegationRewardsTracker.Set(ctx, collections.Join(fp.Bytes(), del.Bytes()), rwd)
}

// SetFinalityProviderCurrentRewards sets a new structure in the store, it fails and returns an error if the rwd fails to marshal.
func (k Keeper) SetFinalityProviderCurrentRewards(ctx context.Context, fp sdk.AccAddress, rwd types.FinalityProviderCurrentRewards) error {
	return k.finalityProviderCurrentRewards.Set(ctx, fp.Bytes(), rwd)
}

// deleteAllFromFinalityProviderRwd deletes all the data related to Finality Provider Rewards
// Historical and current from a fp address key.
func (k Keeper) deleteAllFromFinalityProviderRwd(ctx context.Context, fp sdk.AccAddress) {
	rng := collections.NewPrefixedPairRange[[]byte, uint64](fp.Bytes())
	err := k.finalityProviderHistoricalRewards.Clear(ctx, rng)
	if err != nil {
		k.Logger(sdk.UnwrapSDKContext(ctx)).Error("error deleting FinalityProviderHistoricalRewards", "error", err)
	}

	k.deleteFinalityProviderCurrentRewards(ctx, fp)
}

// deleteFinalityProviderCurrentRewards deletes the current FP reward based on the key received
func (k Keeper) deleteFinalityProviderCurrentRewards(ctx context.Context, fp sdk.AccAddress) {
	if err := k.finalityProviderCurrentRewards.Remove(ctx, fp.Bytes()); err != nil {
		k.Logger(sdk.UnwrapSDKContext(ctx)).Error("error deleting FinalityProviderCurrentRewards", "error", err)
	}
}

// GetFinalityProviderHistoricalRewards returns the FinalityProviderHistoricalRewards based on the key (fp, period)
// It returns an error if the key is not found inside the store.
func (k Keeper) GetFinalityProviderHistoricalRewards(ctx context.Context, fp sdk.AccAddress, period uint64) (types.FinalityProviderHistoricalRewards, error) {
	value, err := k.finalityProviderHistoricalRewards.Get(ctx, collections.Join(fp.Bytes(), period))
	if err != nil {
		return types.FinalityProviderHistoricalRewards{}, types.ErrFPHistoricalRewardsNotFound
	}
	return value, nil
}

// setFinalityProviderHistoricalRewards sets a new value inside the store, it returns an error
// if the marshal of the `rwd` fails.
func (k Keeper) setFinalityProviderHistoricalRewards(ctx context.Context, fp sdk.AccAddress, period uint64, rwd types.FinalityProviderHistoricalRewards) error {
	return k.finalityProviderHistoricalRewards.Set(ctx, collections.Join(fp.Bytes(), period), rwd)
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
	if btcDelRwdTracker.TotalActiveSat.IsNegative() {
		return types.ErrBTCDelegationRewardsTrackerNegativeAmount
	}
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
	if fpCurrentRwd.TotalActiveSat.IsNegative() {
		return types.ErrFPCurrentRewardsTrackerNegativeAmount
	}
	return k.SetFinalityProviderCurrentRewards(ctx, fp, fpCurrentRwd)
}

// IterateFpCurrentRewards iterates over all the finality provider current rewards
func (k Keeper) IterateFpCurrentRewards(ctx context.Context, it func(fp sdk.AccAddress, fpCurrRwds types.FinalityProviderCurrentRewards) error) error {
	iter, err := k.finalityProviderCurrentRewards.Iterate(ctx, nil)
	if err != nil {
		return err
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		key, err := iter.Key()
		if err != nil {
			return err
		}
		fpAddr := sdk.AccAddress(key)

		fpCurrentRwds, err := iter.Value()
		if err != nil {
			return err
		}

		err = it(fpAddr, fpCurrentRwds)
		if err != nil {
			return err
		}
	}

	return nil
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

		// needs to initialize at this point due to the amount of
		// sats for the FP is inside the FinalityProviderCurrentRewards
		fpCurrentRwd, err = k.initializeFinalityProvider(ctx, fp)
		if err != nil {
			return err
		}
	}

	fpCurrentRwd.AddTotalActiveSat(amt)
	return k.SetFinalityProviderCurrentRewards(ctx, fp, fpCurrentRwd)
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
