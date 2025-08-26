package keeper

// TODO remove this file after migrating module to consensusVersion 3

import "context"

// RemoveAllAllowListsRecords removes all allowed staking and multi-staking transaction records.
// This function can be removed after migrating module to version 3
func (k Keeper) RemoveAllAllowListsRecords(ctx context.Context) error {
	// Clear all entries from both KeySets (nil ranger = clear all)
	if err := k.AllowedStakingTxHashesKeySet.Clear(ctx, nil); err != nil {
		return err
	}
	return k.allowedMultiStakingTxHashesKeySet.Clear(ctx, nil)
}
