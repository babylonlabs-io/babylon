package keeper

import (
	"context"

	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
)

// GetConsumerChannelMap creates a map from consumer ID to channels for O(1) lookups
// Returns a map where each consumer ID can have multiple channels
func (k Keeper) GetConsumerChannelMap(ctx context.Context) (map[string]channeltypes.IdentifiedChannel, error) {
	channels := k.channelKeeper.GetAllOpenZCChannels(ctx)
	consumerChannelMap := make(map[string]channeltypes.IdentifiedChannel)
	for _, channel := range channels {
		clientID, err := k.channelKeeper.GetClientID(ctx, channel)
		if err != nil {
			return nil, err
		}
		consumerChannelMap[clientID] = channel
	}
	return consumerChannelMap, nil
}
