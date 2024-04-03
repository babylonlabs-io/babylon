package keeper_test

import (
	"errors"
	"math/rand"
	"testing"

	"github.com/cosmos/cosmos-sdk/types/query"

	"github.com/stretchr/testify/require"

	"github.com/babylonchain/babylon/app"
	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/babylonchain/babylon/x/btcstkconsumer/types"

	btcstaking "github.com/babylonchain/babylon/x/btcstaking/types"
)

type chainRegister struct {
	chainID string
}

func FuzzChainRegistryList(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		bscKeeper := babylonApp.BTCStkConsumerKeeper
		ctx := babylonApp.NewContext(false)

		// invoke the chain registration a random number of times with random chain IDs
		numRegistrations := datagen.RandomInt(r, 100) + 1
		var allChainIDs []string
		for i := uint64(0); i < numRegistrations; i++ {
			var chainID = datagen.GenRandomHexStr(r, 30)
			allChainIDs = append(allChainIDs, chainID)

			bscKeeper.SetChainRegister(ctx, &types.ChainRegister{
				ChainId:   chainID,
				ChainName: datagen.GenRandomHexStr(r, 5),
			})
		}

		limit := datagen.RandomInt(r, len(allChainIDs)) + 1

		// Query to get actual chain IDs
		resp, err := bscKeeper.ChainRegistryList(ctx, &types.QueryChainRegistryListRequest{
			Pagination: &query.PageRequest{
				Limit: limit,
			},
		})
		require.NoError(t, err)
		actualChainIDs := resp.ChainIds

		require.Equal(t, limit, uint64(len(actualChainIDs)))
		for i := uint64(0); i < limit; i++ {
			require.Contains(t, allChainIDs, actualChainIDs[i])
		}
	})
}

func FuzzChainsRegistry(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		bscKeeper := babylonApp.BTCStkConsumerKeeper
		ctx := babylonApp.NewContext(false)

		var (
			chainsRegister []chainRegister
			chainIDs       []string
		)
		// invoke the chain registration a random number of times with random chain IDs
		numChains := datagen.RandomInt(r, 100) + 1
		for i := uint64(0); i < numChains; i++ {
			chainID := datagen.GenRandomHexStr(r, 30)

			chainIDs = append(chainIDs, chainID)
			chainsRegister = append(chainsRegister, chainRegister{
				chainID: chainID,
			})

			bscKeeper.SetChainRegister(ctx, &types.ChainRegister{
				ChainId:   chainID,
				ChainName: datagen.GenRandomHexStr(r, 5),
			})

		}

		resp, err := bscKeeper.ChainsRegistry(ctx, &types.QueryChainsRegistryRequest{
			ChainIds: chainIDs,
		})
		require.NoError(t, err)

		for i, respData := range resp.ChainsRegister {
			require.Equal(t, chainsRegister[i].chainID, respData.ChainId)
		}
	})
}

func FuzzFinalityProviders(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		bscKeeper := babylonApp.BTCStkConsumerKeeper
		ctx := babylonApp.NewContext(false)

		// Generate random finality providers and add them to kv store under a chain id
		chainID := datagen.GenRandomHexStr(r, 30)
		fpsMap := make(map[string]*btcstaking.FinalityProvider)
		for i := 0; i < int(datagen.RandomInt(r, 10)+1); i++ {
			fp, err := datagen.GenRandomFinalityProvider(r)
			require.NoError(t, err)
			fp.ChainId = chainID

			bscKeeper.SetConsumerFinalityProvider(ctx, fp)
			fpsMap[fp.BtcPk.MarshalHex()] = fp
		}
		numOfFpsInStore := len(fpsMap)

		// Test nil request
		resp, err := bscKeeper.FinalityProviders(ctx, nil)
		if resp != nil {
			t.Errorf("Nil input led to a non-nil response")
		}
		if err == nil {
			t.Errorf("Nil input led to a nil error")
		}

		// Generate a page request with a limit and a nil key
		limit := datagen.RandomInt(r, numOfFpsInStore) + 1
		pagination := constructRequestWithLimit(r, limit)
		// Generate the initial query
		req := types.QueryFinalityProvidersRequest{ChainId: chainID, Pagination: pagination}
		// Construct a mapping from the finality providers found to a boolean value
		// Will be used later to evaluate whether all the finality providers were returned
		fpsFound := make(map[string]bool)

		for i := uint64(0); i < uint64(numOfFpsInStore); i += limit {
			resp, err = bscKeeper.FinalityProviders(ctx, &req)
			if err != nil {
				t.Errorf("Valid request led to an error %s", err)
			}
			if resp == nil {
				t.Fatalf("Valid request led to a nil response")
			}

			for _, fp := range resp.FinalityProviders {
				// Check if the pk exists in the map
				if _, ok := fpsMap[fp.BtcPk.MarshalHex()]; !ok {
					t.Fatalf("rpc returned a finality provider that was not created")
				}
				fpsFound[fp.BtcPk.MarshalHex()] = true
			}

			// Construct the next page request
			pagination = constructRequestWithKeyAndLimit(r, resp.Pagination.NextKey, limit)
			req = types.QueryFinalityProvidersRequest{ChainId: chainID, Pagination: pagination}
		}

		if len(fpsFound) != len(fpsMap) {
			t.Errorf("Some finality providers were missed. Got %d while %d were expected", len(fpsFound), len(fpsMap))
		}
	})
}

