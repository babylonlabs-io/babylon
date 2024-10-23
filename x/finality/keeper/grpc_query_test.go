package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/txscript"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	testkeeper "github.com/babylonlabs-io/babylon/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/types"
	btclctypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	epochingtypes "github.com/babylonlabs-io/babylon/x/epoching/types"
	"github.com/babylonlabs-io/babylon/x/finality/keeper"
	"github.com/babylonlabs-io/babylon/x/finality/types"
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
		fp, err := datagen.GenRandomFinalityProvider(r)
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

		// Setup keeper and context
		keeper, ctx := testkeeper.BTCStakingKeeper(t, nil, nil, nil)

		// random finality provider
		fp, err := datagen.GenRandomFinalityProvider(r)
		require.NoError(t, err)
		// add this finality provider
		AddFinalityProvider(t, ctx, *keeper, fp)
		// set random voting power at random height
		randomHeight := datagen.RandomInt(r, 100) + 1
		randomPower := datagen.RandomInt(r, 100) + 1
		keeper.SetVotingPower(ctx, fp.BtcPk.MustMarshal(), randomHeight, randomPower)

		// happy case
		req1 := &types.QueryFinalityProviderPowerAtHeightRequest{
			FpBtcPkHex: fp.BtcPk.MarshalHex(),
			Height:     randomHeight,
		}
		resp, err := keeper.FinalityProviderPowerAtHeight(ctx, req1)
		require.NoError(t, err)
		require.Equal(t, randomPower, resp.VotingPower)

		// case where the voting power store is not updated in
		// the given height
		requestHeight := randomHeight + datagen.RandomInt(r, 10) + 1
		req2 := &types.QueryFinalityProviderPowerAtHeightRequest{
			FpBtcPkHex: fp.BtcPk.MarshalHex(),
			Height:     requestHeight,
		}
		_, err = keeper.FinalityProviderPowerAtHeight(ctx, req2)
		require.ErrorIs(t, err, types.ErrVotingPowerTableNotUpdated)

		// case where the given fp pk does not exist
		randPk, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(t, err)
		req3 := &types.QueryFinalityProviderPowerAtHeightRequest{
			FpBtcPkHex: randPk.MarshalHex(),
			Height:     randomHeight,
		}
		_, err = keeper.FinalityProviderPowerAtHeight(ctx, req3)
		require.ErrorIs(t, err, types.ErrFpNotFound)
	})
}

