package feemarketwrapper

import (
	"context"
	"errors"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"

	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"github.com/cosmos/evm/x/feemarket"
	feemarketkeeper "github.com/cosmos/evm/x/feemarket/keeper"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
)

var (
	_ appmodule.AppModule       = AppModule{}
	_ appmodule.HasBeginBlocker = AppModule{}
	_ module.HasABCIEndBlock    = AppModule{}
)

type AppModule struct {
	feemarket.AppModule
	keeper       feemarketkeeper.Keeper
	transientKey *storetypes.TransientStoreKey
}

func NewAppModule(k feemarketkeeper.Keeper, tKey *storetypes.TransientStoreKey) AppModule {
	return AppModule{
		AppModule:    feemarket.NewAppModule(k),
		keeper:       k,
		transientKey: tKey,
	}
}

// BeginBlock implements the custom base fee calculation excluding refundable gas.
func (am AppModule) BeginBlock(ctx context.Context) error {
	c := sdk.UnwrapSDKContext(ctx)
	return customBeginBlock(c, am.keeper)
}

// EndBlock implements the custom gas tracking excluding refundable transactions.
func (am AppModule) EndBlock(ctx context.Context) ([]abci.ValidatorUpdate, error) {
	c := sdk.UnwrapSDKContext(ctx)
	return customEndBlock(c, am.keeper, am.transientKey)
}

// customBeginBlock calculates base fee excluding refundable gas transactions.
// NOTE: this should be mirrored upon original cosmos evm x/feemarket/keeper/abci.go
func customBeginBlock(ctx sdk.Context, k feemarketkeeper.Keeper) error {
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

// customEndBlock tracks gas usage excluding refundable transactions.
// NOTE: this should be mirrored upon original cosmos evm x/feemarket/keeper/abci.go
func customEndBlock(ctx sdk.Context, k feemarketkeeper.Keeper, tKey *storetypes.TransientStoreKey) ([]abci.ValidatorUpdate, error) {
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

// GetTransientRefundableBlockGasUsed get refundable block gas used
func GetTransientRefundableBlockGasUsed(ctx sdk.Context, tKey *storetypes.TransientStoreKey) uint64 {
	store := ctx.TransientStore(tKey)
	return sdk.BigEndianToUint64(store.Get(KeyPrefixRefundableGasUsed))
}

// SetTransientRefundableBlockGasUsed set refundable block gas used
func SetTransientRefundableBlockGasUsed(ctx sdk.Context, gasUsed uint64, tKey *storetypes.TransientStoreKey) {
	store := ctx.TransientStore(tKey)
	gasBz := sdk.Uint64ToBigEndian(gasUsed)
	store.Set(KeyPrefixRefundableGasUsed, gasBz)
}

// GetTransientRefundableBlockGasWanted get refundable block gas wanted
func GetTransientRefundableBlockGasWanted(ctx sdk.Context, tKey *storetypes.TransientStoreKey) uint64 {
	store := ctx.TransientStore(tKey)
	return sdk.BigEndianToUint64(store.Get(KeyPrefixRefundableGasWanted))
}

// SetTransientRefundableBlockGasWanted set refundable block gas wanted
func SetTransientRefundableBlockGasWanted(ctx sdk.Context, gasWanted uint64, tKey *storetypes.TransientStoreKey) {
	store := ctx.TransientStore(tKey)
	gasBz := sdk.Uint64ToBigEndian(gasWanted)
	store.Set(KeyPrefixRefundableGasWanted, gasBz)
}
