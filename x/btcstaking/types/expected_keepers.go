package types

import (
	"context"

	bbn "github.com/babylonlabs-io/babylon/v3/types"
	btcctypes "github.com/babylonlabs-io/babylon/v3/x/btccheckpoint/types"
	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	btcstkconsumertypes "github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
)

type BTCLightClientKeeper interface {
	GetBaseBTCHeader(ctx context.Context) *btclctypes.BTCHeaderInfo
	GetTipInfo(ctx context.Context) *btclctypes.BTCHeaderInfo
	GetHeaderByHash(ctx context.Context, hash *bbn.BTCHeaderHashBytes) (*btclctypes.BTCHeaderInfo, error)
}

type BtcCheckpointKeeper interface {
	GetParams(ctx context.Context) (p btcctypes.Params)
}

type FinalityKeeper interface {
	HasTimestampedPubRand(ctx context.Context, fpBtcPK *bbn.BIP340PubKey, height uint64) bool
}

type BTCStkConsumerKeeper interface {
	IsConsumerRegistered(ctx context.Context, consumerID string) bool
	IsCosmosConsumer(ctx context.Context, consumerID string) (bool, error)
	GetConsumerRegister(ctx context.Context, consumerID string) (*btcstkconsumertypes.ConsumerRegister, error)
}

type IncentiveKeeper interface {
	IndexRefundableMsg(ctx context.Context, msg sdk.Msg)
	AddFinalityProviderRewardsForBtcDelegations(ctx context.Context, fp sdk.AccAddress, rwd sdk.Coins) error
	AccumulateRewardGaugeForFP(ctx context.Context, addr sdk.AccAddress, reward sdk.Coins)
}

type BankKeeper interface {
	SpendableCoins(ctx context.Context, addr sdk.AccAddress) sdk.Coins
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToModule(ctx context.Context, senderModule, recipientModule string, amt sdk.Coins) error
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
