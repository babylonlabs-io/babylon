package types

import (
	"context"

	sdkmath "cosmossdk.io/math"
	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	etypes "github.com/babylonlabs-io/babylon/v3/x/epoching/types"
	"github.com/btcsuite/btcd/btcec/v2"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type BTCStakingKeeper interface {
	GetParams(ctx context.Context) bstypes.Params
	GetParamsByVersion(ctx context.Context, v uint32) *bstypes.Params
	GetCurrentBTCHeight(ctx context.Context) uint32
	GetBTCHeightAtBabylonHeight(ctx context.Context, babylonHeight uint64) uint32
	GetFinalityProvider(ctx context.Context, fpBTCPK []byte) (*bstypes.FinalityProvider, error)
	HasFinalityProvider(ctx context.Context, fpBTCPK []byte) bool
	BabylonFinalityProviderExists(ctx context.Context, fpBTCPK []byte) bool
	SlashFinalityProvider(ctx context.Context, fpBTCPK []byte) error
	PropagateFPSlashingToConsumers(ctx context.Context, fpBTCSK *btcec.PrivateKey) error
	GetBTCDelegation(ctx context.Context, stakingTxHashStr string) (*bstypes.BTCDelegation, error)
	GetAllPowerDistUpdateEvents(ctx context.Context, lastBTCTipHeight, btcTipHeight uint32) []*bstypes.EventPowerDistUpdate
	ClearPowerDistUpdateEvents(ctx context.Context, btcHeight uint32)
	JailFinalityProvider(ctx context.Context, fpBTCPK []byte) error
	UnjailFinalityProvider(ctx context.Context, fpBTCPK []byte) error
	UpdateFinalityProvider(ctx context.Context, fp *bstypes.FinalityProvider) error
	BtcDelHasCovenantQuorums(ctx context.Context, btcDel *bstypes.BTCDelegation, quorum uint32) (bool, error)
}

type CheckpointingKeeper interface {
	GetEpochByHeight(ctx context.Context, height uint64) uint64
	GetEpoch(ctx context.Context) *etypes.Epoch
	GetLastFinalizedEpoch(ctx context.Context) uint64
}

// IncentiveKeeper defines the expected interface needed for distributing rewards
// and refund transaction fee for finality signatures
type IncentiveKeeper interface {
	RewardBTCStaking(ctx context.Context, height uint64, filteredDc *VotingPowerDistCache, voters map[string]struct{})
	IndexRefundableMsg(ctx context.Context, msg sdk.Msg)
	AddEventBtcDelegationActivated(ctx context.Context, height uint64, fp, del sdk.AccAddress, sat uint64) error
	AddEventBtcDelegationUnbonded(ctx context.Context, height uint64, fp, del sdk.AccAddress, sat uint64) error
	BtcDelegationActivated(ctx context.Context, fp, del sdk.AccAddress, sat sdkmath.Int) error
	BtcDelegationUnbonded(ctx context.Context, fp, del sdk.AccAddress, sat sdkmath.Int) error
}
