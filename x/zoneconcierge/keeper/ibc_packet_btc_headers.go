package keeper

import (
	"context"
	"errors"

	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
)

// BroadcastBTCHeaders sends an IBC packet of BTC headers to all open IBC channels to ZoneConcierge
func (k Keeper) BroadcastBTCHeaders(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	openZCChannels := k.GetAllOpenZCChannels(ctx)
	if len(openZCChannels) == 0 {
		k.Logger(sdkCtx).Info("skipping BTC header broadcast",
			"reason", "no open channels",
		)
		return nil
	}

	// New behavior using Consumer-specific base headers:
	// - If no Consumer base header exists: fallback to sending k+1 from tip
	// - If Consumer base header exists but no headers sent: from Consumer base to tip
	// - If headers previously sent: from child of most recent valid header to tip
	// - If reorg detected: from Consumer base to tip
	// TODO: Improve reorg handling efficiency - instead of sending from Consumer base to tip,
	// we should send a dedicated reorg event and then send headers from the reorged point to tip

	for _, channel := range openZCChannels {
		consumerID, err := k.getClientID(ctx, channel)
		if err != nil {
			return err
		}

		headers := k.getHeadersToBroadcastForConsumer(ctx, consumerID)
		if len(headers) == 0 {
			k.Logger(sdkCtx).Debug("skipping BTC header broadcast for consumer, no headers to broadcast",
				"consumerID", consumerID,
			)
			continue
		}

		packet := types.NewBTCHeadersPacketData(&types.BTCHeaders{Headers: headers})
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

		// Update the consumer-specific last sent segment
		k.SetConsumerLastSentSegment(ctx, consumerID, &types.BTCChainSegment{
			BtcHeaders: headers,
		})
	}

	return nil
}
