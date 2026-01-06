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
	// which colides with FpBbnAddrKey that is prefix decimal 16
	OldHeightToVersionMapKey = []byte{0x10}
)

// Keeper the expected keeper interface to perform the migration
type Keeper interface {
	SetHeightToVersionMap(ctx context.Context, p *types.HeightToVersionMap) error
}

// MigrateStore performs in-place store migrations.
// Migration adds the default value for the new FpPortion param
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

	var oldHeightToVersionMap types.HeightToVersionMap
	err = cdc.Unmarshal(bz, &oldHeightToVersionMap)
	if err != nil {
		return fmt.Errorf("failed to unmarshal HeightToVersionMap: %s", err.Error())
	}

	return k.SetHeightToVersionMap(ctx, &oldHeightToVersionMap)
}
