package keeper

import (
	"context"
	"encoding/binary"
	"fmt"
	"sort"

	"cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"

	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

// cosmos-sdk does not have utils for uint32
func uint32ToBytes(v uint32) []byte {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], v)
	return buf[:]
}
func uint32FromBytes(b []byte) (uint32, error) {
	if len(b) != 4 {
		return 0, fmt.Errorf("invalid uint32 bytes length: %d", len(b))
	}

	return binary.BigEndian.Uint32(b), nil
}

func mustUint32FromBytes(b []byte) uint32 {
	v, err := uint32FromBytes(b)
	if err != nil {
		panic(err)
	}

	return v
}

func (k Keeper) paramsStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.ParamsKey)
}

func (k Keeper) nextParamsVersion(ctx context.Context) uint32 {
	paramsStore := k.paramsStore(ctx)
	it := paramsStore.ReverseIterator(nil, nil)
	defer it.Close()

	if !it.Valid() {
		return 0
	}

	return mustUint32FromBytes(it.Key()) + 1
}

func (k Keeper) getLastParams(ctx context.Context) *types.StoredParams {
	paramsStore := k.paramsStore(ctx)
	it := paramsStore.ReverseIterator(nil, nil)
	defer it.Close()

	if !it.Valid() {
		return nil
	}
	var sp types.StoredParams
	k.cdc.MustUnmarshal(it.Value(), &sp)
	return &sp
}

// SetParams sets the x/btcstaking module parameters.
func (k Keeper) SetParams(ctx context.Context, p types.Params) error {
	if err := p.Validate(); err != nil {
		return err
	}

	heightToVersionMap := k.GetHeightToVersionMap(ctx)

	if heightToVersionMap == nil {
		heightToVersionMap = types.NewHeightToVersionMap()
	}

	nextVersion := k.nextParamsVersion(ctx)

	err := heightToVersionMap.AddNewPair(uint64(p.BtcActivationHeight), nextVersion)
	if err != nil {
		return err
	}

	paramsStore := k.paramsStore(ctx)

	sp := types.StoredParams{
		Params:  p,
		Version: nextVersion,
	}

	paramsStore.Set(uint32ToBytes(nextVersion), k.cdc.MustMarshal(&sp))
	return k.SetHeightToVersionMap(ctx, heightToVersionMap)
}

func (k Keeper) OverwriteParamsAtVersion(ctx context.Context, v uint32, p types.Params) error {
	if err := p.Validate(); err != nil {
		return fmt.Errorf("cannot overwrite params at version %d: %w", v, err)
	}

	paramsStore := k.paramsStore(ctx)
	sp := types.StoredParams{
		Params:  p,
		Version: v,
	}

	heightToVersionMap := k.GetHeightToVersionMap(ctx)
	if heightToVersionMap == nil {
		heightToVersionMap = types.NewHeightToVersionMap()
	}

	// makes sure it is ordered by the version
	sort.Slice(heightToVersionMap.Pairs, func(i, j int) bool {
		return heightToVersionMap.Pairs[i].Version < heightToVersionMap.Pairs[j].Version
	})

	if v >= uint32(len(heightToVersionMap.Pairs)) {
		if err := heightToVersionMap.AddNewPair(uint64(p.BtcActivationHeight), v); err != nil {
			return err
		}
	} else {
		heightToVersionMap.Pairs[v] = types.NewHeightVersionPair(uint64(p.BtcActivationHeight), v)
	}

	paramsStore.Set(uint32ToBytes(v), k.cdc.MustMarshal(&sp))
	return k.SetHeightToVersionMap(ctx, heightToVersionMap)
}

func (k Keeper) GetAllParams(ctx context.Context) []*types.Params {
	paramsStore := k.paramsStore(ctx)
	it := paramsStore.Iterator(nil, nil)
	defer it.Close()

	var p []*types.Params
	for ; it.Valid(); it.Next() {
		var sp types.StoredParams
		k.cdc.MustUnmarshal(it.Value(), &sp)
		p = append(p, &sp.Params)
	}

	return p
}

func (k Keeper) GetParamsByVersion(ctx context.Context, v uint32) *types.Params {
	paramsStore := k.paramsStore(ctx)
	spBytes := paramsStore.Get(uint32ToBytes(v))
	if len(spBytes) == 0 {
		return nil
	}

	var sp types.StoredParams
	k.cdc.MustUnmarshal(spBytes, &sp)
	return &sp.Params
}

func mustGetLastParams(ctx context.Context, k Keeper) types.StoredParams {
	sp := k.getLastParams(ctx)
	if sp == nil {
		panic("last params not found")
	}

	return *sp
}

// GetParams returns the latest x/btcstaking module parameters.
func (k Keeper) GetParams(ctx context.Context) types.Params {
	return mustGetLastParams(ctx, k).Params
}

func (k Keeper) GetParamsWithVersion(ctx context.Context) types.StoredParams {
	return mustGetLastParams(ctx, k)
}

// MinCommissionRate returns the minimal commission rate of finality providers
func (k Keeper) MinCommissionRate(ctx context.Context) math.LegacyDec {
	return k.GetParams(ctx).MinCommissionRate
}

func (k Keeper) SetHeightToVersionMap(ctx context.Context, p *types.HeightToVersionMap) error {
	if err := p.Validate(); err != nil {
		return err
	}

	return k.heightToVersionMap.Set(ctx, *p)
}

// GetHeightToVersionMap returns the height to version map which includes the start height
// in which the parameters are activated and the version number
func (k Keeper) GetHeightToVersionMap(ctx context.Context) *types.HeightToVersionMap {
	exists, err := k.heightToVersionMap.Has(ctx)
	if err != nil {
		panic(fmt.Errorf("must get height to version map in Has: %w", err))
	}
	if !exists {
		return nil
	}

	heightToVersionMap, err := k.heightToVersionMap.Get(ctx)
	if err != nil {
		panic(err)
	}
	return &heightToVersionMap
}

func (k Keeper) GetParamsForBtcHeight(ctx context.Context, height uint64) (*types.Params, uint32, error) {
	heightToVersionMap := k.GetHeightToVersionMap(ctx)
	if heightToVersionMap == nil {
		panic("height to version map not found")
	}

	version, err := heightToVersionMap.GetVersionForHeight(height)
	if err != nil {
		return nil, 0, err
	}

	return k.GetParamsByVersion(ctx, version), version, nil
}
