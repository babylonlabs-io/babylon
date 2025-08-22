package types

import (
	"context"

	storetypes "cosmossdk.io/store/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	etypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
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
	GetBTCDelegation(ctx context.Context, stakingTxHashStr string) (*bstypes.BTCDelegation, error)
	ClearPowerDistUpdateEvents(ctx context.Context, btcHeight uint32)
	JailFinalityProvider(ctx context.Context, fpBTCPK []byte) error
	UnjailFinalityProvider(ctx context.Context, fpBTCPK []byte) error
	UpdateFinalityProvider(ctx context.Context, fp *bstypes.FinalityProvider) error
	BtcDelHasCovenantQuorums(ctx context.Context, btcDel *bstypes.BTCDelegation, quorum uint32) (bool, error)
	PowerDistUpdateEventBtcHeightStoreIterator(ctx context.Context, btcHeight uint32) storetypes.Iterator
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
}

// Event Hooks
// These can be utilized to communicate between a finality keeper and another
// keeper which must take particular actions when finalty providers/delegators change
// state. The second keeper must implement this interface, which then the
// finality keeper can call.

// FinalityHooks event hooks for finality btcdelegation actions
type FinalityHooks interface {
	AfterBtcDelegationActivated(ctx context.Context, fpAddr, btcDelAddr sdk.AccAddress, fpSecuresBabylon bool, sats uint64) error
	AfterBtcDelegationUnbonded(ctx context.Context, fpAddr, btcDelAddr sdk.AccAddress, fpSecuresBabylon bool, sats uint64) error
}
