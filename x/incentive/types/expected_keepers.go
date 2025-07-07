package types

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	epochingtypes "github.com/babylonlabs-io/babylon/v3/x/epoching/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
)

type AccountKeeper interface {
	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
	GetModuleAccount(ctx context.Context, name string) sdk.ModuleAccountI
}

type BankKeeper interface {
	SpendableCoins(ctx context.Context, addr sdk.AccAddress) sdk.Coins
	GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins
	SendCoinsFromModuleToModule(ctx context.Context, senderModule string, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	BlockedAddr(addr sdk.AccAddress) bool
}

type EpochingKeeper interface {
	GetEpoch(ctx context.Context) *epochingtypes.Epoch
}

// ContractKeeper defines the entry points exposed to the IBC callbacks middleware
type ContractKeeper interface {
	// IBCSendPacketCallback is called in the source chain when a PacketSend is executed
	IBCSendPacketCallback(
		cachedCtx sdk.Context,
		sourcePort string,
		sourceChannel string,
		timeoutHeight clienttypes.Height,
		timeoutTimestamp uint64,
		packetData []byte,
		contractAddress,
		packetSenderAddress string,
		version string,
	) error

	// IBCOnAcknowledgementPacketCallback is called in the source chain when a packet acknowledgement is received
	IBCOnAcknowledgementPacketCallback(
		cachedCtx sdk.Context,
		packet channeltypes.Packet,
		acknowledgement []byte,
		relayer sdk.AccAddress,
		contractAddress,
		packetSenderAddress string,
		version string,
	) error

	// IBCOnTimeoutPacketCallback is called in the source chain when a packet times out
	IBCOnTimeoutPacketCallback(
		cachedCtx sdk.Context,
		packet channeltypes.Packet,
		relayer sdk.AccAddress,
		contractAddress,
		packetSenderAddress string,
		version string,
	) error

	// IBCReceivePacketCallback is called in the destination chain when a packet is received
	IBCReceivePacketCallback(
		cachedCtx sdk.Context,
		packet ibcexported.PacketI,
		ack ibcexported.Acknowledgement,
		contractAddress string,
		version string,
	) error
}
