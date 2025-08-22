package coostaking

import (
	"context"
	"encoding/json"

	"cosmossdk.io/core/appmodule"
	"github.com/babylonlabs-io/babylon/v4/x/coostaking/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func InitGenesis(ctx context.Context, k keeper.Keeper, gs appmodule.GenesisSource) error {
	reader, err := gs(types.ModuleName)
	if err != nil {
		return err
	}
	defer reader.Close()

	var genState types.GenesisState
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&genState); err != nil {
		return err
	}

	// stateless validations
	if err := genState.Validate(); err != nil {
		return err
	}

	return k.InitGenesis(ctx, genState)
}
