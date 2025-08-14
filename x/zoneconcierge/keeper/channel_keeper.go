package keeper

import (
	"context"

	"cosmossdk.io/log"
	sdk "github.com/cosmos/cosmos-sdk/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"

	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
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

// GetAllOpenZCChannels returns all open channels that are connected to ZoneConcierge's port
func (k ChannelKeeper) GetAllOpenZCChannels(ctx context.Context) []channeltypes.IdentifiedChannel {
	zcChannels := k.channelKeeper.GetAllChannelsWithPortPrefix(sdk.UnwrapSDKContext(ctx), types.PortID)

	openZCChannels := []channeltypes.IdentifiedChannel{}
	for _, channel := range zcChannels {
		if channel.State != channeltypes.OPEN {
			continue
		}
		openZCChannels = append(openZCChannels, channel)
	}

	return openZCChannels
}

func (k ChannelKeeper) ConsumerHasIBCChannelOpen(ctx context.Context, consumerID, channelID string) bool {
	_, found := k.GetChannelForConsumer(ctx, consumerID, channelID)
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

// GetChannelForConsumer finds an open channel for a given consumer ID and channel ID
func (k ChannelKeeper) GetChannelForConsumer(ctx context.Context, consumerID, channelID string) (channeltypes.IdentifiedChannel, bool) {
	channel, found := k.channelKeeper.GetChannel(sdk.UnwrapSDKContext(ctx), types.PortID, channelID)
	if !found || channel.State != channeltypes.OPEN {
		return channeltypes.IdentifiedChannel{}, false
	}
	identifiedChannel := channeltypes.NewIdentifiedChannel(types.PortID, channelID, channel)

	clientID, _, err := k.channelKeeper.GetChannelClientState(sdk.UnwrapSDKContext(ctx), identifiedChannel.PortId, identifiedChannel.ChannelId)
	if err != nil {
		return channeltypes.IdentifiedChannel{}, false
	}
	if clientID == consumerID {
		return identifiedChannel, true
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
