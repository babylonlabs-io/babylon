package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/x/zoneconcierge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BroadcastBTCHeaders sends an IBC packet of BTC headers to all open IBC channels to ZoneConcierge
func (k Keeper) BroadcastBTCHeaders(ctx context.Context) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	openZCChannels := k.GetAllOpenZCChannels(ctx)
	if len(openZCChannels) == 0 {
		k.Logger(sdkCtx).Info("no open IBC channel with ZoneConcierge, skip broadcasting BTC headers")
		return
	}

	// Currently broadcasting last w+1 headers but this should fetch from BSN base header to tip
	headers := k.getHeadersToBroadcast(ctx)
	if len(headers) == 0 {
		k.Logger(sdkCtx).Info("no new BTC headers to broadcast")
		return
	}

	packet := types.NewBTCHeadersPacketData(&types.BTCHeaders{Headers: headers})
	for _, channel := range openZCChannels {
		if err := k.SendIBCPacket(ctx, channel, packet); err != nil {
			k.Logger(sdkCtx).Error("failed to send BTC headers IBC packet",
				"channelID", channel.ChannelId,
				"error", err)
			continue
		}
	}

	k.setLastSentSegment(ctx, &types.BTCChainSegment{
		BtcHeaders: headers,
	})
}
