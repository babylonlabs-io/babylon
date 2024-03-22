package keeper

import (
	"context"

	"cosmossdk.io/store/prefix"
	bbn "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/babylonchain/babylon/x/btcstkconsumer/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
)

// AddBTCDelegation adds a BTC delegation post verification to the system, including
// - indexing the given BTC delegation in the BTC delegator store, and
// - saving it under BTC delegation store
// CONTRACT: this function only takes BTC delegations that have passed verifications
// imposed in `x/btcstaking/msg_server.go`
// TODO: is it possible to not replicate the storage of restaked BTC delegation?
func (k Keeper) AddBTCDelegation(ctx sdk.Context, btcDel *bstypes.BTCDelegation) error {
	if err := btcDel.ValidateBasic(); err != nil {
		return err
	}

	// get staking tx hash
	stakingTxHash, err := btcDel.GetStakingTxHash()
	if err != nil {
		return err
	}

	// for each finality provider the delegation restakes to, update its index
	for _, fpBTCPK := range btcDel.FpBtcPkList {
		// skip Babylon finality providers
		if !k.HasFinalityProvider(ctx, &fpBTCPK) {
			continue
		}

		// get BTC delegation index under this finality provider
		btcDelIndex := k.getBTCDelegatorDelegationIndex(ctx, &fpBTCPK, btcDel.BtcPk)
		if btcDelIndex == nil {
			btcDelIndex = bstypes.NewBTCDelegatorDelegationIndex()
		}
		// index staking tx hash of this BTC delegation
		if err := btcDelIndex.Add(stakingTxHash); err != nil {
			return bstypes.ErrInvalidStakingTx.Wrapf(err.Error())
		}
		// save the index
		k.setBTCDelegatorDelegationIndex(ctx, &fpBTCPK, btcDel.BtcPk, btcDelIndex)
	}

	// save this BTC delegation
	k.SetBTCDelegation(ctx, btcDel)

	return nil
}

// GetBTCDelegatorDelegationsResponses gets BTC delegations of BTC delegators under
// the given finality provider. The BTC delegators are paginated. This function is
// used by the `FinalityProviderDelegations` query in BTC staking module
func (k Keeper) GetBTCDelegatorDelegationsResponses(
	ctx sdk.Context,
	fpBTCPK *bbn.BIP340PubKey,
	pagination *query.PageRequest,
	wValue uint64,
	btcHeight uint64,
	covenantQuorum uint32,
) ([]*bstypes.BTCDelegatorDelegationsResponse, *query.PageResponse, error) {
	btcDelStore := k.btcDelegatorStore(ctx, fpBTCPK)

	btcDels := []*bstypes.BTCDelegatorDelegationsResponse{}
	pageRes, err := query.Paginate(btcDelStore, pagination, func(key, value []byte) error {
		delBTCPK, err := bbn.NewBIP340PubKey(key)
		if err != nil {
			return err
		}

		curBTCDels := k.getBTCDelegatorDelegations(ctx, fpBTCPK, delBTCPK)

		btcDelsResp := make([]*bstypes.BTCDelegationResponse, len(curBTCDels.Dels))
		for i, btcDel := range curBTCDels.Dels {
			status := btcDel.GetStatus(
				btcHeight,
				wValue,
				covenantQuorum,
			)
			btcDelsResp[i] = bstypes.NewBTCDelegationResponse(btcDel, status)
		}

		btcDels = append(btcDels, &bstypes.BTCDelegatorDelegationsResponse{
			Dels: btcDelsResp,
		})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return btcDels, pageRes, nil
}

func (k Keeper) SetBTCDelegation(ctx context.Context, btcDel *bstypes.BTCDelegation) {
	store := k.btcDelegationStore(ctx)
	stakingTxHash := btcDel.MustGetStakingTxHash()
	btcDelBytes := k.cdc.MustMarshal(btcDel)
	store.Set(stakingTxHash[:], btcDelBytes)
}

func (k Keeper) getBTCDelegation(ctx context.Context, stakingTxHash chainhash.Hash) *bstypes.BTCDelegation {
	store := k.btcDelegationStore(ctx)
	btcDelBytes := store.Get(stakingTxHash[:])
	if len(btcDelBytes) == 0 {
		return nil
	}
	var btcDel bstypes.BTCDelegation
	k.cdc.MustUnmarshal(btcDelBytes, &btcDel)
	return &btcDel
}

// btcDelegationStore returns the KVStore of the BTC delegations
// prefix: BTCDelegationKey
// key: BTC delegation's staking tx hash
// value: BTCDelegation
func (k Keeper) btcDelegationStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.BTCDelegationKey)
}

