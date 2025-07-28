package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v3/app/signingcontext"
	testutil "github.com/babylonlabs-io/babylon/v3/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	testkeeper "github.com/babylonlabs-io/babylon/v3/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	epochingtypes "github.com/babylonlabs-io/babylon/v3/x/epoching/types"
	"github.com/babylonlabs-io/babylon/v3/x/finality/keeper"
	"github.com/babylonlabs-io/babylon/v3/x/finality/types"
)

func FuzzActivatedHeight(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// Setup keeper and context
		keeper, ctx := testkeeper.FinalityKeeper(t, nil, nil, nil)
		ctx = sdk.UnwrapSDKContext(ctx)

		// not activated yet
		_, err := keeper.GetBTCStakingActivatedHeight(ctx)
		require.Error(t, err)

		randomActivatedHeight := datagen.RandomInt(r, 100) + 1
		fp, err := datagen.GenRandomFinalityProvider(r, "", "")
		require.NoError(t, err)
		keeper.SetVotingPower(ctx, fp.BtcPk.MustMarshal(), randomActivatedHeight, uint64(10))

		// now it's activated
		resp, err := keeper.ActivatedHeight(ctx, &types.QueryActivatedHeightRequest{})
		require.NoError(t, err)
		require.Equal(t, randomActivatedHeight, resp.Height)
	})
}

func FuzzFinalityProviderPowerAtHeight(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Setup keeper and context
		bk := types.NewMockBTCStakingKeeper(ctrl)
		keeper, ctx := testkeeper.FinalityKeeper(t, bk, nil, nil)

		// random finality provider
		fp, err := datagen.GenRandomFinalityProvider(r, "", "")
		require.NoError(t, err)
		// set random voting power at random height
		randomHeight := datagen.RandomInt(r, 100) + 1
		randomPower := datagen.RandomInt(r, 100) + 1
		keeper.SetVotingPower(ctx, fp.BtcPk.MustMarshal(), randomHeight, randomPower)

		// happy case
		bk.EXPECT().BabylonFinalityProviderExists(gomock.Any(), gomock.Any()).Return(true).Times(1)
		req1 := &types.QueryFinalityProviderPowerAtHeightRequest{
			FpBtcPkHex: fp.BtcPk.MarshalHex(),
			Height:     randomHeight,
		}
		resp, err := keeper.FinalityProviderPowerAtHeight(ctx, req1)
		require.NoError(t, err)
		require.Equal(t, randomPower, resp.VotingPower)

		// case where the voting power store is not updated in
		// the given height
		bk.EXPECT().BabylonFinalityProviderExists(gomock.Any(), gomock.Any()).Return(true).Times(1)
		requestHeight := randomHeight + datagen.RandomInt(r, 10) + 1
		req2 := &types.QueryFinalityProviderPowerAtHeightRequest{
			FpBtcPkHex: fp.BtcPk.MarshalHex(),
			Height:     requestHeight,
		}
		_, err = keeper.FinalityProviderPowerAtHeight(ctx, req2)
		require.ErrorIs(t, err, types.ErrVotingPowerTableNotUpdated)

		// case where the given fp pk does not exist
		bk.EXPECT().BabylonFinalityProviderExists(gomock.Any(), gomock.Any()).Return(false).Times(1)
		randPk, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(t, err)
		req3 := &types.QueryFinalityProviderPowerAtHeightRequest{
			FpBtcPkHex: randPk.MarshalHex(),
			Height:     randomHeight,
		}
		_, err = keeper.FinalityProviderPowerAtHeight(ctx, req3)
		require.ErrorIs(t, err, bstypes.ErrFpNotFound)
	})
}

