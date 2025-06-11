package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v3/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v3/x/mint/types"
	dbm "github.com/cosmos/cosmos-db"

	"github.com/google/go-cmp/cmp"
	"github.com/test-go/testify/require"
)

func FuzzTestExportGenesis(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		var (
			r          = rand.New(rand.NewSource(seed))
			db         = dbm.NewMemDB()
			stateStore = store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())
			ak         = keepertest.AccountKeeper(t, db, stateStore)
			k, ctx     = keepertest.MintKeeper(t, db, stateStore, nil, ak, nil)
		)

		// set random minter and genesis time
		minter := randomMinter(r)
		require.NoError(t, k.SetMinter(ctx, *minter))

		time := randomGenTime(r)
		require.NoError(t, k.SetGenesisTime(ctx, *time))

		var exported *types.GenesisState
		require.NotPanics(t, func() {
			exported = k.ExportGenesis(ctx)
		})

		require.Equal(t, minter, exported.Minter)
		require.Equal(t, time, exported.GenesisTime)
	})
}

func FuzzTestInitGenesis(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 1)
	f.Fuzz(func(t *testing.T, seed int64) {
		var (
			r          = rand.New(rand.NewSource(seed))
			db         = dbm.NewMemDB()
			stateStore = store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())
			ak         = keepertest.AccountKeeper(t, db, stateStore)
			k, ctx     = keepertest.MintKeeper(t, db, stateStore, nil, ak, nil)
		)

		gs := &types.GenesisState{
			Minter:      randomMinter(r),
			GenesisTime: randomGenTime(r),
		}
		// Run the InitGenesis
		require.NotPanics(t, func() {
			k.InitGenesis(ctx, ak, gs)
		})

		// get the current state
		var exported *types.GenesisState
		require.NotPanics(t, func() {
			exported = k.ExportGenesis(ctx)
		})

		require.Equal(t, gs, exported, fmt.Sprintf("Found diff: %s | seed %d", cmp.Diff(gs, exported), seed))
	})
}

func randomMinter(r *rand.Rand) *types.Minter {
	now := time.Now().UTC()
	t := now.Add(-1 * time.Hour * time.Duration(datagen.RandomInt(r, 100000000)))
	return &types.Minter{
		BondDenom:         datagen.GenRandomDenom(r),
		InflationRate:     datagen.RandomLegacyDec(r, 10, 1),
		AnnualProvisions:  datagen.RandomLegacyDec(r, 10000000, 1),
		PreviousBlockTime: &t,
	}
}

func randomGenTime(r *rand.Rand) *types.GenesisTime {
	now := time.Now().UTC()
	t := now.Add(-1 * time.Hour * time.Duration(datagen.RandomInt(r, 100000000)))
	return &types.GenesisTime{
		GenesisTime: &t,
	}
}
