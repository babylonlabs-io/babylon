package keeper

import (
	"context"
	"errors"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"

	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
)

// BroadcastBTCHeaders sends an IBC packet of BTC headers to all open IBC channels to ZoneConcierge
func (k Keeper) BroadcastBTCHeaders(ctx context.Context, consumerChannelMap map[string]channeltypes.IdentifiedChannel) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if len(consumerChannelMap) == 0 {
		k.Logger(sdkCtx).Info("skipping BTC header broadcast",
			"reason", "no registered consumers",
		)
		return nil
	}

	// Extract keys and sort them for deterministic iteration
	consumerIDs := make([]string, 0, len(consumerChannelMap))
	for consumerID := range consumerChannelMap {
		consumerIDs = append(consumerIDs, consumerID)
	}
	sort.Strings(consumerIDs)

	// New behavior using Consumer-specific base headers:
	// - If no headers sent: send the last k+1 BTC headers
	// - If headers previously sent: from child of most recent valid header to tip
	// - If reorg detected: send the last k+1 BTC headers
	// TODO: Improve reorg handling efficiency - instead of sending from Consumer base to tip,
	// we should send a dedicated reorg event and then send headers from the reorged point to tip

	// Create header cache to avoid duplicate DB queries across consumers
	headerCache := types.NewHeaderCache()

	for _, consumerID := range consumerIDs {
		// Find channels for this consumer using O(1) map lookup
		channel := consumerChannelMap[consumerID]

		headers := k.GetHeadersToBroadcast(ctx, consumerID, headerCache)
		if len(headers) == 0 {
			k.Logger(sdkCtx).Debug("skipping BTC header broadcast for consumer, no headers to broadcast",
				"consumerID", consumerID,
			)
			continue
		}

		packet := types.NewBTCHeadersPacketData(&types.BTCHeaders{Headers: headers})

		// Send to channel for this consumer
		if err := k.SendIBCPacket(ctx, channel, packet); err != nil {
			if errors.Is(err, clienttypes.ErrClientNotActive) {
				k.Logger(sdkCtx).Info("IBC client is not active, skipping channel",
					"channel", channel.ChannelId,
					"error", err.Error(),
				)
				continue
			}

			k.Logger(sdkCtx).Error("failed to send BTC headers to channel, continuing with other channels",
				"channel", channel.ChannelId,
				"error", err.Error(),
			)
			continue
		}

		// Update the BSN-specific last sent segment only if we sent to at least one channel
		k.SetBSNLastSentSegment(ctx, consumerID, &types.BTCChainSegment{
			BtcHeaders: headers,
		})
	}

	return nil
}
