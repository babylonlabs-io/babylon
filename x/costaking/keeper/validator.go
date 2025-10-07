package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// updateValidatorSet stores the current validator set with their original tokens
// This is called upon AfterEpochBegins
func (k Keeper) updateValidatorSet(ctx context.Context) error {
	var validatorSet types.ValidatorSet
	// store the current validator set with their original tokens
	err := k.stkK.IterateLastValidatorPowers(ctx, func(valAddr sdk.ValAddress, power int64) bool {
		// store the original tokens delegated to the validator
		// We can get the validator from staking keeper
		val, err := k.stkK.GetValidator(ctx, valAddr)
		if err != nil {
			return true // stop iteration on error
		}
		validatorSet.Validators = append(
			validatorSet.Validators,
			&types.Validator{
				Addr:   valAddr,
				Tokens: val.Tokens,
			},
		)

		return false // continue iteration
	})

	if err != nil {
		return err
	}

	return k.validatorSet.Set(ctx, validatorSet)
}
