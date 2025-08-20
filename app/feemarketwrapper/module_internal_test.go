package feemarketwrapper

import (
	"errors"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"

	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
)

// testCustomBeginBlock is a testable version of customBeginBlock that accepts interface
func testCustomBeginBlock(ctx sdk.Context, k FeemarketKeeperInterface) error {
	baseFee := k.CalculateBaseFee(ctx)

	// return immediately if base fee is nil
	if baseFee.IsNil() {
		return nil
	}

	k.SetBaseFee(ctx, baseFee)

	defer func() {
		floatBaseFee, err := baseFee.Float64()
		if err != nil {
			ctx.Logger().Error("error converting base fee to float64", "error", err.Error())
			return
		}
		// there'll be no panic if fails to convert to float32. Will only loose precision
		telemetry.SetGauge(float32(floatBaseFee), "feemarket", "base_fee")
	}()

	// Store current base fee in event
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			feemarkettypes.EventTypeFeeMarket,
			sdk.NewAttribute(feemarkettypes.AttributeKeyBaseFee, baseFee.String()),
		),
	})

	return nil
}

// testCustomEndBlock is a testable version of customEndBlock that accepts interface
func testCustomEndBlock(ctx sdk.Context, k FeemarketKeeperInterface, tKey *storetypes.TransientStoreKey) ([]abci.ValidatorUpdate, error) {
	if ctx.BlockGasMeter() == nil {
		err := errors.New("block gas meter is nil when setting block gas wanted")
		k.Logger(ctx).Error(err.Error())
		return nil, err
	}

	// calculate gas wanted and gas used by adjusting refundable gas wanted and gas used
	refundableGasWanted := GetTransientRefundableBlockGasWanted(ctx, tKey)
	refundableGasUsed := GetTransientRefundableBlockGasUsed(ctx, tKey)
	originalGasWanted := k.GetTransientGasWanted(ctx)
	originalGasUsed := ctx.BlockGasMeter().GasConsumedToLimit()
	gasWanted := math.NewIntFromUint64(originalGasWanted - refundableGasWanted)
	gasUsed := math.NewIntFromUint64(originalGasUsed - refundableGasUsed)

	if !gasWanted.IsInt64() {
		err := fmt.Errorf("integer overflow by integer type conversion. Gas wanted > MaxInt64. Gas wanted: %s", gasWanted)
		k.Logger(ctx).Error(err.Error())
		return nil, err
	}

	if !gasUsed.IsInt64() {
		err := fmt.Errorf("integer overflow by integer type conversion. Gas used > MaxInt64. Gas used: %s", gasUsed)
		k.Logger(ctx).Error(err.Error())
		return nil, err
	}

	// to prevent BaseFee manipulation we limit the gasWanted so that
	// gasWanted = max(gasWanted * MinGasMultiplier, gasUsed)
	// this will be keep BaseFee protected from un-penalized manipulation
	// more info here https://github.com/evmos/ethermint/pull/1105#discussion_r888798925
	minGasMultiplier := k.GetParams(ctx).MinGasMultiplier
	limitedGasWanted := math.LegacyNewDec(gasWanted.Int64()).Mul(minGasMultiplier)
	updatedGasWanted := math.LegacyMaxDec(limitedGasWanted, math.LegacyNewDec(gasUsed.Int64())).TruncateInt().Uint64()
	k.SetBlockGasWanted(ctx, updatedGasWanted)

	defer func() {
		telemetry.SetGauge(float32(updatedGasWanted), "feemarket", "block_gas")
	}()

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"block_gas",
		sdk.NewAttribute("height", fmt.Sprintf("%d", ctx.BlockHeight())),
		sdk.NewAttribute("amount", fmt.Sprintf("%d", updatedGasWanted)),
	))

	return []abci.ValidatorUpdate{}, nil
}