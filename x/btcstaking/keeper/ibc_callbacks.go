package keeper

import (
	"encoding/json"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
var _ types.ContractKeeper = (*Keeper)(nil)

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
	_ string,
) error {
	// Parse packet data as ICS20 transfer first (before checking ack success)
	transferData, err := k.parseTransferData(packet)
	if err != nil {
		return err
	}

	// Early return if acknowledgement is not successful
	if !ack.Success() {
		return nil
	}

	// Check for JSON callback format
	var callbackMemo types.CallbackMemo
	if err := json.Unmarshal([]byte(transferData.Memo), &callbackMemo); err != nil {
		return err
	}

	switch callbackMemo.Action {
	case types.CallbackActionAddBsnRewardsMemo:
		if callbackMemo.AddBsnRewards == nil {
			return errorsmod.Wrapf(types.ErrInvalidCallbackAddBsnRewards, "%s property is nil", types.CallbackActionAddBsnRewardsMemo)
		}
		// return k.processAddBsnRewards(cachedCtx, packet.GetSourcePort(), packet.GetSourceChannel(), transferData, callbackMemo.AddBsnRewards)
		err = k.processAddBsnRewards(cachedCtx, packet.GetDestPort(), packet.GetDestChannel(), transferData, callbackMemo.AddBsnRewards)
		if err != nil {
			panic(fmt.Sprintf("failed to run processAddBsnRewards: %s", err.Error()))
		}
	}

	return nil
}

// parseTransferData parses the packet data as ICS20 transfer data
func (k Keeper) parseTransferData(packet ibcexported.PacketI) (*transfertypes.FungibleTokenPacketData, error) {
	var transferData transfertypes.FungibleTokenPacketData
	if err := transfertypes.ModuleCdc.UnmarshalJSON(packet.GetData(), &transferData); err != nil {
		return nil, err
	}
	return &transferData, nil
}

// processAddBsnRewards processes the BSN fee distribution
func (k Keeper) processAddBsnRewards(
	ctx sdk.Context,
	destPort string,
	destChannel string,
	transferData *transfertypes.FungibleTokenPacketData,
	callbackAddBsnRewards *types.CallbackAddBsnRewards,
) error {
	transferAmount, ok := math.NewIntFromString(transferData.Amount)
	if !ok {
		return errorsmod.Wrapf(ictvtypes.ErrInvalidAmount, "invalid transfer amount: %s", transferData.Amount)
	}

	// Calculate the IBC denom representation on babylon chain.
	ibcDenom := transfertypes.NewDenom(transferData.Denom, transfertypes.NewHop(destPort, destChannel)).IBCDenom()
	bsnRewards := sdk.NewCoins(sdk.NewCoin(ibcDenom, transferAmount))

	receiverOnBbnAddr, err := sdk.AccAddressFromBech32(transferData.Receiver)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid address %s: %v", transferData.Receiver, err)
	}

	return k.AddBsnRewards(ctx, receiverOnBbnAddr, callbackAddBsnRewards.BsnConsumerID, bsnRewards, callbackAddBsnRewards.FpRatios)
}
