package keeper_test

import (
	"github.com/babylonchain/babylon/x/zoneconcierge/types"
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
		zcKeeper := babylonApp.ZoneConciergeKeeper
		ctx := babylonApp.NewContext(false)
		// Create a random chain id that starts with "test-"
		czChainID := "test-" + datagen.GenRandomHexStr(r, 10)

		// check that the chain is not registered
		isRegistered := zcKeeper.IsChainRegistered(ctx, czChainID)
		require.False(t, isRegistered)
		// Create a random chain name
		czChainName := datagen.GenRandomHexStr(r, 5)
		// Create a random chain description
		czChainDesc := "Chain description: " + datagen.GenRandomHexStr(r, 15)

		// Check that the chain is not registered
		chainRegister, err := zcKeeper.GetChainRegister(ctx, czChainID)
		require.Error(t, err)
		require.Nil(t, chainRegister)

		// Populate ChainRegister object
		chainRegister = &types.ChainRegister{
			ChainId:          czChainID,
			ChainName:        czChainName,
			ChainDescription: czChainDesc,
		}

		// Register the chain
		zcKeeper.SetChainRegister(ctx, chainRegister)

		// check that the chain is registered
		chainRegister, err = zcKeeper.GetChainRegister(ctx, czChainID)
		require.NoError(t, err)
		require.Equal(t, czChainID, chainRegister.ChainId)
		require.Equal(t, czChainName, chainRegister.ChainName)
		require.Equal(t, czChainDesc, chainRegister.ChainDescription)
	})
}
