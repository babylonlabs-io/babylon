package types

import (
	"context"

	bbn "github.com/babylonlabs-io/babylon/v3/types"
	btcctypes "github.com/babylonlabs-io/babylon/v3/x/btccheckpoint/types"
	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	btcstkconsumertypes "github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
	checkpointingtypes "github.com/babylonlabs-io/babylon/v3/x/checkpointing/types"
	epochingtypes "github.com/babylonlabs-io/babylon/v3/x/epoching/types"
	"github.com/btcsuite/btcd/btcec/v2"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types" //nolint:staticcheck
	connectiontypes "github.com/cosmos/ibc-go/v10/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
)

// AccountKeeper defines the contract required for account APIs.
type AccountKeeper interface {
	GetModuleAddress(name string) sdk.AccAddress
	GetModuleAccount(ctx context.Context, name string) sdk.ModuleAccountI
}

// BankKeeper defines the expected bank keeper
type BankKeeper interface {
	SendCoins(ctx context.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) error
	MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	BlockedAddr(addr sdk.AccAddress) bool
}

// ICS4Wrapper defines the expected ICS4Wrapper for middleware
type ICS4Wrapper interface {
	SendPacket(
		ctx sdk.Context,
		sourcePort string,
		sourceChannel string,
		timeoutHeight clienttypes.Height,
		timeoutTimestamp uint64,
		data []byte,
	) (uint64, error)
}

// ChannelKeeper defines the expected IBC channel keeper
type ChannelKeeper interface {
	GetChannel(ctx sdk.Context, srcPort, srcChan string) (channel channeltypes.Channel, found bool)
	GetNextSequenceSend(ctx sdk.Context, portID, channelID string) (uint64, bool)
	GetAllChannels(ctx sdk.Context) (channels []channeltypes.IdentifiedChannel)
	GetChannelClientState(ctx sdk.Context, portID, channelID string) (string, ibcexported.ClientState, error)
}

// ClientKeeper defines the expected IBC client keeper
type ClientKeeper interface {
	GetClientState(ctx sdk.Context, clientID string) (ibcexported.ClientState, bool)
	SetClientState(ctx sdk.Context, clientID string, clientState ibcexported.ClientState)
}

// ConnectionKeeper defines the expected IBC connection keeper
type ConnectionKeeper interface {
	GetConnection(ctx sdk.Context, connectionID string) (connection connectiontypes.ConnectionEnd, found bool)
}

type BTCLightClientKeeper interface {
	GetTipInfo(ctx context.Context) *btclctypes.BTCHeaderInfo
	GetMainChainFrom(ctx context.Context, startHeight uint32) []*btclctypes.BTCHeaderInfo
	GetMainChainUpTo(ctx context.Context, depth uint32) []*btclctypes.BTCHeaderInfo
	GetHeaderByHash(ctx context.Context, hash *bbn.BTCHeaderHashBytes) (*btclctypes.BTCHeaderInfo, error)
}

type BtcCheckpointKeeper interface {
	GetParams(ctx context.Context) (p btcctypes.Params)
	GetEpochData(ctx context.Context, e uint64) *btcctypes.EpochData
	GetBestSubmission(ctx context.Context, e uint64) (btcctypes.BtcStatus, *btcctypes.SubmissionKey, error)
	GetSubmissionData(ctx context.Context, sk btcctypes.SubmissionKey) *btcctypes.SubmissionData
	GetEpochBestSubmissionBtcInfo(ctx context.Context, ed *btcctypes.EpochData) *btcctypes.SubmissionBtcInfo
}

type CheckpointingKeeper interface {
	GetBLSPubKeySet(ctx context.Context, epochNumber uint64) ([]*checkpointingtypes.ValidatorWithBlsKey, error)
	GetRawCheckpoint(ctx context.Context, epochNumber uint64) (*checkpointingtypes.RawCheckpointWithMeta, error)
	GetLastFinalizedEpoch(ctx context.Context) uint64
}

type EpochingKeeper interface {
	GetHistoricalEpoch(ctx context.Context, epochNumber uint64) (*epochingtypes.Epoch, error)
	GetEpoch(ctx context.Context) *epochingtypes.Epoch
}

type BTCStakingKeeper interface {
	GetAllBTCStakingConsumerIBCPackets(ctx context.Context) map[string]*bstypes.BTCStakingIBCPacket
	DeleteBTCStakingConsumerIBCPacket(ctx context.Context, consumerID string)
	PropagateFPSlashingToConsumers(ctx context.Context, fpBTCSK *btcec.PrivateKey) error
	SlashFinalityProvider(ctx context.Context, fpBTCPK []byte) error
	GetFinalityProvider(ctx context.Context, fpBTCPK []byte) (*bstypes.FinalityProvider, error)
}

type BTCStkConsumerKeeper interface {
	RegisterConsumer(ctx context.Context, consumerRegister *btcstkconsumertypes.ConsumerRegister) error
	UpdateConsumer(ctx context.Context, consumerRegister *btcstkconsumertypes.ConsumerRegister) error
	GetConsumerRegister(ctx context.Context, consumerID string) (*btcstkconsumertypes.ConsumerRegister, error)
	IsConsumerRegistered(ctx context.Context, consumerID string) bool
	GetAllRegisteredConsumerIDs(ctx context.Context) []string
	IsCosmosConsumer(ctx context.Context, consumerID string) (bool, error)
}
