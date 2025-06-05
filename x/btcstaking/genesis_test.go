package btcstaking_test

import (
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	dbm "github.com/cosmos/cosmos-db"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/stretchr/testify/require"

	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/testutil/nullify"
	btcctypes "github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

func TestGenesis(t *testing.T) {
	p := types.DefaultParams()
	genesisState := types.GenesisState{
		Params: []*types.Params{&p},
	}
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())

	lc := btcctypes.NewMockBTCLightClientKeeper()
	cc := btcctypes.NewMockCheckpointingKeeper()
	ic := btcctypes.NewMockIncentiveKeeper()

	btcCkpK, _ := keepertest.NewBTCChkptKeeperWithStoreKeys(t, db, nil, nil, stateStore, lc, cc, ic, chaincfg.SimNetParams.PowLimit)

	k, ctx := keepertest.BTCStakingKeeperWithStore(t, db, stateStore, nil, nil, btcCkpK, nil)

	btcstaking.InitGenesis(ctx, *k, genesisState)
	got := btcstaking.ExportGenesis(ctx, *k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)
}
