package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"

	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"

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
			portKeeper   = types.NewMockPortKeeper(ctrl)
			storeKey     = storetypes.NewKVStoreKey(types.StoreKey)
			k, ctx       = keepertest.ZoneConciergeKeeperWithStoreKey(t, storeKey, nil, portKeeper, nil, nil, nil, nil, nil, nil)
			storeService = runtime.NewKVStoreService(storeKey)
			storeAdapter = runtime.KVStoreAdapter(storeService.OpenKVStore(ctx))
			cdc          = appparams.DefaultEncodingConfig().Codec
		)
		portKeeper.EXPECT().BindPort(gomock.Any(), gomock.Any()).Return(&capabilitytypes.Capability{}).AnyTimes()
		defer ctrl.Finish()

		gs := datagen.GenRandomZoneconciergeGenState(r)

		// set values to state
		require.NoError(t, k.SetParams(ctx, gs.Params))

		k.SetPort(ctx, gs.PortId)
		require.NoError(t, k.BindPort(ctx, gs.PortId))

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

		forkStore := prefix.NewStore(storeAdapter, types.ForkKey)
		for _, f := range gs.ChainsForks {
			for _, h := range f.Headers {
				forks := k.GetForks(ctx, h.ConsumerId, h.Height)
				consumerIDBytes := []byte(h.ConsumerId)
				store := prefix.NewStore(forkStore, consumerIDBytes)
				forks.Headers = append(forks.Headers, h)
				forksBytes := cdc.MustMarshal(forks)
				store.Set(sdk.Uint64ToBigEndian(h.Height), forksBytes)
			}
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

		portKeeper := types.NewMockPortKeeper(ctrl)
		portKeeper.EXPECT().BindPort(gomock.Any(), gomock.Any()).Return(&capabilitytypes.Capability{}).AnyTimes()

		k, ctx := keepertest.ZoneConciergeKeeper(t, nil, portKeeper, nil, nil, nil, nil, nil, nil)
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
