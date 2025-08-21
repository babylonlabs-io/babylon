package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
)

// InitGenesis performs stateful validations and initializes the keeper state from a provided initial genesis state.
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	return k.SetParams(ctx, gs.Params)
}
