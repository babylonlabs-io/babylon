package feemarketwrapper

import (
	"cosmossdk.io/log"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/mock"

	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
)

// FeemarketKeeperInterface defines the interface we need for testing
type FeemarketKeeperInterface interface {
	CalculateBaseFee(ctx sdk.Context) math.LegacyDec
	SetBaseFee(ctx sdk.Context, baseFee math.LegacyDec)
	GetTransientGasWanted(ctx sdk.Context) uint64
	GetParams(ctx sdk.Context) feemarkettypes.Params
	SetBlockGasWanted(ctx sdk.Context, gasWanted uint64)
	Logger(ctx sdk.Context) log.Logger
}

// MockFeemarketKeeper mocks the feemarket keeper
type MockFeemarketKeeper struct {
	mock.Mock
}

var _ FeemarketKeeperInterface = &MockFeemarketKeeper{}

func (m *MockFeemarketKeeper) CalculateBaseFee(ctx sdk.Context) math.LegacyDec {
	args := m.Called(ctx)
	return args.Get(0).(math.LegacyDec)
}

func (m *MockFeemarketKeeper) SetBaseFee(ctx sdk.Context, baseFee math.LegacyDec) {
	m.Called(ctx, baseFee)
}

func (m *MockFeemarketKeeper) GetTransientGasWanted(ctx sdk.Context) uint64 {
	args := m.Called(ctx)
	return args.Get(0).(uint64)
}

func (m *MockFeemarketKeeper) GetParams(ctx sdk.Context) feemarkettypes.Params {
	args := m.Called(ctx)
	return args.Get(0).(feemarkettypes.Params)
}

func (m *MockFeemarketKeeper) SetBlockGasWanted(ctx sdk.Context, gasWanted uint64) {
	m.Called(ctx, gasWanted)
}

func (m *MockFeemarketKeeper) Logger(ctx sdk.Context) log.Logger {
	args := m.Called(ctx)
	return args.Get(0).(log.Logger)
}

// MockParams provides a simple params implementation for testing
type MockParams struct {
	MinGasMultiplier math.LegacyDec
}

// Implement the feemarkettypes.Params interface methods
func (p MockParams) GetMinGasMultiplier() math.LegacyDec {
	return p.MinGasMultiplier
}