func FuzzFinalityProviderCurrentVotingPower(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// Setup keeper and context
		keeper, ctx := testkeeper.BTCStakingKeeper(t, nil, nil, nil)

		// random finality provider
		fp, err := datagen.GenRandomFinalityProvider(r)
		require.NoError(t, err)
		// add this finality provider
		AddFinalityProvider(t, ctx, *keeper, fp)
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

		// Setup keeper and context
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btclcKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(&btclctypes.BTCHeaderInfo{Height: 10}).AnyTimes()
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		btccKeeper.EXPECT().GetParams(gomock.Any()).Return(btcctypes.DefaultParams()).AnyTimes()
		keeper, ctx := testkeeper.BTCStakingKeeper(t, btclcKeeper, btccKeeper, nil)

		// covenant and slashing addr
		covenantSKs, covenantPKs, covenantQuorum := datagen.GenCovenantCommittee(r)
		slashingAddress, err := datagen.GenRandomBTCAddress(r, net)
		require.NoError(t, err)
		slashingPkScript, err := txscript.PayToAddrScript(slashingAddress)
		require.NoError(t, err)
		slashingChangeLockTime := uint16(101)

		// Generate a slashing rate in the range [0.1, 0.50] i.e., 10-50%.
		// NOTE - if the rate is higher or lower, it may produce slashing or change outputs
		// with value below the dust threshold, causing test failure.
		// Our goal is not to test failure due to such extreme cases here;
		// this is already covered in FuzzGeneratingValidStakingSlashingTx
		slashingRate := sdkmath.LegacyNewDecWithPrec(int64(datagen.RandomInt(r, 41)+10), 2)

		// Generate a random batch of finality providers
		var fps []*types.FinalityProvider
		numFpsWithVotingPower := datagen.RandomInt(r, 10) + 1
		numFps := numFpsWithVotingPower + datagen.RandomInt(r, 10)
		for i := uint64(0); i < numFps; i++ {
			fp, err := datagen.GenRandomFinalityProvider(r)
			require.NoError(t, err)
			AddFinalityProvider(t, ctx, *keeper, fp)
			fps = append(fps, fp)
		}

		// For numFpsWithVotingPower finality providers, generate a random number of BTC delegations
		numBTCDels := datagen.RandomInt(r, 10) + 1
		babylonHeight := datagen.RandomInt(r, 10) + 1
		fpsWithVotingPowerMap := make(map[string]*types.FinalityProvider)
		for i := uint64(0); i < numFpsWithVotingPower; i++ {
			fpBTCPK := fps[i].BtcPk
			fpsWithVotingPowerMap[fpBTCPK.MarshalHex()] = fps[i]

			var totalVotingPower uint64
			for j := uint64(0); j < numBTCDels; j++ {
				delSK, _, err := datagen.GenRandomBTCKeyPair(r)
				require.NoError(t, err)
				startHeight, endHeight := uint32(1), uint32(1000)
				stakingTime := endHeight - startHeight
				btcDel, err := datagen.GenRandomBTCDelegation(
					r,
					t,
					net,
					[]bbn.BIP340PubKey{*fpBTCPK},
					delSK,
					covenantSKs,
					covenantPKs,
					covenantQuorum,
					slashingPkScript,
					stakingTime, 1, 1000, 10000,
					slashingRate,
					slashingChangeLockTime,
				)
				require.NoError(t, err)
				err = keeper.AddBTCDelegation(ctx, btcDel)
				require.NoError(t, err)
				totalVotingPower += btcDel.TotalSat
			}

			keeper.SetVotingPower(ctx, fpBTCPK.MustMarshal(), babylonHeight, totalVotingPower)
		}

		// Test nil request
		resp, err := keeper.ActiveFinalityProvidersAtHeight(ctx, nil)
		if resp != nil {
			t.Errorf("Nil input led to a non-nil response")
		}
		if err == nil {
			t.Errorf("Nil input led to a nil error")
		}

		// Generate a page request with a limit and a nil key
		limit := datagen.RandomInt(r, int(numFpsWithVotingPower)) + 1
		pagination := constructRequestWithLimit(r, limit)
		// Generate the initial query
		req := types.QueryActiveFinalityProvidersAtHeightRequest{Height: babylonHeight, Pagination: pagination}
		// Construct a mapping from the finality providers found to a boolean value
		// Will be used later to evaluate whether all the finality providers were returned
		fpsFound := make(map[string]bool, 0)

		for i := uint64(0); i < numFpsWithVotingPower; i += limit {
			resp, err = keeper.ActiveFinalityProvidersAtHeight(ctx, &req)
			if err != nil {
				t.Errorf("Valid request led to an error %s", err)
			}
			if resp == nil {
				t.Fatalf("Valid request led to a nil response")
			}

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

		if len(fpsFound) != len(fpsWithVotingPowerMap) {
			t.Errorf("Some finality providers were missed. Got %d while %d were expected", len(fpsFound), len(fpsWithVotingPowerMap))
		}
	})
}

