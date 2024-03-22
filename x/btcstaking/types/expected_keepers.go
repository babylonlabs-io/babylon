package types

import (
	"context"
	"math/big"

	bbn "github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	etypes "github.com/babylonchain/babylon/x/epoching/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	query "github.com/cosmos/cosmos-sdk/types/query"
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
	IsChainRegistered(ctx context.Context, chainID string) bool
	HasFinalityProvider(ctx context.Context, fpBTCPK *bbn.BIP340PubKey) bool
	SetFinalityProvider(ctx context.Context, fp *FinalityProvider)
	AddBTCDelegation(ctx sdk.Context, btcDel *BTCDelegation) error
	SetBTCDelegation(ctx context.Context, btcDel *BTCDelegation)
	GetBTCDelegatorDelegationsResponses(ctx sdk.Context, fpBTCPK *bbn.BIP340PubKey, pagination *query.PageRequest, wValue uint64, btcHeight uint64, covenantQuorum uint32) ([]*BTCDelegatorDelegationsResponse, *query.PageResponse, error)
}
