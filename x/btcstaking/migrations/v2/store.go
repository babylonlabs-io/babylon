package v2

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

// Keeper exposes minimal methods used by the migration.
type Keeper interface {
	SetParams(ctx context.Context, params types.Params) error
	GetParams(ctx context.Context) types.Params
}

// MigrateStore performs in-place store migrations from v1 to v2.
// Migration adds the default value for the new MaxStakerQuorum and MaxStakerNum
func MigrateStore(ctx sdk.Context, k Keeper) error {
	dp := types.DefaultParams()
	currParams := k.GetParams(ctx)

	currParams.MaxStakerQuorum = dp.MaxStakerQuorum
	currParams.MaxStakerNum = dp.MaxStakerNum

	return k.SetParams(ctx, currParams)
}