func FuzzFinalityProviderCurrentVotingPower(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Setup keeper and context
		bk := types.NewMockBTCStakingKeeper(ctrl)
		bk.EXPECT().BabylonFinalityProviderExists(gomock.Any(), gomock.Any()).Return(true).AnyTimes()
		keeper, ctx := testkeeper.FinalityKeeper(t, bk, nil, nil)

		// random finality provider
		fp, err := datagen.GenRandomFinalityProvider(r, "", "")
		require.NoError(t, err)
		// set random voting power at random height
		randomHeight := datagen.RandomInt(r, 100) + 1
		ctx = datagen.WithCtxHeight(ctx, randomHeight)
		randomPower := datagen.RandomInt(r, 100) + 1
		keeper.SetVotingPower(ctx, fp.BtcPk.MustMarshal(), randomHeight, randomPower)

		// assert voting power at current height
		req := &types.QueryFinalityProviderCurrentPowerRequest{
			FpBtcPkHex: fp.BtcPk.MarshalHex(),
		}
		resp, err := keeper.FinalityProviderCurrentPower(ctx, req)
		require.NoError(t, err)
		require.Equal(t, randomHeight, resp.Height)
		require.Equal(t, randomPower, resp.VotingPower)

		// if height increments but voting power hasn't recorded yet, then
		// we need to return the height and voting power at the last height
		ctx = datagen.WithCtxHeight(ctx, randomHeight+1)
		resp, err = keeper.FinalityProviderCurrentPower(ctx, req)
		require.NoError(t, err)
		require.Equal(t, randomHeight, resp.Height)
		require.Equal(t, randomPower, resp.VotingPower)

		// test the case when the finality provider has 0 voting power
		ctx = datagen.WithCtxHeight(ctx, randomHeight+2)
		keeper.SetVotingPower(ctx, fp.BtcPk.MustMarshal(), randomHeight+2, 0)
		resp, err = keeper.FinalityProviderCurrentPower(ctx, req)
		require.NoError(t, err)
		require.Equal(t, randomHeight+2, resp.Height)
		require.Equal(t, uint64(0), resp.VotingPower)
	})
}

func FuzzActiveFinalityProvidersAtHeight(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := bstypes.NewMockBTCLightClientKeeper(ctrl)
		btclcKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(&btclctypes.BTCHeaderInfo{Height: 30}).AnyTimes()
		btccKeeper := bstypes.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

		h.GenAndApplyParams(r)

		// Generate a random batch of finality providers
		var fps []*bstypes.FinalityProvider
		numFpsWithVotingPower := datagen.RandomInt(r, 10) + 1
		numFps := numFpsWithVotingPower + datagen.RandomInt(r, 10)
		for i := uint64(0); i < numFps; i++ {
			_, _, fp := h.CreateFinalityProvider(r)
			fps = append(fps, fp)
		}

		// For numFpsWithVotingPower finality providers, generate a random number of BTC delegations
		babylonHeight := datagen.RandomInt(r, 10) + 1
		fpsWithVotingPowerMap := make(map[string]*bstypes.FinalityProvider)
		for i := uint64(0); i < numFpsWithVotingPower; i++ {
			fpBTCPK := fps[i].BtcPk
			fpsWithVotingPowerMap[fpBTCPK.MarshalHex()] = fps[i]
			h.FinalityKeeper.SetVotingPower(h.Ctx, fpBTCPK.MustMarshal(), babylonHeight, 1)
		}

		h.BeginBlocker()

		// Test nil request
		resp, err := h.FinalityKeeper.ActiveFinalityProvidersAtHeight(h.Ctx, nil)
		require.Nil(t, resp)
		require.Error(t, err)

		// Generate a page request with a limit and a nil key
		limit := datagen.RandomInt(r, int(numFpsWithVotingPower)) + 1
		pagination := constructRequestWithLimit(r, limit)
		// Generate the initial query
		req := types.QueryActiveFinalityProvidersAtHeightRequest{Height: babylonHeight, Pagination: pagination}
		// Construct a mapping from the finality providers found to a boolean value
		// Will be used later to evaluate whether all the finality providers were returned
		fpsFound := make(map[string]bool, 0)

		for i := uint64(0); i < numFpsWithVotingPower; i += limit {
			resp, err = h.FinalityKeeper.ActiveFinalityProvidersAtHeight(h.Ctx, &req)
			h.NoError(err)
			require.NotNil(t, resp)

			for _, fp := range resp.FinalityProviders {
				// Check if the pk exists in the map
				if _, ok := fpsWithVotingPowerMap[fp.BtcPkHex.MarshalHex()]; !ok {
					t.Fatalf("rpc returned a finality provider that was not created")
				}
				fpsFound[fp.BtcPkHex.MarshalHex()] = true
			}

			// Construct the next page request
			pagination = constructRequestWithKeyAndLimit(r, resp.Pagination.NextKey, limit)
			req = types.QueryActiveFinalityProvidersAtHeightRequest{Height: babylonHeight, Pagination: pagination}
		}

		require.Equal(t, len(fpsFound), len(fpsWithVotingPowerMap), "some finality providers were missed, got %d while %d were expected", len(fpsFound), len(fpsWithVotingPowerMap))
	})
}

