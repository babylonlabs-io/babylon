package btcstaking

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func InitGenesis(ctx context.Context, k keeper.Keeper, gs types.GenesisState) {
	if err := gs.Validate(); err != nil {
		panic(err)
	}

	p := k.BtccKeeper().GetParams(ctx)
	btcConfirmationDepth := p.BtcConfirmationDepth

	if gs.LargestBtcReorg != nil && gs.LargestBtcReorg.BlockDiff >= btcConfirmationDepth {
		panic(fmt.Sprintf("genesis LargestBtcReOrg block_diff %d must be less than btc_confirmation_depth %d to prevent immediate chain halt",
			gs.LargestBtcReorg.BlockDiff, btcConfirmationDepth))
	}

	if err := k.InitGenesis(ctx, gs); err != nil {
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
