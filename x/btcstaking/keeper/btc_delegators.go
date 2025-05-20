package keeper

import (
	"context"

	"github.com/cosmos/cosmos-sdk/runtime"

	"cosmossdk.io/store/prefix"

	"github.com/btcsuite/btcd/chaincfg/chainhash"

	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
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

// HandleFPBTCDelegations processes all BTC delegations for a given finality provider using a provided handler function.
// This function works for both Babylon finality providers and consumer finality providers.
// It automatically determines and selects the appropriate KV store based on the finality provider type.
//
// Parameters:
// - ctx: The context for the operation
// - fpBTCPK: The Bitcoin public key of the finality provider
// - handler: A function that processes each BTCDelegation
//
// Returns:
// - An error if the finality provider is not found or if there's an issue processing the delegations
func (k Keeper) HandleFPBTCDelegations(ctx context.Context, fpBTCPK *bbn.BIP340PubKey, handler func(*types.BTCDelegation) error) error {
	var store prefix.Store
	// Determine which store to use based on the finality provider type
	switch {
	case k.HasFinalityProvider(ctx, *fpBTCPK):
		// Babylon finality provider
		store = k.btcDelegatorFpStore(ctx, fpBTCPK)
	case k.BscKeeper.HasConsumerFinalityProvider(ctx, fpBTCPK):
		// Consumer finality provider
		store = k.btcConsumerDelegatorStore(ctx, fpBTCPK)
	default:
		// if not found in either store, return error
		return types.ErrFpNotFound
	}

	iterator := store.Iterator(nil, nil)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		var btcDelIndex types.BTCDelegatorDelegationIndex
		if err := btcDelIndex.Unmarshal(iterator.Value()); err != nil {
			return err
		}

		for _, stakingTxHashBytes := range btcDelIndex.StakingTxHashList {
			stakingTxHash, err := chainhash.NewHash(stakingTxHashBytes)
			if err != nil {
				return err
			}
			btcDel := k.getBTCDelegation(ctx, *stakingTxHash)
			if btcDel != nil {
				if err := handler(btcDel); err != nil {
					return err
				}
			}
		}
	}

	return nil
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
