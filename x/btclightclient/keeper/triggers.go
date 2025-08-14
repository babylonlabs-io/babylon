package keeper

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
)

func (k Keeper) triggerHeaderInserted(ctx context.Context, headerInfo *types.BTCHeaderInfo) {
	// Trigger AfterBTCHeaderInserted hook
	k.AfterBTCHeaderInserted(ctx, headerInfo)
	// Emit HeaderInserted event
	k.emitTypedEventWithLog(ctx, &types.EventBTCHeaderInserted{Header: headerInfo})
}

// triggerRollBack calls the hook and emits an event, the rollbackFrom is the latest tip
// prior rollbackTo best block is sent
func (k Keeper) triggerRollBack(ctx context.Context, rollbackFrom, rollbackTo *types.BTCHeaderInfo) {
	// Safety check variables
	if err := CheckRollBackInvariants(rollbackFrom, rollbackTo); err != nil {
		panic(err)
	}
	// Trigger AfterBTCRollBack hook
	k.AfterBTCRollBack(ctx, rollbackFrom, rollbackTo)
	// Emit BTCRollBack event
	k.emitTypedEventWithLog(ctx, &types.EventBTCRollBack{Header: rollbackTo, RollbackFrom: rollbackFrom})
}

func (k Keeper) triggerRollForward(ctx context.Context, headerInfo *types.BTCHeaderInfo) {
	// Trigger AfterBTCRollForward hook
	k.AfterBTCRollForward(ctx, headerInfo)
	// Emit BTCRollForward event
	k.emitTypedEventWithLog(ctx, &types.EventBTCRollForward{Header: headerInfo})
}

// CheckRollBackInvariants validates that the values being called to trigger rollback
// are expected, if return an error it is probably an programming error.
func CheckRollBackInvariants(rollbackFrom, rollbackTo *types.BTCHeaderInfo) error {
	if rollbackFrom == nil {
		return fmt.Errorf("Call BTC rollback without tip")
	}

	if rollbackTo == nil {
		return fmt.Errorf("Call BTC rollback without rollbackTo")
	}

	// should verify that the BTC height it is rolling back is lower than the latest tip
	if rollbackTo.Height >= rollbackFrom.Height {
		return fmt.Errorf(
			"BTC rollback with rollback 'To' higher or equal than 'From'\n%s\n%s",
			fmt.Sprintf("'From' -> %d - %s", rollbackFrom.Height, rollbackFrom.Hash.MarshalHex()),
			fmt.Sprintf("'To' -> %d - %s", rollbackTo.Height, rollbackTo.Hash.MarshalHex()),
		)
	}

	return nil
}
