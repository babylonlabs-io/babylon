package finality

import (
	"context"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v3/x/finality/keeper"
	"github.com/babylonlabs-io/babylon/v3/x/finality/types"
)

func BeginBlocker(ctx context.Context, k keeper.Keeper) error {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), telemetry.MetricKeyBeginBlocker)

	return k.BeginBlocker(ctx)
}

func EndBlocker(ctx context.Context, k keeper.Keeper) ([]abci.ValidatorUpdate, error) {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), telemetry.MetricKeyEndBlocker)

	// if the BTC staking protocol is activated, i.e., there exists a height where a finality provider
	// has voting power, start indexing and tallying blocks
	if k.IsFinalityActive(ctx) {
		// tally all non-finalised blocks
		k.TallyBlocks(ctx, types.MaxFinalizedRewardedBlocksPerEndBlock)

		// detect sluggish finality providers if there are any
		// heightToExamine is determined by the current height - params.FinalitySigTimeout
		// which indicates that finality providers have up to `params.FinalitySigTimeout` blocks
		// to send votes on the height to be examined as whether `missed` or not (1 or 0 of a
		// bit in a bit array of size params.SignedBlocksWindow)
		// once this height is judged as `missed`, the judgement is irreversible
		heightToExamine := sdk.UnwrapSDKContext(ctx).HeaderInfo().Height - k.GetParams(ctx).FinalitySigTimeout

		if heightToExamine >= 1 {
			k.HandleLiveness(ctx, heightToExamine)

			k.HandleRewarding(ctx, heightToExamine, types.MaxFinalizedRewardedBlocksPerEndBlock)
		}
	}

	return []abci.ValidatorUpdate{}, nil
}
