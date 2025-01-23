package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
)

func (k Keeper) GetAllChannels(ctx context.Context) []channeltypes.IdentifiedChannel {
	return k.channelKeeper.GetAllChannels(sdk.UnwrapSDKContext(ctx))
}

// GetAllOpenZCChannels returns all open channels that are connected to ZoneConcierge's port
func (k Keeper) GetAllOpenZCChannels(ctx context.Context) []channeltypes.IdentifiedChannel {
	zcPort := k.GetPort(ctx)
	channels := k.GetAllChannels(ctx)

	openZCChannels := []channeltypes.IdentifiedChannel{}
	for _, channel := range channels {
		if channel.State != channeltypes.OPEN {
			continue
		}
		if channel.PortId != zcPort {
			continue
		}
		openZCChannels = append(openZCChannels, channel)
	}

	return openZCChannels
}

// MapClientIDToChannels creates a map of IBC client IDs to their corresponding channels
func (k Keeper) MapClientIDToChannels(ctx context.Context) map[string][]channeltypes.IdentifiedChannel {
	openZCChannels := k.GetAllOpenZCChannels(ctx)
	clientChannelMap := make(map[string][]channeltypes.IdentifiedChannel)
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	for _, channel := range openZCChannels {
		ibcClientID, err := k.getIBCClientID(ctx, channel)
		if err != nil {
			k.Logger(sdkCtx).Error("Failed to get IBC client ID, skipping channel", "channelID", channel.ChannelId, "error", err)
			continue
		}

		clientChannelMap[ibcClientID] = append(clientChannelMap[ibcClientID], channel)
	}

	return clientChannelMap
}

// getIBCClientID gets the IBC client ID associated with the given channel
func (k Keeper) getIBCClientID(ctx context.Context, channel channeltypes.IdentifiedChannel) (string, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	// get clientState under this channel
	clientID, _, err := k.channelKeeper.GetChannelClientState(sdkCtx, channel.PortId, channel.ChannelId)
	if err != nil {
		return "", err
	}

	return clientID, nil
}

// isChannelUninitialized checks whether the channel is not initilialised yet
// it's done by checking whether the packet sequence number is 1 (the first sequence number) or not
func (k Keeper) isChannelUninitialized(ctx context.Context, channel channeltypes.IdentifiedChannel) bool {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	portID := channel.PortId
	channelID := channel.ChannelId
	// NOTE: channeltypes.IdentifiedChannel object is guaranteed to exist, so guaranteed to be found
	nextSeqSend, _ := k.channelKeeper.GetNextSequenceSend(sdkCtx, portID, channelID)
	return nextSeqSend == 1
}
