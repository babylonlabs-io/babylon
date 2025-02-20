package incentive

import (
	"context"

	"github.com/babylonlabs-io/babylon/x/incentive/keeper"
	"github.com/babylonlabs-io/babylon/x/incentive/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func InitGenesis(ctx context.Context, k keeper.Keeper, genState types.GenesisState) {
	// stateless validations
	if err := genState.Validate(); err != nil {
		panic(err)
	}

	if err := k.InitGenesis(ctx, genState); err != nil {
		panic(err)
	}
}

// ExportGenesis returns the module's exported genesis
func ExportGenesis(ctx context.Context, k keeper.Keeper) *types.GenesisState {
	gs, err := k.ExportGenesis(ctx)
	if err != nil {
		panic(err)
	}
	return gs
}