func FuzzFinalityProviderDelegations(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Setup keeper and context
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		btccKeeper.EXPECT().GetParams(gomock.Any()).Return(btcctypes.DefaultParams()).AnyTimes()
		keeper, ctx := testkeeper.BTCStakingKeeper(t, btclcKeeper, btccKeeper, nil)

		// covenant and slashing addr
		covenantSKs, covenantPKs, covenantQuorum := datagen.GenCovenantCommittee(r)
		slashingAddress, err := datagen.GenRandomBTCAddress(r, net)
		require.NoError(t, err)
		slashingPkScript, err := txscript.PayToAddrScript(slashingAddress)
		require.NoError(t, err)
		slashingChangeLockTime := uint16(101)

		// Generate a slashing rate in the range [0.1, 0.50] i.e., 10-50%.
		// NOTE - if the rate is higher or lower, it may produce slashing or change outputs
		// with value below the dust threshold, causing test failure.
		// Our goal is not to test failure due to such extreme cases here;
		// this is already covered in FuzzGeneratingValidStakingSlashingTx
		slashingRate := sdkmath.LegacyNewDecWithPrec(int64(datagen.RandomInt(r, 41)+10), 2)

		// Generate a finality provider
		fp, err := datagen.GenRandomFinalityProvider(r)
		require.NoError(t, err)
		AddFinalityProvider(t, ctx, *keeper, fp)

		startHeight := uint32(datagen.RandomInt(r, 100)) + 1
		endHeight := uint32(datagen.RandomInt(r, 1000)) + startHeight + btcctypes.DefaultParams().CheckpointFinalizationTimeout + 1
		stakingTime := endHeight - startHeight
		btclcKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(&btclctypes.BTCHeaderInfo{Height: startHeight}).AnyTimes()
		// Generate a random number of BTC delegations under this finality provider
		numBTCDels := datagen.RandomInt(r, 10) + 1
		expectedBtcDelsMap := make(map[string]*types.BTCDelegation)
		for j := uint64(0); j < numBTCDels; j++ {
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			require.NoError(t, err)
			btcDel, err := datagen.GenRandomBTCDelegation(
				r,
				t,
				net,
				[]bbn.BIP340PubKey{*fp.BtcPk},
				delSK,
				covenantSKs,
				covenantPKs,
				covenantQuorum,
				slashingPkScript,
				stakingTime, startHeight, endHeight, 10000,
				slashingRate,
				slashingChangeLockTime,
			)
			require.NoError(t, err)
			expectedBtcDelsMap[btcDel.BtcPk.MarshalHex()] = btcDel
			err = keeper.AddBTCDelegation(ctx, btcDel)
			require.NoError(t, err)
		}

		// Test nil request
		resp, err := keeper.FinalityProviderDelegations(ctx, nil)
		require.Nil(t, resp)
		require.Error(t, err)

		babylonHeight := datagen.RandomInt(r, 10) + 1
		ctx = datagen.WithCtxHeight(ctx, babylonHeight)
		keeper.IndexBTCHeight(ctx)

		// Generate a page request with a limit and a nil key
		// query a page of BTC delegations and assert consistency
		limit := datagen.RandomInt(r, len(expectedBtcDelsMap)) + 1

		// FinalityProviderDelegations loads status, which calls GetTipInfo
		btclcKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(&btclctypes.BTCHeaderInfo{Height: startHeight}).AnyTimes()

		keeper.IndexBTCHeight(ctx)

		pagination := constructRequestWithLimit(r, limit)
		// Generate the initial query
		req := types.QueryFinalityProviderDelegationsRequest{
			FpBtcPkHex: fp.BtcPk.MarshalHex(),
			Pagination: pagination,
		}
		// Construct a mapping from the finality providers found to a boolean value
		// Will be used later to evaluate whether all the finality providers were returned
		btcDelsFound := make(map[string]bool, 0)

		for i := uint64(0); i < numBTCDels; i += limit {
			resp, err = keeper.FinalityProviderDelegations(ctx, &req)
			require.NoError(t, err)
			require.NotNil(t, resp)
			for _, btcDels := range resp.BtcDelegatorDelegations {
				require.Len(t, btcDels.Dels, 1)
				btcDel := btcDels.Dels[0]
				require.Equal(t, fp.BtcPk, &btcDel.FpBtcPkList[0])
				// Check if the pk exists in the map
				_, ok := expectedBtcDelsMap[btcDel.BtcPk.MarshalHex()]
				require.True(t, ok)
				btcDelsFound[btcDel.BtcPk.MarshalHex()] = true
			}
			// Construct the next page request
			pagination = constructRequestWithKeyAndLimit(r, resp.Pagination.NextKey, limit)
			req = types.QueryFinalityProviderDelegationsRequest{
				FpBtcPkHex: fp.BtcPk.MarshalHex(),
				Pagination: pagination,
			}
		}
		require.Equal(t, len(btcDelsFound), len(expectedBtcDelsMap))

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
		fp, err := datagen.GenRandomFinalityProviderWithBTCSK(r, sk)
		require.NoError(t, err)
		bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), gomock.Eq(bip340PK.MustMarshal())).Return(fp, nil).AnyTimes()
		bsKeeper.EXPECT().HasFinalityProvider(gomock.Any(), gomock.Eq(bip340PK.MustMarshal())).Return(true).AnyTimes()
		cKeeper.EXPECT().GetEpoch(gomock.Any()).Return(&epochingtypes.Epoch{EpochNumber: 1}).AnyTimes()

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
			hash, err := msg.HashToSign()
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
			} else {
				if randomFirstSlashableEvidence == nil {
					randomFirstSlashableEvidence = evidence // first slashable
				}
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
	}
}
