package zoneconcierge

import (
	"context"
	"time"

	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/telemetry"
)

// BeginBlocker sends a pending packet for every channel upon each new block,
// so that the relayer is kept awake to relay headers
func BeginBlocker(ctx context.Context, k keeper.Keeper) error {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), telemetry.MetricKeyBeginBlocker)
	return nil
}

func EndBlocker(ctx context.Context, k keeper.Keeper) ([]abci.ValidatorUpdate, error) {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), telemetry.MetricKeyEndBlocker)

	// error in generating IBC packet data or sending packets is consensus-critical
	if err := k.BroadcastBTCHeaders(ctx); err != nil {
		panic(err)
	}
	if err := k.BroadcastBTCStakingConsumerEvents(ctx); err != nil {
		panic(err)
	}

	return []abci.ValidatorUpdate{}, nil
}
