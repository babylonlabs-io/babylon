package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/x/incentive/types"
)

// InitGenesis initializes the keeper state from a provided initial genesis state.
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {

	return k.SetParams(ctx, gs.Params)
}
