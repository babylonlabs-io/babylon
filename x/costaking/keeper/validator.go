package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// updateValidatorSet stores the current validator set with their original tokens and shares
// This is called upon AfterEpochBegins
func (k Keeper) updateValidatorSet(ctx context.Context, newValAddrs []sdk.ValAddress) error {
	var validatorSet types.ValidatorSet
	// Iterate over the new validator set map
	for _, valAddr := range newValAddrs {
		// store the original tokens delegated to the validator
		// We can get the validator from staking keeper
		val, err := k.stkK.GetValidator(ctx, valAddr)
		if err != nil {
			return err
		}
		validatorSet.Validators = append(
			validatorSet.Validators,
			&types.Validator{
				Addr:   valAddr,
				Tokens: val.Tokens,
				Shares: val.DelegatorShares,
			},
		)
	}

	return k.validatorSet.Set(ctx, validatorSet)
}
