package keeper

import (
	"context"

	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	corestoretypes "cosmossdk.io/core/store"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	"github.com/btcsuite/btcd/chaincfg/chainhash"

	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

// getBTCDelegatorDelegationIndex gets the BTC delegation index with a given BTC PK under a given finality provider
func (k Keeper) getBTCDelegatorDelegationIndex(ctx context.Context, fpBTCPK *bbn.BIP340PubKey, delBTCPK *bbn.BIP340PubKey) *types.BTCDelegatorDelegationIndex {
	fpBTCPKBytes := fpBTCPK.MustMarshal()
	delBTCPKBytes := delBTCPK.MustMarshal()
	store := k.btcDelegatorFpStore(ctx, fpBTCPK)

	// ensure the finality provider exists
	if !k.HasFinalityProvider(ctx, fpBTCPKBytes) {
		return nil
	}

	// ensure BTC delegator exists
	if !store.Has(delBTCPKBytes) {
		return nil
	}
	// get and unmarshal
	var btcDelIndex types.BTCDelegatorDelegationIndex
	btcDelIndexBytes := store.Get(delBTCPKBytes)
	k.cdc.MustUnmarshal(btcDelIndexBytes, &btcDelIndex)
	return &btcDelIndex
}

func (k Keeper) setBTCDelegatorDelegationIndex(ctx context.Context, fpBTCPK, delBTCPK *bbn.BIP340PubKey, btcDelIndex *types.BTCDelegatorDelegationIndex) {
	store := k.btcDelegatorFpStore(ctx, fpBTCPK)
	btcDelIndexBytes := k.cdc.MustMarshal(btcDelIndex)
	store.Set(*delBTCPK, btcDelIndexBytes)
}

// getBTCDelegatorDelegations gets the BTC delegations with a given BTC PK under a given finality provider
func (k Keeper) getBTCDelegatorDelegations(ctx context.Context, fpBTCPK *bbn.BIP340PubKey, delBTCPK *bbn.BIP340PubKey) *types.BTCDelegatorDelegations {
	btcDelIndex := k.getBTCDelegatorDelegationIndex(ctx, fpBTCPK, delBTCPK)
	if btcDelIndex == nil {
		return nil
	}
	// get BTC delegation from each staking tx hash
	btcDels := []*types.BTCDelegation{}
	for _, stakingTxHashBytes := range btcDelIndex.StakingTxHashList {
		stakingTxHash, err := chainhash.NewHash(stakingTxHashBytes)
		if err != nil {
			// failing to unmarshal hash bytes in DB's BTC delegation index is a programming error
			panic(err)
		}
		btcDel := k.getBTCDelegation(ctx, *stakingTxHash)
		btcDels = append(btcDels, btcDel)
	}
	return &types.BTCDelegatorDelegations{Dels: btcDels}
}

// btcDelegatorFpStore returns the KVStore of the BTC delegators
// prefix: BTCDelegatorKey || finality provider's Bitcoin secp256k1 PK
// key: delegator's Bitcoin secp256k1 PK
// value: BTCDelegatorDelegationIndex (a list of BTCDelegations' staking tx hashes)
func (k Keeper) btcDelegatorFpStore(ctx context.Context, fpBTCPK *bbn.BIP340PubKey) prefix.Store {
	delegationStore := k.btcDelegatorStore(ctx)
	return prefix.NewStore(delegationStore, fpBTCPK.MustMarshal())
}

// btcDelegatorFpStore returns the KVStore of the BTC delegators
// prefix: BTCDelegatorKey
// key: finality provider's Bitcoin secp256k1 PK || delegator's Bitcoin secp256k1 PK
// value: BTCDelegatorDelegationIndex (a list of BTCDelegations' staking tx hashes)
func (k Keeper) btcDelegatorStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.BTCDelegatorKey)
}

// storeDelStaked returns the KVStore of the delegator amount staked
// prefix: (DelegatorStakedBTCKey)
// key: Del addr
// value: sdk math Int
func (k Keeper) storeDelStaked(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.DelegatorStakedBTCKey)
}

// Total active satoshi staked that is entitled to earn rewards.
func (k Keeper) TotalSatoshiStaked(ctx context.Context) (sdkmath.Int, error) {
	kv := k.storeService.OpenKVStore(ctx)
	key := types.TotalStakedBTCKey
	return StoreGetInt(kv, key)
}