func FuzzBlock(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// Setup keeper and context
		keeper, ctx := testkeeper.FinalityKeeper(t, nil, nil, nil)
		ctx = sdk.UnwrapSDKContext(ctx)

		height := datagen.RandomInt(r, 100)
		appHash := datagen.GenRandomByteArray(r, 32)
		ib := &types.IndexedBlock{
			Height:  height,
			AppHash: appHash,
		}

		if datagen.RandomInt(r, 2) == 1 {
			ib.Finalized = true
		}

		keeper.SetBlock(ctx, ib)
		req := &types.QueryBlockRequest{
			Height: height,
		}
		resp, err := keeper.Block(ctx, req)
		require.NoError(t, err)
		require.Equal(t, height, resp.Block.Height)
		require.Equal(t, appHash, resp.Block.AppHash)
	})
}

func FuzzListBlocks(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// Setup keeper and context
		keeper, ctx := testkeeper.FinalityKeeper(t, nil, nil, nil)
		ctx = sdk.UnwrapSDKContext(ctx)

		// index a random list of finalised blocks
		startHeight := datagen.RandomInt(r, 100)
		numIndexedBlocks := datagen.RandomInt(r, 100) + 1
		finalizedIndexedBlocks := make(map[uint64]*types.IndexedBlock)
		nonFinalizedIndexedBlocks := make(map[uint64]*types.IndexedBlock)
		indexedBlocks := make(map[uint64]*types.IndexedBlock)
		for i := startHeight; i < startHeight+numIndexedBlocks; i++ {
			ib := &types.IndexedBlock{
				Height:  i,
				AppHash: datagen.GenRandomByteArray(r, 32),
			}
			// randomly finalise some of them
			if datagen.RandomInt(r, 2) == 1 {
				ib.Finalized = true
				finalizedIndexedBlocks[ib.Height] = ib
			} else {
				nonFinalizedIndexedBlocks[ib.Height] = ib
			}
			indexedBlocks[ib.Height] = ib
			// insert to KVStore
			keeper.SetBlock(ctx, ib)
		}

		// perform a query to fetch finalized blocks and assert consistency
		// NOTE: pagination is already tested in Cosmos SDK so we don't test it here again,
		// instead only ensure it takes effect
		if len(finalizedIndexedBlocks) != 0 {
			limit := datagen.RandomInt(r, len(finalizedIndexedBlocks)) + 1
			req := &types.QueryListBlocksRequest{
				Status: types.QueriedBlockStatus_FINALIZED,
				Pagination: &query.PageRequest{
					CountTotal: true,
					Limit:      limit,
				},
			}
			resp1, err := keeper.ListBlocks(ctx, req)
			require.NoError(t, err)
			require.LessOrEqual(t, len(resp1.Blocks), int(limit)) // check if pagination takes effect
			require.EqualValues(t, resp1.Pagination.Total, len(finalizedIndexedBlocks))
			for _, actualIB := range resp1.Blocks {
				require.Equal(t, finalizedIndexedBlocks[actualIB.Height].AppHash, actualIB.AppHash)
			}
		}

		if len(nonFinalizedIndexedBlocks) != 0 {
			// perform a query to fetch non-finalized blocks and assert consistency
			limit := datagen.RandomInt(r, len(nonFinalizedIndexedBlocks)) + 1
			req := &types.QueryListBlocksRequest{
				Status: types.QueriedBlockStatus_NON_FINALIZED,
				Pagination: &query.PageRequest{
					CountTotal: true,
					Limit:      limit,
				},
			}
			resp2, err := keeper.ListBlocks(ctx, req)
			require.NoError(t, err)
			require.LessOrEqual(t, len(resp2.Blocks), int(limit)) // check if pagination takes effect
			require.EqualValues(t, resp2.Pagination.Total, len(nonFinalizedIndexedBlocks))
			for _, actualIB := range resp2.Blocks {
				require.Equal(t, nonFinalizedIndexedBlocks[actualIB.Height].AppHash, actualIB.AppHash)
			}
		}

		// perform a query to fetch all blocks and assert consistency
		limit := datagen.RandomInt(r, len(indexedBlocks)) + 1
		req := &types.QueryListBlocksRequest{
			Status: types.QueriedBlockStatus_ANY,
			Pagination: &query.PageRequest{
				CountTotal: true,
				Limit:      limit,
			},
		}
		resp3, err := keeper.ListBlocks(ctx, req)
		require.NoError(t, err)
		require.LessOrEqual(t, len(resp3.Blocks), int(limit)) // check if pagination takes effect
		require.EqualValues(t, resp3.Pagination.Total, len(indexedBlocks))
		for _, actualIB := range resp3.Blocks {
			require.Equal(t, indexedBlocks[actualIB.Height].AppHash, actualIB.AppHash)
		}
	})
}

