package btcdistribution

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/x/btcdistribution/keeper"
	abci "github.com/cometbft/cometbft/abci/types"
)

func BeginBlocker(ctx context.Context, k keeper.Keeper) error {
	return nil
}

func EndBlocker(ctx context.Context, k keeper.Keeper) ([]abci.ValidatorUpdate, error) {
	err := k.EndBlocker(ctx)
	if err != nil {
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		k.Logger(sdkCtx).Error(fmt.Sprintf("err in endBlocker %s", err.Error()))
	}
	return []abci.ValidatorUpdate{}, err
}
