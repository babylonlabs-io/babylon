package types

import (
	"fmt"
	"math"

	storetypes "cosmossdk.io/store/types"
)

type bypassGasMeter struct {
}

func NewBypassGasMeter() storetypes.GasMeter {
	return &bypassGasMeter{}
}

func (bm bypassGasMeter) GasConsumed() storetypes.Gas {
	return 0
}

func (bm bypassGasMeter) GasConsumedToLimit() storetypes.Gas {
	return 0
}

func (bm bypassGasMeter) GasRemaining() storetypes.Gas {
	return math.MaxUint64
}

func (bm bypassGasMeter) Limit() storetypes.Gas {
	return math.MaxUint64
}

func (bm bypassGasMeter) ConsumeGas(gas storetypes.Gas, descriptor string) {
}

func (bm bypassGasMeter) RefundGas(gas storetypes.Gas, descriptor string) {
}

func (bm bypassGasMeter) IsPastLimit() bool {
	return false
}

func (bm bypassGasMeter) IsOutOfGas() bool {
	return false
}

func (bm bypassGasMeter) String() string {
	return fmt.Sprintf("Bypass Gas Meter:\n  consumed: %d", 0)
}