package types

import (
	"context"

	bbn "github.com/babylonlabs-io/babylon/types"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	etypes "github.com/babylonlabs-io/babylon/x/epoching/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type BTCStakingKeeper interface {
	GetParams(ctx context.Context) bstypes.Params
	GetCurrentBTCHeight(ctx context.Context) uint32
	GetBTCHeightAtBabylonHeight(ctx context.Context, babylonHeight uint64) uint32
	GetFinalityProvider(ctx context.Context, fpBTCPK []byte) (*bstypes.FinalityProvider, error)
	HasFinalityProvider(ctx context.Context, fpBTCPK []byte) bool
	SlashFinalityProvider(ctx context.Context, fpBTCPK []byte) error
	GetBTCDelegation(ctx context.Context, stakingTxHashStr string) (*bstypes.BTCDelegation, error)
	GetVotingPower(ctx context.Context, fpBTCPK []byte, height uint64) uint64
	GetVotingPowerTable(ctx context.Context, height uint64) map[string]uint64
	SetVotingPower(ctx context.Context, fpBTCPK []byte, height uint64, votingPower uint64)
	GetBTCStakingActivatedHeight(ctx context.Context) (uint64, error)
	GetVotingPowerDistCache(ctx context.Context, height uint64) *bstypes.VotingPowerDistCache
	SetVotingPowerDistCache(ctx context.Context, height uint64, dc *bstypes.VotingPowerDistCache)
	GetAllPowerDistUpdateEvents(ctx context.Context, lastBTCTipHeight, btcTipHeight uint32) []*bstypes.EventPowerDistUpdate
	ClearPowerDistUpdateEvents(ctx context.Context, btcHeight uint32)
	RemoveVotingPowerDistCache(ctx context.Context, height uint64)
	UnjailFinalityProvider(ctx context.Context, fpBTCPK []byte) error
}

type CheckpointingKeeper interface {
	GetEpoch(ctx context.Context) *etypes.Epoch
	GetLastFinalizedEpoch(ctx context.Context) uint64
}

// IncentiveKeeper defines the expected interface needed for distributing rewards
// and refund transaction fee for finality signatures
type IncentiveKeeper interface {
	RewardBTCStaking(ctx context.Context, height uint64, filteredDc *bstypes.VotingPowerDistCache)
	IndexRefundableMsg(ctx context.Context, msg sdk.Msg)
}

type FinalityHooks interface {
	AfterSluggishFinalityProviderDetected(ctx context.Context, btcPk *bbn.BIP340PubKey) error
}
