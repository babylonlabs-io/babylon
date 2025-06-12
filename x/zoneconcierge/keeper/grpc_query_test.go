package keeper_test

import (
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v3/app"
	btclightclienttypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	ibctmtypes "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	testkeeper "github.com/babylonlabs-io/babylon/v3/testutil/keeper"
	btcctypes "github.com/babylonlabs-io/babylon/v3/x/btccheckpoint/types"
	checkpointingtypes "github.com/babylonlabs-io/babylon/v3/x/checkpointing/types"
	zctypes "github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
)

type chainInfo struct {
	consumerID        string
	numHeaders        uint64
	numForkHeaders    uint64
	headerStartHeight uint64
}

func FuzzChainList(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		zcKeeper := babylonApp.ZoneConciergeKeeper
		ctx := babylonApp.NewContext(false)

		// invoke the hook a random number of times with random chain IDs
		numHeaders := datagen.RandomInt(r, 100) + 1
		allConsumerIDs := []string{}
		for i := uint64(0); i < numHeaders; i++ {
			var consumerID string
			// simulate the scenario that some headers belong to the same chain
			if i > 0 && datagen.OneInN(r, 2) {
				consumerID = allConsumerIDs[r.Intn(len(allConsumerIDs))]
			} else {
				consumerID = datagen.GenRandomHexStr(r, 30)
				allConsumerIDs = append(allConsumerIDs, consumerID)
			}
			header := datagen.GenRandomIBCTMHeader(r, 0)
			zcKeeper.HandleHeaderWithValidCommit(ctx, datagen.GenRandomByteArray(r, 32), datagen.NewZCHeaderInfo(header, consumerID), false)
		}

		limit := datagen.RandomInt(r, len(allConsumerIDs)) + 1

		// make query to get actual chain IDs
		resp, err := zcKeeper.ChainList(ctx, &zctypes.QueryChainListRequest{
			Pagination: &query.PageRequest{
				Limit: limit,
			},
		})
		require.NoError(t, err)
		actualConsumerIDs := resp.ConsumerIds

		require.Equal(t, limit, uint64(len(actualConsumerIDs)))
		allConsumerIDs = zcKeeper.GetAllConsumerIDs(ctx)
		for i := uint64(0); i < limit; i++ {
			require.Equal(t, allConsumerIDs[i], actualConsumerIDs[i])
		}
	})
}

func FuzzChainsInfo(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		zcKeeper := babylonApp.ZoneConciergeKeeper
		ctx := babylonApp.NewContext(false)

		var (
			chainsInfo  []chainInfo
			consumerIDs []string
		)
		numChains := datagen.RandomInt(r, 100) + 1
		for i := uint64(0); i < numChains; i++ {
			consumerID := datagen.GenRandomHexStr(r, 30)
			numHeaders := datagen.RandomInt(r, 100) + 1
			numForkHeaders := datagen.RandomInt(r, 10) + 1
			SimulateNewHeadersAndForks(ctx, r, &zcKeeper, consumerID, 0, numHeaders, numForkHeaders)

			consumerIDs = append(consumerIDs, consumerID)
			chainsInfo = append(chainsInfo, chainInfo{
				consumerID:     consumerID,
				numHeaders:     numHeaders,
				numForkHeaders: numForkHeaders,
			})
		}

		resp, err := zcKeeper.ChainsInfo(ctx, &zctypes.QueryChainsInfoRequest{
			ConsumerIds: consumerIDs,
		})
		require.NoError(t, err)

		for i, respData := range resp.ChainsInfo {
			require.Equal(t, chainsInfo[i].consumerID, respData.ConsumerId)
			require.Equal(t, chainsInfo[i].numHeaders-1, respData.LatestHeader.Height)
			require.Equal(t, chainsInfo[i].numForkHeaders, uint64(len(respData.LatestForks.Headers)))
		}
	})
}

