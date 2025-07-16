package keeper

import (
	"context"
	"errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"

	checkpointingtypes "github.com/babylonlabs-io/babylon/v3/x/checkpointing/types"
	epochingtypes "github.com/babylonlabs-io/babylon/v3/x/epoching/types"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
)

type Hooks struct {
	k Keeper
}

// ensures Hooks implements ClientHooks interfaces
var _ checkpointingtypes.CheckpointingHooks = Hooks{}
var _ epochingtypes.EpochingHooks = Hooks{}

func (k Keeper) Hooks() Hooks { return Hooks{k} }

func (h Hooks) AfterEpochEnds(ctx context.Context, epoch uint64) {
	// upon an epoch has ended, index the current chain info for each Consumer
	// TODO: do this together when epoch is sealed?
	for _, consumerID := range h.k.GetAllConsumerIDs(ctx) {
		h.k.recordEpochChainInfo(ctx, consumerID, epoch)
	}
}

func (h Hooks) AfterRawCheckpointSealed(ctx context.Context, epoch uint64) error {
	// upon a raw checkpoint is sealed, index the current chain info for each consumer,
	// and generate/save the proof that the epoch is sealed
	h.k.recordEpochChainInfoProofs(ctx, epoch)
	h.k.recordSealedEpochProof(ctx, epoch)
	return nil
}

// AfterRawCheckpointFinalized is triggered upon an epoch has been finalised
func (h Hooks) AfterRawCheckpointFinalized(ctx context.Context, epoch uint64) error {
	headersToBroadcast := h.k.getHeadersToBroadcast(ctx)

	// send BTC timestamp to all open channels with ZoneConcierge
	if err := h.k.BroadcastBTCTimestamps(ctx, epoch, headersToBroadcast); err != nil {
		h.handleHookBroadcastError(ctx, "BroadcastBTCTimestamps", err)
	}

	// only update the segment if we have broadcasted some headers
	if len(headersToBroadcast) > 0 {
		h.k.setLastSentSegment(ctx, &types.BTCChainSegment{
			BtcHeaders: headersToBroadcast,
		})
	}
	return nil
}

// handleHookBroadcastError provides structured error handling for IBC broadcast operations in hooks
func (h Hooks) handleHookBroadcastError(ctx context.Context, operation string, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	if errors.Is(err, clienttypes.ErrClientNotActive) {
		h.k.Logger(sdkCtx).Info("IBC client is not active, skipping broadcast in hook",
			"operation", operation,
			"error", err.Error(),
		)
		return
	}

	h.k.Logger(sdkCtx).Error("failed to broadcast IBC packet in hook, continuing operation",
		"operation", operation,
		"error", err.Error(),
	)
}

// Other unused hooks

func (h Hooks) AfterBlsKeyRegistered(ctx context.Context, valAddr sdk.ValAddress) error { return nil }
func (h Hooks) AfterRawCheckpointConfirmed(ctx context.Context, epoch uint64) error     { return nil }
func (h Hooks) AfterRawCheckpointForgotten(ctx context.Context, ckpt *checkpointingtypes.RawCheckpoint) error {
	return nil
}
func (h Hooks) AfterRawCheckpointBlsSigVerified(ctx context.Context, ckpt *checkpointingtypes.RawCheckpoint) error {
	return nil
}

func (h Hooks) AfterEpochBegins(ctx context.Context, epoch uint64)                          {}
func (h Hooks) BeforeSlashThreshold(ctx context.Context, valSet epochingtypes.ValidatorSet) {}
