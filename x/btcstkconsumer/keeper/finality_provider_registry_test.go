package keeper_test

import (
	btcstaking "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/babylonchain/babylon/x/btcstkconsumer/types"
	"math/rand"
	"testing"

	"github.com/babylonchain/babylon/app"
	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/stretchr/testify/require"
)

func FuzzFPRegistry(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		bscKeeper := babylonApp.BTCStkConsumerKeeper
		ctx := babylonApp.NewContext(false)
		// Create a random chain id that starts with "test-"
		czChainID := "test-" + datagen.GenRandomHexStr(r, 10)

		// check that the chain is not registered
		isRegistered := bscKeeper.IsChainRegistered(ctx, czChainID)
		require.False(t, isRegistered)

		// Create a random finality provider public key
		fpBtcPk, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(t, err)

		// Create a random chain name
		czChainName := datagen.GenRandomHexStr(r, 5)
		// Create a random chain description
		czChainDesc := "Chain description: " + datagen.GenRandomHexStr(r, 15)

		// Populate ChainRegister object
		chainRegister := &types.ChainRegister{
			ChainId:          czChainID,
			ChainName:        czChainName,
			ChainDescription: czChainDesc,
		}

		// Register the chain
		bscKeeper.SetChainRegister(ctx, chainRegister)

		// Now add a finality provider for the chain to the registry

		fp := btcstaking.FinalityProvider{
			BtcPk:   fpBtcPk,
			ChainId: czChainID,
		}
		bscKeeper.SetFinalityProvider(ctx, &fp)

		// Check that the finality provider is being registered
		hasFP := bscKeeper.HasFinalityProvider(ctx, czChainID, fpBtcPk)
		require.True(t, hasFP)
	})
}