func FuzzHeader(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		zcKeeper := babylonApp.ZoneConciergeKeeper
		ctx := babylonApp.NewContext(false)

		// invoke the hook a random number of times to simulate a random number of blocks
		numHeaders := datagen.RandomInt(r, 100) + 2
		numForkHeaders := datagen.RandomInt(r, 10) + 1
		headers, forkHeaders := SimulateNewHeadersAndForks(ctx, r, &zcKeeper, consumerID, 0, numHeaders, numForkHeaders)

		// find header at a random height and assert correctness against the expected header
		randomHeight := datagen.RandomInt(r, int(numHeaders-1))
		resp, err := zcKeeper.Header(ctx, &zctypes.QueryHeaderRequest{ConsumerId: consumerID, Height: randomHeight})
		require.NoError(t, err)
		require.Equal(t, headers[randomHeight].Header.AppHash, resp.Header.Hash)
		require.Len(t, resp.ForkHeaders.Headers, 0)

		// find the last header and fork headers then assert correctness
		resp, err = zcKeeper.Header(ctx, &zctypes.QueryHeaderRequest{ConsumerId: consumerID, Height: numHeaders - 1})
		require.NoError(t, err)
		require.Equal(t, headers[numHeaders-1].Header.AppHash, resp.Header.Hash)
		require.Len(t, resp.ForkHeaders.Headers, int(numForkHeaders))
		for i := 0; i < int(numForkHeaders); i++ {
			require.Equal(t, forkHeaders[i].Header.AppHash, resp.ForkHeaders.Headers[i].Hash)
		}
	})
}

