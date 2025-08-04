package replay

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	connectiontypes "github.com/cosmos/ibc-go/v10/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v10/modules/core/23-commitment/types"

	babylonApp "github.com/babylonlabs-io/babylon/v3/app"
)

func OpenChannelForConsumer(ctx sdk.Context, app *babylonApp.BabylonApp, consumerID string) {
	// set channel
	channelID := fmt.Sprintf("channel-%s", consumerID)
	channel := channeltypes.NewChannel(
		channeltypes.OPEN, channeltypes.UNORDERED, channeltypes.NewCounterparty(consumerID, channelID),
		[]string{consumerID}, "1.0.0",
	)
	app.IBCKeeper.ChannelKeeper.SetChannel(ctx, app.ZoneConciergeKeeper.GetPort(ctx), channelID, channel)

	// set connection
	prefix := app.IBCKeeper.ConnectionKeeper.GetCommitmentPrefix()
	counterParty := connectiontypes.NewCounterparty(consumerID, "", commitmenttypes.NewMerklePrefix(prefix.Bytes()))

	connection := connectiontypes.NewConnectionEnd(connectiontypes.OPEN, consumerID, counterParty, nil, 10)
	app.IBCKeeper.ConnectionKeeper.SetConnection(
		ctx,
		channel.ConnectionHops[0],
		connection,
	)
}
