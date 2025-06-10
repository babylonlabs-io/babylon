package keeper

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
)

// HaltIfBtcReorgLargerThanConfirmationDepth safety mechanism to stop the chain in case there is an BTC reorg
// higher than the BtcConfirmationDepth. In theory this should only happen if the babylon chain goes down for
// a period longer than (2 * BtcConfirmationDepth * 10min) and a malicious miner mines a large fork.
func (k *Keeper) HaltIfBtcReorgLargerThanConfirmationDepth(ctx context.Context) {
	p := k.btccKeeper.GetParams(ctx)

	largestReorg := k.GetLargestBtcReorg(ctx)
	if largestReorg == nil {
		return
	}

	if largestReorg.BlockDiff >= p.BtcConfirmationDepth {
		msg := fmt.Sprintf(
			"Reorg %d is larger than BTC confirmation Depth %d.\n%s\n%s", largestReorg.BlockDiff, p.BtcConfirmationDepth,
			fmt.Sprintf("'From' -> %d - %s", largestReorg.RollbackFrom.Height, largestReorg.RollbackFrom.Hash.MarshalHex()),
			fmt.Sprintf("'To' -> %d - %s", largestReorg.RollbackTo.Height, largestReorg.RollbackTo.Hash.MarshalHex()),
		)
		panic(msg)
	}
}

// SetLargestBtcReorg sets the new largest BTC block reorg if it is higher than the current
// value in the store.
func (k *Keeper) SetLargestBtcReorg(ctx context.Context, newLargestBlockReorg types.LargestBtcReOrg) error {
	exists, err := k.LargestBtcReorg.Has(ctx)
	if err != nil {
		panic(fmt.Errorf("setting largest btc reorg failed decode in Has: %w", err))
	}
	if !exists {
		return k.LargestBtcReorg.Set(ctx, newLargestBlockReorg)
	}

	currentLargestReorg, err := k.LargestBtcReorg.Get(ctx)
	if err != nil {
		panic(fmt.Errorf("setting largest btc reorg failed decode in Get: %w", err))
	}

	if currentLargestReorg.BlockDiff >= newLargestBlockReorg.BlockDiff {
		// no need to update if the current is higher
		return nil
	}

	return k.LargestBtcReorg.Set(ctx, newLargestBlockReorg)
}

// GetLargestBtcReorg returns nil if the value is not set yet
// but it panics if it fails to parse the value from the store after
// it was already set, as it is probably an programming error.
func (k *Keeper) GetLargestBtcReorg(ctx context.Context) *types.LargestBtcReOrg {
	exists, err := k.LargestBtcReorg.Has(ctx)
	if err != nil {
		panic(fmt.Errorf("must get largest btc reorg failed encode in Has: %w", err))
	}
	if !exists {
		return nil
	}

	largestReorg, err := k.LargestBtcReorg.Get(ctx)
	if err != nil {
		panic(fmt.Errorf("setting largest btc reorg failed encode in Get: %w", err))
	}

	return &largestReorg
}
