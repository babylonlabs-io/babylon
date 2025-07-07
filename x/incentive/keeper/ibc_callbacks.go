package keeper

import (
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	"crypto/sha256"
	"encoding/json"
	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"

	"github.com/babylonlabs-io/babylon/v3/x/incentive/types"
)

// Ensure that the incentive Keeper implements the ContractKeeper interface
var _ types.ContractKeeper = (*Keeper)(nil)

const (
	// BSNRewardDistributionMemo is the memo string indicating BSN reward distribution
	// TODO: we should use this to check if the correct memo is used in the transfer
	BSNRewardDistributionMemo = "bsn_reward_distribution"
)

// CallbackMemo defines the structure for callback memo in IBC transfers
// TODO: We can add the fp distribution here in the future
type CallbackMemo struct {
	DestCallback *CallbackInfo `json:"dest_callback,omitempty"`
	Action       string        `json:"action,omitempty"`
}

// CallbackInfo contains the callback information
type CallbackInfo struct {
	Address string `json:"address"`
}

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
	var callbackMemo CallbackMemo
	if err := json.Unmarshal([]byte(transferData.Memo), &callbackMemo); err != nil {
		return err
	}

	// TODO: Here we can directly distribute to BTC stakers and do whatever with the rest.
	// Process the BSN fee distribution
	return k.processBSNFeeDistribution(cachedCtx, packet.GetDestPort(), packet.GetDestChannel(), transferData)
}

// parseTransferData parses the packet data as ICS20 transfer data
func (k Keeper) parseTransferData(packet ibcexported.PacketI) (*transfertypes.FungibleTokenPacketData, error) {
	var transferData transfertypes.FungibleTokenPacketData
	if err := transfertypes.ModuleCdc.UnmarshalJSON(packet.GetData(), &transferData); err != nil {
		return nil, err
	}
	return &transferData, nil
}

// getTestDistributionAddress returns a deterministic test address for distribution
// This is used instead of the distribution module which can't receive custom tokens
func (k Keeper) getTestDistributionAddress() sdk.AccAddress {
	// Create a deterministic address based on a fixed seed
	// This ensures the same address is used across test runs
	hash := sha256.Sum256([]byte("test_distribution_account"))
	return sdk.AccAddress(hash[:20])
}

// processBSNFeeDistribution processes the BSN fee distribution
func (k Keeper) processBSNFeeDistribution(
	ctx sdk.Context,
	destPort string,
	destChannel string,
	transferData *transfertypes.FungibleTokenPacketData,
) error {
	// Parse the transfer amount
	transferAmount, ok := math.NewIntFromString(transferData.Amount)
	if !ok {
		return errorsmod.Wrapf(types.ErrInvalidAmount, "invalid transfer amount: %s", transferData.Amount)
	}

	// Calculate the IBC denom representation.
	ibcDenom := transfertypes.NewDenom(transferData.Denom, transfertypes.NewHop(destPort, destChannel)).IBCDenom()

	// NOTE: This is a PoC implementation. Where just split 50/50 between
	// a random address and the bsn_fee_collector account.

	// Calculate distribution amount (50% of transfer)
	distributionAmount := transferAmount.QuoRaw(2)
	distributionPortion := sdk.NewCoins(sdk.NewCoin(ibcDenom, distributionAmount))

	// Send portion to a test account (since distribution module can't receive custom tokens)
	// Use a deterministic test address for consistent testing
	testDistributionAddr := k.getTestDistributionAddress()

	if err := k.bankKeeper.SendCoinsFromModuleToAccount(
		ctx,
		types.BSNFeeCollectorName,
		testDistributionAddr,
		distributionPortion,
	); err != nil {
		return errorsmod.Wrapf(err, "failed to send portion to test distribution account")
	}

	// TODO: Emit some events for traceability

	return nil
}
