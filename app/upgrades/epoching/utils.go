package epoching

import (
	"time"

	epoching "github.com/babylonlabs-io/babylon/v4/x/epoching"
	epochingkeeper "github.com/babylonlabs-io/babylon/v4/x/epoching/keeper"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ProgressToNextEpochOptions defines options for epoch progression
type ProgressToNextEpochOptions struct {
	// CallEndBlocker determines whether to call EndBlocker during progression
	CallEndBlocker bool
}

// ProgressToNextEpoch progresses the context to the next epoch using sequential block progression
// This function mimics real blockchain behavior by advancing through each block sequentially
func ProgressToNextEpoch(ctx sdk.Context, keeper epochingkeeper.Keeper, opts *ProgressToNextEpochOptions) (sdk.Context, error) {
	if opts == nil {
		opts = &ProgressToNextEpochOptions{CallEndBlocker: false}
	}

	currentEpoch := keeper.GetEpoch(ctx)

	// Calculate target height for next epoch
	// Next epoch starts at: FirstBlockHeight + CurrentEpochInterval
	targetHeight := int64(currentEpoch.FirstBlockHeight + currentEpoch.CurrentEpochInterval)

	// Progress through blocks sequentially until we reach the target height
	for ctx.BlockHeight() < targetHeight {
		currentHeight := ctx.BlockHeight() + 1

		// Update block header with new height and time
		blkHeader := ctx.BlockHeader()
		blkHeader.Height = currentHeight
		blkHeader.Time = ctx.BlockTime().Add(time.Duration(currentHeight) * time.Second)
		ctx = ctx.WithBlockHeader(blkHeader).WithBlockHeight(currentHeight)

		// Update header info for consistency
		info := ctx.HeaderInfo()
		info.Height = currentHeight
		info.Time = blkHeader.Time
		ctx = ctx.WithHeaderInfo(info)

		// Call BeginBlocker to handle epoch transitions
		if err := epoching.BeginBlocker(ctx, keeper); err != nil {
			return ctx, err
		}

		// Optionally call EndBlocker if requested
		if opts.CallEndBlocker {
			if _, err := epoching.EndBlocker(ctx, keeper); err != nil {
				return ctx, err
			}
		}
	}

	return ctx, nil
}