func FuzzEpochChainsInfo(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		zcKeeper := babylonApp.ZoneConciergeKeeper
		ctx := babylonApp.NewContext(false)

		hooks := zcKeeper.Hooks()

		// generate a random number of chains
		numChains := datagen.RandomInt(r, 10) + 1
		var consumerIDs []string
		for j := uint64(0); j < numChains; j++ {
			consumerID := datagen.GenRandomHexStr(r, 30)
			consumerIDs = append(consumerIDs, consumerID)
		}

		// generate a random number of epochNums
		totalNumEpochs := datagen.RandomInt(r, 5) + 1
		epochNums := []uint64{datagen.RandomInt(r, 10) + 1}
		for i := uint64(1); i < totalNumEpochs; i++ {
			nextEpoch := epochNums[i-1] + datagen.RandomInt(r, 10) + 1
			epochNums = append(epochNums, nextEpoch)
		}

		// we insert random number of headers and fork headers for each chain in each epoch,
		// chainHeaderStartHeights keeps track of the next start height of header for each chain
		chainHeaderStartHeights := make([]uint64, numChains)
		epochToChainInfo := make(map[uint64]map[string]chainInfo)
		for _, epochNum := range epochNums {
			epochToChainInfo[epochNum] = make(map[string]chainInfo)
			for j, consumerID := range consumerIDs {
				// generate a random number of headers and fork headers for each chain
				numHeaders := datagen.RandomInt(r, 100) + 1
				numForkHeaders := datagen.RandomInt(r, 10) + 1

				// trigger hooks to append these headers and fork headers
				SimulateNewHeadersAndForks(ctx, r, &zcKeeper, consumerID, chainHeaderStartHeights[j], numHeaders, numForkHeaders)

				epochToChainInfo[epochNum][consumerID] = chainInfo{
					consumerID:        consumerID,
					numHeaders:        numHeaders,
					numForkHeaders:    numForkHeaders,
					headerStartHeight: chainHeaderStartHeights[j],
				}

				// update next insertion height for this chain
				chainHeaderStartHeights[j] += numHeaders
			}

			// simulate the scenario that a random epoch has ended
			hooks.AfterEpochEnds(ctx, epochNum)
		}

		// assert correctness of best case scenario
		for _, epochNum := range epochNums {
			resp, err := zcKeeper.EpochChainsInfo(ctx, &zctypes.QueryEpochChainsInfoRequest{EpochNum: epochNum, ConsumerIds: consumerIDs})
			require.NoError(t, err)
			epochChainsInfo := resp.ChainsInfo
			require.Len(t, epochChainsInfo, int(numChains))
			for _, info := range epochChainsInfo {
				require.Equal(t, epochToChainInfo[epochNum][info.ConsumerId].numForkHeaders, uint64(len(info.LatestForks.Headers)))

				actualHeight := epochToChainInfo[epochNum][info.ConsumerId].headerStartHeight + (epochToChainInfo[epochNum][info.ConsumerId].numHeaders - 1)
				require.Equal(t, actualHeight, info.LatestHeader.Height)
			}
		}

		// if num of chain ids exceed the max limit, query should fail
		largeNumChains := datagen.RandomInt(r, 10) + 101
		var maxConsumerIDs []string
		for i := uint64(0); i < largeNumChains; i++ {
			maxConsumerIDs = append(maxConsumerIDs, datagen.GenRandomHexStr(r, 30))
		}
		randomEpochNum := datagen.RandomInt(r, 10) + 1
		_, err := zcKeeper.EpochChainsInfo(ctx, &zctypes.QueryEpochChainsInfoRequest{EpochNum: randomEpochNum, ConsumerIds: maxConsumerIDs})
		require.Error(t, err)

		// if no input is passed in, query should fail
		_, err = zcKeeper.EpochChainsInfo(ctx, &zctypes.QueryEpochChainsInfoRequest{EpochNum: randomEpochNum, ConsumerIds: nil})
		require.Error(t, err)

		// if len of chain ids is 0, query should fail
		_, err = zcKeeper.EpochChainsInfo(ctx, &zctypes.QueryEpochChainsInfoRequest{EpochNum: randomEpochNum, ConsumerIds: []string{}})
		require.Error(t, err)

		// if chain ids contain duplicates, query should fail
		randomConsumerID := datagen.GenRandomHexStr(r, 30)
		dupConsumerIds := []string{randomConsumerID, randomConsumerID}
		_, err = zcKeeper.EpochChainsInfo(ctx, &zctypes.QueryEpochChainsInfoRequest{EpochNum: randomEpochNum, ConsumerIds: dupConsumerIds})
		require.Error(t, err)
	})
}

func FuzzListHeaders(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		zcKeeper := babylonApp.ZoneConciergeKeeper
		ctx := babylonApp.NewContext(false)

		// invoke the hook a random number of times to simulate a random number of blocks
		numHeaders := datagen.RandomInt(r, 100) + 1
		numForkHeaders := datagen.RandomInt(r, 10) + 1
		headers, _ := SimulateNewHeadersAndForks(ctx, r, &zcKeeper, consumerID, 0, numHeaders, numForkHeaders)

		// a request with randomised pagination
		limit := datagen.RandomInt(r, int(numHeaders)) + 1
		req := &zctypes.QueryListHeadersRequest{
			ConsumerId: consumerID,
			Pagination: &query.PageRequest{
				Limit: limit,
			},
		}
		resp, err := zcKeeper.ListHeaders(ctx, req)
		require.NoError(t, err)
		require.Equal(t, int(limit), len(resp.Headers))
		for i := uint64(0); i < limit; i++ {
			require.Equal(t, headers[i].Header.AppHash, resp.Headers[i].Hash)
		}
	})
}

