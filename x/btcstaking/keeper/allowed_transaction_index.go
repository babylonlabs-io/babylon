package keeper

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
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
// for multi-staking. This logic is used during the multi-staking allow-list period
func (k Keeper) IsMultiStakingAllowed(ctx context.Context, parsedMsg *types.ParsedCreateDelegationMessage) (bool, error) {
	// if is not stake expansion, it is not allowed to create new delegations with multi-staking
	if parsedMsg == nil || parsedMsg.StkExp == nil {
		return false, types.ErrInvalidStakingTx.Wrap("it is not allowed to create new delegations with multi-staking during the multi-staking allow-list period")
	}

	// if it is stake expansion, we need to check if the previous staking tx hash
	// is in the allow list or the previous staking tx is a multi-staking tx
	txHash := parsedMsg.StkExp.PreviousActiveStkTxHash
	inAllowList, err := k.allowedMultiStakingTxHashesKeySet.Has(ctx, txHash[:])
	if err != nil {
		return false, fmt.Errorf("failed to check if the previous staking tx hash is eligible for multi-staking: %w", err)
	}

	// if tx hash is not in the allow list, but is already
	// a multi-staking tx (has more than one FP BTC PK), then it is allowed.
	del := k.getBTCDelegation(ctx, *txHash)
	if del == nil {
		return false, fmt.Errorf("failed to find BTC delegation for tx hash: %s when checking multi-staking eligibility", txHash.String())
	}

	// tx hash is not in the allow list,
	// and it is NOT a multi-staking tx,
	// then it is NOT allowed.
	if !inAllowList && len(del.FpBtcPkList) == 1 {
		return false, nil
	}

	// During the multi-staking allow-list period,
	// it is not allowed to increase the staking amount.
	stakeAmtChanged := uint64(parsedMsg.StakingValue) != del.TotalSat
	if stakeAmtChanged {
		return false, types.ErrInvalidStakingTx.Wrapf("it is not allowed to modify the staking amount during the multi-staking allow-list period. Previous amount: %d, new amount: %d",
			del.TotalSat, uint64(parsedMsg.StakingValue))
	}
	// If we reach here, it means the tx hash is in the allow list
	// or it is an already existing multi-staking delegation (extended from the allow-list).
	return true, nil
}
