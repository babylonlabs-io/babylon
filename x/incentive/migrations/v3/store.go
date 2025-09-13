package v3

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

// Keeper the expected keeper interface to perform the migration
type Keeper interface {
	SetParams(ctx context.Context, p types.Params) error
	GetParams(ctx context.Context) (p types.Params)
}

// MigrateStore performs in-place store migrations.
// Migration adds the default value for the new FpPortion param
func MigrateStore(
	ctx sdk.Context,
	k Keeper,
) error {
	dp := types.DefaultParams()
	currParams := k.GetParams(ctx)
	currParams.FpPortion = dp.FpPortion
	return k.SetParams(ctx, currParams)
}
