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
