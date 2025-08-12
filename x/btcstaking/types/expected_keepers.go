package types

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	bbn "github.com/babylonlabs-io/babylon/v3/types"
	btcctypes "github.com/babylonlabs-io/babylon/v3/x/btccheckpoint/types"
	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	btcstkconsumertypes "github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
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
	GetConsumerID(ctx sdk.Context, portID, channelID string) (consumerID string, err error)
	ConsumerHasIBCChannelOpen(ctx context.Context, consumerID, channelID string) bool
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
