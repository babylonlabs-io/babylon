package keeper_test

import (
	"math/rand"
	"testing"

	"github.com/cosmos/cosmos-sdk/types/query"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v3/app"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
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
		for i := uint64(0); i < numRegistrations; i++ {
			var consumerID = datagen.GenRandomHexStr(r, 30)
			allConsumerIDs = append(allConsumerIDs, consumerID)

			err := bscKeeper.RegisterConsumer(ctx, &types.ConsumerRegister{
				ConsumerId:   consumerID,
				ConsumerName: datagen.GenRandomHexStr(r, 5),
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
			require.Contains(t, allConsumerIDs, actualConsumerRegisters[i].ConsumerId)
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
		)
		// invoke the consumer registration a random number of times with random consumer IDs
		numConsumers := datagen.RandomInt(r, 100) + 1
		for i := uint64(0); i < numConsumers; i++ {
			consumerID := datagen.GenRandomHexStr(r, 30)

			consumerIDs = append(consumerIDs, consumerID)
			consumersRegister = append(consumersRegister, consumerRegister{
				consumerID: consumerID,
			})

			err := bscKeeper.RegisterConsumer(ctx, &types.ConsumerRegister{
				ConsumerId:   consumerID,
				ConsumerName: datagen.GenRandomHexStr(r, 5),
			})
			require.NoError(t, err)
		}

		resp, err := bscKeeper.ConsumersRegistry(ctx, &types.QueryConsumersRegistryRequest{
			ConsumerIds: consumerIDs,
		})
		require.NoError(t, err)

		for i, respData := range resp.ConsumerRegisters {
			require.Equal(t, consumersRegister[i].consumerID, respData.ConsumerId)
		}
	})
}
