package btcstaking_test

import (
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	keepertest "github.com/babylonlabs-io/babylon/v2/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v2/testutil/nullify"
	"github.com/babylonlabs-io/babylon/v2/x/btcstaking"
	"github.com/babylonlabs-io/babylon/v2/x/btcstaking/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"
)

func TestGenesis(t *testing.T) {
	p := types.DefaultParams()
	genesisState := types.GenesisState{
		Params: []*types.Params{&p},
	}
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())
	k, ctx := keepertest.BTCStakingKeeperWithStore(t, db, stateStore, nil, nil, nil, nil)

	btcstaking.InitGenesis(ctx, *k, genesisState)
	got := btcstaking.ExportGenesis(ctx, *k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)
}
