package coostaking

import (
	"context"

	"github.com/babylonlabs-io/babylon/v4/x/coostaking/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func InitGenesis(ctx context.Context, k keeper.Keeper, genState types.GenesisState) error {
	// // stateless validations
	// if err := genState.Validate(); err != nil {
	// 	panic(err)
	// }

	return k.InitGenesis(ctx, genState)
}
