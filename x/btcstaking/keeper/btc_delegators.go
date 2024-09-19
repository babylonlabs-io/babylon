package keeper

import (
	"context"

	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

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

func (k Keeper) getFPBTCDelegations(ctx context.Context, fpBTCPK *bbn.BIP340PubKey) ([]*types.BTCDelegation, error) {
	var store prefix.Store

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Determine which store to use based on the finality provider type
	if k.HasFinalityProvider(ctx, *fpBTCPK) {
		store = k.btcDelegatorFpStore(ctx, fpBTCPK)
		k.Logger(sdkCtx).Info("DEBUG: Using btcDelegatorFpStore for Babylon finality provider", "fpBTCPK", fpBTCPK)
	} else if k.bscKeeper.HasConsumerFinalityProvider(ctx, fpBTCPK) {
		store = k.btcConsumerDelegatorStore(ctx, fpBTCPK)
		k.Logger(sdkCtx).Info("DEBUG: Using btcConsumerDelegatorStore for consumer finality provider", "fpBTCPK", fpBTCPK)
	} else {
		k.Logger(sdkCtx).Error("DEBUG: Finality provider not found", "fpBTCPK", fpBTCPK)
		return nil, types.ErrFpNotFound
	}

	iterator := store.Iterator(nil, nil)
	defer iterator.Close()

	btcDels := make([]*types.BTCDelegation, 0)
	for ; iterator.Valid(); iterator.Next() {
		var btcDelIndex types.BTCDelegatorDelegationIndex
		if err := btcDelIndex.Unmarshal(iterator.Value()); err != nil {
			return nil, err
		}

		for _, stakingTxHashBytes := range btcDelIndex.StakingTxHashList {
			stakingTxHash, err := chainhash.NewHash(stakingTxHashBytes)
			if err != nil {
				return nil, err
			}
			btcDel := k.getBTCDelegation(ctx, *stakingTxHash)
			if btcDel != nil {
				btcDels = append(btcDels, btcDel)
			}
		}
	}

	return btcDels, nil
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
