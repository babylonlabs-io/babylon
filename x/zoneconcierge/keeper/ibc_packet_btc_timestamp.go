package keeper

import (
	"context"
	"errors"
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"

	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btcctypes "github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/types"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	checkpointingtypes "github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
)

// finalizedInfo is a private struct that stores metadata and proofs
// identical to all BTC timestamps in the same epoch
type finalizedInfo struct {
	EpochInfo           *epochingtypes.Epoch
	RawCheckpoint       *checkpointingtypes.RawCheckpoint
	BTCSubmissionKey    *btcctypes.SubmissionKey
	ProofEpochSealed    *types.ProofEpochSealed
	ProofEpochSubmitted []*btcctypes.TransactionInfo
	BTCHeaders          []*btclctypes.BTCHeaderInfo
}

// getFinalizedInfo returns metadata and proofs that are identical to all BTC timestamps in the same epoch
func (k Keeper) getFinalizedInfo(
	ctx context.Context,
	epochNum uint64,
	headersToBroadcast []*btclctypes.BTCHeaderInfo,
) (*finalizedInfo, error) {
	finalizedEpochInfo, err := k.epochingKeeper.GetHistoricalEpoch(ctx, epochNum)
	if err != nil {
		return nil, err
	}

	// get proof that the epoch is sealed
	proofEpochSealed := k.getSealedEpochProof(ctx, epochNum)
	if proofEpochSealed == nil {
		return nil, fmt.Errorf("proof epoch sealed is nil for epoch %d", epochNum)
	}

	// assign raw checkpoint
	rawCheckpoint, err := k.checkpointingKeeper.GetRawCheckpoint(ctx, epochNum)
	if err != nil {
		return nil, err
	}

	// assign BTC submission key
	ed := k.btccKeeper.GetEpochData(ctx, epochNum)
	bestSubmissionBtcInfo := k.btccKeeper.GetEpochBestSubmissionBtcInfo(ctx, ed)
	if bestSubmissionBtcInfo == nil {
		return nil, fmt.Errorf("empty bestSubmissionBtcInfo")
	}
	btcSubmissionKey := &bestSubmissionBtcInfo.SubmissionKey

	// proof that the epoch's checkpoint is submitted to BTC
	// i.e., the two `TransactionInfo`s for the checkpoint
	proofEpochSubmitted, err := k.ProveEpochSubmitted(ctx, btcSubmissionKey)
	if err != nil {
		return nil, err
	}

	// construct finalizedInfo
	finalizedInfo := &finalizedInfo{
		EpochInfo:           finalizedEpochInfo,
		RawCheckpoint:       rawCheckpoint.Ckpt,
		BTCSubmissionKey:    btcSubmissionKey,
		ProofEpochSealed:    proofEpochSealed,
		ProofEpochSubmitted: proofEpochSubmitted,
		BTCHeaders:          headersToBroadcast,
	}

	return finalizedInfo, nil
}

// createBTCTimestamp creates a BTC timestamp from finalizedInfo for a given IBC channel
// where the counterparty is a Cosmos zone
func (k Keeper) createBTCTimestamp(
	ctx context.Context,
	consumerID string,
	channel channeltypes.IdentifiedChannel,
	finalizedInfo *finalizedInfo,
) (*types.BTCTimestamp, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	// if the Babylon contract in this channel has not been initialised, get headers from
	// the tip to (k+1+len(finalizedInfo.BTCHeaders))-deep header for efficient initialization
	var btcHeaders []*btclctypes.BTCHeaderInfo
	if k.channelKeeper.IsChannelUninitialized(ctx, channel) {
		kValue := k.btccKeeper.GetParams(ctx).BtcConfirmationDepth
		depth := kValue + 1 + uint32(len(finalizedInfo.BTCHeaders))

		btcHeaders = k.btclcKeeper.GetMainChainUpTo(ctx, depth)
		if btcHeaders == nil {
			return nil, fmt.Errorf("failed to get Bitcoin main chain up to depth %d", depth)
		}
		bbn.Reverse(btcHeaders)
	} else {
		btcHeaders = finalizedInfo.BTCHeaders
	}

	// construct BTC timestamp from everything
	// NOTE: it's possible that there is no header checkpointed in this epoch
	btcTimestamp := &types.BTCTimestamp{
		Header:           nil,
		BtcHeaders:       &types.BTCHeaders{Headers: btcHeaders},
		EpochInfo:        finalizedInfo.EpochInfo,
		RawCheckpoint:    finalizedInfo.RawCheckpoint,
		BtcSubmissionKey: finalizedInfo.BTCSubmissionKey,
		Proof: &types.ProofFinalizedHeader{
			ProofEpochSealed:    finalizedInfo.ProofEpochSealed,
			ProofEpochSubmitted: finalizedInfo.ProofEpochSubmitted,
		},
	}

	// get finalized header for this consumer and epoch
	// NOTE: it's possible that this consumer does not have a header in this epoch
	epochNum := finalizedInfo.EpochInfo.EpochNumber
	finalizedHeader, err := k.GetFinalizedHeader(ctx, consumerID, epochNum)
	if err == nil {
		// if there is a Consumer header checkpointed in this finalised epoch,
		// add this Consumer header and corresponding proofs to the BTC timestamp
		epochOfHeader := finalizedHeader.Header.BabylonEpoch
		if epochOfHeader == epochNum {
			btcTimestamp.Header = finalizedHeader.Header
			btcTimestamp.Proof.ProofConsumerHeaderInEpoch = finalizedHeader.Proof
		}
	} else {
		k.Logger(sdkCtx).Debug("no finalized header for consumer",
			"consumerID", consumerID,
			"epoch", epochNum,
			"error", err,
		)
	}

	return btcTimestamp, nil
}

