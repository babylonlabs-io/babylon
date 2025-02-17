package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/x/zoneconcierge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BroadcastBTCHeaders sends an IBC packet of BTC headers to all open IBC channels to ZoneConcierge
func (k Keeper) BroadcastBTCHeaders(ctx context.Context) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// 1. Check channels first - fail fast
	openZCChannels := k.GetAllOpenZCChannels(ctx)
	if len(openZCChannels) == 0 {
		k.Logger(sdkCtx).Info("no open IBC channel with ZoneConcierge, skip broadcasting BTC headers")
		return
	}

	// 2. Get headers to broadcast
	headers := k.getBTCHeadersToSend(ctx, types.FullChainFetch)

	// 3. Broadcast headers
	packet := types.NewBTCHeadersPacketData(&types.BTCHeaders{Headers: headers})
	for _, channel := range openZCChannels {
		if err := k.SendIBCPacket(ctx, channel, packet); err != nil {
			k.Logger(sdkCtx).Error("failed to send BTC headers IBC packet",
				"channelID", channel.ChannelId,
				"error", err)
			continue
		}
	}

	// 4. Update last segment
	k.setLastSentSegment(ctx, &types.BTCChainSegment{
		BtcHeaders: headers,
	})
}
