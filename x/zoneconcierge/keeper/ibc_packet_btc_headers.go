package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	btclctypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/x/zoneconcierge/types"
)

// BroadcastBTCHeaders sends an IBC packet of BTC headers to all open IBC channels to ZoneConcierge
func (k Keeper) BroadcastBTCHeaders(
	ctx context.Context,
	headers []*btclctypes.BTCHeaderInfo,
) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// get all channels that are open and are connected to ZoneConcierge's port
	openZCChannels := k.GetAllOpenZCChannels(ctx)
	if len(openZCChannels) == 0 {
		k.Logger(sdkCtx).Info("no open IBC channel with ZoneConcierge, skip broadcasting BTC headers")
		return
	}

	k.Logger(sdkCtx).Info("broadcasting BTC headers to open ZoneConcierge channels",
		"number of channels", len(openZCChannels),
		"number of headers", len(headers))

	btcHeaders := &types.BTCHeaders{
		Headers: headers,
	}

	// for each channel, send BTC headers
	for _, channel := range openZCChannels {
		// wrap BTC headers to IBC packet
		packet := types.NewBTCHeadersPacketData(btcHeaders)

		// send IBC packet
		if err := k.SendIBCPacket(ctx, channel, packet); err != nil {
			k.Logger(sdkCtx).Error("failed to send BTC headers IBC packet",
				"channelID", channel.ChannelId,
				"error", err)
			continue
		}
	}
}
