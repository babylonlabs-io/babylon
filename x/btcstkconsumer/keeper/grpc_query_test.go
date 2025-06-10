package keeper_test

import (
	"errors"
	"math/rand"
	"testing"

	"github.com/cosmos/cosmos-sdk/types/query"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v3/app"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"

	btcstaking "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
)

type consumerRegister struct {
	consumerID string
}

func FuzzConsumerRegistryList(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		bscKeeper := babylonApp.BTCStkConsumerKeeper
		ctx := babylonApp.NewContext(false)

		// invoke the consumer registration a random number of times with random consumer IDs
		numRegistrations := datagen.RandomInt(r, 100) + 1
		var allConsumerIDs []string
		consumerMaxFps := make(map[string]uint32) // Track max_multi_staked_fps for each consumer
		for i := uint64(0); i < numRegistrations; i++ {
			var consumerID = datagen.GenRandomHexStr(r, 30)
			maxFps := uint32(datagen.RandomInt(r, 10) + 2)
			allConsumerIDs = append(allConsumerIDs, consumerID)
			consumerMaxFps[consumerID] = maxFps

			err := bscKeeper.RegisterConsumer(ctx, &types.ConsumerRegister{
				ConsumerId:        consumerID,
				ConsumerName:      datagen.GenRandomHexStr(r, 5),
				MaxMultiStakedFps: maxFps,
			})
			require.NoError(t, err)
		}

		limit := datagen.RandomInt(r, len(allConsumerIDs)) + 1

		// Query to get actual consumer IDs
		resp, err := bscKeeper.ConsumerRegistryList(ctx, &types.QueryConsumerRegistryListRequest{
			Pagination: &query.PageRequest{
				Limit: limit,
			},
		})
		require.NoError(t, err)
		actualConsumerRegisters := resp.ConsumerRegisters

		require.Equal(t, limit, uint64(len(actualConsumerRegisters)))
		for i := uint64(0); i < limit; i++ {
			consumerID := actualConsumerRegisters[i].ConsumerId
			require.Contains(t, allConsumerIDs, consumerID)
			require.Equal(t, consumerMaxFps[consumerID], actualConsumerRegisters[i].MaxMultiStakedFps)
		}
	})
}

