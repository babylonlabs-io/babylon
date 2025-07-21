package zoneconcierge

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/keeper"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
)

type IBCModule struct {
	keeper keeper.Keeper
}

func NewIBCModule(k keeper.Keeper) IBCModule {
	return IBCModule{
		keeper: k,
	}
}

// OnChanOpenInit implements the IBCModule interface
func (im IBCModule) OnChanOpenInit(
	ctx sdk.Context,
	order channeltypes.Order,
	connectionHops []string,
	portID string,
	channelID string,
	counterparty channeltypes.Counterparty,
	version string,
) (string, error) {
	// the IBC channel has to be ordered
	if order != channeltypes.ORDERED {
		return "", errorsmod.Wrapf(channeltypes.ErrInvalidChannelOrdering, "expected %s channel, got %s ", channeltypes.ORDERED, order)
	}

	// Require portID to be the one that ZoneConcierge is bound to
	boundPort := im.keeper.GetPort(ctx)
	if boundPort != portID {
		return "", errorsmod.Wrapf(porttypes.ErrInvalidPort, "invalid port: %s, expected %s", portID, boundPort)
	}

	// ensure consistency of the protocol version
	if version != types.Version {
		return "", errorsmod.Wrapf(types.ErrInvalidVersion, "got %s, expected %s", version, types.Version)
	}

	// Get the first connection ID from the channel's connection hops
	if len(connectionHops) == 0 {
		return "", fmt.Errorf("no connection hops found for channel")
	}
	connectionID := connectionHops[0]

	// Handle the IBC handshake request, i.e., ensuring the client ID is registered as
	// a Cosmos consumer
	if err := im.keeper.HandleIBCChannelCreation(ctx, connectionID, channelID); err != nil {
		return "", err
	}

	return version, nil
}

// OnChanOpenTry implements the IBCModule interface
func (im IBCModule) OnChanOpenTry(
	ctx sdk.Context,
	order channeltypes.Order,
	connectionHops []string,
	portID,
	channelID string,
	counterparty channeltypes.Counterparty,
	counterpartyVersion string,
) (string, error) {
	// the IBC channel has to be ordered
	if order != channeltypes.ORDERED {
		return "", errorsmod.Wrapf(channeltypes.ErrInvalidChannelOrdering, "expected %s channel, got %s ", channeltypes.ORDERED, order)
	}

	// Require portID to be the one that ZoneConcierge is bound to
	boundPort := im.keeper.GetPort(ctx)
	if boundPort != portID {
		return "", errorsmod.Wrapf(porttypes.ErrInvalidPort, "invalid port: %s, expected %s", portID, boundPort)
	}

	// ensure consistency of the protocol version
	if counterpartyVersion != types.Version {
		return "", errorsmod.Wrapf(types.ErrInvalidVersion, "invalid counterparty version: got: %s, expected %s", counterpartyVersion, types.Version)
	}

	// Get the first connection ID from the channel's connection hops
	if len(connectionHops) == 0 {
		return "", fmt.Errorf("no connection hops found for channel")
	}
	connectionID := connectionHops[0]

	// Handle the IBC handshake request, i.e., ensuring the client ID is registered as
	// a Cosmos consumer
	if err := im.keeper.HandleIBCChannelCreation(ctx, connectionID, channelID); err != nil {
		return "", err
	}

	return types.Version, nil
}

// OnChanOpenAck implements the IBCModule interface
func (im IBCModule) OnChanOpenAck(
	ctx sdk.Context,
	portID,
	channelID string,
	_,
	counterpartyVersion string,
) error {
	// check version consistency
	if counterpartyVersion != types.Version {
		return errorsmod.Wrapf(types.ErrInvalidVersion, "invalid counterparty version: %s, expected %s", counterpartyVersion, types.Version)
	}

	return nil
}

// OnChanOpenConfirm implements the IBCModule interface
func (im IBCModule) OnChanOpenConfirm(
	ctx sdk.Context,
	portID,
	channelID string,
) error {
	return nil
}

// OnChanCloseInit implements the IBCModule interface
func (im IBCModule) OnChanCloseInit(
	ctx sdk.Context,
	portID,
	channelID string,
) error {
	// Disallow user-initiated channel closing for channels
	return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "user cannot close channel")
}

// OnChanCloseConfirm implements the IBCModule interface
func (im IBCModule) OnChanCloseConfirm(
	ctx sdk.Context,
	portID,
	channelID string,
) error {
	return nil
}

