package keeper

import (
	"context"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

// IndexAllowedStakingTransaction indexes the given allowed staking transaction by its hash.
func (k Keeper) IndexAllowedStakingTransaction(ctx context.Context, txHash *chainhash.Hash) {
	err := k.AllowedStakingTxHashesKeySet.Set(ctx, txHash[:])
	if err != nil {
		panic(err) // encoding issue; this can only be a programming error
	}
}

// IsStakingTransactionAllowed checks if the given staking transaction is allowed.
func (k Keeper) IsStakingTransactionAllowed(ctx context.Context, txHash *chainhash.Hash) bool {
	has, err := k.AllowedStakingTxHashesKeySet.Has(ctx, txHash[:])
	if err != nil {
		panic(err) // encoding issue; this can only be a programming error
	}
	return has
}

// IndexAllowedMultiStakingTransaction indexes the given staking transaction hash
// as allowed for multi-staking during the multi-staking allow-list period.
func (k Keeper) IndexAllowedMultiStakingTransaction(ctx context.Context, txHash *chainhash.Hash) {
	err := k.allowedMultiStakingTxHashesKeySet.Set(ctx, txHash[:])
	if err != nil {
		panic(err) // encoding issue; this can only be a programming error
	}
}

// IsMultiStakingAllowed checks if the given staking transaction is elegible
// for multi-staking.
func (k Keeper) IsMultiStakingAllowed(ctx context.Context, txHash *chainhash.Hash) (bool, error) {
	inAllowList, err := k.allowedMultiStakingTxHashesKeySet.Has(ctx, txHash[:])
	if err != nil {
		return false, err
	}

	if inAllowList {
		return true, nil
	}

	// if tx hash is not in the allow list, but is already
	// a multi-staking tx, then it is allowed
	del := k.getBTCDelegation(ctx, *txHash)
	if del == nil {
		return false, nil
	}

	// if the delegation has more than one FP BTC PK, then it is a multi-staking tx
	return len(del.FpBtcPkList) > 1, nil
}
