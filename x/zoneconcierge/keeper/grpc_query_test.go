package keeper_test

import (
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v3/app"
	btclightclienttypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
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
	headerStartHeight uint64
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

		// we insert random number of headers for each chain in each epoch,
		// chainHeaderStartHeights keeps track of the next start height of header for each chain
		chainHeaderStartHeights := make([]uint64, numChains)
		epochToChainInfo := make(map[uint64]map[string]chainInfo)
		for _, epochNum := range epochNums {
			epochToChainInfo[epochNum] = make(map[string]chainInfo)
			for j, consumerID := range consumerIDs {
				// generate a random number of headers for each chain
				numHeaders := datagen.RandomInt(r, 100) + 1

				// trigger hooks to append these headers
				SimulateNewHeaders(ctx, r, &zcKeeper, consumerID, chainHeaderStartHeights[j], numHeaders)

				epochToChainInfo[epochNum][consumerID] = chainInfo{
					consumerID:        consumerID,
					numHeaders:        numHeaders,
					headerStartHeight: chainHeaderStartHeights[j],
				}

				// update next insertion height for this chain
				chainHeaderStartHeights[j] += numHeaders
			}

			// simulate the scenario that a random epoch has ended
			hooks.AfterEpochEnds(ctx, epochNum)
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
			SimulateNewHeaders(ctx, r, zcKeeper, consumerID, 0, numHeaders)

			consumerIDs = append(consumerIDs, consumerID)
			chainsInfo = append(chainsInfo, chainInfo{
				consumerID: consumerID,
				numHeaders: numHeaders,
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
		}
	})
}
