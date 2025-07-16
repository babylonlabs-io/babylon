package keeper

import (
	"context"
	"errors"
	"fmt"
	"sort"

	bbn "github.com/babylonlabs-io/babylon/v3/types"
	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	bsctypes "github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
	finalitytypes "github.com/babylonlabs-io/babylon/v3/x/finality/types"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
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

	// Extract keys and sort them for deterministic iteration
	consumerIDs := make([]string, 0, len(consumerIBCPacketMap))
	for consumerID := range consumerIBCPacketMap {
		consumerIDs = append(consumerIDs, consumerID)
	}
	sort.Strings(consumerIDs)

	// Iterate through consumer IDs in sorted order
	for _, consumerID := range consumerIDs {
		ibcPacket := consumerIBCPacketMap[consumerID]

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
				if errors.Is(err, clienttypes.ErrClientNotActive) {
					k.Logger(sdkCtx).Info("IBC client is not active, skipping channel",
						"consumerID", consumerID,
						"channel", channel.ChannelId,
						"error", err.Error(),
					)
					continue
				}

				k.Logger(sdkCtx).Error("failed to send BTC staking consumer event to channel, continuing with other channels",
					"consumerID", consumerID,
					"channel", channel.ChannelId,
					"error", err.Error(),
				)
				continue
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

	// Initialize Consumer BTC state to current tip
	if err := k.InitializeConsumerBTCState(ctx, clientID); err != nil {
		return fmt.Errorf("failed to initialize consumer BTC state: %w", err)
	}

	// Get current tip height for logging
	currentTip := k.btclcKeeper.GetTipInfo(ctx)

	k.Logger(ctx).Info("IBC channel created successfully",
		"consumerID", clientID,
		"channelID", channelID,
		"currentTipHeight", currentTip.Height,
		"note", "Consumer base BTC header initialized to current tip",
	)

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

	// Get the finality provider associated with slashing
	fp, err := k.bsKeeper.GetFinalityProvider(ctx, evidence.FpBtcPk.MustMarshal())
	if err != nil {
		return fmt.Errorf("failed to get finality provider: %w", err)
	}

	if fp == nil {
		return fmt.Errorf("finality provider not found")
	}
	// Check if the finality provider is associated with a consumer
	// Verify that the consumer ID matches the client ID
	if fp.BsnId != clientID {
		return fmt.Errorf("consumer ID (%s) does not match client ID (%s)", fp.BsnId, clientID)
	}

	// Update the consumer finality provider's slashed height and
	// send power distribution update event so the affected Babylon FP's voting power can be adjusted
	if err := k.bsKeeper.SlashFinalityProvider(ctx, slashedFpBTCPK.MustMarshal()); err != nil {
		return fmt.Errorf("failed to slash finality provider: %w", err)
	}

	// Send slashing event to other involved consumers
	if err := k.bsKeeper.PropagateFPSlashingToConsumers(ctx, slashedFpBTCSK); err != nil {
		return fmt.Errorf("failed to propagate slashing to consumers: %w", err)
	}

	// Emit slashed finality provider event so btc slasher/vigilante can slash the finality provider
	eventSlashing := finalitytypes.NewEventSlashedFinalityProvider(evidence)
	if err := sdk.UnwrapSDKContext(ctx).EventManager().EmitTypedEvent(eventSlashing); err != nil {
		return fmt.Errorf("failed to emit EventSlashedFinalityProvider event: %w", err)
	}

	return nil
}

// HandleBSNBaseBTCHeader processes a BSN base BTC header received.
// This allows BSNs to inform Babylon about which BTC header they consider as their base
func (k Keeper) HandleBSNBaseBTCHeader(
	ctx sdk.Context,
	portID string,
	channelID string,
	baseBTCHeader *btclctypes.BTCHeaderInfo,
) error {
	// Get the client ID from the channel to identify the BSN
	clientID, _, err := k.channelKeeper.GetChannelClientState(ctx, portID, channelID)
	if err != nil {
		return fmt.Errorf("failed to get client state: %w", err)
	}

	// Validate the base BTC header
	if err := baseBTCHeader.Validate(); err != nil {
		return fmt.Errorf("base BTC header is invalid: %w", err)
	}

	// Verify that the base BTC header exists in Babylon's BTC light client
	// This ensures the BSN is not trying to set a base header that Babylon doesn't know about
	existingHeader, err := k.btclcKeeper.GetHeaderByHash(ctx, baseBTCHeader.Hash)
	if err != nil {
		return fmt.Errorf("failed to retrieve base BTC header from Babylon's BTC light client: %w", err)
	}
	if existingHeader == nil {
		return fmt.Errorf("base BTC header not found in Babylon's BTC light client: %v", baseBTCHeader.Hash)
	}

	// Store the BSN's reported base BTC header
	k.SetConsumerBaseBTCHeader(ctx, clientID, baseBTCHeader)

	return nil
}
