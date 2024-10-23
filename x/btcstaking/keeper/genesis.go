package keeper

import (
	"context"
	"fmt"
	"math"

	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	// save all past params versions
	for _, p := range gs.Params {
		params := p
		if err := k.SetParams(ctx, *params); err != nil {
			return err
		}
	}

	for _, fp := range gs.FinalityProviders {
		k.setFinalityProvider(ctx, fp)
	}

	for _, btcDel := range gs.BtcDelegations {
		k.setBTCDelegation(ctx, btcDel)
	}

	for _, blocks := range gs.BlockHeightChains {
		k.setBlockHeightChains(ctx, blocks)
	}

	for _, del := range gs.BtcDelegators {
		k.setBTCDelegatorDelegationIndex(ctx, del.FpBtcPk, del.DelBtcPk, del.Idx)
	}

	// Events are generated on block `N` to be processed at block `N+1`
	// When ExportGenesis is called the node already stopped at block N.
	// In this case the events on the state would refer to the block `N+1`
	// Since InitGenesis occurs before BeginBlock, the genesis state would be properly
	// stored in the KV store for when BeginBlock process the events.
	for _, evt := range gs.Events {
		if err := k.setEventIdx(ctx, evt); err != nil {
			return err
		}
	}

	return nil
}

// ExportGenesis returns the module's exported genesis
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	fps, err := k.finalityProviders(ctx)
	if err != nil {
		return nil, err
	}

	dels, err := k.btcDelegations(ctx)
	if err != nil {
		return nil, err
	}

	btcDels, err := k.btcDelegators(ctx)
	if err != nil {
		return nil, err
	}

	evts, err := k.eventIdxs(ctx)
	if err != nil {
		return nil, err
	}

	return &types.GenesisState{
		Params:            k.GetAllParams(ctx),
		FinalityProviders: fps,
		BtcDelegations:    dels,
		BlockHeightChains: k.blockHeightChains(ctx),
		BtcDelegators:     btcDels,
		Events:            evts,
	}, nil
}

func (k Keeper) finalityProviders(ctx context.Context) ([]*types.FinalityProvider, error) {
	fps := make([]*types.FinalityProvider, 0)
	iter := k.finalityProviderStore(ctx).Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var fp types.FinalityProvider
		if err := fp.Unmarshal(iter.Value()); err != nil {
			return nil, err
		}
		fps = append(fps, &fp)
	}

	return fps, nil
}

func (k Keeper) btcDelegations(ctx context.Context) ([]*types.BTCDelegation, error) {
	dels := make([]*types.BTCDelegation, 0)
	iter := k.btcDelegationStore(ctx).Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var del types.BTCDelegation
		if err := del.Unmarshal(iter.Value()); err != nil {
			return nil, err
		}
		dels = append(dels, &del)
	}

	return dels, nil
}

func (k Keeper) blockHeightChains(ctx context.Context) []*types.BlockHeightBbnToBtc {
	iter := k.btcHeightStore(ctx).Iterator(nil, nil)
	defer iter.Close()

	blocks := make([]*types.BlockHeightBbnToBtc, 0)
	for ; iter.Valid(); iter.Next() {
		blkHeightUint64 := sdk.BigEndianToUint64(iter.Value())
		if blkHeightUint64 > math.MaxUint32 {
			panic("block height value in storage is larger than math.MaxUint64")
		}
		blocks = append(blocks, &types.BlockHeightBbnToBtc{
			BlockHeightBbn: sdk.BigEndianToUint64(iter.Key()),
			BlockHeightBtc: uint32(blkHeightUint64),
		})
	}

	return blocks
}

