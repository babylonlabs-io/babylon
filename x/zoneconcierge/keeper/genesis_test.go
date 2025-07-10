package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"

	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v3/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func FuzzTestExportGenesis(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		var (
			r            = rand.New(rand.NewSource(seed))
			ctrl         = gomock.NewController(t)
			storeKey     = storetypes.NewKVStoreKey(types.StoreKey)
			k, ctx       = keepertest.ZoneConciergeKeeperWithStoreKey(t, storeKey, nil, nil, nil, nil, nil, nil, nil)
			storeService = runtime.NewKVStoreService(storeKey)
			storeAdapter = runtime.KVStoreAdapter(storeService.OpenKVStore(ctx))
			cdc          = appparams.DefaultEncodingConfig().Codec
		)
		defer ctrl.Finish()

		gs := datagen.GenRandomZoneconciergeGenState(r)

		// set values to state
		require.NoError(t, k.SetParams(ctx, gs.Params))

		k.SetPort(ctx, gs.PortId)

		ciStore := prefix.NewStore(storeAdapter, types.ChainInfoKey)
		for _, ci := range gs.ChainsInfo {
			ciStore.Set([]byte(ci.ConsumerId), cdc.MustMarshal(ci))
		}

		chainHeadersStore := prefix.NewStore(storeAdapter, types.CanonicalChainKey)
		for _, h := range gs.ChainsIndexedHeaders {
			consumerIDBytes := []byte(h.ConsumerId)
			store := prefix.NewStore(chainHeadersStore, consumerIDBytes)
			store.Set(sdk.Uint64ToBigEndian(h.Height), cdc.MustMarshal(h))
		}

		epochChainInfoStore := prefix.NewStore(storeAdapter, types.EpochChainInfoKey)
		for _, ei := range gs.ChainsEpochsInfo {
			consumerIDBytes := []byte(ei.ChainInfo.ChainInfo.ConsumerId)
			store := prefix.NewStore(epochChainInfoStore, consumerIDBytes)
			store.Set(sdk.Uint64ToBigEndian(ei.EpochNumber), cdc.MustMarshal(ei.ChainInfo))
		}

		sealedEpochStore := prefix.NewStore(storeAdapter, types.SealedEpochProofKey)
		for _, se := range gs.SealedEpochsProofs {
			sealedEpochStore.Set(
				sdk.Uint64ToBigEndian(se.EpochNumber),
				cdc.MustMarshal(se.Proof),
			)
		}

		store := storeService.OpenKVStore(ctx)
		store.Set(types.LastSentBTCSegmentKey, cdc.MustMarshal(gs.LastSentSegment))

		// export stored module state
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
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		k, ctx := keepertest.ZoneConciergeKeeper(t, nil, nil, nil, nil, nil, nil, nil)
		gs := datagen.GenRandomZoneconciergeGenState(r)

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
