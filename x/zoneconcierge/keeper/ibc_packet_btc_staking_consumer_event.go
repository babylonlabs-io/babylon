package keeper

import (
	"context"
	"errors"
	"fmt"
	"sort"

<<<<<<< HEAD
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	bsctypes "github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
	finalitytypes "github.com/babylonlabs-io/babylon/v3/x/finality/types"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
=======
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"

	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bsctypes "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
	finalitytypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
>>>>>>> b10c56e4 (perf(zc): packet broadcast logic trigger only when needed instead of every block (#1612))
)

// HasBTCStakingConsumerIBCPackets checks if any BTC staking consumer IBC packets exist in the store.
func (k Keeper) HasBTCStakingConsumerIBCPackets(ctx context.Context) bool {
	return k.bsKeeper.HasBTCStakingConsumerIBCPackets(ctx)
}

// BroadcastBTCStakingConsumerEvents retrieves all BTC staking consumer events from the event store,
// sends them to corresponding consumers via open IBC channels, and then deletes the events from the store.
func (k Keeper) BroadcastBTCStakingConsumerEvents(
	ctx context.Context,
	consumerChannelMap map[string]channeltypes.IdentifiedChannel,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if len(consumerChannelMap) == 0 {
		k.Logger(sdkCtx).Info("skipping BTC staking consumer event broadcast",
			"reason", "no open channels",
		)
		return nil
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
		channel, ok := consumerChannelMap[consumerID]
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

		if err := k.SendIBCPacket(ctx, channel, outPacket); err != nil {
			if errors.Is(err, clienttypes.ErrClientNotActive) {
				k.Logger(sdkCtx).Info("IBC client is not active, skipping",
					"consumerID", consumerID,
					"channel", channel.ChannelId,
					"error", err.Error(),
				)
				continue
			}

			k.Logger(sdkCtx).Error("failed to send BTC staking consumer event",
				"consumerID", consumerID,
				"channel", channel.ChannelId,
				"error", err.Error(),
			)
			continue
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

	// Get current tip height for logging
	currentTip := k.btclcKeeper.GetTipInfo(ctx)

	// Set the transient store flag to trigger broadcast packets at Endblock
	k.MarkNewConsumerChannel(ctx, clientID)
	k.Logger(ctx).Info("IBC channel created successfully",
		"consumerID", clientID,
		"channelID", channelID,
		"currentTipHeight", currentTip.Height,
		"note", "BSN base BTC header initialized to current tip",
	)

	return nil
}

func (k Keeper) HandleConsumerSlashing(
	ctx sdk.Context,
	portID string,
	channelID string,
	consumerSlashing *types.BSNSlashingIBCPacket,
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

	// Emit slashed finality provider event so btc slasher/vigilante can slash the finality provider
	eventSlashing := finalitytypes.NewEventSlashedFinalityProvider(evidence)
	if err := sdk.UnwrapSDKContext(ctx).EventManager().EmitTypedEvent(eventSlashing); err != nil {
		return fmt.Errorf("failed to emit EventSlashedFinalityProvider event: %w", err)
	}

	return nil
}
