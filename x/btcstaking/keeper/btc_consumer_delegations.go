package keeper

import (
	"context"

	"cosmossdk.io/store/prefix"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
)

// indexBTCConsumerDelegation indexes a BTC delegation into the BTC consumer delegator store
// CONTRACT: this function only takes BTC delegations that have passed verifications
// imposed in `x/btcstaking/msg_server.go`
func (k Keeper) indexBTCConsumerDelegation(ctx sdk.Context, btcDel *bstypes.BTCDelegation) error {
	// get staking tx hash
	stakingTxHash, err := btcDel.GetStakingTxHash()
	if err != nil {
		return err
	}

	// for each finality provider the delegation restakes to, update its index
	for i := range btcDel.FpBtcPkList {
		fpBTCPK := btcDel.FpBtcPkList[i]

		// skip Babylon finality providers
		if !k.BscKeeper.HasConsumerFinalityProvider(ctx, &fpBTCPK) {
			continue
		}

		// get BTC delegation index under this finality provider
		btcDelIndex := k.getBTCConsumerDelegatorDelegationIndex(ctx, &fpBTCPK, btcDel.BtcPk)
		if btcDelIndex == nil {
			btcDelIndex = bstypes.NewBTCDelegatorDelegationIndex()
		}
		// index staking tx hash of this BTC delegation
		if err := btcDelIndex.Add(stakingTxHash); err != nil {
			return bstypes.ErrInvalidStakingTx.Wrapf("%s", err.Error())
		}
		// save the index
		k.setBTCConsumerDelegatorDelegationIndex(ctx, &fpBTCPK, btcDel.BtcPk, btcDelIndex)
	}

	return nil
}

// GetBTCConsumerDelegatorDelegationsResponses gets BTC delegations of BTC delegators under
// the given finality provider. The BTC delegators are paginated. This function is
// used by the `FinalityProviderDelegations` query in BTC staking module
func (k Keeper) GetBTCConsumerDelegatorDelegationsResponses(
	ctx sdk.Context,
	fpBTCPK *bbn.BIP340PubKey,
	pagination *query.PageRequest,
	btcHeight uint32,
) ([]*bstypes.BTCDelegatorDelegationsResponse, *query.PageResponse, error) {
	btcDelStore := k.btcConsumerDelegatorStore(ctx, fpBTCPK)

	btcDels := []*bstypes.BTCDelegatorDelegationsResponse{}
	pageRes, err := query.Paginate(btcDelStore, pagination, func(key, value []byte) error {
		delBTCPK, err := bbn.NewBIP340PubKey(key)
		if err != nil {
			return err
		}

		curBTCDels := k.getBTCConsumerDelegatorDelegations(ctx, fpBTCPK, delBTCPK)

		btcDelsResp := make([]*bstypes.BTCDelegationResponse, len(curBTCDels.Dels))
		for i, btcDel := range curBTCDels.Dels {
			params := k.GetParamsByVersion(ctx, btcDel.ParamsVersion)

			status := btcDel.GetStatus(
				btcHeight,
				params.CovenantQuorum,
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

// SetConsumerFinalityProvider checks conditions and sets a new finality provider for a consumer.
func (k Keeper) SetConsumerFinalityProvider(ctx context.Context, fp *bstypes.FinalityProvider, consumerID string) error {
	// check if the finality provider already exists to prevent duplicates
	if k.BscKeeper.HasConsumerFinalityProvider(ctx, fp.BtcPk) {
		return bstypes.ErrFpRegistered
	}
	// verify that the consumer is registered within the btcstkconsumer module
	if !k.BscKeeper.IsConsumerRegistered(ctx, consumerID) {
		return bstypes.ErrConsumerIDNotRegistered
	}
	// set the finality provider in btcstkconsumer module
	k.BscKeeper.SetConsumerFinalityProvider(ctx, fp)

	// record the event in btc staking consumer event store
	if err := k.AddBTCStakingConsumerEvent(ctx, consumerID, bstypes.CreateNewFinalityProviderEvent(fp)); err != nil {
		return err
	}

	return nil
}

// getBTCConsumerDelegatorDelegationIndex gets the BTC delegation index with a given BTC PK under a given finality provider
func (k Keeper) getBTCConsumerDelegatorDelegationIndex(ctx context.Context, fpBTCPK *bbn.BIP340PubKey, delBTCPK *bbn.BIP340PubKey) *bstypes.BTCDelegatorDelegationIndex {
	// ensure the finality provider exists
	if !k.BscKeeper.HasConsumerFinalityProvider(ctx, fpBTCPK) {
		return nil
	}

	delBTCPKBytes := delBTCPK.MustMarshal()
	store := k.btcConsumerDelegatorStore(ctx, fpBTCPK)

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

func (k Keeper) setBTCConsumerDelegatorDelegationIndex(ctx context.Context, fpBTCPK *bbn.BIP340PubKey, delBTCPK *bbn.BIP340PubKey, btcDelIndex *bstypes.BTCDelegatorDelegationIndex) {
	store := k.btcConsumerDelegatorStore(ctx, fpBTCPK)
	btcDelIndexBytes := k.cdc.MustMarshal(btcDelIndex)
	store.Set(*delBTCPK, btcDelIndexBytes)
}

// getBTCConsumerDelegatorDelegations gets the BTC delegations with a given BTC PK under a given finality provider
func (k Keeper) getBTCConsumerDelegatorDelegations(ctx context.Context, fpBTCPK *bbn.BIP340PubKey, delBTCPK *bbn.BIP340PubKey) *bstypes.BTCDelegatorDelegations {
	btcDelIndex := k.getBTCConsumerDelegatorDelegationIndex(ctx, fpBTCPK, delBTCPK)
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

// btcConsumerDelegatorStore returns the KVStore of the BTC Consumer delegators
// prefix: BTCConsumerDelegatorKey || finality provider's Bitcoin secp256k1 PK
// key: delegator's Bitcoin secp256k1 PK
// value: BTCDelegatorDelegationIndex (a list of BTCDelegations' staking tx hashes)
func (k Keeper) btcConsumerDelegatorStore(ctx context.Context, fpBTCPK *bbn.BIP340PubKey) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	delegationStore := prefix.NewStore(storeAdapter, bstypes.BTCConsumerDelegatorKey)
	return prefix.NewStore(delegationStore, fpBTCPK.MustMarshal())
}
