package keeper_test

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	storetypes "cosmossdk.io/store/types"
	appparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	"github.com/babylonlabs-io/babylon/x/btccheckpoint/keeper"
	"github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func FuzzTestExportGenesis(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 1)
	f.Fuzz(func(t *testing.T, seed int64) {
		ctx, k, sk, gs := setupTest(t, seed)

		var (
			encConfig    = appparams.DefaultEncodingConfig()
			cdc          = encConfig.Codec
			storeService = runtime.NewKVStoreService(sk)
			store        = storeService.OpenKVStore(ctx)
		)

		// Setup current state
		require.NoError(t, k.SetParams(ctx, gs.Params))
		l := len(gs.Epochs)
		for i := 0; i < l; i++ {
			// save epochs
			ek := types.GetEpochIndexKey(gs.Epochs[i].EpochNumber)
			eBz := cdc.MustMarshal(gs.Epochs[i].Data)
			require.NoError(t, store.Set(ek, eBz))

			// save submissions
			kBytes := types.PrefixedSubmisionKey(cdc, gs.Submissions[i].SubmissionKey)
			sBytes := cdc.MustMarshal(gs.Submissions[i].Data)
			require.NoError(t, store.Set(kBytes, sBytes))
		}
		// save last finalized epoch
		require.NoError(t, store.Set(types.GetLatestFinalizedEpochKey(), sdk.Uint64ToBigEndian(gs.LastFinalizedEpochNumber)))

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
		ctx, k, _, gs := setupTest(t, seed)
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
// and setup the incentive keeper with the accounts keeper mock
func setupTest(t *testing.T, seed int64) (sdk.Context, *keeper.Keeper, *storetypes.KVStoreKey, *types.GenesisState) {
	var (
		r        = rand.New(rand.NewSource(seed))
		powLimit = datagen.RandomMathInt(r, 1000).BigInt()
		storeKey = storetypes.NewKVStoreKey(types.StoreKey)
		k, ctx   = keepertest.NewBTCChkptKeeperWithStoreKeys(t, storeKey, nil, nil, nil, nil, powLimit)
		l        = int(math.Abs(float64(r.Int()%50 + 1))) // cap it to 50 entries
		e        = make([]types.EpochEntry, l)
		s        = make([]types.SubmissionEntry, l)
	)

	// make sure that BTC staking gauge are unique per height
	for i := 0; i < l; i++ {
		epochNum := uint64(i + 1)
		e[i] = randomEpochEntry(r, epochNum)
		s[i] = randomSubmissionEntry(r)
	}

	chkptFinTimeout := datagen.RandomUInt32(r, 10000)
	gs := &types.GenesisState{
		Params: types.Params{
			BtcConfirmationDepth:          datagen.RandomUInt32(r, chkptFinTimeout-1),
			CheckpointFinalizationTimeout: chkptFinTimeout,
			CheckpointTag:                 datagen.GenRandomHexStr(r, 4),
		},
		LastFinalizedEpochNumber: datagen.RandomInt(r, l),
		Epochs:                   e,
		Submissions:              s,
	}

	require.NoError(t, gs.Validate())
	return ctx, k, storeKey, gs
}

func randomEpochEntry(r *rand.Rand, epochNum uint64) types.EpochEntry {
	return types.EpochEntry{
		EpochNumber: epochNum,
		Data: &types.EpochData{
			Status: datagen.RandomBtcStatus(r),
			Keys:   []*types.SubmissionKey{{Key: []*types.TransactionKey{datagen.RandomTxKey(r)}}},
		},
	}
}

func randomSubmissionEntry(r *rand.Rand) types.SubmissionEntry {
	txKey := datagen.RandomTxKey(r)
	return types.SubmissionEntry{
		SubmissionKey: &types.SubmissionKey{
			Key: []*types.TransactionKey{txKey},
		},
		Data: &types.SubmissionData{
			VigilanteAddresses: &types.CheckpointAddresses{
				Submitter: datagen.GenRandomAddress().Bytes(),
				Reporter:  datagen.GenRandomAddress().Bytes(),
			},
			TxsInfo: []*types.TransactionInfo{{
				Key:         txKey,
				Transaction: datagen.GenRandomByteArray(r, 32),
				Proof:       datagen.GenRandomByteArray(r, 32),
			}},
		},
	}
}
