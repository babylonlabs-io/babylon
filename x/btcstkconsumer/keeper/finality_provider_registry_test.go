package keeper_test

import (
	"math/rand"
	"testing"

	btcstaking "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"

	"github.com/babylonlabs-io/babylon/v3/app"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	"github.com/stretchr/testify/require"
)

func FuzzFPRegistry(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		bscKeeper := babylonApp.BTCStkConsumerKeeper
		ctx := babylonApp.NewContext(false)
		// Create a random consumer id that starts with "test-"
		consumerID := "test-" + datagen.GenRandomHexStr(r, 10)

		// check that the consumer is not registered
		isRegistered := bscKeeper.IsConsumerRegistered(ctx, consumerID)
		require.False(t, isRegistered)

		// Create a random finality provider public key
		fpBtcPk, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(t, err)

		// Create a random consumer name
		consumerName := datagen.GenRandomHexStr(r, 5)
		// Create a random consumer description
		consumerDesc := "Consumer description: " + datagen.GenRandomHexStr(r, 15)

		// Populate ConsumerRegister object
		consumerRegister := &types.ConsumerRegister{
			ConsumerId:          consumerID,
			ConsumerName:        consumerName,
			ConsumerDescription: consumerDesc,
		}

		// Register the consumer
		err = bscKeeper.RegisterConsumer(ctx, consumerRegister)
		require.NoError(t, err)

		// Now add a finality provider for the consumer to the registry
		fp := btcstaking.FinalityProvider{
			BtcPk:      fpBtcPk,
			ConsumerId: consumerID,
		}
		bscKeeper.SetConsumerFinalityProvider(ctx, &fp)

		// Check that the finality provider is being registered
		hasFP := bscKeeper.HasConsumerFinalityProvider(ctx, fpBtcPk)
		require.True(t, hasFP)
	})
}