func FuzzVotesAtHeight(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// Setup keeper and context
		keeper, ctx := testkeeper.FinalityKeeper(t, nil, nil, nil)
		ctx = sdk.UnwrapSDKContext(ctx)

		// Add random number of voted finality providers to the store
		babylonHeight := datagen.RandomInt(r, 10) + 1
		numVotedFps := datagen.RandomInt(r, 10) + 1
		votedFpsMap := make(map[string]bool, numVotedFps)
		for i := uint64(0); i < numVotedFps; i++ {
			votedFpPK, err := datagen.GenRandomBIP340PubKey(r)
			require.NoError(t, err)
			votedSig, err := bbn.NewSchnorrEOTSSig(datagen.GenRandomByteArray(r, 32))
			require.NoError(t, err)
			keeper.SetSig(ctx, babylonHeight, votedFpPK, votedSig)

			votedFpsMap[votedFpPK.MarshalHex()] = true
		}

		resp, err := keeper.VotesAtHeight(ctx, &types.QueryVotesAtHeightRequest{
			Height: babylonHeight,
		})
		require.NoError(t, err)

		// Check if all voted finality providers are returned
		fpsFoundMap := make(map[string]bool)
		for _, pk := range resp.BtcPks {
			if _, ok := votedFpsMap[pk.MarshalHex()]; !ok {
				t.Fatalf("rpc returned a finality provider that was not created")
			}
			fpsFoundMap[pk.MarshalHex()] = true
		}
		if len(fpsFoundMap) != len(votedFpsMap) {
			t.Errorf("Some finality providers were missed. Got %d while %d were expected", len(fpsFoundMap), len(votedFpsMap))
		}
	})
}

