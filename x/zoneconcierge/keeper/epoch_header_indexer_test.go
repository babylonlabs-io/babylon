package keeper_test

import (
	"math/rand"
	"testing"

	ibctmtypes "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v3/app"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	btcstkconsumertypes "github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
)

func FuzzEpochHeaderIndexer(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		zcKeeper := babylonApp.ZoneConciergeKeeper
		ctx := babylonApp.NewContext(false)

		hooks := zcKeeper.Hooks()
		consumerID := datagen.GenRandomHexStr(r, 10)

		// Register the consumer through the btcstkconsumer keeper
		consumerRegister := &btcstkconsumertypes.ConsumerRegister{
			ConsumerId:          consumerID,
			ConsumerName:        "test-consumer",
			ConsumerDescription: "Test consumer for epoch headers",
			ConsumerMetadata: &btcstkconsumertypes.ConsumerRegister_CosmosConsumerMetadata{
				CosmosConsumerMetadata: &btcstkconsumertypes.CosmosConsumerMetadata{},
			},
			BabylonRewardsCommission: datagen.GenBabylonRewardsCommission(r),
		}
		err := babylonApp.BTCStkConsumerKeeper.RegisterConsumer(ctx, consumerRegister)
		require.NoError(t, err)

		// enter a random epoch
		epochNum := datagen.RandomInt(r, 10) + 1 // start from epoch 1
		for j := uint64(1); j < epochNum; j++ {
			babylonApp.EpochingKeeper.IncEpoch(ctx)
		}

		// verify we're in the correct epoch
		currentEpoch := zcKeeper.GetEpoch(ctx).EpochNumber
		require.Equal(t, epochNum, currentEpoch)

		// invoke the hook a random number of times to simulate a random number of blocks
		numHeaders := datagen.RandomInt(r, 100) + 1
		SimulateNewHeaders(ctx, r, &zcKeeper, consumerID, 0, numHeaders)

		// verify that headers were added to the latest epoch headers store
		latestHeader := zcKeeper.GetLatestEpochHeader(ctx, consumerID)
		require.NotNil(t, latestHeader)
		require.Equal(t, epochNum, latestHeader.BabylonEpoch)

		// end this epoch
		hooks.AfterEpochEnds(ctx, epochNum)

		// check if the finalized header of this epoch is recorded or not
		headerWithProof, err := zcKeeper.GetFinalizedHeader(ctx, consumerID, epochNum)
		require.NoError(t, err)
		require.NotNil(t, headerWithProof)
		require.Equal(t, numHeaders-1, headerWithProof.Header.Height)
		require.Equal(t, consumerID, headerWithProof.Header.ConsumerId)
	})
}

func FuzzGetFinalizedHeaders(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		zcKeeper := babylonApp.ZoneConciergeKeeper
		ctx := babylonApp.NewContext(false)

		hooks := zcKeeper.Hooks()
		consumerID := datagen.GenRandomHexStr(r, 10)

		// Register the consumer through the btcstkconsumer keeper
		consumerRegister := &btcstkconsumertypes.ConsumerRegister{
			ConsumerId:          consumerID,
			ConsumerName:        "test-consumer",
			ConsumerDescription: "Test consumer for epoch headers",
			ConsumerMetadata: &btcstkconsumertypes.ConsumerRegister_CosmosConsumerMetadata{
				CosmosConsumerMetadata: &btcstkconsumertypes.CosmosConsumerMetadata{},
			},
			BabylonRewardsCommission: datagen.GenBabylonRewardsCommission(r),
		}
		err := babylonApp.BTCStkConsumerKeeper.RegisterConsumer(ctx, consumerRegister)
		require.NoError(t, err)

		numReqs := datagen.RandomInt(r, 5) + 1
		epochNumList := []uint64{datagen.RandomInt(r, 10) + 1}
		nextHeightList := []uint64{0}
		numHeadersList := []uint64{}
		expectedHeadersMap := map[uint64][]*ibctmtypes.Header{}

		// we test the scenario of ending an epoch for multiple times, in order to ensure that
		// consecutive epoch headers do not affect each other.
		for i := uint64(0); i < numReqs; i++ {
			epochNum := epochNumList[i]
			// enter a random epoch
			if i == 0 {
				for j := uint64(1); j < epochNum; j++ { // starting from epoch 1
					babylonApp.EpochingKeeper.IncEpoch(ctx)
				}
			} else {
				for j := uint64(0); j < epochNum-epochNumList[i-1]; j++ {
					babylonApp.EpochingKeeper.IncEpoch(ctx)
				}
			}

			// verify we're in the correct epoch
			currentEpoch := zcKeeper.GetEpoch(ctx).EpochNumber
			require.Equal(t, epochNum, currentEpoch)

			// generate a random number of headers
			numHeadersList = append(numHeadersList, datagen.RandomInt(r, 100)+1)
			// trigger hooks to append these headers
			expectedHeaders := SimulateNewHeaders(ctx, r, &zcKeeper, consumerID, nextHeightList[i], numHeadersList[i])
			expectedHeadersMap[epochNum] = expectedHeaders
			// prepare nextHeight for the next request
			nextHeightList = append(nextHeightList, nextHeightList[i]+numHeadersList[i])

			// verify that headers were added to the latest epoch headers store
			latestHeader := zcKeeper.GetLatestEpochHeader(ctx, consumerID)
			require.NotNil(t, latestHeader)
			require.Equal(t, epochNum, latestHeader.BabylonEpoch)

			// simulate the scenario that a random epoch has ended
			hooks.AfterEpochEnds(ctx, epochNum)
			// prepare epochNum for the next request
			epochNumList = append(epochNumList, epochNum+datagen.RandomInt(r, 10)+1)
		}

		// attest the correctness of finalized headers for each tested epoch
		for i := uint64(0); i < numReqs; i++ {
			epochNum := epochNumList[i]
			// check if the finalized header exists
			headerWithProof, err := zcKeeper.GetFinalizedHeader(ctx, consumerID, epochNum)
			require.NoError(t, err)
			require.NotNil(t, headerWithProof)
			require.NotNil(t, headerWithProof.Header)
			// verify the header corresponds to the last header in this epoch
			lastExpectedHeader := expectedHeadersMap[epochNum][len(expectedHeadersMap[epochNum])-1]
			require.Equal(t, lastExpectedHeader.Header.AppHash, headerWithProof.Header.Hash)
		}
	})
}
