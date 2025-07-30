package keeper

import (
	"context"
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
	"github.com/cosmos/cosmos-sdk/telemetry"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types" //nolint:staticcheck
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	"github.com/hashicorp/go-metrics"
)

const (
	// LabelDestinationPort label for the metric for destination port
	LabelDestinationPort = "destination_port"
	// LabelDestinationChannel label for the metric for destination channel
	LabelDestinationChannel = "destination_channel"
)

// SendIBCPacket sends an IBC packet to a channel
// (adapted from https://github.com/cosmos/ibc-go/blob/v5.0.0/modules/apps/transfer/keeper/relay.go)
func (k Keeper) SendIBCPacket(ctx context.Context, channel channeltypes.IdentifiedChannel, packetData *types.OutboundPacket) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	// get src/dst ports and channels
	sourcePort := channel.PortId
	sourceChannel := channel.ChannelId
	destinationPort := channel.Counterparty.PortId
	destinationChannel := channel.Counterparty.ChannelId

	// Validate packet before attempting to send
if err := k.validatePacket(packetData); err != nil {
		k.Logger(sdkCtx).Error(fmt.Sprintf("packet validation failed for channel %v port %s: %v", destinationChannel, destinationPort, err))
		return err
	}

	// timeout
	timeoutPeriod := time.Duration(k.GetParams(sdkCtx).IbcPacketTimeoutSeconds) * time.Second
	timeoutTime := uint64(sdkCtx.HeaderInfo().Time.Add(timeoutPeriod).UnixNano())
	zeroheight := clienttypes.ZeroHeight()

	seq, err := k.ics4Wrapper.SendPacket(
		sdkCtx,
		sourcePort,
		sourceChannel,
		zeroheight,  // no need to set timeout height if timeout timestamp is set
		timeoutTime, // if the packet is not relayed after this time, then the packet will be time out
		k.cdc.MustMarshal(packetData),
	)
	if err != nil {
		k.Logger(sdkCtx).Error(fmt.Sprintf("failed to send IBC packet (sequence number: %d) to channel %v port %s: %v", seq, destinationChannel, destinationPort, err))
		return err
	}

	k.Logger(sdkCtx).Info(fmt.Sprintf("successfully sent IBC packet (sequence number: %d) to channel %v port %s", seq, destinationChannel, destinationPort))

	// metrics stuff
	labels := []metrics.Label{
		telemetry.NewLabel(LabelDestinationPort, destinationPort),
		telemetry.NewLabel(LabelDestinationChannel, destinationChannel),
	}
	defer func() {
		telemetry.IncrCounterWithLabels(
			[]string{"ibc", types.ModuleName, "send"},
			1,
			labels,
		)
	}()

	return nil
}

// validatePacket performs basic validation on the packet before sending
func (k Keeper) validatePacket(packetData *types.OutboundPacket) error {
	packetBytes := k.cdc.MustMarshal(packetData)

	if len(packetBytes) > channeltypes.MaximumPayloadsSize {
		return fmt.Errorf("packet payload size (%d bytes) exceeds maximum allowed size (%d bytes)", len(packetBytes), channeltypes.MaximumPayloadsSize)
	}

	return nil
}
