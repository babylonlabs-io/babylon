package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	testutilkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/epoching/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func FuzzTestExportGenesis(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		ctx, k, gs := setupTest(t, seed)

		// Setup current state
		require.NoError(t, k.SetParams(ctx, gs.Params))
		require.NoError(t, k.InitEpoch(ctx, gs.Epochs))
		require.NoError(t, k.InitGenMsgQueue(ctx, gs.Queues))
		require.NoError(t, k.InitGenValidatorSet(ctx, gs.ValidatorSets))
		require.NoError(t, k.InitGenSlashedVotingPower(ctx, gs.SlashedValidatorSets))

		// set validators lifecycles
		for _, vl := range gs.ValidatorsLifecycle {
			valAddr, err := sdk.ValAddressFromBech32(vl.ValAddr)
			require.NoError(t, err)
			k.SetValLifecycle(ctx, valAddr, vl)
		}

		// set delegations lifecycles
		for _, dl := range gs.DelegationsLifecycle {
			k.SetDelegationLifecycle(ctx, sdk.MustAccAddressFromBech32(dl.DelAddr), dl)
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

func setupTest(t *testing.T, seed int64) (sdk.Context, *keeper.Keeper, *types.GenesisState) {
	var (
		r      = rand.New(rand.NewSource(seed))
		k, ctx = testutilkeeper.EpochingKeeper(t)
		gs     = datagen.GenRandomEpochingGenesisState(r)
	)

	require.NoError(t, gs.Validate())
	return ctx, k, gs
}
