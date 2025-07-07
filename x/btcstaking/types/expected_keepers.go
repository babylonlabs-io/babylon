package types

import (
	"context"

	bbn "github.com/babylonlabs-io/babylon/v3/types"
	btcctypes "github.com/babylonlabs-io/babylon/v3/x/btccheckpoint/types"
	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
}

type IncentiveKeeper interface {
	IndexRefundableMsg(ctx context.Context, msg sdk.Msg)
}
