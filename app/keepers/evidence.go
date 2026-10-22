package keepers

import (
	"context"
	"errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// evidenceStakingKeeper reports a deleted validator as nil instead of an
// error. Babylon deletes unbonded validators at checkpoint finalization, long
// before CometBFT stops accepting evidence against them; x/evidence skips nil
// validators but returns the lookup error, which would fail FinalizeBlock.
type evidenceStakingKeeper struct{ *stakingkeeper.Keeper }

func (k evidenceStakingKeeper) ValidatorByConsAddr(ctx context.Context, addr sdk.ConsAddress) (stakingtypes.ValidatorI, error) {
	v, err := k.Keeper.ValidatorByConsAddr(ctx, addr)
	if errors.Is(err, stakingtypes.ErrNoValidatorFound) {
		return nil, nil
	}
	return v, err
}
