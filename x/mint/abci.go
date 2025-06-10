package mint

import (
	"context"
	"time"

	"github.com/babylonlabs-io/babylon/v3/x/mint/keeper"
	"github.com/babylonlabs-io/babylon/v3/x/mint/types"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BeginBlocker updates the inflation rate, annual provisions, and then mints
// the block provision for the current block.
func BeginBlocker(ctx context.Context, k keeper.Keeper) {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), telemetry.MetricKeyBeginBlocker)

	maybeUpdateMinter(ctx, k)
	mintBlockProvision(ctx, k)
	setPreviousBlockTime(ctx, k)
}

// maybeUpdateMinter updates the inflation rate and annual provisions if the
// inflation rate has changed. The inflation rate is expected to change once per
// year at the genesis time anniversary until the TargetInflationRate is
// reached.
func maybeUpdateMinter(ctx context.Context, k keeper.Keeper) {
	minter := k.GetMinter(ctx)
	genesisTime := k.GetGenesisTime(ctx).GenesisTime
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	newInflationRate := minter.CalculateInflationRate(sdkCtx, *genesisTime)

	isNonZeroAnnualProvisions := !minter.AnnualProvisions.IsZero()
	if newInflationRate.Equal(minter.InflationRate) && isNonZeroAnnualProvisions {
		// The minter's InflationRate and AnnualProvisions already reflect the
		// values for this year. Exit early because we don't need to update
		// them. AnnualProvisions must be updated if it is zero (expected at
		// genesis).
		return
	}

	totalSupply, err := k.StakingTokenSupply(ctx)
	if err != nil {
		panic(err)
	}

	minter.InflationRate = newInflationRate
	minter.AnnualProvisions = newInflationRate.MulInt(totalSupply)
	if err := k.SetMinter(ctx, minter); err != nil {
		panic(err)
	}
}

// mintBlockProvision mints the block provision for the current block.
func mintBlockProvision(ctx context.Context, k keeper.Keeper) {
	minter := k.GetMinter(ctx)
	if minter.PreviousBlockTime == nil {
		// exit early if previous block time is nil
		// this is expected to happen for block height = 1
		return
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	toMintCoin, err := minter.CalculateBlockProvision(sdkCtx.BlockTime(), *minter.PreviousBlockTime)
	if err != nil {
		panic(err)
	}
	toMintCoins := sdk.NewCoins(toMintCoin)

	err = k.MintCoins(ctx, toMintCoins)
	if err != nil {
		panic(err)
	}

	err = k.SendCoinsToFeeCollector(ctx, toMintCoins)
	if err != nil {
		panic(err)
	}

	if toMintCoin.Amount.IsInt64() {
		defer telemetry.ModuleSetGauge(types.ModuleName, float32(toMintCoin.Amount.Int64()), "minted_tokens")
	}

	// TODO: emit typed event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeMint,
			sdk.NewAttribute(types.AttributeKeyInflationRate, minter.InflationRate.String()),
			sdk.NewAttribute(types.AttributeKeyAnnualProvisions, minter.AnnualProvisions.String()),
			sdk.NewAttribute(sdk.AttributeKeyAmount, toMintCoin.Amount.String()),
		),
	)
}

func setPreviousBlockTime(ctx context.Context, k keeper.Keeper) {
	minter := k.GetMinter(ctx)
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime()
	minter.PreviousBlockTime = &blockTime
	if err := k.SetMinter(ctx, minter); err != nil {
		panic(err)
	}
}
