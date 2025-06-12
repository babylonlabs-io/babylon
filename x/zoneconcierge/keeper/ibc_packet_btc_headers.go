package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BroadcastBTCHeaders sends an IBC packet of BTC headers to all open IBC channels to ZoneConcierge
func (k Keeper) BroadcastBTCHeaders(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	openZCChannels := k.GetAllOpenZCChannels(ctx)
	if len(openZCChannels) == 0 {
		k.Logger(sdkCtx).Info("skipping BTC header broadcast",
			"reason", "no open channels",
		)
		return nil
	}

	// Current behavior:
	// - If no headers sent: Returns last w+1 headers from tip
	// - If headers previously sent:
	//   - If last segment valid: Returns headers from last sent to tip
	//   - If last segment invalid (reorg): Returns last w+1 headers from tip
	//
	// TODO: Should use Consumer base BTC header as starting point:
	// - If no headers sent: Return from Consumer base to tip
	// - If headers previously sent:
	//   - If last segment valid: Return from last sent header to tip
	//   - If last segment invalid (reorg): Return from Consumer base to tip
	headers := k.getHeadersToBroadcast(ctx)
	if len(headers) == 0 {
		k.Logger(sdkCtx).Info("skipping BTC header broadcast",
			"reason", "no headers to broadcast",
		)
		return nil
	}

	packet := types.NewBTCHeadersPacketData(&types.BTCHeaders{Headers: headers})
	for _, channel := range openZCChannels {
		if err := k.SendIBCPacket(ctx, channel, packet); err != nil {
			return err
		}
	}

	k.setLastSentSegment(ctx, &types.BTCChainSegment{
		BtcHeaders: headers,
	})

	return nil
}
