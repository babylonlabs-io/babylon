package keeper_test

import (
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v3/app"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	"github.com/stretchr/testify/require"
)

func FuzzConsumerRegistry(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		bscKeeper := babylonApp.BTCStkConsumerKeeper
		ctx := babylonApp.NewContext(false)

		// generate a random consumer register
		validConsumer := datagen.GenRandomCosmosConsumerRegister(r)

		// check that the consumer is not registered
		isRegistered := bscKeeper.IsConsumerRegistered(ctx, validConsumer.ConsumerId)
		require.False(t, isRegistered)

		// Check that the consumer is not registered
		retrievedConsumer, err := bscKeeper.GetConsumerRegister(ctx, validConsumer.ConsumerId)
		require.Error(t, err)
		require.Nil(t, retrievedConsumer)

		// Register the consumer
		err = bscKeeper.RegisterConsumer(ctx, validConsumer)
		require.NoError(t, err)
		// check that the consumer is registered
		retrievedConsumer, err = bscKeeper.GetConsumerRegister(ctx, validConsumer.ConsumerId)
		require.NoError(t, err)
		require.Equal(t, validConsumer.ConsumerId, retrievedConsumer.ConsumerId)
		require.Equal(t, validConsumer.ConsumerName, retrievedConsumer.ConsumerName)
		require.Equal(t, validConsumer.ConsumerDescription, retrievedConsumer.ConsumerDescription)
		require.Equal(t, validConsumer.ConsumerMaxMultiStakedFps, retrievedConsumer.ConsumerMaxMultiStakedFps)
	})
}
