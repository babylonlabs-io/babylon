package keeper

import (
	"context"
	"fmt"

	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bsctypes "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
	finalitytypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
)

// BroadcastBTCStakingConsumerEvents retrieves all BTC staking consumer events from the event store,
// sends them to corresponding consumers via open IBC channels, and then deletes the events from the store.
func (k Keeper) BroadcastBTCStakingConsumerEvents(
	ctx context.Context,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	openZCChannels := k.GetAllOpenZCChannels(ctx)
	if len(openZCChannels) == 0 {
		k.Logger(sdkCtx).Info("skipping BTC staking consumer event broadcast",
			"reason", "no open channels",
		)
		return nil
	}

	// Map consumer client IDs to their corresponding open channels.
	consumerChannelMap := make(map[string][]channeltypes.IdentifiedChannel)
	for _, channel := range openZCChannels {
		consumerID, err := k.getClientID(ctx, channel)
		if err != nil {
			return err
		}

		consumerChannelMap[consumerID] = append(consumerChannelMap[consumerID], channel)
	}

	// Iterate through all consumer events and send them to the corresponding open IBC channel.
	consumerIBCPacketMap := k.bsKeeper.GetAllBTCStakingConsumerIBCPackets(ctx)
	for consumerID, ibcPacket := range consumerIBCPacketMap {
		// Check if there are open channels for the current consumer ID.
		channels, ok := consumerChannelMap[consumerID]
		if !ok {
			k.Logger(sdkCtx).Warn("skipping BTC staking consumer event broadcast",
				"reason", "no channels found for consumer",
				"consumerID", consumerID,
			)
			continue
		}

		outPacket := &types.OutboundPacket{
			Packet: &types.OutboundPacket_BtcStaking{
				BtcStaking: ibcPacket,
			},
		}

		for _, channel := range channels {
			if err := k.SendIBCPacket(ctx, channel, outPacket); err != nil {
				return err
			}
		}

		k.bsKeeper.DeleteBTCStakingConsumerIBCPacket(ctx, consumerID)
	}

	return nil
}

// HandleIBCChannelCreation processes the IBC handshake request. The handshake is successful
// only if the client ID is registered as a consumer in the ZoneConcierge
func (k Keeper) HandleIBCChannelCreation(
	ctx sdk.Context,
	connectionID string,
	channelID string,
) error {
	// get the client ID from the connection
	conn, found := k.connectionKeeper.GetConnection(ctx, connectionID)
	if !found {
		return fmt.Errorf("connection %s not found", connectionID)
	}
	clientID := conn.ClientId

	// Check if the client ID is registered as a consumer
	consumerRegister, err := k.btcStkKeeper.GetConsumerRegister(ctx, clientID)
	if err != nil {
		return fmt.Errorf("client ID %s is not registered as a consumer: %w", clientID, err)
	}

	// Ensure the consumer is a Cosmos consumer
	cosmosMetadata := consumerRegister.GetCosmosConsumerMetadata()
	if cosmosMetadata == nil {
		return fmt.Errorf("consumer %s is not a Cosmos consumer", clientID)
	}

	// Ensure the client ID hasn't integrated yet, i.e., the channel ID is not set
	if len(cosmosMetadata.ChannelId) > 0 {
		return fmt.Errorf("consumer %s has already integrated with channel %s", clientID, cosmosMetadata.ChannelId)
	}

	// all good, update the channel ID in the consumer register
	cosmosMetadata.ChannelId = channelID
	consumerRegister.ConsumerMetadata = &bsctypes.ConsumerRegister_CosmosConsumerMetadata{
		CosmosConsumerMetadata: cosmosMetadata,
	}
	if err := k.btcStkKeeper.UpdateConsumer(ctx, consumerRegister); err != nil {
		return fmt.Errorf("failed to update consumer register: %w", err)
	}

	return nil
}

func (k Keeper) HandleConsumerSlashing(
	ctx sdk.Context,
	portID string,
	channelID string,
	consumerSlashing *types.ConsumerSlashingIBCPacket,
) error {
	clientID, _, err := k.channelKeeper.GetChannelClientState(ctx, portID, channelID)
	if err != nil {
		return fmt.Errorf("failed to get client state: %w", err)
	}

	evidence := consumerSlashing.Evidence
	if evidence == nil {
		return fmt.Errorf("consumer slashing evidence is nil")
	}

	slashedFpBTCSK, err := evidence.ExtractBTCSK()
	if err != nil {
		return fmt.Errorf("failed to extract BTCSK: %w", err)
	}

	slashedFpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(slashedFpBTCSK.PubKey())
	evidenceFpBTCPKHex := evidence.FpBtcPk.MarshalHex()
	if slashedFpBTCPK.MarshalHex() != evidenceFpBTCPKHex {
		return fmt.Errorf("slashed FP BTC PK does not match with the one in the evidence")
	}

	// Check if the finality provider is associated with a consumer
	consumerID, err := k.btcStkKeeper.GetConsumerOfFinalityProvider(ctx, slashedFpBTCPK)
	if err != nil {
		return fmt.Errorf("failed to get consumer of finality provider: %w", err)
	}

	// Verify that the consumer ID matches the client ID
	if consumerID != clientID {
		return fmt.Errorf("consumer ID (%s) does not match client ID (%s)", consumerID, clientID)
	}

	// Update the consumer finality provider's slashed height and
	// send power distribution update event so the affected Babylon FP's voting power can be adjusted
	if err := k.bsKeeper.SlashConsumerFinalityProvider(ctx, consumerID, slashedFpBTCPK); err != nil {
		return fmt.Errorf("failed to slash consumer finality provider: %w", err)
	}

	// Send slashing event to other involved consumers
	if err := k.bsKeeper.PropagateFPSlashingToConsumers(ctx, slashedFpBTCPK); err != nil {
		return fmt.Errorf("failed to propagate slashing to consumers: %w", err)
	}

	// Emit slashed finality provider event so btc slasher/vigilante can slash the finality provider
	eventSlashing := finalitytypes.NewEventSlashedFinalityProvider(evidence)
	if err := sdk.UnwrapSDKContext(ctx).EventManager().EmitTypedEvent(eventSlashing); err != nil {
		return fmt.Errorf("failed to emit EventSlashedFinalityProvider event: %w", err)
	}

	return nil
}