func FuzzListPubRandCommit(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Setup keeper and context
		bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
		cKeeper := types.NewMockCheckpointingKeeper(ctrl)
		fKeeper, ctx := testkeeper.FinalityKeeper(t, bsKeeper, nil, cKeeper)
		ctx = sdk.UnwrapSDKContext(ctx)
		ms := keeper.NewMsgServerImpl(*fKeeper)

		// set random BTC SK PK
		sk, _, err := datagen.GenRandomBTCKeyPair(r)
		bip340PK := bbn.NewBIP340PubKeyFromBTCPK(sk.PubKey())
		require.NoError(t, err)
		// register finality provider
		fp, err := datagen.GenRandomFinalityProviderWithBTCSK(r, sk, "", "")
		require.NoError(t, err)
		bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), gomock.Eq(bip340PK.MustMarshal())).Return(fp, nil).AnyTimes()
		bsKeeper.EXPECT().BabylonFinalityProviderExists(gomock.Any(), gomock.Eq(bip340PK.MustMarshal())).Return(true).AnyTimes()
		cKeeper.EXPECT().GetEpoch(gomock.Any()).Return(&epochingtypes.Epoch{EpochNumber: 1}).AnyTimes()

		commitCtxString := signingcontext.FpRandCommitContextV0(ctx.ChainID(), fKeeper.ModuleAddress())

		numPrCommitList := datagen.RandomInt(r, 10) + 1
		prCommitList := []*types.PubRandCommit{}

		// set a list of random public randomness commitment
		startHeight := datagen.RandomInt(r, 10) + 1
		for i := uint64(0); i < numPrCommitList; i++ {
			numPubRand := datagen.RandomInt(r, 10) + 100
			randListInfo, err := datagen.GenRandomPubRandList(r, numPubRand)
			require.NoError(t, err)
			prCommit := &types.PubRandCommit{
				StartHeight: startHeight,
				NumPubRand:  numPubRand,
				Commitment:  randListInfo.Commitment,
			}
			msg := &types.MsgCommitPubRandList{
				Signer:      datagen.GenRandomAccount().Address,
				FpBtcPk:     bip340PK,
				StartHeight: startHeight,
				NumPubRand:  numPubRand,
				Commitment:  prCommit.Commitment,
			}
			hash, err := msg.HashToSign(commitCtxString)
			require.NoError(t, err)
			schnorrSig, err := schnorr.Sign(sk, hash)
			require.NoError(t, err)
			msg.Sig = bbn.NewBIP340SignatureFromBTCSig(schnorrSig)
			_, err = ms.CommitPubRandList(ctx, msg)
			require.NoError(t, err)

			prCommitList = append(prCommitList, prCommit)

			startHeight += numPubRand
		}

		resp, err := fKeeper.ListPubRandCommit(ctx, &types.QueryListPubRandCommitRequest{
			FpBtcPkHex: bip340PK.MarshalHex(),
		})
		require.NoError(t, err)

		for _, prCommit := range prCommitList {
			prCommitResp, ok := resp.PubRandCommitMap[prCommit.StartHeight]
			require.True(t, ok)
			require.Equal(t, prCommitResp.NumPubRand, prCommit.NumPubRand)
			require.Equal(t, prCommitResp.Commitment, prCommit.Commitment)
		}
	})
}

