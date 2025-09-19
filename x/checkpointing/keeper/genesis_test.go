package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"

	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	testutilkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func FuzzTestExportGenesis(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		ctx, k, sk, gs := setupTest(t, seed)

		var (
			cdc          = appparams.DefaultEncodingConfig().Codec
			storeService = runtime.NewKVStoreService(sk)
			store        = storeService.OpenKVStore(ctx)
			storeAdaptor = runtime.KVStoreAdapter(store)
		)
		// Setup current state
		// gen keys
		require.NoError(t, k.SetGenBlsKeys(ctx, gs.GenesisKeys))
		// last finalized epoch
		k.SetLastFinalizedEpoch(ctx, gs.LastFinalizedEpoch)

		l := len(gs.Checkpoints)
		cs := k.CheckpointsState(ctx)
		for i := 0; i < l; i++ {
			// checkpoints
			require.NoError(t, cs.CreateRawCkptWithMeta(gs.Checkpoints[i]))

			// validator sets
			valSetStore := prefix.NewStore(storeAdaptor, types.ValidatorBlsKeySetPrefix)
			valBlsSetBytes := types.ValidatorBlsKeySetToBytes(cdc, gs.ValidatorSets[i].ValidatorSet)
			valSetStore.Set(types.ValidatorBlsKeySetKey(gs.ValidatorSets[i].EpochNumber), valBlsSetBytes)
		}

		// Run the ExportGenesis
		exported, err := k.ExportGenesis(ctx)

		require.NoError(t, err)
		types.SortData(gs)
		types.SortData(exported)

		// exported genesis will not contain the PoP
		// and validator ed25519 pub key,
		// so we populate this data to check equality after
		for i, v := range gs.GenesisKeys {
			exported.GenesisKeys[i].ValPubkey = v.ValPubkey
			exported.GenesisKeys[i].BlsKey.Pop = v.BlsKey.Pop
		}

		require.Equal(t, gs, exported, fmt.Sprintf("Found diff: %s | seed %d", cmp.Diff(gs, exported), seed))
	})
}

func FuzzTestInitGenesis(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		ctx, k, _, gs := setupTest(t, seed)

		err := k.InitGenesis(ctx, *gs)
		require.NoError(t, err)

		exportedGs, err := k.ExportGenesis(ctx)
		require.NoError(t, err)

		types.SortData(exportedGs)
		types.SortData(gs)

		// exported genesis will not contain the PoP
		// and validator ed25519 pub key,
		// so we populate this data to check equality after
		for i, v := range gs.GenesisKeys {
			exportedGs.GenesisKeys[i].ValPubkey = v.ValPubkey
			exportedGs.GenesisKeys[i].BlsKey.Pop = v.BlsKey.Pop
		}

		require.Equal(t, gs, exportedGs)
	})
}

func setupTest(t *testing.T, seed int64) (sdk.Context, *keeper.Keeper, *storetypes.KVStoreKey, *types.GenesisState) {
	var (
		r                  = rand.New(rand.NewSource(seed))
		storeKey           = storetypes.NewKVStoreKey(types.StoreKey)
		k, ctx, _          = testutilkeeper.CheckpointingKeeperWithStoreKey(t, storeKey, nil, nil)
		entriesCount       = rand.Intn(20) + 1
		gk                 = make([]*types.GenesisKey, entriesCount)
		vSets              = make([]*types.ValidatorSetEntry, entriesCount)
		chkpts             = make([]*types.RawCheckpointWithMeta, entriesCount)
		lastFinalizedEpoch = uint64(entriesCount - 1)
	)

	for i := range entriesCount {
		epochNum := uint64(i) + 1
		gk[i] = datagen.GenerateGenesisKey()
		vs, _ := datagen.GenerateValidatorSetWithBLSPrivKeys(entriesCount)
		vSets[i] = &types.ValidatorSetEntry{
			EpochNumber:  epochNum,
			ValidatorSet: vs,
		}
		chkpts[i] = datagen.GenRandomRawCheckpointWithMeta(r)
		chkpts[i].Ckpt.EpochNum = epochNum
	}

	gs := &types.GenesisState{
		GenesisKeys:        gk,
		ValidatorSets:      vSets,
		Checkpoints:        chkpts,
		LastFinalizedEpoch: lastFinalizedEpoch,
	}
	require.NoError(t, gs.Validate())
	return ctx, k, storeKey, gs
}
