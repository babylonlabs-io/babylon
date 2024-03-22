package types

import (
	"context"
	"math/big"

	bbn "github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
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

type BTCStkConsumerKeeper interface {
	IsChainRegistered(ctx context.Context, chainID string) bool
	HasFinalityProvider(ctx context.Context, fpBTCPK *bbn.BIP340PubKey) bool
	GetFinalityProviderChain(ctx context.Context, fpBTCPK *bbn.BIP340PubKey) (string, error)
	GetFinalityProvider(ctx context.Context, chainID string, fpBTCPK *bbn.BIP340PubKey) (*FinalityProvider, error)
	SetFinalityProvider(ctx context.Context, fp *FinalityProvider)
	AddBTCDelegation(ctx sdk.Context, btcDel *BTCDelegation) error
	SetBTCDelegation(ctx context.Context, btcDel *BTCDelegation)
	GetBTCDelegatorDelegationsResponses(ctx sdk.Context, fpBTCPK *bbn.BIP340PubKey, pagination *query.PageRequest, wValue uint64, btcHeight uint64, covenantQuorum uint32) ([]*BTCDelegatorDelegationsResponse, *query.PageResponse, error)
}