func FuzzListEpochHeaders(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp := app.Setup(t, false)
		zcKeeper := babylonApp.ZoneConciergeKeeper
		epochingKeeper := babylonApp.EpochingKeeper
		ctx := babylonApp.NewContext(false)

		hooks := zcKeeper.Hooks()

		numReqs := datagen.RandomInt(r, 5) + 1

		epochNumList := []uint64{datagen.RandomInt(r, 10) + 1}
		nextHeightList := []uint64{0}
		numHeadersList := []uint64{}
		expectedHeadersMap := map[uint64][]*ibctmtypes.Header{}
		numForkHeadersList := []uint64{}

		// we test the scenario of ending an epoch for multiple times, in order to ensure that
		// consecutive epoch infos do not affect each other.
		for i := uint64(0); i < numReqs; i++ {
			epochNum := epochNumList[i]
			// enter a random epoch
			if i == 0 {
				for j := uint64(1); j < epochNum; j++ { // starting from epoch 1
					epochingKeeper.IncEpoch(ctx)
				}
			} else {
				for j := uint64(0); j < epochNum-epochNumList[i-1]; j++ {
					epochingKeeper.IncEpoch(ctx)
				}
			}

			// generate a random number of headers and fork headers
			numHeadersList = append(numHeadersList, datagen.RandomInt(r, 100)+1)
			numForkHeadersList = append(numForkHeadersList, datagen.RandomInt(r, 10)+1)
			// trigger hooks to append these headers and fork headers
			expectedHeaders, _ := SimulateNewHeadersAndForks(ctx, r, &zcKeeper, consumerID, nextHeightList[i], numHeadersList[i], numForkHeadersList[i])
			expectedHeadersMap[epochNum] = expectedHeaders
			// prepare nextHeight for the next request
			nextHeightList = append(nextHeightList, nextHeightList[i]+numHeadersList[i])

			// simulate the scenario that a random epoch has ended
			hooks.AfterEpochEnds(ctx, epochNum)
			// prepare epochNum for the next request
			epochNumList = append(epochNumList, epochNum+datagen.RandomInt(r, 10)+1)
		}

		// attest the correctness of epoch info for each tested epoch
		for i := uint64(0); i < numReqs; i++ {
			epochNum := epochNumList[i]
			// make request
			req := &zctypes.QueryListEpochHeadersRequest{
				ConsumerId: consumerID,
				EpochNum:   epochNum,
			}
			resp, err := zcKeeper.ListEpochHeaders(ctx, req)
			require.NoError(t, err)

			// check if the headers are same as expected
			headers := resp.Headers
			require.Equal(t, len(expectedHeadersMap[epochNum]), len(headers))
			for j := 0; j < len(expectedHeadersMap[epochNum]); j++ {
				require.Equal(t, expectedHeadersMap[epochNum][j].Header.AppHash, headers[j].Hash)
			}
		}
	})
}

