package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"

	storetypes "cosmossdk.io/store/types"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/monitor/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/google/go-cmp/cmp"
	"github.com/test-go/testify/require"
)

func FuzzTestExportGenesis(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		var (
			r            = rand.New(rand.NewSource(seed))
			storeKey     = storetypes.NewKVStoreKey(types.StoreKey)
			k, ctx       = keepertest.MonitorKeeperWithStoreKey(t, storeKey, nil)
			storeService = runtime.NewKVStoreService(storeKey)
			store        = storeService.OpenKVStore(ctx)
		)

		gs := &types.GenesisState{
			EpochEndRecords:     randomEpochEndLightClient(r),
			CheckpointsReported: randomCheckpointReportedLightClient(r),
		}

		// set values to state
		for _, e := range gs.EpochEndRecords {
			k := types.GetEpochEndLightClientHeightKey(e.Epoch)
			v := sdk.Uint64ToBigEndian(e.BtcLightClientHeight)
			require.NoError(t, store.Set(k, v))
		}
		for _, c := range gs.CheckpointsReported {
			k, err := types.GetCheckpointReportedLightClientHeightKey(c.CkptHash)
			require.NoError(t, err)
			v := sdk.Uint64ToBigEndian(c.BtcLightClientHeight)
			require.NoError(t, store.Set(k, v))
		}

		exported, err := k.ExportGenesis(ctx)
		require.NoError(t, err)

		types.SortData(gs)
		types.SortData(exported)

		require.Equal(t, gs.EpochEndRecords, exported.EpochEndRecords)
		require.Equal(t, gs.CheckpointsReported, exported.CheckpointsReported)
	})
}

func FuzzTestInitGenesis(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		var (
			r      = rand.New(rand.NewSource(seed))
			k, ctx = keepertest.MonitorKeeper(t, nil)
		)

		gs := &types.GenesisState{
			EpochEndRecords:     randomEpochEndLightClient(r),
			CheckpointsReported: randomCheckpointReportedLightClient(r),
		}
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

func randomEpochEndLightClient(r *rand.Rand) []*types.EpochEndLightClient {
	var (
		entriesCount = int(datagen.RandomIntOtherThan(r, 0, 20))
		es           = make([]*types.EpochEndLightClient, entriesCount)
	)

	for i := range entriesCount {
		es[i] = &types.EpochEndLightClient{
			Epoch:                uint64(i + 1),
			BtcLightClientHeight: datagen.RandomInt(r, 100000000),
		}
	}
	return es
}

func randomCheckpointReportedLightClient(r *rand.Rand) []*types.CheckpointReportedLightClient {
	var (
		entriesCount = int(datagen.RandomIntOtherThan(r, 0, 20))
		cs           = make([]*types.CheckpointReportedLightClient, entriesCount)
	)

	for i := range entriesCount {
		cs[i] = &types.CheckpointReportedLightClient{
			CkptHash:             datagen.GenRandomHexStr(r, 32),
			BtcLightClientHeight: datagen.RandomInt(r, 100000000),
		}
	}
	return cs
}
