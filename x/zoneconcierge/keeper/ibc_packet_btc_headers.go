package keeper

import (
	"context"
	"errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"

	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
)

// BroadcastBTCHeaders sends an IBC packet of BTC headers to all open IBC channels to ZoneConcierge
func (k Keeper) BroadcastBTCHeaders(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	// get all registered consumers
	consumers := k.btcStkKeeper.GetAllRegisteredCosmosConsumers(ctx)
	if len(consumers) == 0 {
		k.Logger(sdkCtx).Info("skipping BTC header broadcast",
			"reason", "no registered consumers",
		)
		return nil
	}

	// New behavior using Consumer-specific base headers:
	// - If no headers sent: send the last k+1 BTC headers
	// - If headers previously sent: from child of most recent valid header to tip
	// - If reorg detected: send the last k+1 BTC headers
	// TODO: Improve reorg handling efficiency - instead of sending from Consumer base to tip,
	// we should send a dedicated reorg event and then send headers from the reorged point to tip

	for _, consumer := range consumers {
		// Find the channel for this consumer
		channel, found := k.channelKeeper.GetChannelForConsumer(ctx, consumer.ConsumerId, consumer.GetCosmosConsumerMetadata().ChannelId)
		if !found {
			k.Logger(sdkCtx).Debug("no open channel found for consumer, skipping BTC header broadcast",
				"consumerID", consumer.ConsumerId,
			)
			continue
		}

		headers := k.GetHeadersToBroadcast(ctx, consumer.ConsumerId)
		if len(headers) == 0 {
			k.Logger(sdkCtx).Debug("skipping BTC header broadcast for consumer, no headers to broadcast",
				"consumerID", consumer.ConsumerId,
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

		// Update the BSN-specific last sent segment
		k.SetBSNLastSentSegment(ctx, consumer.ConsumerId, &types.BTCChainSegment{
			BtcHeaders: headers,
		})
	}

	return nil
}