func FuzzFinalizedChainInfo(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// simulate the scenario that a random epoch has ended and finalised
		epoch := datagen.GenRandomEpoch(r)

		// mock checkpointing keeper
		// TODO: tests with a set of validators
		checkpointingKeeper := zctypes.NewMockCheckpointingKeeper(ctrl)
		checkpointingKeeper.EXPECT().GetBLSPubKeySet(gomock.Any(), gomock.Eq(epoch.EpochNumber)).Return([]*checkpointingtypes.ValidatorWithBlsKey{}, nil).AnyTimes()
		// mock btccheckpoint keeper
		// TODO: test with BTCSpvProofs
		randomRawCkpt := datagen.GenRandomRawCheckpoint(r)
		randomRawCkpt.EpochNum = epoch.EpochNumber
		btccKeeper := zctypes.NewMockBtcCheckpointKeeper(ctrl)
		checkpointingKeeper.EXPECT().GetRawCheckpoint(gomock.Any(), gomock.Eq(epoch.EpochNumber)).Return(
			&checkpointingtypes.RawCheckpointWithMeta{
				Ckpt: randomRawCkpt,
			}, nil,
		).AnyTimes()
		btccKeeper.EXPECT().GetParams(gomock.Any()).Return(btcctypes.DefaultParams()).AnyTimes()
		btccKeeper.EXPECT().GetBestSubmission(gomock.Any(), gomock.Eq(epoch.EpochNumber)).Return(
			btcctypes.Finalized,
			&btcctypes.SubmissionKey{
				Key: []*btcctypes.TransactionKey{},
			},
			nil,
		).AnyTimes()
		mockSubmissionData := &btcctypes.SubmissionData{TxsInfo: []*btcctypes.TransactionInfo{}}
		btccKeeper.EXPECT().GetSubmissionData(gomock.Any(), gomock.Any()).Return(mockSubmissionData).AnyTimes()
		// mock epoching keeper
		epochingKeeper := zctypes.NewMockEpochingKeeper(ctrl)
		epochingKeeper.EXPECT().GetEpoch(gomock.Any()).Return(epoch).AnyTimes()
		epochingKeeper.EXPECT().GetHistoricalEpoch(gomock.Any(), gomock.Eq(epoch.EpochNumber)).Return(epoch, nil).AnyTimes()
		// mock btclc keeper
		btclcKeeper := zctypes.NewMockBTCLightClientKeeper(ctrl)
		mockBTCHeaderInfo := datagen.GenRandomBTCHeaderInfo(r)
		btclcKeeper.EXPECT().GetMainChainFrom(gomock.Any(), gomock.Any()).Return([]*btclightclienttypes.BTCHeaderInfo{mockBTCHeaderInfo}).AnyTimes()
		btclcKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(mockBTCHeaderInfo).AnyTimes()
		// mock channel/port keeper
		channelKeeper := zctypes.NewMockChannelKeeper(ctrl)
		channelKeeper.EXPECT().GetAllChannels(gomock.Any()).Return(nil).AnyTimes()

		zcKeeper, ctx := testkeeper.ZoneConciergeKeeper(t, channelKeeper, btclcKeeper, checkpointingKeeper, btccKeeper, epochingKeeper, nil, nil)
		hooks := zcKeeper.Hooks()

		var (
			chainsInfo  []chainInfo
			consumerIDs []string
		)
		numChains := datagen.RandomInt(r, 100) + 1
		for i := uint64(0); i < numChains; i++ {
			consumerIDLen := datagen.RandomInt(r, 40) + 10
			consumerID := string(datagen.GenRandomByteArray(r, consumerIDLen))

			// invoke the hook a random number of times to simulate a random number of blocks
			numHeaders := datagen.RandomInt(r, 100) + 1
			numForkHeaders := datagen.RandomInt(r, 10) + 1
			SimulateNewHeadersAndForks(ctx, r, zcKeeper, consumerID, 0, numHeaders, numForkHeaders)

			consumerIDs = append(consumerIDs, consumerID)
			chainsInfo = append(chainsInfo, chainInfo{
				consumerID:     consumerID,
				numHeaders:     numHeaders,
				numForkHeaders: numForkHeaders,
			})
		}

		hooks.AfterEpochEnds(ctx, epoch.EpochNumber)
		err := hooks.AfterRawCheckpointFinalized(ctx, epoch.EpochNumber)
		require.NoError(t, err)
		checkpointingKeeper.EXPECT().GetLastFinalizedEpoch(gomock.Any()).Return(epoch.EpochNumber).AnyTimes()

		// check if the chain info of this epoch is recorded or not
		resp, err := zcKeeper.FinalizedChainsInfo(ctx, &zctypes.QueryFinalizedChainsInfoRequest{ConsumerIds: consumerIDs, Prove: true})
		require.NoError(t, err)
		for i, respData := range resp.FinalizedChainsInfo {
			require.Equal(t, chainsInfo[i].consumerID, respData.FinalizedChainInfo.ConsumerId)
			require.Equal(t, chainsInfo[i].numHeaders-1, respData.FinalizedChainInfo.LatestHeader.Height)
			require.Equal(t, chainsInfo[i].numForkHeaders, uint64(len(respData.FinalizedChainInfo.LatestForks.Headers)))
		}
	})
}