// getDeepEnoughBTCHeaders returns the last k+1 BTC headers for fork scenarios,
// where k is the confirmation depth. This provides sufficient safety against reorgs.
func (k Keeper) getDeepEnoughBTCHeaders(ctx context.Context) []*btclctypes.BTCHeaderInfo {
	kValue := k.btccKeeper.GetParams(ctx).BtcConfirmationDepth
	startHeight := k.btclcKeeper.GetTipInfo(ctx).Height - kValue
	return k.btclcKeeper.GetMainChainFrom(ctx, startHeight)
}

// GetHeadersToBroadcast retrieves headers using the fallback method of k+1.
// If a consumer ID is not provided, a global LastSentSegment is used to track the timestamped header
// for all consumers when the checkpoint is finalized.
func (k Keeper) GetHeadersToBroadcast(ctx context.Context, consumerID string, headerCache *types.HeaderCache) []*btclctypes.BTCHeaderInfo {
	lastSegment := k.GetBSNLastSentSegment(ctx, consumerID)

	if lastSegment == nil {
		// we did not send any headers yet, so we need to send the last k+1 BTC headers
		// where k is the confirmation depth. This provides sufficient safety for BSNs
		// while being more efficient than using the finalization timeout w.
		return k.getDeepEnoughBTCHeaders(ctx)
	}

	// we already sent some headers, so we need to send headers from the child of the most recent header we sent
	// which is still in the main chain.
	// In most cases it will be header just after the tip, but in case of the forks it may as well be some older header
	// of the segment
	var initHeader *btclctypes.BTCHeaderInfo
	for i := len(lastSegment.BtcHeaders) - 1; i >= 0; i-- {
		header := lastSegment.BtcHeaders[i]
		if header, err := headerCache.GetHeaderByHash(
			header.Hash,
			func() (*btclctypes.BTCHeaderInfo, error) {
				return k.btclcKeeper.GetHeaderByHash(ctx, header.Hash)
			},
		); err == nil && header != nil {
			initHeader = header
			break
		}
	}

	if initHeader == nil {
		// if initHeader is nil, then this means a reorg happens such that all headers
		// in the last segment are reverted. In this case, send the last k+1 BTC headers
		// using confirmation depth k instead of finalization timeout w for efficiency
		return k.getDeepEnoughBTCHeaders(ctx)
	}

	headersToSend := k.btclcKeeper.GetMainChainFrom(ctx, initHeader.Height+1)

	return headersToSend
}

// BroadcastBTCTimestamps sends an IBC packet of BTC timestamp to all open IBC channels to ZoneConcierge
func (k Keeper) BroadcastBTCTimestamps(
	ctx context.Context,
	epochNum uint64,
	consumerChannelMap map[string]channeltypes.IdentifiedChannel,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	// Babylon does not broadcast BTC timestamps until finalising epoch 1
	if epochNum < 1 {
		k.Logger(sdkCtx).Info("skipping BTC timestamp broadcast",
			"reason", "epoch less than 1",
			"epoch", epochNum,
		)
		return nil
	}

	if len(consumerChannelMap) == 0 {
		k.Logger(sdkCtx).Info("skipping BTC timestamp broadcast",
			"reason", "no registered consumers",
		)
		return nil
	}

	// get all registered consumers
	// Extract keys and sort them for deterministic iteration
	consumerIDs := make([]string, 0, len(consumerChannelMap))
	for consumerID := range consumerChannelMap {
		consumerIDs = append(consumerIDs, consumerID)
	}
	sort.Strings(consumerIDs)

	k.Logger(sdkCtx).Info("broadcasting BTC timestamps",
		"consumers", len(consumerIDs),
		"epoch", epochNum,
	)

	// Create header cache to avoid duplicate DB queries across consumers
	headerCache := types.NewHeaderCache()

	// for each registered consumer, find its channels and send BTC timestamp
	for _, consumerID := range consumerIDs {
		// Find channels for this consumer using O(1) map lookup
		channel := consumerChannelMap[consumerID]

		headersToBroadcast := k.GetHeadersToBroadcast(ctx, consumerID, headerCache)

		// get all metadata shared across BTC timestamps in the same epoch
		finalizedInfo, err := k.getFinalizedInfo(ctx, epochNum, headersToBroadcast)
		if err != nil {
			k.Logger(sdkCtx).Error("failed to get finalized info for BTC timestamp broadcast",
				"epoch", epochNum,
				"error", err.Error(),
			)
			return err
		}

		// Send to consumer's channel
		btcTimestamp, err := k.createBTCTimestamp(ctx, consumerID, channel, finalizedInfo)
		if err != nil {
			k.Logger(sdkCtx).Error("failed to create BTC timestamp for consumer, skipping consumer",
				"channel", channel.ChannelId,
				"consumerID", consumerID,
				"error", err.Error(),
			)
			continue
		}

		packet := types.NewBTCTimestampPacketData(btcTimestamp)
		if err := k.SendIBCPacket(ctx, channel, packet); err != nil {
			if errors.Is(err, clienttypes.ErrClientNotActive) {
				k.Logger(sdkCtx).Info("IBC client is not active, skipping consumer",
					"channel", channel.ChannelId,
					"consumerID", consumerID,
					"error", err.Error(),
				)
				continue
			}

			k.Logger(sdkCtx).Error("failed to send BTC timestamp to consumer, continuing with other consumers",
				"channel", channel.ChannelId,
				"consumerID", consumerID,
				"error", err.Error(),
			)
			continue
		}

		// only update the segment if we have broadcasted some headers
		if len(headersToBroadcast) > 0 {
			k.SetBSNLastSentSegment(ctx, consumerID, &types.BTCChainSegment{
				BtcHeaders: headersToBroadcast,
			})
		}
	}

	return nil
}
