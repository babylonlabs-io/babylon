package costaking

import (
	"context"
	"time"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/costaking/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
)

func BeginBlocker(ctx context.Context, k keeper.Keeper) error {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), telemetry.MetricKeyBeginBlocker)

	if sdk.UnwrapSDKContext(ctx).HeaderInfo().Height <= 0 {
		return nil
	}
	// handle coins in the fee collector account, including
	// - send a portion of coins in the fee collector account to the costaking module account
	// - accumulate the entitled portion to the current rewards
	return k.HandleCoinsInFeeCollector(ctx)
}

func EndBlocker(ctx context.Context, k keeper.Keeper) error {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), telemetry.MetricKeyEndBlocker)

	return k.EndBlock(ctx)
}
