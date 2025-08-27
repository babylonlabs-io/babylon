package keeper_test

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v2/testutil/datagen"
	testkeeper "github.com/babylonlabs-io/babylon/v2/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/v2/types"
	btcctypes "github.com/babylonlabs-io/babylon/v2/x/btccheckpoint/types"
	btclctypes "github.com/babylonlabs-io/babylon/v2/x/btclightclient/types"
	btcstakingkeeper "github.com/babylonlabs-io/babylon/v2/x/btcstaking/keeper"
	"github.com/babylonlabs-io/babylon/v2/x/btcstaking/types"
)

var net = &chaincfg.SimNetParams

func FuzzFinalityProviders(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// Setup keeper and context
		keeper, ctx := testkeeper.BTCStakingKeeper(t, nil, nil, nil)
		ctx = sdk.UnwrapSDKContext(ctx)

		// Generate random finality providers and add them to kv store
		fpsMap := make(map[string]*types.FinalityProvider)
		for i := 0; i < int(datagen.RandomInt(r, 10)+1); i++ {
			fp, err := datagen.GenRandomFinalityProvider(r)
			require.NoError(t, err)

<<<<<<< HEAD
			AddFinalityProvider(t, ctx, *keeper, fp)
			fpsMap[fp.BtcPk.MarshalHex()] = fp
=======
			// Randomly choose BSN ID to test both Babylon and consumer cases
			var bsnId string
			randomChoice := datagen.RandomInt(r, 3)
			switch randomChoice {
			case 0:
				bsnId = "" // Empty string defaults to Babylon
			case 1:
				bsnId = babylonBsnId
			case 2:
				bsnId = registeredBsnId // Use registered consumer BSN ID
			}

			fp, err := datagen.GenRandomFinalityProviderWithBTCSK(r, fpSK, h.FpPopContext(), bsnId)
			require.NoError(t, err)

			// Add the finality provider
			err = h.BTCStakingKeeper.AddFinalityProvider(h.Ctx, &types.MsgCreateFinalityProvider{
				Addr:        fp.Addr,
				Description: fp.Description,
				Commission: types.NewCommissionRates(
					*fp.Commission,
					fp.CommissionInfo.MaxRate,
					fp.CommissionInfo.MaxChangeRate,
				),
				BtcPk: fp.BtcPk,
				Pop:   fp.Pop,
				BsnId: fp.BsnId,
			})
			require.NoError(t, err)

			// Store in maps, resolving empty BSN ID to Babylon
			actualBsnId := bsnId
			if actualBsnId == "" {
				actualBsnId = babylonBsnId
			}

			// Update the FP object to have the resolved BSN ID
			fp.BsnId = actualBsnId

			if fpsMapByBsn[actualBsnId] == nil {
				fpsMapByBsn[actualBsnId] = make(map[string]*types.FinalityProvider)
			}
			if i%2 == 0 {
				err = h.BTCStakingKeeper.SoftDeleteFinalityProvider(h.Ctx, fp.BtcPk)
				require.NoError(t, err)
			}

			fpsMapByBsn[actualBsnId][fp.BtcPk.MarshalHex()] = fp
			allFpsMap[fp.BtcPk.MarshalHex()] = fp
>>>>>>> ae7142f (chore: add soft deleted to fp resp (#1594))
		}
		numOfFpsInStore := len(fpsMap)

		// Test nil request
<<<<<<< HEAD
		resp, err := keeper.FinalityProviders(ctx, nil)
		if resp != nil {
			t.Errorf("Nil input led to a non-nil response")
		}
		if err == nil {
			t.Errorf("Nil input led to a nil error")
=======
		resp, err := h.BTCStakingKeeper.FinalityProviders(h.Ctx, nil)
		require.Error(t, err)
		require.Nil(t, resp)

		// Test 1: Query without BSN ID (should default to Babylon BSN)
		babylonFpsMap := fpsMapByBsn[babylonBsnId]

		if len(babylonFpsMap) > 0 {
			// Generate a page request with a limit and a nil key
			limit := datagen.RandomInt(r, len(babylonFpsMap)) + 1
			pagination := constructRequestWithLimit(r, limit)

			req := types.QueryFinalityProvidersRequest{
				Pagination: pagination,
				// BsnId not provided, should default to Babylon
			}

			// Test pagination through all Babylon FPs
			fpsFound := make(map[string]bool)
			for {
				resp, err = h.BTCStakingKeeper.FinalityProviders(h.Ctx, &req)
				require.NoError(t, err)
				require.NotNil(t, resp)

				for _, fp := range resp.FinalityProviders {
					// Should be Babylon FPs only
					require.Equal(t, babylonBsnId, fp.BsnId)

					// Check if the pk exists in the babylon map
					if _, ok := babylonFpsMap[fp.BtcPk.MarshalHex()]; !ok {
						t.Fatalf("rpc returned a finality provider that was not created for Babylon BSN")
					}
					fpsFound[fp.BtcPk.MarshalHex()] = true
					isDeleted := h.BTCStakingKeeper.IsFinalityProviderDeleted(h.Ctx, fp.BtcPk)
					require.Equal(t, fp.SoftDeleted, isDeleted)
				}

				// Break if no more pages
				if resp.Pagination.NextKey == nil {
					break
				}

				// Construct the next page request
				pagination = constructRequestWithKeyAndLimit(r, resp.Pagination.NextKey, limit)
				req = types.QueryFinalityProvidersRequest{
					Pagination: pagination,
					// BsnId still not provided
				}
			}

			if len(fpsFound) != len(babylonFpsMap) {
				t.Errorf("Some Babylon finality providers were missed. Got %d while %d were expected", len(fpsFound), len(babylonFpsMap))
			}
>>>>>>> ae7142f (chore: add soft deleted to fp resp (#1594))
		}

		// Generate a page request with a limit and a nil key
		limit := datagen.RandomInt(r, numOfFpsInStore) + 1
		pagination := constructRequestWithLimit(r, limit)
		// Generate the initial query
		req := types.QueryFinalityProvidersRequest{Pagination: pagination}
		// Construct a mapping from the finality providers found to a boolean value
		// Will be used later to evaluate whether all the finality providers were returned
		fpsFound := make(map[string]bool, 0)

		for i := uint64(0); i < uint64(numOfFpsInStore); i += limit {
			resp, err = keeper.FinalityProviders(ctx, &req)
			if err != nil {
				t.Errorf("Valid request led to an error %s", err)
			}
			if resp == nil {
				t.Fatalf("Valid request led to a nil response")
			}

			for _, fp := range resp.FinalityProviders {
				// Check if the pk exists in the map
				if _, ok := fpsMap[fp.BtcPk.MarshalHex()]; !ok {
					t.Fatalf("rpc returned a finality provider that was not created")
				}
				fpsFound[fp.BtcPk.MarshalHex()] = true
			}

			// Construct the next page request
			pagination = constructRequestWithKeyAndLimit(r, resp.Pagination.NextKey, limit)
			req = types.QueryFinalityProvidersRequest{Pagination: pagination}
		}

		if len(fpsFound) != len(fpsMap) {
			t.Errorf("Some finality providers were missed. Got %d while %d were expected", len(fpsFound), len(fpsMap))
		}
	})
}

func FuzzFinalityProvider(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		// Setup keeper and context
		keeper, ctx := testkeeper.BTCStakingKeeper(t, nil, nil, nil)
		ctx = sdk.UnwrapSDKContext(ctx)

		// Generate random finality providers and add them to kv store
		fpsMap := make(map[string]*types.FinalityProvider)
		for i := 0; i < int(datagen.RandomInt(r, 10)+1); i++ {
			fp, err := datagen.GenRandomFinalityProvider(r)
			require.NoError(t, err)

			AddFinalityProvider(t, ctx, *keeper, fp)
			fp.HighestVotedHeight = uint32(datagen.RandomInt(r, 1000) + 1)
			err = keeper.UpdateFinalityProvider(ctx, fp)
			require.NoError(t, err)
			fpsMap[fp.BtcPk.MarshalHex()] = fp
		}

		// Test nil request
		resp, err := keeper.FinalityProvider(ctx, nil)
		require.Error(t, err)
		require.Nil(t, resp)

		for k, v := range fpsMap {
			// Generate a request with a valid key
			req := types.QueryFinalityProviderRequest{FpBtcPkHex: k}
			resp, err := keeper.FinalityProvider(ctx, &req)
			if err != nil {
				t.Errorf("Valid request led to an error %s", err)
			}
			if resp == nil {
				t.Fatalf("Valid request led to a nil response")
			}

			// check keys from map matches those in returned response
			require.Equal(t, v.BtcPk.MarshalHex(), resp.FinalityProvider.BtcPk.MarshalHex())
			require.Equal(t, v.Addr, resp.FinalityProvider.Addr)
			require.Equal(t, v.HighestVotedHeight, resp.FinalityProvider.HighestVotedHeight)
		}

		// check some random non-existing guy
		fp, err := datagen.GenRandomFinalityProvider(r)
		require.NoError(t, err)
		req := types.QueryFinalityProviderRequest{FpBtcPkHex: fp.BtcPk.MarshalHex()}
		respNonExists, err := keeper.FinalityProvider(ctx, &req)
		require.Error(t, err)
		require.Nil(t, respNonExists)
		require.True(t, errors.Is(err, types.ErrFpNotFound))
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

func FuzzPendingBTCDelegations(f *testing.F) {
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

		// Generate a random number of finality providers
		numFps := datagen.RandomInt(r, 5) + 1
		fps := []*types.FinalityProvider{}
		for i := uint64(0); i < numFps; i++ {
			fp, err := datagen.GenRandomFinalityProvider(r)
			require.NoError(t, err)
			AddFinalityProvider(t, ctx, *keeper, fp)
			fps = append(fps, fp)
		}

		// Generate a random number of BTC delegations under each finality provider
		startHeight := uint32(datagen.RandomInt(r, 100)) + 1
		btclcKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(&btclctypes.BTCHeaderInfo{Height: startHeight}).AnyTimes()

		endHeight := uint32(datagen.RandomInt(r, 1000)) + startHeight + btcctypes.DefaultParams().CheckpointFinalizationTimeout + 1
		stakingTime := endHeight - startHeight
		numBTCDels := datagen.RandomInt(r, 10) + 1
		pendingBtcDelsMap := make(map[string]*types.BTCDelegation)
		for _, fp := range fps {
			for j := uint64(0); j < numBTCDels; j++ {
				delSK, _, err := datagen.GenRandomBTCKeyPair(r)
				require.NoError(t, err)
				// 0.5 chance that the delegation is created via pre-approval flow
				if r.Intn(2) == 0 {
					startHeight, endHeight = 0, 0
				}
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
				if datagen.RandomInt(r, 2) == 1 {
					// remove covenant sig in random BTC delegations to make them inactive
					btcDel.CovenantSigs = nil
					pendingBtcDelsMap[btcDel.BtcPk.MarshalHex()] = btcDel
				}
				err = keeper.AddBTCDelegation(ctx, btcDel)
				require.NoError(t, err)

				txHash := btcDel.MustGetStakingTxHash().String()
				delView, err := keeper.BTCDelegation(ctx, &types.QueryBTCDelegationRequest{
					StakingTxHashHex: txHash,
				})
				require.NoError(t, err)
				require.NotNil(t, delView)
			}
		}

		babylonHeight := datagen.RandomInt(r, 10) + 1
		ctx = datagen.WithCtxHeight(ctx, babylonHeight)

		// querying paginated BTC delegations and assert
		// Generate a page request with a limit and a nil key
		if len(pendingBtcDelsMap) == 0 {
			return
		}
		limit := datagen.RandomInt(r, len(pendingBtcDelsMap)) + 1
		pagination := constructRequestWithLimit(r, limit)
		req := &types.QueryBTCDelegationsRequest{
			Status:     types.BTCDelegationStatus_PENDING,
			Pagination: pagination,
		}
		for i := uint64(0); i < numBTCDels; i += limit {
			resp, err := keeper.BTCDelegations(ctx, req)
			require.NoError(t, err)
			require.NotNil(t, resp)
			for _, btcDel := range resp.BtcDelegations {
				_, ok := pendingBtcDelsMap[btcDel.BtcPk.MarshalHex()]
				require.True(t, ok)
				require.Equal(t, stakingTime, btcDel.StakingTime)
			}
			// Construct the next page request
			pagination.Key = resp.Pagination.NextKey
		}
	})
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

func AddFinalityProvider(t *testing.T, goCtx context.Context, k btcstakingkeeper.Keeper, fp *types.FinalityProvider) {
	err := k.AddFinalityProvider(goCtx, &types.MsgCreateFinalityProvider{
		Addr:        fp.Addr,
		Description: fp.Description,
		Commission: types.NewCommissionRates(
			*fp.Commission,
			fp.CommissionInfo.MaxRate,
			fp.CommissionInfo.MaxChangeRate,
		),
		BtcPk: fp.BtcPk,
		Pop:   fp.Pop,
	})
	require.NoError(t, err)
}

func TestCorrectParamsVersionIsUsed(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

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

	fp, err := datagen.GenRandomFinalityProvider(r)
	require.NoError(t, err)
	AddFinalityProvider(t, ctx, *keeper, fp)

	startHeight := uint32(datagen.RandomInt(r, 100)) + 1
	btclcKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(&btclctypes.BTCHeaderInfo{Height: startHeight}).AnyTimes()

	endHeight := uint32(datagen.RandomInt(r, 1000)) + startHeight + btcctypes.DefaultParams().CheckpointFinalizationTimeout + 1
	stakingTime := endHeight - startHeight
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

	err = keeper.AddBTCDelegation(ctx, btcDel)
	require.NoError(t, err)

	// delegation is active as it have all covenant sigs
	txHash := btcDel.MustGetStakingTxHash().String()
	delView, err := keeper.BTCDelegation(ctx, &types.QueryBTCDelegationRequest{
		StakingTxHashHex: txHash,
	})
	require.NoError(t, err)
	require.NotNil(t, delView)

	require.True(t, delView.BtcDelegation.Active)

	dp := types.DefaultParams()

	// Generate 3 new key pairs and increase keys and quorum size in params, this
	// will mean new delegations will require more signatures to be active
	_, pk1, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	_, pk2, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	_, pk3, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	// Convert public keys to BIP340 format
	bip340pk1 := bbn.NewBIP340PubKeyFromBTCPK(pk1)
	bip340pk2 := bbn.NewBIP340PubKeyFromBTCPK(pk2)
	bip340pk3 := bbn.NewBIP340PubKeyFromBTCPK(pk3)

	dp.BtcActivationHeight = 10
	dp.CovenantPks = append(dp.CovenantPks, *bip340pk1, *bip340pk2, *bip340pk3)
	dp.CovenantQuorum += 3

	err = keeper.SetParams(ctx, dp)
	require.NoError(t, err)

	// check delegation is still active in every endpoint
	delView, err = keeper.BTCDelegation(ctx, &types.QueryBTCDelegationRequest{
		StakingTxHashHex: txHash,
	})
	require.NoError(t, err)
	require.NotNil(t, delView)

	require.True(t, delView.BtcDelegation.Active)

	delegationsView, err := keeper.BTCDelegations(ctx, &types.QueryBTCDelegationsRequest{
		Status: types.BTCDelegationStatus_ACTIVE,
	})
	require.NoError(t, err)
	require.NotNil(t, delegationsView)
	require.Len(t, delegationsView.BtcDelegations, 1)

	pagination := constructRequestWithLimit(r, 10)
	// Generate the initial query
	req := types.QueryFinalityProviderDelegationsRequest{
		FpBtcPkHex: fp.BtcPk.MarshalHex(),
		Pagination: pagination,
	}

	fpView, err := keeper.FinalityProviderDelegations(ctx, &req)
	require.NoError(t, err)
	require.NotNil(t, fpView)
	require.Len(t, fpView.BtcDelegatorDelegations, 1)
	require.Len(t, fpView.BtcDelegatorDelegations[0].Dels, 1)
	require.True(t, fpView.BtcDelegatorDelegations[0].Dels[0].Active)
}

func FuzzParamsVersions(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))
		k, ctx := testkeeper.BTCStakingKeeper(t, nil, nil, nil)

		qntParams := datagen.RandomInt(r, 120) + 1

		paramsToSet := k.GetParams(ctx)
		for i := uint32(0); i < uint32(qntParams); i++ {
			paramsToSet.BtcActivationHeight += 1 + i
			err := k.SetParams(ctx, paramsToSet)
			require.NoError(t, err)
		}

		limit := (qntParams / 2) + 1
		pagination := constructRequestWithLimit(r, limit)
		req := types.QueryParamsVersionsRequest{
			Pagination: pagination,
		}

		var (
			err  error
			resp *types.QueryParamsVersionsResponse
		)

		paramsFromQuery := make([]types.Params, 0)
		for {
			resp, err = k.ParamsVersions(ctx, &req)
			require.NoError(t, err)
			require.NotNil(t, resp)

			for _, storedParams := range resp.Params {
				paramsFromQuery = append(paramsFromQuery, storedParams.Params)
			}

			if len(resp.Params) != int(limit) || len(resp.Pagination.NextKey) == 0 {
				break
			}

			pagination = constructRequestWithKeyAndLimit(r, resp.Pagination.NextKey, limit)
			req = types.QueryParamsVersionsRequest{Pagination: pagination}
		}

		allParams := k.GetAllParams(ctx)

		require.Equal(t, len(allParams), len(paramsFromQuery))
		for i, paramFromQuery := range paramsFromQuery {
			require.EqualValues(t, *allParams[i], paramFromQuery)
		}
	})
}
