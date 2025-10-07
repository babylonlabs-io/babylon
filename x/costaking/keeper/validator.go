package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// updateValidatorSet stores the current validator set with their original tokens
// This is called upon AfterEpochBegins
func (k Keeper) updateValidatorSet(ctx context.Context, newValSetMap map[string]struct{}) error {
	var validatorSet types.ValidatorSet
	// Iterate over the new validator set map
	for valAddrStr := range newValSetMap {
		valAddr, err := sdk.ValAddressFromBech32(valAddrStr)
		if err != nil {
			return err
		}
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
			},
		)
	}

	return k.validatorSet.Set(ctx, validatorSet)
}
