package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
)

func (k Keeper) GetAllOpenChannels(ctx sdk.Context) []types.IdentifiedChannel {
	return k.channelKeeper.GetAllOpenZCChannels(ctx)
}

// buildConsumerChannelMap creates a map from consumer ID to channels for O(1) lookups
// Returns a map where each consumer ID can have multiple channels
func (k Keeper) buildConsumerChannelMap(ctx context.Context, channels []channeltypes.IdentifiedChannel) (map[string][]channeltypes.IdentifiedChannel, error) {
	consumerChannelMap := make(map[string][]channeltypes.IdentifiedChannel)
	for _, channel := range channels {
		clientID, err := k.channelKeeper.GetClientID(ctx, channel)
		if err != nil {
			return nil, err
		}
		consumerChannelMap[clientID] = append(consumerChannelMap[clientID], channel)
	}
	return consumerChannelMap, nil
}
