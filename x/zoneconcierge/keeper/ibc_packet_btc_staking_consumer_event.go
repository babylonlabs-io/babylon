package keeper

import (
	"context"
	"fmt"

	bbn "github.com/babylonlabs-io/babylon/types"
	btcstkconsumertypes "github.com/babylonlabs-io/babylon/x/btcstkconsumer/types"
	finalitytypes "github.com/babylonlabs-io/babylon/x/finality/types"
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

func (k Keeper) HandleConsumerSlashing(
	ctx sdk.Context,
	destinationPort string,
	destinationChannel string,
	consumerSlashing *types.ConsumerSlashingIBCPacket,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	clientID, _, err := k.channelKeeper.GetChannelClientState(ctx, destinationPort, destinationChannel)
	if err != nil {
		return fmt.Errorf("DEBUG: failed to get client state: %w", err)
	}

	k.Logger(sdkCtx).Info("DEBUG: Handling consumer slashing", "clientID", clientID, "evidence", consumerSlashing.Evidence)

	slashingEvidence := consumerSlashing.Evidence
	if slashingEvidence == nil {
		return fmt.Errorf("DEBUG: consumer slashing evidence is nil")
	}

	slashedFpBTCSK, err := slashingEvidence.ExtractBTCSK()
	if err != nil {
		return fmt.Errorf("DEBUG: failed to extract BTCSK: %w", err)
	}

	bip340PK := bbn.NewBIP340PubKeyFromBTCPK(slashedFpBTCSK.PubKey())
	// slashedFpBTCPKHex := hex.EncodeToString(slashedFpBTCSK.PubKey().SerializeCompressed())
	k.Logger(sdkCtx).Info("DEBUG: slashedFpBTCPKHex", "slashedFpBTCPKHex", bip340PK.MarshalHex())

	evidenceFpBTCPKHex := slashingEvidence.FpBtcPk.MarshalHex()
	k.Logger(sdkCtx).Info("DEBUG: evidenceFpBTCPKHex", "evidenceFpBTCPKHex", evidenceFpBTCPKHex)

	if bip340PK.MarshalHex() != evidenceFpBTCPKHex {
		return fmt.Errorf("DEBUG: slashed FP BTC PK does not match with the one in the evidence")
	}

	// Check if the finality provider is associated with a consumer
	consumerID, err := k.btcStkKeeper.GetConsumerOfFinalityProvider(ctx, bip340PK)
	if err != nil {
		k.Logger(sdkCtx).Error("failed to get consumer of finality provider", "error", err)
		return fmt.Errorf("failed to get consumer of finality provider: %w", err)
	}

	// consumer ID should match with clientID
	if consumerID != clientID {
		return fmt.Errorf("DEBUG: consumer ID does not match with client ID")
	}

	consumerFP, err := k.btcStkKeeper.GetConsumerFinalityProvider(ctx, consumerID, bip340PK)
	if err != nil {
		k.Logger(sdkCtx).Error("failed to get consumer finality provider", "error", err)
		return fmt.Errorf("failed to get consumer finality provider: %w", err)
	}

	// Ensure the finality provider is not already slashed
	// TODO: remove this check as babylon height will never be set for consumer finality provider
	if consumerFP.IsSlashed() {
		k.Logger(sdkCtx).Error("finality provider is already slashed", "fp", bip340PK.MarshalHex())
		return fmt.Errorf("finality provider is already slashed")
	}

	k.Logger(sdkCtx).Info("DEBUG: consumerID", "consumerID", consumerID)

	// Step 1: Identify associated Babylon FPs
	associatedBabylonFPs, err := k.findAssociatedBabylonFPs(ctx, consumerFP)
	if err != nil {
		panic(fmt.Errorf("failed to find associated Babylon FPs: %v", err))
	}

	// Step 2: Discount voting power for each associated Babylon FP
	for _, babylonFP := range associatedBabylonFPs {
		if err := k.discountVotingPower(ctx, babylonFP, consumerFP); err != nil {
			panic(fmt.Errorf("failed to discount voting power for Babylon FP %s: %v", babylonFP, err))
		}
	}

	// Step 3: Propagate slashing information to other consumers
	if err := k.bsKeeper.PropagateFPSlashingToConsumers(ctx, bip340PK); err != nil {
		k.Logger(sdkCtx).Error("failed to propagate slashing to consumers", "error", err)
		return fmt.Errorf("failed to propagate slashing to consumers: %w", err)
	}

	// Step 4: Emit Cosmos SDK event for the slashing reaction
	eventSlashing := finalitytypes.NewEventSlashedFinalityProvider(slashingEvidence)
	if err := sdk.UnwrapSDKContext(ctx).EventManager().EmitTypedEvent(eventSlashing); err != nil {
		panic(fmt.Errorf("failed to emit EventSlashedFinalityProvider event: %w", err))
	}

	return nil
}
