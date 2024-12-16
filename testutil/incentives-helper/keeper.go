package testutil

import (
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	dbm "github.com/cosmos/cosmos-db"
	bankk "github.com/cosmos/cosmos-sdk/x/bank/keeper"

	btcstkhelper "github.com/babylonlabs-io/babylon/testutil/btcstaking-helper"
	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/x/finality/types"
	"github.com/babylonlabs-io/babylon/x/incentive/keeper"
)

type IncentiveHelper struct {
	*btcstkhelper.Helper
	BankKeeper       bankk.Keeper
	IncentivesKeeper *keeper.Keeper
}

func NewIncentiveHelper(
	t testing.TB,
	btclcKeeper *bstypes.MockBTCLightClientKeeper,
	btccKForBtcStaking *bstypes.MockBtcCheckpointKeeper,
	btccKForFinality *ftypes.MockCheckpointingKeeper,
) *IncentiveHelper {
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())

	accK := keepertest.AccountKeeper(t, db, stateStore)
	bankK := keepertest.BankKeeper(t, db, stateStore, accK)

	ictvK, _ := keepertest.IncentiveKeeperWithStore(t, db, stateStore, bankK, accK, nil)
	btcstkH := btcstkhelper.NewHelperWithStoreAndIncentive(t, db, stateStore, btclcKeeper, btccKForBtcStaking, btccKForFinality, ictvK)

	return &IncentiveHelper{
		Helper:           btcstkH,
		BankKeeper:       bankK,
		IncentivesKeeper: ictvK,
	}
}
