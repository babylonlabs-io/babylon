package keeper_test

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	sdkmath "cosmossdk.io/math"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	testkeeper "github.com/babylonlabs-io/babylon/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/types"
	btcctypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	btcstakingkeeper "github.com/babylonlabs-io/babylon/x/btcstaking/keeper"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
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

			AddFinalityProvider(t, ctx, *keeper, fp)
			fpsMap[fp.BtcPk.MarshalHex()] = fp
		}
		numOfFpsInStore := len(fpsMap)

		// Test nil request
		resp, err := keeper.FinalityProviders(ctx, nil)
		if resp != nil {
			t.Errorf("Nil input led to a non-nil response")
		}
		if err == nil {
			t.Errorf("Nil input led to a nil error")
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
		Commission:  fp.Commission,
		BtcPk:       fp.BtcPk,
		Pop:         fp.Pop,
	})
	require.NoError(t, err)
}
