package testutil

import (
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	dbm "github.com/cosmos/cosmos-db"

	btcstkhelper "github.com/babylonlabs-io/babylon/testutil/btcstaking-helper"
	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/x/incentive/keeper"
)

type Helper struct {
	*btcstkhelper.Helper
	IncentivesKeeper *keeper.Keeper
}

func NewHelper(
	t testing.TB,
	btclcKeeper *types.MockBTCLightClientKeeper,
	btccKeeper *types.MockBtcCheckpointKeeper,
) *Helper {
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())

	ictvK, _ := keepertest.IncentiveKeeperWithStore(t, db, stateStore, nil, nil, nil)
	btcstkH := btcstkhelper.NewHelperWithStoreAndIncentive(t, db, stateStore, btclcKeeper, btccKeeper, ictvK)

	return &Helper{
		Helper:           btcstkH,
		IncentivesKeeper: ictvK,
	}
}
