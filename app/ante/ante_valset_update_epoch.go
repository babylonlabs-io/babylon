package ante

import (
	epochingk "github.com/babylonlabs-io/babylon/v2/x/epoching/keeper"
	epochingtypes "github.com/babylonlabs-io/babylon/v2/x/epoching/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	slashtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
)

type BlockValsetUpdateAtEndOfEpoch struct {
	epochK *epochingk.Keeper
}

func NewBlockValsetUpdateAtEndOfEpoch(
	k *epochingk.Keeper,
) BlockValsetUpdateAtEndOfEpoch {
	return BlockValsetUpdateAtEndOfEpoch{
		epochK: k,
	}
}

func (b BlockValsetUpdateAtEndOfEpoch) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	epoch := b.epochK.GetEpoch(ctx)

	if epoch.IsLastBlock(ctx) { // only validate if it is the last epoch block
		for _, m := range tx.GetMsgs() {
			switch m.(type) {
			case *slashtypes.MsgUnjail:
				return ctx, epochingtypes.ErrValsetUpdateAtEndBlock.Wrap("slashtypes.MsgUnjail is invalid at the end of epoch")
			default:
				// NOOP in case of other messages
			}
		}
	}

	return next(ctx, tx, simulate)
}
