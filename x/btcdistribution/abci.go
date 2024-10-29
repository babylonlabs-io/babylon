package btcdistribution

import (
	"context"

	"github.com/babylonlabs-io/babylon/x/btcdistribution/keeper"
	abci "github.com/cometbft/cometbft/abci/types"
)

func BeginBlocker(ctx context.Context, k keeper.Keeper) error {
	return nil
}

func EndBlocker(ctx context.Context, k keeper.Keeper) ([]abci.ValidatorUpdate, error) {
	return []abci.ValidatorUpdate{}, k.EndBlocker(ctx)
}