// getBTCDelegatorDelegationIndex gets the BTC delegation index with a given BTC PK under a given finality provider
func (k Keeper) getBTCDelegatorDelegationIndex(ctx context.Context, fpBTCPK *bbn.BIP340PubKey, delBTCPK *bbn.BIP340PubKey) *bstypes.BTCDelegatorDelegationIndex {
	// ensure the finality provider exists
	if !k.HasFinalityProvider(ctx, fpBTCPK) {
		return nil
	}

	delBTCPKBytes := delBTCPK.MustMarshal()
	store := k.btcDelegatorStore(ctx, fpBTCPK)

	// ensure BTC delegator exists
	if !store.Has(delBTCPKBytes) {
		return nil
	}
	// get and unmarshal
	var btcDelIndex bstypes.BTCDelegatorDelegationIndex
	btcDelIndexBytes := store.Get(delBTCPKBytes)
	k.cdc.MustUnmarshal(btcDelIndexBytes, &btcDelIndex)
	return &btcDelIndex
}

func (k Keeper) setBTCDelegatorDelegationIndex(ctx context.Context, fpBTCPK *bbn.BIP340PubKey, delBTCPK *bbn.BIP340PubKey, btcDelIndex *bstypes.BTCDelegatorDelegationIndex) {
	store := k.btcDelegatorStore(ctx, fpBTCPK)
	btcDelIndexBytes := k.cdc.MustMarshal(btcDelIndex)
	store.Set(*delBTCPK, btcDelIndexBytes)
}

// getBTCDelegatorDelegations gets the BTC delegations with a given BTC PK under a given finality provider
func (k Keeper) getBTCDelegatorDelegations(ctx context.Context, fpBTCPK *bbn.BIP340PubKey, delBTCPK *bbn.BIP340PubKey) *bstypes.BTCDelegatorDelegations {
	btcDelIndex := k.getBTCDelegatorDelegationIndex(ctx, fpBTCPK, delBTCPK)
	if btcDelIndex == nil {
		return nil
	}
	// get BTC delegation from each staking tx hash
	btcDels := []*bstypes.BTCDelegation{}
	for _, stakingTxHashBytes := range btcDelIndex.StakingTxHashList {
		stakingTxHash, err := chainhash.NewHash(stakingTxHashBytes)
		if err != nil {
			// failing to unmarshal hash bytes in DB's BTC delegation index is a programming error
			panic(err)
		}
		btcDel := k.getBTCDelegation(ctx, *stakingTxHash)
		btcDels = append(btcDels, btcDel)
	}
	return &bstypes.BTCDelegatorDelegations{Dels: btcDels}
}

// btcDelegatorStore returns the KVStore of the BTC delegators
// prefix: BTCDelegatorKey || finality provider's Bitcoin secp256k1 PK
// key: delegator's Bitcoin secp256k1 PK
// value: BTCDelegatorDelegationIndex (a list of BTCDelegations' staking tx hashes)
func (k Keeper) btcDelegatorStore(ctx context.Context, fpBTCPK *bbn.BIP340PubKey) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	delegationStore := prefix.NewStore(storeAdapter, types.BTCDelegatorKey)
	return prefix.NewStore(delegationStore, fpBTCPK.MustMarshal())
}