func (k Keeper) btcDelegators(ctx context.Context) ([]*types.BTCDelegator, error) {
	iter := k.btcDelegatorStore(ctx).Iterator(nil, nil)
	defer iter.Close()

	dels := make([]*types.BTCDelegator, 0)
	for ; iter.Valid(); iter.Next() {
		fpBTCPK, delBTCPK, err := parseBIP340PubKeysFromStoreKey(iter.Key())
		if err != nil {
			return nil, err
		}
		var btcDelIndex types.BTCDelegatorDelegationIndex
		if err := btcDelIndex.Unmarshal(iter.Value()); err != nil {
			return nil, err
		}

		dels = append(dels, &types.BTCDelegator{
			Idx:      &btcDelIndex,
			FpBtcPk:  fpBTCPK,
			DelBtcPk: delBTCPK,
		})
	}

	return dels, nil
}

// eventIdxs sets an event into the store.
func (k Keeper) eventIdxs(
	ctx context.Context,
) ([]*types.EventIndex, error) {
	iter := k.powerDistUpdateEventStore(ctx).Iterator(nil, nil)
	defer iter.Close()

	evts := make([]*types.EventIndex, 0)
	for ; iter.Valid(); iter.Next() {
		blkHeight, idx, err := parseUintsFromStoreKey(iter.Key())
		if err != nil {
			return nil, err
		}

		var evt types.EventPowerDistUpdate
		if err := evt.Unmarshal(iter.Value()); err != nil {
			return nil, err
		}

		evts = append(evts, &types.EventIndex{
			Idx:            idx,
			BlockHeightBtc: blkHeight,
			Event:          &evt,
		})
	}

	return evts, nil
}

func (k Keeper) setBlockHeightChains(ctx context.Context, blocks *types.BlockHeightBbnToBtc) {
	store := k.btcHeightStore(ctx)
	store.Set(sdk.Uint64ToBigEndian(blocks.BlockHeightBbn), sdk.Uint64ToBigEndian(uint64(blocks.BlockHeightBtc)))
}

// setEventIdx sets an event into the store.
func (k Keeper) setEventIdx(
	ctx context.Context,
	evt *types.EventIndex,
) error {
	store := k.powerDistUpdateEventBtcHeightStore(ctx, evt.BlockHeightBtc)

	bz, err := evt.Event.Marshal()
	if err != nil {
		return err
	}
	store.Set(sdk.Uint64ToBigEndian(evt.Idx), bz)

	return nil
}

// parseUintsFromStoreKey expects to receive a key with
// BigEndianUint64(blkHeight) || BigEndianUint64(Idx)
func parseUintsFromStoreKey(key []byte) (blkHeight uint32, idx uint64, err error) {
	sizeBigEndian := 8
	if len(key) < sizeBigEndian*2 {
		return 0, 0, fmt.Errorf("key not long enough to parse two uint64: %s", key)
	}

	blkHeightUint64 := sdk.BigEndianToUint64(key[:sizeBigEndian])
	if blkHeightUint64 > math.MaxUint32 {
		return 0, 0, fmt.Errorf("block height %d is larger than math.MaxUint32", blkHeightUint64)
	}
	idx = sdk.BigEndianToUint64(key[sizeBigEndian:])
	return uint32(blkHeightUint64), idx, nil
}

// parseBIP340PubKeysFromStoreKey expects to receive a key with
// BIP340PubKey(fpBTCPK) || BIP340PubKey(delBTCPK)
func parseBIP340PubKeysFromStoreKey(key []byte) (fpBTCPK, delBTCPK *bbn.BIP340PubKey, err error) {
	if len(key) < bbn.BIP340PubKeyLen*2 {
		return nil, nil, fmt.Errorf("key not long enough to parse two BIP340PubKey: %s", key)
	}

	fpBTCPK, err = bbn.NewBIP340PubKey(key[:bbn.BIP340PubKeyLen])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse pub key from key %w: %w", bbn.ErrUnmarshal, err)
	}

	delBTCPK, err = bbn.NewBIP340PubKey(key[bbn.BIP340PubKeyLen:])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse pub key from key %w: %w", bbn.ErrUnmarshal, err)
	}

	return fpBTCPK, delBTCPK, nil
}
