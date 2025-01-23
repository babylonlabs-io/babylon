package zoneconcierge

import (
	"context"
	"github.com/babylonlabs-io/babylon/x/zoneconcierge/keeper"
	"github.com/babylonlabs-io/babylon/x/zoneconcierge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func InitGenesis(ctx context.Context, k keeper.Keeper, genState types.GenesisState) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	// set params for this module
	if err := k.SetParams(ctx, genState.Params); err != nil {
		panic(err)
	}

	k.SetPort(ctx, genState.PortId)
	// Only try to bind to port if it is not already bound, since we may already own
	// port capability from capability InitGenesis
	if !k.IsBound(sdkCtx, genState.PortId) {
		// module binds to the port on InitChain
		// and claims the returned capability
		err := k.BindPort(sdkCtx, genState.PortId)
		if err != nil {
			panic("could not claim port capability: " + err.Error())
		}
	}
}

// ExportGenesis returns the module's exported genesis
func ExportGenesis(ctx context.Context, k keeper.Keeper) *types.GenesisState {
	genesis := types.DefaultGenesis()
	genesis.Params = k.GetParams(ctx)
	genesis.PortId = k.GetPort(ctx)
	return genesis
}