func FuzzQueryEvidence(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// Setup keeper and context
		keeper, ctx := testkeeper.FinalityKeeper(t, nil, nil, nil)
		ctx = sdk.UnwrapSDKContext(ctx)

		// set random BTC SK PK
		sk, _, err := datagen.GenRandomBTCKeyPair(r)
		bip340PK := bbn.NewBIP340PubKeyFromBTCPK(sk.PubKey())
		require.NoError(t, err)

		var randomFirstSlashableEvidence *types.Evidence = nil
		numEvidences := datagen.RandomInt(r, 10) + 1
		height := uint64(5)

		// set a list of evidences, in which some of them are slashable while the others are not
		for i := uint64(0); i < numEvidences; i++ {
			evidence, err := datagen.GenRandomEvidence(r, sk, height)
			require.NoError(t, err)
			if datagen.RandomInt(r, 2) == 1 {
				evidence.CanonicalFinalitySig = nil // not slashable
			} else if randomFirstSlashableEvidence == nil {
				randomFirstSlashableEvidence = evidence // first slashable
			}
			keeper.SetEvidence(ctx, evidence)

			height += datagen.RandomInt(r, 5) + 1
		}

		// get first slashable evidence
		evidenceResp, err := keeper.Evidence(ctx, &types.QueryEvidenceRequest{FpBtcPkHex: bip340PK.MarshalHex()})
		if randomFirstSlashableEvidence == nil {
			require.Error(t, err)
			require.Nil(t, evidenceResp)
		} else {
			require.NoError(t, err)
			require.Equal(t, randomFirstSlashableEvidence, convertToEvidence(evidenceResp.Evidence))
			require.True(t, convertToEvidence(evidenceResp.Evidence).IsSlashable())
		}
	})
}

func FuzzListEvidences(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// Setup keeper and context
		keeper, ctx := testkeeper.FinalityKeeper(t, nil, nil, nil)
		ctx = sdk.UnwrapSDKContext(ctx)

		// generate a random list of evidences since startHeight
		startHeight := datagen.RandomInt(r, 1000) + 100
		numEvidences := datagen.RandomInt(r, 100) + 10
		evidences := map[string]*types.Evidence{}
		for i := uint64(0); i < numEvidences; i++ {
			// random key pair
			sk, pk, err := datagen.GenRandomBTCKeyPair(r)
			require.NoError(t, err)
			btcPK := bbn.NewBIP340PubKeyFromBTCPK(pk)
			// random height
			height := datagen.RandomInt(r, 100) + startHeight + 1
			// generate evidence
			evidence, err := datagen.GenRandomEvidence(r, sk, height)
			require.NoError(t, err)
			// add evidence to map and finlaity keeper
			evidences[btcPK.MarshalHex()] = evidence
			keeper.SetEvidence(ctx, evidence)
		}

		// generate another list of evidences before startHeight
		// these evidences will not be included in the response if
		// the request specifies the above startHeight
		for i := uint64(0); i < numEvidences; i++ {
			// random key pair
			sk, _, err := datagen.GenRandomBTCKeyPair(r)
			require.NoError(t, err)
			// random height before startHeight
			height := datagen.RandomInt(r, int(startHeight))
			// generate evidence
			evidence, err := datagen.GenRandomEvidence(r, sk, height)
			require.NoError(t, err)
			// add evidence to finlaity keeper
			keeper.SetEvidence(ctx, evidence)
		}

		// perform a query to fetch all evidences and assert consistency
		limit := datagen.RandomInt(r, int(numEvidences)) + 1
		req := &types.QueryListEvidencesRequest{
			StartHeight: startHeight,
			Pagination: &query.PageRequest{
				CountTotal: true,
				Limit:      limit,
			},
		}
		resp, err := keeper.ListEvidences(ctx, req)
		require.NoError(t, err)
		require.LessOrEqual(t, len(resp.Evidences), int(limit))     // check if pagination takes effect
		require.EqualValues(t, resp.Pagination.Total, numEvidences) // ensure evidences before startHeight are not included
		for _, actualEvidenceResponse := range resp.Evidences {
			actualEvidence := convertToEvidence(actualEvidenceResponse)
			expectedEvidence := evidences[actualEvidenceResponse.FpBtcPkHex]
			require.Equal(t, expectedEvidence, actualEvidence)
		}
	})
}

