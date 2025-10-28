package v2

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

// Keeper exposes minimal methods used by the migration.
type Keeper interface {
	GetAllStoredParams(ctx context.Context) []*types.StoredParams
	OverwriteParamsAtVersion(ctx context.Context, version uint32, params types.Params) error
}

// MigrateStore performs in-place store migrations from v1 to v2.
func MigrateStore(ctx sdk.Context, k Keeper) error {
	allParams := k.GetAllStoredParams(ctx)

	for _, params := range allParams {
		params.Params.MaxStakerNum = 1
		params.Params.MaxStakerQuorum = 1

		if err := k.OverwriteParamsAtVersion(ctx, params.Version, params.Params); err != nil {
			return err
		}
	}

	return nil
}
