package keeper_test

import (
	"math/rand"
	"testing"

	"github.com/babylonchain/babylon/app"
	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/stretchr/testify/require"
)

func FuzzChainRegistry(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		zcKeeper := babylonApp.BTCStkConsumerKeeper
		ctx := babylonApp.NewContext(false)

		// generate a random chain register
		chainRegister := datagen.GenRandomChainRegister(r)

		// check that the chain is not registered
		isRegistered := zcKeeper.IsConsumerChainRegistered(ctx, chainRegister.ChainId)
		require.False(t, isRegistered)

		// Check that the chain is not registered
		chainRegister2, err := zcKeeper.GetChainRegister(ctx, chainRegister.ChainId)
		require.Error(t, err)
		require.Nil(t, chainRegister2)

		// Register the chain
		zcKeeper.SetChainRegister(ctx, chainRegister)

		// check that the chain is registered
		chainRegister2, err = zcKeeper.GetChainRegister(ctx, chainRegister.ChainId)
		require.NoError(t, err)
		require.Equal(t, chainRegister.ChainId, chainRegister2.ChainId)
		require.Equal(t, chainRegister.ChainName, chainRegister2.ChainName)
		require.Equal(t, chainRegister.ChainDescription, chainRegister2.ChainDescription)
	})
}
