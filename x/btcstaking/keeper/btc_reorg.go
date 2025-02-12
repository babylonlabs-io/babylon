package keeper

import (
	"context"
	"fmt"
)

// HaltIfBtcReorgLargerThanConfirmationDepth safety mechanism to stop the chain in case there is an BTC reorg
// higher than the BtcConfirmationDepth. In teory this should only happen if the babylon chain goes down for
// a period longer than (2 * BtcConfirmationDepth * 10min).
func (k *Keeper) HaltIfBtcReorgLargerThanConfirmationDepth(ctx context.Context) {
	p := k.btccKeeper.GetParams(ctx)

	largestReorg := k.MustGetLargestBtcReorg(ctx)
	if largestReorg > p.BtcConfirmationDepth {
		panic(fmt.Sprintf("Reorg %d is larger than BTC confirmation Depth %d", largestReorg, p.BtcConfirmationDepth))
	}
}

// SetLargestBtcReorg sets the new largest BTC block reorg if it is higher than the current
// value in the store.
func (k *Keeper) SetLargestBtcReorg(ctx context.Context, newLargestBlockReorg uint32) error {
	exists, err := k.LargestBtcReorgInBlocks.Has(ctx)
	if err != nil || !exists {
		return k.LargestBtcReorgInBlocks.Set(ctx, newLargestBlockReorg)
	}

	currentLargestReorg, err := k.LargestBtcReorgInBlocks.Get(ctx)
	if err != nil {
		return err
	}

	if currentLargestReorg >= newLargestBlockReorg {
		// no need to update if the current is higher
		return nil
	}

	return k.LargestBtcReorgInBlocks.Set(ctx, newLargestBlockReorg)
}

// MustGetLargestBtcReorg returns zero if the value is not set yet
// but it panics if it fails to parse the value from the store after
// it was already set, as it is probably an programming error.
func (k *Keeper) MustGetLargestBtcReorg(ctx context.Context) uint32 {
	exists, err := k.LargestBtcReorgInBlocks.Has(ctx)
	if err != nil || !exists {
		return 0
	}

	largestReorg, err := k.LargestBtcReorgInBlocks.Get(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to get the largest btc reorg: %w", err))
	}

	return largestReorg
}
