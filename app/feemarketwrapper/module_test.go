package feemarketwrapper

import (
	"testing"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
)

func TestCustomBeginBlock_NilBaseFee(t *testing.T) {
	// Create basic test context
	key := storetypes.NewKVStoreKey("test")
	ctx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test")).Ctx
	
	keeper := &MockFeemarketKeeper{}
	keeper.On("CalculateBaseFee", ctx).Return(math.LegacyDec{})

	err := testCustomBeginBlock(ctx, keeper)
	require.NoError(t, err)
	
	// Should return early without calling SetBaseFee
	keeper.AssertNotCalled(t, "SetBaseFee")
}

func TestCustomBeginBlock_ValidBaseFee(t *testing.T) {
	// Create basic test context
	key := storetypes.NewKVStoreKey("test")
	ctx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test")).Ctx
	
	keeper := &MockFeemarketKeeper{}
	baseFee := math.LegacyNewDec(1000)
	
	keeper.On("CalculateBaseFee", ctx).Return(baseFee)
	keeper.On("SetBaseFee", ctx, baseFee).Return()

	err := testCustomBeginBlock(ctx, keeper)
	require.NoError(t, err)
	
	keeper.AssertCalled(t, "SetBaseFee", ctx, baseFee)
}

func TestCustomEndBlock_NilBlockGasMeter(t *testing.T) {
	// Create basic test context
	key := storetypes.NewKVStoreKey("test") 
	ctx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test")).Ctx
	
	keeper := &MockFeemarketKeeper{}
	keeper.On("Logger", ctx).Return(ctx.Logger())
	
	tKey := storetypes.NewTransientStoreKey("test_transient")
	
	// Create context with nil block gas meter
	ctx = ctx.WithBlockGasMeter(nil)
	
	_, err := testCustomEndBlock(ctx, keeper, tKey)
	require.Error(t, err)
	require.Contains(t, err.Error(), "block gas meter is nil")
}

func TestCustomEndBlock_NoRefundableGas(t *testing.T) {
	// Create basic test context
	key := storetypes.NewKVStoreKey("test")
	tKey := storetypes.NewTransientStoreKey("test_transient")
	ctx := testutil.DefaultContextWithDB(t, key, tKey).Ctx
	
	keeper := &MockFeemarketKeeper{}
	gasMeter := storetypes.NewGasMeter(10000)
	gasMeter.ConsumeGas(800, "test")
	
	keeper.On("GetTransientGasWanted", mock.Anything).Return(uint64(1000))
	params := feemarkettypes.DefaultParams()
	params.MinGasMultiplier = math.LegacyOneDec()
	keeper.On("GetParams", mock.Anything).Return(params)
	keeper.On("SetBlockGasWanted", mock.Anything, mock.Anything).Return()
	keeper.On("Logger", mock.Anything).Return(ctx.Logger())

	ctx = ctx.WithBlockGasMeter(gasMeter)
	
	_, err := testCustomEndBlock(ctx, keeper, tKey)
	require.NoError(t, err)
	
	keeper.AssertCalled(t, "SetBlockGasWanted", mock.Anything, uint64(1000))
}

func TestCustomEndBlock_WithRefundableGas(t *testing.T) {
	// Create basic test context
	key := storetypes.NewKVStoreKey("test")
	tKey := storetypes.NewTransientStoreKey("test_transient")
	ctx := testutil.DefaultContextWithDB(t, key, tKey).Ctx
	
	keeper := &MockFeemarketKeeper{}
	gasMeter := storetypes.NewGasMeter(10000)
	gasMeter.ConsumeGas(800, "test")
	
	// Set refundable gas in transient store
	SetTransientRefundableBlockGasWanted(ctx, 200, tKey)
	SetTransientRefundableBlockGasUsed(ctx, 150, tKey)
	
	keeper.On("GetTransientGasWanted", mock.Anything).Return(uint64(1000))
	params := feemarkettypes.DefaultParams()
	params.MinGasMultiplier = math.LegacyOneDec()
	keeper.On("GetParams", mock.Anything).Return(params)
	keeper.On("SetBlockGasWanted", mock.Anything, mock.Anything).Return() // 1000 - 200 = 800
	keeper.On("Logger", mock.Anything).Return(ctx.Logger())

	ctx = ctx.WithBlockGasMeter(gasMeter)
	
	_, err := testCustomEndBlock(ctx, keeper, tKey)
	require.NoError(t, err)
	
	keeper.AssertCalled(t, "SetBlockGasWanted", mock.Anything, uint64(800))
}

func TestTransientStoreOperations(t *testing.T) {
	// Create basic test context
	key := storetypes.NewKVStoreKey("test")
	tKey := storetypes.NewTransientStoreKey("test_transient")
	ctx := testutil.DefaultContextWithDB(t, key, tKey).Ctx
	
	// Test SetTransientRefundableBlockGasWanted and GetTransientRefundableBlockGasWanted
	SetTransientRefundableBlockGasWanted(ctx, 500, tKey)
	gasWanted := GetTransientRefundableBlockGasWanted(ctx, tKey)
	require.Equal(t, uint64(500), gasWanted)
	
	// Test SetTransientRefundableBlockGasUsed and GetTransientRefundableBlockGasUsed
	SetTransientRefundableBlockGasUsed(ctx, 300, tKey)
	gasUsed := GetTransientRefundableBlockGasUsed(ctx, tKey)
	require.Equal(t, uint64(300), gasUsed)
	
	// Test zero values (empty store)
	emptyKey := storetypes.NewTransientStoreKey("empty_transient")
	emptyCtx := testutil.DefaultContextWithDB(t, storetypes.NewKVStoreKey("empty_kv"), emptyKey).Ctx
	
	zeroWanted := GetTransientRefundableBlockGasWanted(emptyCtx, emptyKey)
	zeroUsed := GetTransientRefundableBlockGasUsed(emptyCtx, emptyKey)
	require.Equal(t, uint64(0), zeroWanted)
	require.Equal(t, uint64(0), zeroUsed)
}