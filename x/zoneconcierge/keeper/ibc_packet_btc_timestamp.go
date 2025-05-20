package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"

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
		panic(err) // only programming error
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
	// the tip to (w+1+len(finalizedInfo.BTCHeaders))-deep header
	var btcHeaders []*btclctypes.BTCHeaderInfo
	if k.isChannelUninitialized(ctx, channel) {
		w := k.btccKeeper.GetParams(ctx).CheckpointFinalizationTimeout
		depth := w + 1 + uint32(len(finalizedInfo.BTCHeaders))

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
		Proof: &types.ProofFinalizedChainInfo{
			ProofConsumerHeaderInEpoch: nil,
			ProofEpochSealed:           finalizedInfo.ProofEpochSealed,
			ProofEpochSubmitted:        finalizedInfo.ProofEpochSubmitted,
		},
	}

	// get finalised chainInfo
	// NOTE: it's possible that this chain does not have chain info at the moment
	epochNum := finalizedInfo.EpochInfo.EpochNumber
	epochChainInfo, err := k.GetEpochChainInfo(ctx, consumerID, epochNum)
	if err == nil {
		// if there is a Consumer header checkpointed in this finalised epoch,
		// add this Consumer header and corresponding proofs to the BTC timestamp
		epochOfHeader := epochChainInfo.ChainInfo.LatestHeader.BabylonEpoch
		if epochOfHeader == epochNum {
			btcTimestamp.Header = epochChainInfo.ChainInfo.LatestHeader
			btcTimestamp.Proof.ProofConsumerHeaderInEpoch = epochChainInfo.ProofHeaderInEpoch
		}
	} else {
		k.Logger(sdkCtx).Debug("no epochChainInfo for consumer",
			"consumerID", consumerID,
			"epoch", epochNum,
			"error", err,
		)
	}

	return btcTimestamp, nil
}

// getDeepEnoughBTCHeaders returns the last w+1 BTC headers, in which the 1st BTC header
// must be in the canonical chain assuming w-long reorg will never happen
// This function will only be triggered upon a finalised epoch, where w-deep BTC checkpoint
// is guaranteed. Thus the function is safe to be called upon generating BTC timestamps
func (k Keeper) getDeepEnoughBTCHeaders(ctx context.Context) []*btclctypes.BTCHeaderInfo {
	wValue := k.btccKeeper.GetParams(ctx).CheckpointFinalizationTimeout
	startHeight := k.btclcKeeper.GetTipInfo(ctx).Height - wValue
	return k.btclcKeeper.GetMainChainFrom(ctx, startHeight)
}

// getHeadersToBroadcast retrieves headers to be broadcasted to all open IBC channels to ZoneConcierge
// The header to be broadcasted are:
// - either the whole known chain if we did not broadcast any headers yet
// - headers from the child of the most recent header we sent which is still in the main chain up to the current tip
func (k Keeper) getHeadersToBroadcast(ctx context.Context) []*btclctypes.BTCHeaderInfo {
	lastSegment := k.GetLastSentSegment(ctx)

	if lastSegment == nil {
		// we did not send any headers yet, so we need to send the last w+1 BTC headers
		// where w+1 is imposed by Babylon contract. This ensures that the first BTC header
		// in Babylon contract will be w-deep
		return k.getDeepEnoughBTCHeaders(ctx)
	}

	// we already sent some headers, so we need to send headers from the child of the most recent header we sent
	// which is still in the main chain.
	// In most cases it will be header just after the tip, but in case of the forks it may as well be some older header
	// of the segment
	var initHeader *btclctypes.BTCHeaderInfo
	for i := len(lastSegment.BtcHeaders) - 1; i >= 0; i-- {
		header := lastSegment.BtcHeaders[i]
		if header, err := k.btclcKeeper.GetHeaderByHash(ctx, header.Hash); err == nil && header != nil {
			initHeader = header
			break
		}
	}

	if initHeader == nil {
		// if initHeader is nil, then this means a reorg happens such that all headers
		// in the last segment are reverted. In this case, send the last w+1 BTC headers
		return k.getDeepEnoughBTCHeaders(ctx)
	}

	headersToSend := k.btclcKeeper.GetMainChainFrom(ctx, initHeader.Height+1)

	return headersToSend
}

// BroadcastBTCTimestamps sends an IBC packet of BTC timestamp to all open IBC channels to ZoneConcierge
func (k Keeper) BroadcastBTCTimestamps(
	ctx context.Context,
	epochNum uint64,
	headersToBroadcast []*btclctypes.BTCHeaderInfo,
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

	// get all channels that are open and are connected to ZoneConcierge's port
	openZCChannels := k.GetAllOpenZCChannels(ctx)
	if len(openZCChannels) == 0 {
		k.Logger(sdkCtx).Info("skipping BTC timestamp broadcast",
			"reason", "no open channels",
		)
		return nil
	}

	k.Logger(sdkCtx).Info("broadcasting BTC timestamps",
		"channels", len(openZCChannels),
		"epoch", epochNum,
	)

	// get all metadata shared across BTC timestamps in the same epoch
	finalizedInfo, err := k.getFinalizedInfo(ctx, epochNum, headersToBroadcast)
	if err != nil {
		return err
	}

	// for each channel, construct and send BTC timestamp
	for _, channel := range openZCChannels {
		consumerID, err := k.getClientID(ctx, channel)
		if err != nil {
			return err
		}

		btcTimestamp, err := k.createBTCTimestamp(ctx, consumerID, channel, finalizedInfo)
		if err != nil {
			return err
		}

		packet := types.NewBTCTimestampPacketData(btcTimestamp)
		if err := k.SendIBCPacket(ctx, channel, packet); err != nil {
			return err
		}
	}

	return nil
}
