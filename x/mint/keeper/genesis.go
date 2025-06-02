package keeper

import (
	"github.com/babylonlabs-io/babylon/v4/x/mint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis initializes the x/mint store with data from the genesis state.
func (k Keeper) InitGenesis(ctx sdk.Context, ak types.AccountKeeper, gs *types.GenesisState) {
	if gs.Minter == nil {
		dm := types.DefaultMinter()
		gs.Minter = &dm
	}
	if err := k.SetMinter(ctx, *gs.Minter); err != nil {
		panic(err)
	}

	if gs.GenesisTime == nil {
		// If no genesis time, use the block time supplied in `InitChain`
		blockTime := ctx.BlockTime()
		gs.GenesisTime = &types.GenesisTime{
			GenesisTime: &blockTime,
		}
	}
	if err := k.SetGenesisTime(ctx, *gs.GenesisTime); err != nil {
		panic(err)
	}
	// Although ak.GetModuleAccount appears to be a no-op, it actually creates a
	// new module account in the x/auth account store if it doesn't exist. See
	// the x/auth keeper for more details.
	ak.GetModuleAccount(ctx, types.ModuleName)
}

// ExportGenesis returns a x/mint GenesisState for the given context.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	minter := k.GetMinter(ctx)
	genTime := k.GetGenesisTime(ctx)
	return types.NewGenesisState(minter, genTime)
}
