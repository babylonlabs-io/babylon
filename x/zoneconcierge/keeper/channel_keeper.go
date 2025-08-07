package keeper

import (
	"context"

	"cosmossdk.io/log"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
)

// ChannelKeeper is a wrapper around the IBC channel keeper
// that provides additional functionality specific to ZoneConcierge.
type ChannelKeeper struct {
	channelKeeper types.ChannelKeeper
}

func NewChannelKeeper(
	channelKeeper types.ChannelKeeper,
) *ChannelKeeper {
	return &ChannelKeeper{
		channelKeeper: channelKeeper,
	}
}

// Logger returns a module-specific logger.
func (k ChannelKeeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", "x/"+ibcexported.ModuleName+"-"+types.ModuleName+"-channel")
}

func (k ChannelKeeper) GetAllChannels(ctx context.Context) []channeltypes.IdentifiedChannel {
	return k.channelKeeper.GetAllChannels(sdk.UnwrapSDKContext(ctx))
}

// GetAllOpenZCChannels returns all open channels that are connected to ZoneConcierge's port
func (k ChannelKeeper) GetAllOpenZCChannels(ctx context.Context) []channeltypes.IdentifiedChannel {
	channels := k.GetAllChannels(ctx)

	openZCChannels := []channeltypes.IdentifiedChannel{}
	for _, channel := range channels {
		if channel.State != channeltypes.OPEN {
			continue
		}
		if channel.PortId != types.PortID {
			continue
		}
		openZCChannels = append(openZCChannels, channel)
	}

	return openZCChannels
}

func (k ChannelKeeper) ConsumerHasIBCChannelOpen(ctx context.Context, consumerID string) bool {
	_, found := k.GetChannelForConsumer(ctx, consumerID)
	return found
}

// GetClientID gets the ID of the IBC client under the given channel
// We will use the client ID as the consumer ID to uniquely identify
// the consumer chain
func (k ChannelKeeper) GetClientID(ctx context.Context, channel channeltypes.IdentifiedChannel) (string, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	clientID, _, err := k.channelKeeper.GetChannelClientState(sdkCtx, channel.PortId, channel.ChannelId)
	if err != nil {
		return "", err
	}
	return clientID, nil
}

// getChannelForConsumer finds an open channel for a given consumer ID
func (k ChannelKeeper) GetChannelForConsumer(ctx context.Context, consumerID string) (channeltypes.IdentifiedChannel, bool) {
	openChannels := k.GetAllOpenZCChannels(ctx)

	for _, channel := range openChannels {
		clientID, err := k.GetClientID(ctx, channel)
		if err != nil {
			continue
		}
		if clientID == consumerID {
			return channel, true
		}
	}

	return channeltypes.IdentifiedChannel{}, false
}

func (k ChannelKeeper) GetChannelClientState(ctx sdk.Context, portID, channelID string) (string, ibcexported.ClientState, error) {
	return k.channelKeeper.GetChannelClientState(ctx, portID, channelID)
}

// isChannelUninitialized checks whether the channel is not initilialised yet
// it's done by checking whether the packet sequence number is 1 (the first sequence number) or not
func (k ChannelKeeper) IsChannelUninitialized(ctx context.Context, channel channeltypes.IdentifiedChannel) bool {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	portID := channel.PortId
	channelID := channel.ChannelId
	// NOTE: channeltypes.IdentifiedChannel object is guaranteed to exist, so guaranteed to be found
	nextSeqSend, _ := k.channelKeeper.GetNextSequenceSend(sdkCtx, portID, channelID)
	return nextSeqSend == 1
}
