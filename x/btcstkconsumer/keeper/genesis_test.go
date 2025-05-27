package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/google/go-cmp/cmp"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"

	"github.com/stretchr/testify/require"
)

func FuzzTestExportGenesis(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		ctx, k, gs := setupTest(t, seed)

		// Setup current state
		require.NoError(t, k.SetParams(ctx, gs.Params))
		l := len(gs.Consumers)
		for i := 0; i < l; i++ {
			// set consumers
			require.NoError(t, k.RegisterConsumer(ctx, gs.Consumers[i]))
			// set FPs
			k.SetConsumerFinalityProvider(ctx, gs.FinalityProviders[i])
		}

		// Run the ExportGenesis
		exported, err := k.ExportGenesis(ctx)

		require.NoError(t, err)
		types.SortData(gs)
		types.SortData(exported)
		require.Equal(t, gs, exported, fmt.Sprintf("Found diff: %s | seed %d", cmp.Diff(gs, exported), seed))
	})
}

func FuzzTestInitGenesis(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		ctx, k, gs := setupTest(t, seed)

		// Run the InitGenesis
		err := k.InitGenesis(ctx, *gs)
		require.NoError(t, err)

		// get the current state
		exported, err := k.ExportGenesis(ctx)
		require.NoError(t, err)

		types.SortData(gs)
		types.SortData(exported)
		require.Equal(t, gs, exported, fmt.Sprintf("Found diff: %s | seed %d", cmp.Diff(gs, exported), seed))
	})
}

// setupTest is a helper function to generate a random genesis state
// and setup the btc staking consumer keeper
func setupTest(t *testing.T, seed int64) (sdk.Context, *keeper.Keeper, *types.GenesisState) {
	var (
		r      = rand.New(rand.NewSource(seed))
		k, ctx = keepertest.BTCStkConsumerKeeper(t)
		l      = r.Intn(50)
		cs     = make([]*types.ConsumerRegister, l)
		fps    = make([]*btcstktypes.FinalityProvider, l)
	)

	for i := 0; i < l; i++ {
		cs[i] = datagen.GenRandomCosmosConsumerRegister(r)
		fp, err := datagen.GenRandomFinalityProvider(r)
		require.NoError(t, err)
		fp.ConsumerId = cs[i].ConsumerId
		fps[i] = fp
	}

	gs := &types.GenesisState{
		Params:            types.DefaultParams(),
		Consumers:         cs,
		FinalityProviders: fps,
	}

	require.NoError(t, gs.Validate())
	return ctx, &k, gs
}