func (k Keeper) addTotalSatoshiStaked(ctx context.Context, amtToAdd sdkmath.Int) (sdkmath.Int, error) {
	kv := k.storeService.OpenKVStore(ctx)
	key := types.TotalStakedBTCKey

	current, err := StoreGetInt(kv, key)
	if err != nil {
		return sdkmath.Int{}, err
	}

	total := current.Add(amtToAdd)
	if err := StoreSetInt(kv, key, total); err != nil {
		return sdkmath.Int{}, err
	}

	return total, nil
}

func (k Keeper) subTotalSatoshiStaked(ctx context.Context, amtToAdd sdkmath.Int) (sdkmath.Int, error) {
	kv := k.storeService.OpenKVStore(ctx)
	key := types.TotalStakedBTCKey

	current, err := StoreGetInt(kv, key)
	if err != nil {
		return sdkmath.Int{}, err
	}

	total := current.Sub(amtToAdd)
	if err := StoreSetInt(kv, key, total); err != nil {
		return sdkmath.Int{}, err
	}

	return total, nil
}

func (k Keeper) AddDelStaking(ctx context.Context, del sdk.AccAddress, amt sdkmath.Int) error {
	st := k.storeDelStaked(ctx)

	currentStk, err := PrefixStoreGetInt(st, del)
	if err != nil {
		return err
	}

	totalDelStaked := currentStk.Add(amt)
	bz, err := totalDelStaked.Marshal()
	if err != nil {
		return err
	}

	st.Set(del, bz)
	_, err = k.addTotalSatoshiStaked(ctx, amt)
	return err
}

func (k Keeper) SubDelStaking(ctx context.Context, del sdk.AccAddress, amt sdkmath.Int) error {
	st := k.storeDelStaked(ctx)

	currentStk, err := PrefixStoreGetInt(st, del)
	if err != nil {
		return err
	}

	totalDelStaked := currentStk.Sub(amt)
	bz, err := totalDelStaked.Marshal()
	if err != nil {
		return err
	}

	st.Set(del, bz)
	_, err = k.subTotalSatoshiStaked(ctx, amt)
	return err
}

func PrefixStoreGetInt(st prefix.Store, key []byte) (vInt sdkmath.Int, err error) {
	if !st.Has(key) {
		return sdkmath.NewInt(0), nil
	}

	bz := st.Get(key)
	vInt, err = ParseInt(bz)
	if err != nil {
		return sdkmath.Int{}, err
	}

	return vInt, nil
}

// StoreSetInt stores an sdkmath.Int from the KVStore.
func StoreSetInt(kv corestoretypes.KVStore, key []byte, vInt sdkmath.Int) (err error) {
	bz, err := vInt.Marshal()
	if err != nil {
		return err
	}
	return kv.Set(key, bz)
}

// StoreGetInt retrieves an sdkmath.Int from the KVStore. It returns zero int if not found.
func StoreGetInt(kv corestoretypes.KVStore, key []byte) (vInt sdkmath.Int, err error) {
	exists, err := kv.Has(key)
	if err != nil {
		return sdkmath.Int{}, err
	}

	if !exists {
		return sdkmath.NewInt(0), nil
	}

	bz, err := kv.Get(key)
	if err != nil {
		return sdkmath.Int{}, err
	}

	vInt, err = ParseInt(bz)
	if err != nil {
		return sdkmath.Int{}, err
	}
	return vInt, nil
}

// ParseInt parses an sdkmath.Int from bytes.
func ParseInt(bz []byte) (sdkmath.Int, error) {
	var val sdkmath.Int
	if err := val.Unmarshal(bz); err != nil {
		return val, err
	}
	return val, nil
}

// Iterate over all the delegators that have some active BTC delegator staked
// and the total satoshi staked for that delegator address until an error is returned
// or the iterator finishes. Stops if error is returned.
// Should keep track of the total satoshi staked per delegator to avoid iterating over the
// delegator delegations
func (k Keeper) IterateDelegators(ctx context.Context, i func(delegator sdk.AccAddress, totalSatoshiStaked sdkmath.Int) error) error {
	st := k.storeDelStaked(ctx)

	iter := st.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		sdkAddrBz := iter.Key()
		delAddr := sdk.AccAddress(sdkAddrBz)

		delBtcStaked, err := ParseInt(iter.Value())
		if err != nil {
			return err
		}

		err = i(delAddr, delBtcStaked)
		if err != nil {
			return err
		}
	}

	return nil
}