// OnRecvPacket implements the IBCModule interface
func (im IBCModule) OnRecvPacket(
	ctx sdk.Context,
	channelVersion string,
	modulePacket channeltypes.Packet,
	relayer sdk.AccAddress,
) ibcexported.Acknowledgement {
	var packetData types.InboundPacket
	if errProto := types.ModuleCdc.Unmarshal(modulePacket.GetData(), &packetData); errProto != nil {
		im.keeper.Logger(ctx).Error("Failed to unmarshal packet data with protobuf", "error", errProto)
		if errJSON := types.ModuleCdc.UnmarshalJSON(modulePacket.GetData(), &packetData); errJSON != nil {
			im.keeper.Logger(ctx).Error("Failed to unmarshal packet data with JSON", "error", errJSON)
			return channeltypes.NewErrorAcknowledgement(fmt.Errorf("cannot unmarshal packet data with protobuf (error: %v) or JSON (error: %v)", errProto, errJSON))
		}
	}

	switch packet := packetData.Packet.(type) {
	case *types.InboundPacket_BsnSlashing:
		err := im.keeper.HandleConsumerSlashing(ctx, modulePacket.DestinationPort, modulePacket.DestinationChannel, packet.BsnSlashing)
		if err != nil {
			return channeltypes.NewErrorAcknowledgement(err)
		}
		return channeltypes.NewResultAcknowledgement([]byte("Consumer slashing handled successfully"))
	case *types.InboundPacket_BsnBaseBtcHeader:
		err := im.keeper.HandleBSNBaseBTCHeader(ctx, modulePacket.DestinationPort, modulePacket.DestinationChannel, packet.BsnBaseBtcHeader.BaseBtcHeader)
		if err != nil {
			return channeltypes.NewErrorAcknowledgement(err)
		}
		return channeltypes.NewResultAcknowledgement([]byte("BSN base BTC header updated successfully"))
	default:
		errMsg := fmt.Sprintf("unrecognized inbound packet type: %T", packet)
		return channeltypes.NewErrorAcknowledgement(errorsmod.Wrap(sdkerrors.ErrUnknownRequest, errMsg))
	}
}

// OnAcknowledgementPacket implements the IBCModule interface
func (im IBCModule) OnAcknowledgementPacket(
	ctx sdk.Context,
	channelVersion string,
	modulePacket channeltypes.Packet,
	acknowledgement []byte,
	relayer sdk.AccAddress,
) error {
	var ack channeltypes.Acknowledgement
	// `x/wasm` uses both protobuf and json to encoded acknowledgement, so we need to try both here
	// - for acknowledgment message with errors defined in `x/wasm`, it uses json
	// - for all other acknowledgement messages, it uses protobuf
	if errProto := types.ModuleCdc.Unmarshal(acknowledgement, &ack); errProto != nil {
		im.keeper.Logger(ctx).Warn("cannot unmarshal packet acknowledgement with protobuf, trying json.")
		if errJson := types.ModuleCdc.UnmarshalJSON(acknowledgement, &ack); errJson != nil {
			im.keeper.Logger(ctx).Error("cannot unmarshal packet acknowledgement with json.", "error", errJson)
			return errorsmod.Wrapf(sdkerrors.ErrUnknownRequest, "cannot unmarshal packet acknowledgement with protobuf (error: %v) or json (error: %v)", errProto, errJson)
		}
	}

	switch resp := ack.Response.(type) {
	case *channeltypes.Acknowledgement_Result:
		im.keeper.Logger(ctx).Info("received an Acknowledgement message.", "result", string(resp.Result))
		// TODO: emit typed event
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeAck,
				sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
				sdk.NewAttribute(types.AttributeKeyAckSuccess, string(resp.Result)),
			),
		)
	case *channeltypes.Acknowledgement_Error:
		im.keeper.Logger(ctx).Error("received an Acknowledgement error message.", "error", resp.Error)
		// TODO: emit typed event
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeAck,
				sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
				sdk.NewAttribute(types.AttributeKeyAckError, resp.Error),
			),
		)
	}

	return nil
}

// OnTimeoutPacket implements the IBCModule interface
func (im IBCModule) OnTimeoutPacket(
	ctx sdk.Context,
	channelVersion string,
	modulePacket channeltypes.Packet,
	relayer sdk.AccAddress,
) error {
	var packetData types.InboundPacket
	if err := packetData.Unmarshal(modulePacket.GetData()); err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrUnknownRequest, "cannot unmarshal packet data: %s", err.Error())
	}

	// TODO: close channel upon timeout

	return nil
}
