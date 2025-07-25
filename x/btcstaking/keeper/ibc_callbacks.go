package keeper

import (
	"encoding/json"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	callbacktypes "github.com/cosmos/ibc-go/v10/modules/apps/callbacks/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v3/x/incentive/types"
)

// Ensure that the incentive Keeper implements the ContractKeeper interface
var _ callbacktypes.ContractKeeper = (*Keeper)(nil)

// IBCSendPacketCallback is called when a packet is sent
// Not needed for BSN fee collection scenario
func (k Keeper) IBCSendPacketCallback(
	_ sdk.Context,
	_ string,
	_ string,
	_ clienttypes.Height,
	_ uint64,
	_ []byte,
	_ string,
	_ string,
	_ string,
) error {
	return nil
}

// IBCOnAcknowledgementPacketCallback is called when a packet acknowledgement is received
// Not needed for BSN fee collection scenario (this runs on source chain)
func (k Keeper) IBCOnAcknowledgementPacketCallback(
	_ sdk.Context,
	_ channeltypes.Packet,
	_ []byte,
	_ sdk.AccAddress,
	_ string,
	_ string,
	_ string,
) error {
	return nil
}

// IBCOnTimeoutPacketCallback is called when a packet times out
// Not needed for BSN fee collection scenario
func (k Keeper) IBCOnTimeoutPacketCallback(
	_ sdk.Context,
	_ channeltypes.Packet,
	_ sdk.AccAddress,
	_ string,
	_ string,
	_ string,
) error {
	return nil
}

// IBCReceivePacketCallback is called when a packet is received
// This is where we handle ICS20 transfers to the bsn_fee_collector account
func (k Keeper) IBCReceivePacketCallback(
	cachedCtx sdk.Context,
	packet ibcexported.PacketI,
	ack ibcexported.Acknowledgement,
	_ string,
	version string,
) error {
	k.Logger(cachedCtx).Info("IBCReceivePacketCallback called")

	// Early return if acknowledgement is not successful
	if !ack.Success() {
		return nil
	}

	// Parse packet data as ICS20 transfer first (before checking ack success)
	transferData, err := transfertypes.UnmarshalPacketData(packet.GetData(), version, "")
	if err != nil {
		return errorsmod.Wrap(err, "unmarshal transfer packet data")
	}

	// Check for JSON callback format
	var callbackMemo types.CallbackMemo
	if err := json.Unmarshal([]byte(transferData.Memo), &callbackMemo); err != nil {
		return err
	}

	k.Logger(cachedCtx).Info(
		"IBCReceivePacketCallback",
		"action", callbackMemo.Action,
		"memo_parse", transferData.Memo,
	)

	switch callbackMemo.Action {
	case types.CallbackActionAddBsnRewardsMemo:
		if callbackMemo.DestCallback == nil {
			return errorsmod.Wrap(types.ErrInvalidCallbackAddBsnRewards, "dest_callback property is nil")
		}

		addBsnRewards := callbackMemo.DestCallback.AddBsnRewards
		if addBsnRewards == nil {
			return errorsmod.Wrapf(types.ErrInvalidCallbackAddBsnRewards, "%s property is nil", types.CallbackActionAddBsnRewardsMemo)
		}

		err = k.processAddBsnRewards(cachedCtx, packet, &transferData, addBsnRewards)
		if err != nil {
			k.Logger(cachedCtx).Error(
				"ibc callback had an error processing add bsn rewards",
				"err", err.Error(),
			)
			return err
		}
	default:
		return nil
	}

	return nil
}

// processAddBsnRewards processes the BSN fee distribution
func (k Keeper) processAddBsnRewards(
	ctx sdk.Context,
	packet ibcexported.PacketI,
	transferData *transfertypes.InternalTransferRepresentation,
	callbackAddBsnRewards *types.CallbackAddBsnRewards,
) error {
	// Calculate the proper denom and amount of the ics20 packet
	bsnReward, err := BabylonRepresentationIcs20TransferCoin(packet, transferData.Token)
	if err != nil {
		return err
	}

	receiverOnBbnAddr, err := sdk.AccAddressFromBech32(transferData.Receiver)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid address %s: %v", transferData.Receiver, err)
	}

	return k.AddBsnRewards(ctx, receiverOnBbnAddr, callbackAddBsnRewards.BsnConsumerID, sdk.NewCoins(bsnReward), callbackAddBsnRewards.FpRatios)
}

// BabylonRepresentationIcs20TransferCoin it checks if the coin was from babylon genesis
// in the first place, if it was it removes the last denom trace to correctly parses the
// denom. If it did not come from babylon, it is a token created from BSN chain and it
// just adds the destination port and channel to the denom trace
func BabylonRepresentationIcs20TransferCoin(
	packet ibcexported.PacketI,
	token transfertypes.Token,
) (sdk.Coin, error) {
	transferAmount, ok := math.NewIntFromString(token.Amount)
	if !ok {
		return sdk.Coin{}, errorsmod.Wrapf(ictvtypes.ErrInvalidAmount, "invalid transfer amount: %s", token.Amount)
	}

	// This is the prefix that would have been prefixed to the denomination
	// on sender chain IF and only if the token originally came from the
	// receiving chain.
	//
	// NOTE: We use SourcePort and SourceChannel here, because the counterparty
	// chain would have prefixed with DestPort and DestChannel when originally
	// receiving this token.
	// https://github.com/cosmos/ibc-go/blob/a6217ab02a4d57c52a938eeaff8aeb383e523d12/modules/apps/transfer/keeper/relay.go#L147-L175
	if token.Denom.HasPrefix(packet.GetSourcePort(), packet.GetSourceChannel()) {
		// sender chain is not the source, unescrow tokens

		// remove prefix added by sender chain
		token.Denom.Trace = token.Denom.Trace[1:]
		return sdk.NewCoin(token.Denom.IBCDenom(), transferAmount), nil
	}

	// since SendPacket did not prefix the denomination, we must add the destination port and channel to the trace
	trace := []transfertypes.Hop{transfertypes.NewHop(packet.GetDestPort(), packet.GetDestChannel())}
	token.Denom.Trace = append(trace, token.Denom.Trace...)

	bsnDenom := token.Denom.IBCDenom()
	return sdk.NewCoin(bsnDenom, transferAmount), nil
}