func FuzzSigningInfo(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// Setup keeper and context
		fKeeper, ctx := testkeeper.FinalityKeeper(t, nil, nil, nil)
		ctx = sdk.UnwrapSDKContext(ctx)

		// generate a random list of signing info
		numSigningInfo := datagen.RandomInt(r, 100) + 10

		fpSigningInfos := map[string]*types.FinalityProviderSigningInfo{}
		fpPks := make([]string, 0)
		for i := uint64(0); i < numSigningInfo; i++ {
			// random key pair
			fpPk, err := datagen.GenRandomBIP340PubKey(r)
			require.NoError(t, err)
			fpPks = append(fpPks, fpPk.MarshalHex())

			// random height and missed block counter
			height := int64(datagen.RandomInt(r, 100) + 1)
			missedBlockCounter := int64(datagen.RandomInt(r, 100) + 1)

			// create signing info and add it to map and finality keeper
			signingInfo := types.NewFinalityProviderSigningInfo(fpPk, height, missedBlockCounter)
			err = fKeeper.FinalityProviderSigningTracker.Set(ctx, fpPk.MustMarshal(), signingInfo)
			require.NoError(t, err)
			fpSigningInfos[fpPk.MarshalHex()] = &signingInfo
		}

		// perform queries for signing info of a given finality provider
		for i := uint64(0); i < numSigningInfo; i++ {
			fpPk := fpPks[i]
			req := &types.QuerySigningInfoRequest{FpBtcPkHex: fpPk}
			resp, err := fKeeper.SigningInfo(ctx, req)
			require.NoError(t, err)
			require.Equal(t, fpSigningInfos[fpPk].StartHeight, resp.SigningInfo.StartHeight)
			require.Equal(t, fpSigningInfos[fpPk].MissedBlocksCounter, resp.SigningInfo.MissedBlocksCounter)
			require.Equal(t, fpPk, resp.SigningInfo.FpBtcPkHex)
		}

		// perform a query for signing info of non-exist finality provider
		nonExistFpPk, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(t, err)
		require.NoError(t, err)
		invalidReq := &types.QuerySigningInfoRequest{FpBtcPkHex: nonExistFpPk.MarshalHex()}
		_, err = fKeeper.SigningInfo(ctx, invalidReq)
		require.Contains(t, err.Error(), fmt.Sprintf("SigningInfo not found for the finality provider %s", nonExistFpPk.MarshalHex()))

		// perform a query for signing infos of all the finality providers
		limit := datagen.RandomInt(r, int(numSigningInfo)) + 1
		req := &types.QuerySigningInfosRequest{
			Pagination: &query.PageRequest{
				CountTotal: true,
				Limit:      limit,
			},
		}
		resp, err := fKeeper.SigningInfos(ctx, req)
		require.NoError(t, err)
		require.LessOrEqual(t, len(resp.SigningInfos), int(limit))    // check if pagination takes effect
		require.EqualValues(t, resp.Pagination.Total, numSigningInfo) // ensure evidences before startHeight are not included
		for _, si := range resp.SigningInfos {
			require.Equal(t, fpSigningInfos[si.FpBtcPkHex].MissedBlocksCounter, si.MissedBlocksCounter)
			require.Equal(t, fpSigningInfos[si.FpBtcPkHex].StartHeight, si.StartHeight)
		}
	})
}

func convertToEvidence(er *types.EvidenceResponse) *types.Evidence {
	fpBtcPk, err := bbn.NewBIP340PubKeyFromHex(er.FpBtcPkHex)
	if err != nil {
		return nil
	}
	return &types.Evidence{
		FpBtcPk:              fpBtcPk,
		BlockHeight:          er.BlockHeight,
		PubRand:              er.PubRand,
		CanonicalAppHash:     er.CanonicalAppHash,
		ForkAppHash:          er.ForkAppHash,
		CanonicalFinalitySig: er.CanonicalFinalitySig,
		ForkFinalitySig:      er.ForkFinalitySig,
		SigningContext:       er.SigningContext,
	}
}

// Constructors for PageRequest objects
func constructRequestWithKeyAndLimit(r *rand.Rand, key []byte, limit uint64) *query.PageRequest {
	// If limit is 0, set one randomly
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
