package keeper

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

// HaltIfBtcReorgLargerThanConfirmationDepth safety mechanism to stop the chain in case there is an BTC reorg
// higher than the BtcConfirmationDepth. In teory this should only happen if the babylon chain goes down for
// a period longer than (2 * BtcConfirmationDepth * 10min).
func (k *Keeper) HaltIfBtcReorgLargerThanConfirmationDepth(ctx context.Context) {
	p := k.btccKeeper.GetParams(ctx)

	largestReorg := k.MustGetLargestBtcReorgBlockDiff(ctx)
	if largestReorg >= p.BtcConfirmationDepth {
		panic(fmt.Sprintf("Reorg %d is larger than BTC confirmation Depth %d", largestReorg, p.BtcConfirmationDepth))
	}
}

// SetLargestBtcReorg sets the new largest BTC block reorg if it is higher than the current
// value in the store.
func (k *Keeper) SetLargestBtcReorg(ctx context.Context, newLargestBlockReorg types.LargestBtcReOrg) error {
	exists, err := k.LargestBtcReorg.Has(ctx)
	if err != nil {
		panic(fmt.Errorf("setting largest btc reorg failed encode in Has: %w", err))
	}
	if !exists {
		return k.LargestBtcReorg.Set(ctx, newLargestBlockReorg)
	}

	currentLargestReorg, err := k.LargestBtcReorg.Get(ctx)
	if err != nil {
		panic(fmt.Errorf("setting largest btc reorg failed encode in Get: %w", err))
	}

	if currentLargestReorg.BlockDiff >= newLargestBlockReorg.BlockDiff {
		// no need to update if the current is higher
		return nil
	}

	return k.LargestBtcReorg.Set(ctx, newLargestBlockReorg)
}

// MustGetLargestBtcReorg returns zero if the value is not set yet
// but it panics if it fails to parse the value from the store after
// it was already set, as it is probably an programming error.
func (k *Keeper) MustGetLargestBtcReorgBlockDiff(ctx context.Context) uint32 {
	exists, err := k.LargestBtcReorg.Has(ctx)
	if err != nil {
		panic(fmt.Errorf("must get largest btc reorg failed encode in Has: %w", err))
	}
	if !exists {
		return 0
	}

	largestReorg, err := k.LargestBtcReorg.Get(ctx)
	if err != nil {
		panic(fmt.Errorf("setting largest btc reorg failed encode in Get: %w", err))
	}

	return largestReorg.BlockDiff
}
