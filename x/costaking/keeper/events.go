package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	proto "github.com/cosmos/gogoproto/proto"
)

func (k Keeper) EmitEventCostakersAddRewards(ctx context.Context, addRewards sdk.Coins, currRwd types.CurrentRewards) {
	evt := types.NewEventCostakersAddRewards(addRewards, currRwd)
	k.emitTypedEventWithLog(ctx, &evt)
}

// emitTypedEventWithLog emits an event and logs if it errors.
func (k Keeper) emitTypedEventWithLog(ctx context.Context, evt proto.Message) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err := sdkCtx.EventManager().EmitTypedEvent(evt); err != nil {
		k.Logger(sdkCtx).Error(
			"failed to emit event",
			"type", evt.String(),
			"reason", err.Error(),
		)
	}
}
