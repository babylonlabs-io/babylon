package replay

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	connectiontypes "github.com/cosmos/ibc-go/v10/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v10/modules/core/23-commitment/types"
	ibctmtypes "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"

	babylonApp "github.com/babylonlabs-io/babylon/v3/app"
	zctypes "github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
)

func OpenChannelForConsumer(ctx sdk.Context, app *babylonApp.BabylonApp, consumerID string) {
	// set client state
	app.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerID, &ibctmtypes.ClientState{})

	// set channel
	channelID := fmt.Sprintf("channel-%s", consumerID)
	channel := channeltypes.NewChannel(
		channeltypes.OPEN, channeltypes.ORDERED, channeltypes.NewCounterparty(consumerID, channelID),
		[]string{consumerID}, zctypes.Version,
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
