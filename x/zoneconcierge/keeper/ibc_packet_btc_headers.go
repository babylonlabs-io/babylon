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

	// Current behavior:
	// - If no headers sent: Returns last w+1 headers from tip
	// - If headers previously sent:
	//   - If last segment valid: Returns headers from last sent to tip
	//   - If last segment invalid (reorg): Returns last w+1 headers from tip
	//
	// TODO: Should use BSN base BTC header as starting point:
	// - If no headers sent: Return from BSN base to tip
	// - If headers previously sent:
	//   - If last segment valid: Return from last sent header to tip
	//   - If last segment invalid (reorg): Return from BSN base to tip
	headers := k.GetHeadersToBroadcast(ctx)
	if len(headers) == 0 {
		k.Logger(sdkCtx).Info("no new BTC headers to broadcast")
		return
	}

	packet := types.NewBTCHeadersPacketData(&types.BTCHeaders{Headers: headers})
	broadcastsSuccessful := true
	for _, channel := range openZCChannels {
		if err := k.SendIBCPacket(ctx, channel, packet); err != nil {
			k.Logger(sdkCtx).Error("failed to send BTC headers IBC packet",
				"channelID", channel.ChannelId,
				"error", err)
			broadcastsSuccessful = false
			continue
		}
	}

	if broadcastsSuccessful {
		k.SetLastSentSegment(ctx, &types.BTCChainSegment{
			BtcHeaders: headers,
		})
	}
}