func FuzzFinalityProvider(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		bscKeeper := babylonApp.BTCStkConsumerKeeper
		ctx := babylonApp.NewContext(false)

		// Generate random finality providers and add them to kv store under a chain id
		fpsMap := make(map[string]*btcstaking.FinalityProvider)
		chainID := datagen.GenRandomHexStr(r, 30)
		var existingFp string
		for i := 0; i < int(datagen.RandomInt(r, 10)+1); i++ {
			fp, err := datagen.GenRandomFinalityProvider(r)
			require.NoError(t, err)
			fp.ChainId = chainID

			bscKeeper.SetConsumerFinalityProvider(ctx, fp)
			existingFp = fp.BtcPk.MarshalHex()
			fpsMap[existingFp] = fp
		}

		// Test nil request
		resp, err := bscKeeper.FinalityProvider(ctx, nil)
		require.Error(t, err)
		require.Nil(t, resp)

		for k, v := range fpsMap {
			// Generate a request with a valid key
			req := types.QueryFinalityProviderRequest{ChainId: chainID, FpBtcPkHex: k}
			resp, err := bscKeeper.FinalityProvider(ctx, &req)
			if err != nil {
				t.Errorf("Valid request led to an error %s", err)
			}
			if resp == nil {
				t.Fatalf("Valid request led to a nil response")
			}

			// check keys from map matches those in returned response
			require.Equal(t, v.BtcPk.MarshalHex(), resp.FinalityProvider.BtcPk.MarshalHex())
			require.Equal(t, v.BabylonPk, resp.FinalityProvider.BabylonPk)
		}

		// check some random non-existing guy
		fp, err := datagen.GenRandomFinalityProvider(r)
		require.NoError(t, err)
		req := types.QueryFinalityProviderRequest{ChainId: chainID, FpBtcPkHex: fp.BtcPk.MarshalHex()}
		respNonExists, err := bscKeeper.FinalityProvider(ctx, &req)
		require.Error(t, err)
		require.Nil(t, respNonExists)
		require.True(t, errors.Is(err, btcstaking.ErrFpNotFound))

		// check some existing fp over a random non-existing chain-id
		req = types.QueryFinalityProviderRequest{ChainId: "nonexistent", FpBtcPkHex: existingFp}
		respNonExists, err = bscKeeper.FinalityProvider(ctx, &req)
		require.Error(t, err)
		require.Nil(t, respNonExists)
		require.True(t, errors.Is(err, btcstaking.ErrFpNotFound))
	})
}

// FuzzFinalityProviderChain tests the FinalityProviderChain gRPC query endpoint
func FuzzFinalityProviderChain(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		bscKeeper := babylonApp.BTCStkConsumerKeeper
		ctx := babylonApp.NewContext(false)

		// Generate random finality providers and add them to kv store under a chain id
		fpsMap := make(map[string]*btcstaking.FinalityProvider)
		chainID := datagen.GenRandomHexStr(r, 30)
		var existingFp string
		for i := 0; i < int(datagen.RandomInt(r, 10)+1); i++ {
			fp, err := datagen.GenRandomFinalityProvider(r)
			require.NoError(t, err)
			fp.ChainId = chainID

			bscKeeper.SetConsumerFinalityProvider(ctx, fp)
			existingFp = fp.BtcPk.MarshalHex()
			fpsMap[existingFp] = fp
		}

		// Test nil request
		resp, err := bscKeeper.FinalityProviderChain(ctx, nil)
		require.Error(t, err)
		require.Nil(t, resp)

		// Generate a request with a valid key
		req := types.QueryFinalityProviderChainRequest{FpBtcPkHex: existingFp}
		resp, err = bscKeeper.FinalityProviderChain(ctx, &req)
		if err != nil {
			t.Errorf("Valid request led to an error %s", err)
		}
		if resp == nil {
			t.Fatalf("Valid request led to a nil response")
		}

		// check keys from map matches those in returned response
		require.Equal(t, chainID, resp.ChainId)

		// check some random non-existing guy
		fp, err := datagen.GenRandomFinalityProvider(r)
		require.NoError(t, err)
		req = types.QueryFinalityProviderChainRequest{FpBtcPkHex: fp.BtcPk.MarshalHex()}
		respNonExists, err := bscKeeper.FinalityProviderChain(ctx, &req)
		require.Error(t, err)
		require.Nil(t, respNonExists)
		require.True(t, errors.Is(err, btcstaking.ErrFpNotFound))
	})
}

// Constructors for PageRequest objects
func constructRequestWithKeyAndLimit(r *rand.Rand, key []byte, limit uint64) *query.PageRequest {
	// If the limit is 0, set one randomly
	if limit == 0 {
		limit = uint64(r.Int63() + 1) // Use Int63 instead of Uint64 to avoid overflows
	}
	return &query.PageRequest{
		Key:        key,
		Offset:     0, // only offset or key is set
		Limit:      limit,
		CountTotal: false, // only used when offset is used
		Reverse:    false,
	}
}

func constructRequestWithLimit(r *rand.Rand, limit uint64) *query.PageRequest {
	return constructRequestWithKeyAndLimit(r, nil, limit)
}
