package v2

import (
	"context"
	"fmt"

	"cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

var (
	// (hex 0x10 = decimal 16) key prefix for height to version map
	// which collides with FpBbnAddrKey that is prefix decimal 16
	OldHeightToVersionMapKey = []byte{0x10}
)

// Keeper the expected keeper interface to perform the migration
type Keeper interface {
	SetHeightToVersionMap(ctx context.Context, p *types.HeightToVersionMap) error
	GetAllParams(ctx context.Context) []*types.Params
}

// MigrateStore performs in-place store migrations.
// Migration moves HeightToVersionMap from the old key (hex 0x10) to the new key (decimal 10)
// to resolve the key prefix collision with FpBbnAddrKey.
// If the old key is empty, it rebuilds the map from existing stored params.
func MigrateStore(
	ctx sdk.Context,
	cdc codec.BinaryCodec,
	store store.KVStore,
	k Keeper,
) error {
	bz, err := store.Get(OldHeightToVersionMapKey)
	if err != nil {
		return fmt.Errorf("failed to get height to version map key using the old prefix hex 0x10: %s", err.Error())
	}

	if bz == nil {
		heightToVersionMap, err := rebuildHeightToVersionMap(ctx, k)
		if err != nil {
			return fmt.Errorf("failed to rebuild HeightToVersionMap from existing params: %s", err.Error())
		}
		return k.SetHeightToVersionMap(ctx, heightToVersionMap)
	}

	var heightToVersionMap types.HeightToVersionMap
	if err := cdc.Unmarshal(bz, &heightToVersionMap); err != nil {
		return fmt.Errorf("failed to unmarshal HeightToVersionMap: %s", err.Error())
	}
	store.Delete(OldHeightToVersionMapKey)

	return k.SetHeightToVersionMap(ctx, &heightToVersionMap)
}

// rebuildHeightToVersionMap reconstructs the HeightToVersionMap from all existing
// stored params. Each param version maps to its BtcActivationHeight.
func rebuildHeightToVersionMap(ctx context.Context, k Keeper) (*types.HeightToVersionMap, error) {
	allParams := k.GetAllParams(ctx)
	if len(allParams) == 0 {
		return nil, fmt.Errorf("no stored params found to rebuild HeightToVersionMap")
	}

	heightToVersionMap := types.NewHeightToVersionMap()
	for i, params := range allParams {
		if err := heightToVersionMap.AddNewPair(uint64(params.BtcActivationHeight), uint32(i)); err != nil {
			return nil, err
		}
	}

	return heightToVersionMap, nil
}
