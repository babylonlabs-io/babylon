package types

import (
	"context"

	"cosmossdk.io/math"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type BTCStakingKeeper interface {
	GetParams(ctx context.Context) bstypes.Params
	// Total active satoshi staked that is entitled to earn rewards.
	TotalSatoshiStaked(ctx context.Context) (math.Int, error)
	// Iterate over all the delegators that have some active BTC delegator staked
	// and the total satoshi staked for that delegator address until an error is returned
	// or the iterator finishes. Stops if error is returned.
	// Should keep track of the total satoshi staked per delegator to avoid iterating over the
	// delegator delegations
	IterateDelegators(ctx context.Context, i func(delegator sdk.AccAddress, totalSatoshiStaked math.Int) error) error
}

type StakingKeeper interface {
	TotalBondedTokens(ctx context.Context) (math.Int, error)
	GetDelegatorBonded(ctx context.Context, delegator sdk.AccAddress) (math.Int, error)
}
