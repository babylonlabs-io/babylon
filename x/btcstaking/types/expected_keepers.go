package types

import (
	"context"
	"math/big"

	bbn "github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	etypes "github.com/babylonchain/babylon/x/epoching/types"
)

type BTCLightClientKeeper interface {
	GetBaseBTCHeader(ctx context.Context) *btclctypes.BTCHeaderInfo
	GetTipInfo(ctx context.Context) *btclctypes.BTCHeaderInfo
	GetHeaderByHash(ctx context.Context, hash *bbn.BTCHeaderHashBytes) *btclctypes.BTCHeaderInfo
}

type BtcCheckpointKeeper interface {
	GetPowLimit() *big.Int
	GetParams(ctx context.Context) (p btcctypes.Params)
}

type CheckpointingKeeper interface {
	GetEpoch(ctx context.Context) *etypes.Epoch
	GetLastFinalizedEpoch(ctx context.Context) uint64
}

type BTCStkConsumerKeeper interface {
	IsConsumerChainRegistered(ctx context.Context, chainID string) bool
	HasConsumerFinalityProvider(ctx context.Context, fpBTCPK *bbn.BIP340PubKey) bool
	GetConsumerFinalityProviderChain(ctx context.Context, fpBTCPK *bbn.BIP340PubKey) (string, error)
	GetConsumerFinalityProvider(ctx context.Context, chainID string, fpBTCPK *bbn.BIP340PubKey) (*FinalityProvider, error)
	SetConsumerFinalityProvider(ctx context.Context, fp *FinalityProvider)
}
