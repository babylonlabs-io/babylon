package v3

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

// Keeper the expected keeper interface to perform the migration
type Keeper interface {
	GetVotingPowerDistCache(ctx context.Context, height uint64) *types.VotingPowerDistCache
	SetVotingPowerDistCache(ctx context.Context, height uint64, dc *types.VotingPowerDistCache)
}

// MigrateStore performs in-place store migrations.
// Migration adds the default value for the new FpPortion param
func MigrateStore(
	ctx sdk.Context,
	k Keeper,
) error {
	height := uint64(sdk.UnwrapSDKContext(ctx).HeaderInfo().Height)
	dc := k.GetVotingPowerDistCache(ctx, height-1)
	if dc == nil {
		return nil
	}

	for _, fp := range dc.FinalityProviders {
		fp.Status = fp.FpStatusCalculated()
	}

	k.SetVotingPowerDistCache(ctx, height, dc)
	return nil
}
