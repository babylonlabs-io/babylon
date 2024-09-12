package keeper

import (
	"context"
	"fmt"

	btcstkconsumertypes "github.com/babylonlabs-io/babylon/x/btcstkconsumer/types"
	"github.com/babylonlabs-io/babylon/x/zoneconcierge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BroadcastBTCStakingConsumerEvents retrieves all BTC staking consumer events from the event store,
// sends them to corresponding consumers via open IBC channels, and then deletes the events from the store.
func (k Keeper) BroadcastBTCStakingConsumerEvents(
	ctx context.Context,
) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Retrieve all BTC staking consumer events.
	consumerIBCPacketMap := k.bsKeeper.GetAllBTCStakingConsumerIBCPackets(ctx)

	// Map client IDs to their corresponding open channels.
	clientChannelMap := k.MapClientIDToChannels(ctx)

	// Iterate through all consumer events and send them to the corresponding open IBC channel.
	for consumerID, ibcPacket := range consumerIBCPacketMap {
		// Check if there are open channels for the current consumer ID.
		channels, ok := clientChannelMap[consumerID]
		if !ok {
			k.Logger(sdkCtx).Error("No channels found for clientID", "clientID", consumerID)
			continue
		}

		// Log the IBC packet
		k.Logger(sdkCtx).Info("BroadcastBTCStakingConsumerEvents: Preparing ZoneConcierge packet",
			"consumerID", consumerID,
			"packetType", "BtcStaking",
			"packetContent", fmt.Sprintf("%+v", ibcPacket))

		// Prepare the packet for ZoneConcierge.
		zcPacket := &types.ZoneconciergePacketData{
			Packet: &types.ZoneconciergePacketData_BtcStaking{
				BtcStaking: ibcPacket,
			},
		}

		// Iterate through the list of channels and send the IBC packet to each.
		for _, channel := range channels {
			// Send the IBC packet.
			if err := k.SendIBCPacket(ctx, channel, zcPacket); err != nil {
				k.Logger(sdkCtx).Error("Failed to send BTC staking consumer events", "clientID", consumerID, "channelID", channel.ChannelId, "error", err)
				continue
			}
		}

		// Delete the events for the current consumer ID from the store after successful transmission.
		k.bsKeeper.DeleteBTCStakingConsumerIBCPacket(ctx, consumerID)
	}
}

// HandleConsumerRegistration processes the consumer registration packet and registers the consumer
func (k Keeper) HandleConsumerRegistration(
	ctx sdk.Context,
	destinationPort string,
	destinationChannel string,
	consumerRegister *types.ConsumerRegisterIBCPacket,
) error {
	clientID, _, err := k.channelKeeper.GetChannelClientState(ctx, destinationPort, destinationChannel)
	if err != nil {
		return fmt.Errorf("failed to get client state: %w", err)
	}

	consumerRegisterData := &btcstkconsumertypes.ConsumerRegister{
		ConsumerId:          clientID,
		ConsumerName:        consumerRegister.ConsumerName,
		ConsumerDescription: consumerRegister.ConsumerDescription,
	}

	return k.btcStkKeeper.RegisterConsumer(ctx, consumerRegisterData)
}

func (k Keeper) HandleConsumerFPSlashing(
	ctx sdk.Context,
	destinationPort string,
	destinationChannel string,
	slashingEvent *types.ConsumerSlashingIBCPacket,
) error {
	clientID, _, err := k.channelKeeper.GetChannelClientState(ctx, destinationPort, destinationChannel)
	if err != nil {
		return fmt.Errorf("failed to get client state: %w", err)
	}

	fmt.Println("client id in HandleConsumerFPSlashing", clientID)

	// Log the details of the slashing event
	//k.Logger(ctx).Info("Received Consumer FP Slashing Event",
	//	"clientID", clientID,
	//	"btcPublicKey", hex.EncodeToString(slashingEvent.BtcPublicKey),
	//	"slashingHeight", slashingEvent.SlashingHeight,
	//	"extractedSecretKey", hex.EncodeToString(slashingEvent.ExtractedSecretKey),
	//)

	// // Emit an event for tracking purposes
	// ctx.EventManager().EmitEvent(
	// 	sdk.NewEvent(
	// 		types.EventTypeConsumerFPSlashing,
	// 		sdk.NewAttribute(types.AttributeKeyClientID, clientID),
	// 		sdk.NewAttribute(types.AttributeKeyBTCPublicKey, hex.EncodeToString(slashingEvent.BtcPublicKey)),
	// 		sdk.NewAttribute(types.AttributeKeySlashingHeight, fmt.Sprintf("%d", slashingEvent.SlashingHeight)),
	// 	),
	// )

	return nil
}
