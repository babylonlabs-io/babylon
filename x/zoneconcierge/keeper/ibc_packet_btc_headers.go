package keeper

import (
	"context"
	"errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"

	bbn "github.com/babylonlabs-io/babylon/v3/types"
	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
)

// HeaderCache provides in-memory caching for BTC headers to avoid duplicate DB queries
type HeaderCache struct {
	cache map[string]*btclctypes.BTCHeaderInfo
}

// NewHeaderCache creates a new header cache
func NewHeaderCache() *HeaderCache {
	return &HeaderCache{
		cache: make(map[string]*btclctypes.BTCHeaderInfo),
	}
}

// GetHeaderByHash retrieves a header by hash, using cache first, then falling back to DB
func (hc *HeaderCache) GetHeaderByHash(ctx context.Context, k Keeper, hash *bbn.BTCHeaderHashBytes) (*btclctypes.BTCHeaderInfo, error) {
	hashStr := hash.String()

	// Check cache first
	if header, found := hc.cache[hashStr]; found {
		return header, nil
	}

	// Cache miss - retrieve from DB
	header, err := k.btclcKeeper.GetHeaderByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	// Store in cache for future use
	if header != nil {
		hc.cache[hashStr] = header
	}

	return header, nil
}

// BroadcastBTCHeaders sends an IBC packet of BTC headers to all open IBC channels to ZoneConcierge
func (k Keeper) BroadcastBTCHeaders(ctx context.Context, consumerChannels []channeltypes.IdentifiedChannel) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	// get all registered consumers
	consumerIDs := k.GetAllConsumerIDs(ctx)
	if len(consumerIDs) == 0 {
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

	// Build a map for O(1) channel lookups
	consumerChannelMap, err := k.buildConsumerChannelMap(ctx, consumerChannels)
	if err != nil {
		return err
	}

	// Create header cache to avoid duplicate DB queries across consumers
	headerCache := NewHeaderCache()

	for _, consumerID := range consumerIDs {
		// Find channels for this consumer using O(1) map lookup
		channels, found := consumerChannelMap[consumerID]
		if !found || len(channels) == 0 {
			k.Logger(sdkCtx).Debug("no open channels found for consumer, skipping BTC header broadcast",
				"consumerID", consumerID,
			)
			continue
		}

		headers := k.GetHeadersToBroadcast(ctx, consumerID, headerCache)
		if len(headers) == 0 {
			k.Logger(sdkCtx).Debug("skipping BTC header broadcast for consumer, no headers to broadcast",
				"consumerID", consumerID,
			)
			continue
		}

		packet := types.NewBTCHeadersPacketData(&types.BTCHeaders{Headers: headers})

		// Send to all channels for this consumer
		sentToAtLeastOneChannel := false
		for _, channel := range channels {
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
			sentToAtLeastOneChannel = true
		}

		// Update the BSN-specific last sent segment only if we sent to at least one channel
		if sentToAtLeastOneChannel {
			k.SetBSNLastSentSegment(ctx, consumerID, &types.BTCChainSegment{
				BtcHeaders: headers,
			})
		}
	}

	return nil
}