func FuzzConsumersRegistry(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		bscKeeper := babylonApp.BTCStkConsumerKeeper
		ctx := babylonApp.NewContext(false)

		var (
			consumersRegister []consumerRegister
			consumerIDs       []string
			maxMultiStakedFps []uint32
		)
		// invoke the consumer registration a random number of times with random consumer IDs
		numConsumers := datagen.RandomInt(r, 100) + 1
		for i := uint64(0); i < numConsumers; i++ {
			consumerID := datagen.GenRandomHexStr(r, 30)
			maxFps := uint32(datagen.RandomInt(r, 10) + 2)

			consumerIDs = append(consumerIDs, consumerID)
			maxMultiStakedFps = append(maxMultiStakedFps, maxFps)
			consumersRegister = append(consumersRegister, consumerRegister{
				consumerID: consumerID,
			})

			err := bscKeeper.RegisterConsumer(ctx, &types.ConsumerRegister{
				ConsumerId:        consumerID,
				ConsumerName:      datagen.GenRandomHexStr(r, 5),
				MaxMultiStakedFps: maxFps,
			})
			require.NoError(t, err)
		}

		resp, err := bscKeeper.ConsumersRegistry(ctx, &types.QueryConsumersRegistryRequest{
			ConsumerIds: consumerIDs,
		})
		require.NoError(t, err)

		for i, respData := range resp.ConsumerRegisters {
			require.Equal(t, consumersRegister[i].consumerID, respData.ConsumerId)
			require.Equal(t, maxMultiStakedFps[i], respData.MaxMultiStakedFps)
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

		// Generate random finality providers and add them to kv store under a consumer id
		consumerID := datagen.GenRandomHexStr(r, 30)
		fpsMap := make(map[string]*btcstaking.FinalityProvider)
		for i := 0; i < int(datagen.RandomInt(r, 10)+1); i++ {
			fp, err := datagen.GenRandomFinalityProvider(r)
			require.NoError(t, err)
			fp.ConsumerId = consumerID

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
		req := types.QueryFinalityProvidersRequest{ConsumerId: consumerID, Pagination: pagination}
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
			req = types.QueryFinalityProvidersRequest{ConsumerId: consumerID, Pagination: pagination}
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

		// Generate random finality providers and add them to kv store under a consumer id
		fpsMap := make(map[string]*btcstaking.FinalityProvider)
		consumerID := datagen.GenRandomHexStr(r, 30)
		var existingFp string
		for i := 0; i < int(datagen.RandomInt(r, 10)+1); i++ {
			fp, err := datagen.GenRandomFinalityProvider(r)
			require.NoError(t, err)
			fp.ConsumerId = consumerID

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
			req := types.QueryFinalityProviderRequest{ConsumerId: consumerID, FpBtcPkHex: k}
			resp, err := bscKeeper.FinalityProvider(ctx, &req)
			if err != nil {
				t.Errorf("Valid request led to an error %s", err)
			}
			if resp == nil {
				t.Fatalf("Valid request led to a nil response")
			}

			// check keys from map matches those in returned response
			require.Equal(t, v.BtcPk.MarshalHex(), resp.FinalityProvider.BtcPk.MarshalHex())
			require.Equal(t, v.Addr, resp.FinalityProvider.Addr)
		}

		// check some random non-existing guy
		fp, err := datagen.GenRandomFinalityProvider(r)
		require.NoError(t, err)
		req := types.QueryFinalityProviderRequest{ConsumerId: consumerID, FpBtcPkHex: fp.BtcPk.MarshalHex()}
		respNonExists, err := bscKeeper.FinalityProvider(ctx, &req)
		require.Error(t, err)
		require.Nil(t, respNonExists)
		require.True(t, errors.Is(err, btcstaking.ErrFpNotFound))

		// check some existing fp over a random non-existing consumer-id
		req = types.QueryFinalityProviderRequest{ConsumerId: "nonexistent", FpBtcPkHex: existingFp}
		respNonExists, err = bscKeeper.FinalityProvider(ctx, &req)
		require.Error(t, err)
		require.Nil(t, respNonExists)
		require.True(t, errors.Is(err, btcstaking.ErrFpNotFound))
	})
}

// FuzzFinalityProviderConsumer tests the FinalityProviderConsumer gRPC query endpoint
func FuzzFinalityProviderConsumer(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		bscKeeper := babylonApp.BTCStkConsumerKeeper
		ctx := babylonApp.NewContext(false)

		// Generate random finality providers and add them to kv store under a consumer id
		fpsMap := make(map[string]*btcstaking.FinalityProvider)
		consumerID := datagen.GenRandomHexStr(r, 30)
		var existingFp string
		for i := 0; i < int(datagen.RandomInt(r, 10)+1); i++ {
			fp, err := datagen.GenRandomFinalityProvider(r)
			require.NoError(t, err)
			fp.ConsumerId = consumerID

			bscKeeper.SetConsumerFinalityProvider(ctx, fp)
			existingFp = fp.BtcPk.MarshalHex()
			fpsMap[existingFp] = fp
		}

		// Test nil request
		resp, err := bscKeeper.FinalityProviderConsumer(ctx, nil)
		require.Error(t, err)
		require.Nil(t, resp)

		// Generate a request with a valid key
		req := types.QueryFinalityProviderConsumerRequest{FpBtcPkHex: existingFp}
		resp, err = bscKeeper.FinalityProviderConsumer(ctx, &req)
		if err != nil {
			t.Errorf("Valid request led to an error %s", err)
		}
		if resp == nil {
			t.Fatalf("Valid request led to a nil response")
		}

		// check keys from map matches those in returned response
		require.Equal(t, consumerID, resp.ConsumerId)

		// check some random non-existing guy
		fp, err := datagen.GenRandomFinalityProvider(r)
		require.NoError(t, err)
		req = types.QueryFinalityProviderConsumerRequest{FpBtcPkHex: fp.BtcPk.MarshalHex()}
		respNonExists, err := bscKeeper.FinalityProviderConsumer(ctx, &req)
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
