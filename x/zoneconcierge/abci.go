package zoneconcierge

import (
	"context"
	"errors"
	"time"

	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/keeper"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
)

// BeginBlocker sends a pending packet for every channel upon each new block,
// so that the relayer is kept awake to relay headers
func BeginBlocker(ctx context.Context, k keeper.Keeper) error {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), telemetry.MetricKeyBeginBlocker)
	return nil
}

func EndBlocker(ctx context.Context, k keeper.Keeper) ([]abci.ValidatorUpdate, error) {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), telemetry.MetricKeyEndBlocker)

	// Retrieve the open ZC channels
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	chs := k.GetAllOpenChannels(sdkCtx)

	// Handle BTC headers broadcast with structured error handling
	if err := k.BroadcastBTCHeaders(ctx, chs); err != nil {
		handleBroadcastError(ctx, k, "BroadcastBTCHeaders", err)
	}

	// Handle BTC staking consumer events broadcast with structured error handling
	if err := k.BroadcastBTCStakingConsumerEvents(ctx, chs); err != nil {
		handleBroadcastError(ctx, k, "BroadcastBTCStakingConsumerEvents", err)
	}

	return []abci.ValidatorUpdate{}, nil
}

// handleBroadcastError provides structured error handling for IBC broadcast operations
// It logs errors but doesn't panic, preventing chain halts
func handleBroadcastError(ctx context.Context, k keeper.Keeper, operation string, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	if errors.Is(err, clienttypes.ErrClientNotActive) {
		k.Logger(sdkCtx).Info("IBC client is not active, skipping broadcast",
			"operation", operation,
			"error", err.Error(),
		)
		return
	}

	k.Logger(sdkCtx).Error("failed to broadcast IBC packet, continuing operation",
		"operation", operation,
		"error", err.Error(),
	)
}
