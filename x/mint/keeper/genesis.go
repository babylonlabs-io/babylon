package keeper

import (
	"github.com/babylonlabs-io/babylon/x/mint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis initializes the x/mint store with data from the genesis state.
func (k Keeper) InitGenesis(ctx sdk.Context, ak types.AccountKeeper, data *types.GenesisState) {
	minter := types.DefaultMinter()
	minter.BondDenom = data.BondDenom
	if err := k.SetMinter(ctx, minter); err != nil {
		panic(err)
	}
	// override the genesis time with the actual genesis time supplied in `InitChain`
	blockTime := ctx.BlockTime()
	gt := types.GenesisTime{
		GenesisTime: &blockTime,
	}
	if err := k.SetGenesisTime(ctx, gt); err != nil {
		panic(err)
	}
	// Although ak.GetModuleAccount appears to be a no-op, it actually creates a
	// new module account in the x/auth account store if it doesn't exist. See
	// the x/auth keeper for more details.
	ak.GetModuleAccount(ctx, types.ModuleName)
}

// ExportGenesis returns a x/mint GenesisState for the given context.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	bondDenom := k.GetMinter(ctx).BondDenom
	return types.NewGenesisState(bondDenom)
}
