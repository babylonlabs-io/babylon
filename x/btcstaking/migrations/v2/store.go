package v2

import (
	"context"

	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// MigrateStore performs in-place store migrations.
// Migration sets every parameter's max finality providers to 1.
func MigrateStore(
	ctx sdk.Context,
	s storetypes.KVStore,
	c codec.BinaryCodec,
	modifyParams func(ctx context.Context, p *types.Params) error) error {
	paramsStore := prefix.NewStore(s, types.ParamsKey)

	iter := paramsStore.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var sp types.StoredParams
		c.MustUnmarshal(iter.Value(), &sp)
		if err := modifyParams(ctx, &sp.Params); err != nil {
			return err
		}
		paramsStore.Set(iter.Key(), c.MustMarshal(&sp))
	}

	return nil
}
