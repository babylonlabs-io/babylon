package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
)

// HandleHeaderWithValidCommit handles a Consumer header with a valid QC
func (k Keeper) HandleHeaderWithValidCommit(ctx context.Context, txHash []byte, header *types.HeaderInfo, isOnFork bool) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Check if the consumer is registered before processing its header
	if !k.HasConsumer(ctx, header.ClientId) {
		k.Logger(sdkCtx).Debug("ignoring header from unregistered consumer",
			"consumer_id", header.ClientId,
			"height", header.Height,
		)
		return
	}

	babylonHeader := sdkCtx.HeaderInfo()
	indexedHeader := types.IndexedHeader{
		ConsumerId:          header.ClientId,
		Hash:                header.AppHash,
		Height:              header.Height,
		Time:                &header.Time,
		BabylonHeaderHash:   babylonHeader.AppHash,
		BabylonHeaderHeight: uint64(babylonHeader.Height),
		BabylonEpoch:        k.GetEpoch(ctx).EpochNumber,
		BabylonTxHash:       txHash,
	}

	k.Logger(sdkCtx).Debug("found new IBC header", "header", indexedHeader)

	if isOnFork {
		// Log the fork event
		k.Logger(sdkCtx).Info(
			"fork detected",
			"consumer_id", indexedHeader.ConsumerId,
			"height", indexedHeader.Height,
			"babylon_height", indexedHeader.BabylonHeaderHeight,
		)
	} else {
		// Update header if it's newer than existing one
		existingHeader := k.GetLatestEpochHeader(ctx, indexedHeader.ConsumerId)
		if existingHeader == nil || indexedHeader.Height > existingHeader.Height {
			k.SetLatestEpochHeader(ctx, indexedHeader.ConsumerId, &indexedHeader)
		}
	}
}
