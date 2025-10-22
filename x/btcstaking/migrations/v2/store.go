package v2

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

// Keeper exposes minimal methods used by the migration.
type Keeper interface {
	GetParamsWithVersion(ctx context.Context) types.StoredParams
	OverwriteParamsAtVersion(ctx context.Context, version uint32, params types.Params) error
}

// MigrateStore performs in-place store migrations from v1 to v2.
func MigrateStore(ctx sdk.Context, k Keeper) error {
	dp := types.DefaultParams()

	storedParams := k.GetParamsWithVersion(ctx)
	currParams := storedParams.Params

	currParams.MaxStakerQuorum = dp.MaxStakerQuorum
	currParams.MaxStakerNum = dp.MaxStakerNum

	return k.OverwriteParamsAtVersion(ctx, storedParams.Version, currParams)
}
