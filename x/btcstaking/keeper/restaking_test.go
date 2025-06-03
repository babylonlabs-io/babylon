package keeper_test

import (
	"math/rand"
	"testing"

	testutil "github.com/babylonlabs-io/babylon/v4/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	btcstkconsumertypes "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func FuzzRestaking_RestakedBTCDelegation(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)

		bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

		// generate and insert new Babylon finality provider
		_, fpPK, _ := h.CreateFinalityProvider(r)

		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)

		/*
			ensure that registering a consumer finality provider with non-existing
			consumer ID will fail
		*/
		_, _, _, err = h.CreateConsumerFinalityProvider(r, "non-existing chain ID")
		h.Error(err)

		/*
			register multiple consumers with different max_multi_staked_fps values
			and create finality providers under them
		*/
		// Consumer 1 with max_multi_staked_fps = 2
		consumerRegister1 := datagen.GenRandomCosmosConsumerRegister(r)
		consumerRegister1.MaxMultiStakedFps = 2
		err = h.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, consumerRegister1)
		require.NoError(t, err)
		_, consumerFPPK1, _, err := h.CreateConsumerFinalityProvider(r, consumerRegister1.ConsumerId)
		h.NoError(err)

		// Consumer 2 with max_multi_staked_fps = 3
		consumerRegister2 := datagen.GenRandomCosmosConsumerRegister(r)
		consumerRegister2.MaxMultiStakedFps = 3
		err = h.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, consumerRegister2)
		require.NoError(t, err)
		_, consumerFPPK2, _, err := h.CreateConsumerFinalityProvider(r, consumerRegister2.ConsumerId)
		h.NoError(err)

		// Consumer 3 with max_multi_staked_fps = 4
		consumerRegister3 := datagen.GenRandomCosmosConsumerRegister(r)
		consumerRegister3.MaxMultiStakedFps = 4
		err = h.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, consumerRegister3)
		require.NoError(t, err)
		_, consumerFPPK3, _, err := h.CreateConsumerFinalityProvider(r, consumerRegister3.ConsumerId)
		h.NoError(err)

		stakingValue := int64(2 * 10e8)

		/*
			Test multiple consumers with different max_multi_staked_fps values
		*/
		// Test case 1: Invalid delegation with 1 Babylon FP and 1 FP from each consumer (total 4 FPs)
		// This should fail because min(max_multi_staked_fps) = 2, but we're trying to use 4 FPs
		_, _, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{fpPK, consumerFPPK1, consumerFPPK2, consumerFPPK3},
			stakingValue,
			1000,
			0,
			0,
			false,
			false,
			10,
			30,
		)
		h.Error(err)
		require.ErrorIs(t, err, types.ErrTooManyFPs)

		// Test case 2: Valid delegation with 1 Babylon FP and 1 FP from consumer1 (total 2 FPs)
		// This should succeed because it's within the minimum max_multi_staked_fps (2)
		_, msgBTCDel, actualDel, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{fpPK, consumerFPPK1},
			stakingValue,
			1000,
			0,
			0,
			false,
			false,
			10,
			30,
		)
		h.NoError(err)

		// Test case 3: Valid delegation with 1 Babylon FP and 1 FP from consumer2 (total 2 FPs)
		// This should succeed because it's within the minimum max_multi_staked_fps (2)
		_, _, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{fpPK, consumerFPPK2},
			stakingValue,
			1000,
			0,
			0,
			false,
			false,
			10,
			30,
		)
		h.NoError(err)

		// Test case 4: Invalid delegation with 1 Babylon FP and 2 FPs from consumer1 (total 3 FPs)
		// This should fail because it exceeds the minimum max_multi_staked_fps (2)
		_, consumerFPPK1_2, _, err := h.CreateConsumerFinalityProvider(r, consumerRegister1.ConsumerId)
		h.NoError(err)
		_, _, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{fpPK, consumerFPPK1, consumerFPPK1_2},
			stakingValue,
			1000,
			0,
			0,
			false,
			false,
			10,
			30,
		)
		h.Error(err)
		require.ErrorIs(t, err, types.ErrTooManyFPsFromSameConsumer)

		// Test case 5: Invalid delegation with 2 Babylon FPs (should fail with ErrTooManyBabylonFPs)
		// Create a second Babylon finality provider
		_, fpPK2, _ := h.CreateFinalityProvider(r)
		_, _, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{fpPK, fpPK2}, // 2 Babylon FPs
			stakingValue,
			1000,
			0,
			0,
			false,
			false,
			10,
			30,
		)
		h.Error(err)
		require.ErrorIs(t, err, types.ErrTooManyBabylonFPs)

		/*
			Test Babylon's max_multi_staked_fps parameter with values >= 2
		*/
		// Test with Babylon's limit set to 4
		err = h.BTCStkConsumerKeeper.SetParams(h.Ctx, btcstkconsumertypes.Params{
			PermissionedIntegration: false,
			MaxMultiStakedFps:       4, // Babylon allows max 4 FPs per delegation
		})
		require.NoError(t, err)

		// Verify Babylon's limit is set correctly
		babylonParams := h.BTCStkConsumerKeeper.GetParams(h.Ctx)
		require.Equal(t, uint32(4), babylonParams.MaxMultiStakedFps)

		// Test case 6: Valid delegation with 4 FPs (1 Babylon + 3 consumer)
		// This should succeed because it equals Babylon's limit of 4
		_, _, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{fpPK, consumerFPPK1, consumerFPPK2, consumerFPPK3},
			stakingValue,
			1000,
			0,
			0,
			false,
			false,
			10,
			30,
		)
		h.NoError(err)

		// Test case 7: Invalid delegation with 5 FPs (1 Babylon + 4 consumer)
		// This should fail because it exceeds Babylon's limit of 4
		_, consumerFPPK1_2, _, err = h.CreateConsumerFinalityProvider(r, consumerRegister1.ConsumerId)
		h.NoError(err)
		_, _, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{fpPK, consumerFPPK1, consumerFPPK1_2, consumerFPPK2, consumerFPPK3},
			stakingValue,
			1000,
			0,
			0,
			false,
			false,
			10,
			30,
		)
		h.Error(err)
		require.ErrorIs(t, err, types.ErrTooManyFPs)

		// Test case 8: Test that Babylon's limit takes precedence when changed
		// Set Babylon's limit to 3 (more restrictive than some consumer limits)
		err = h.BTCStkConsumerKeeper.SetParams(h.Ctx, btcstkconsumertypes.Params{
			PermissionedIntegration: false,
			MaxMultiStakedFps:       3, // Babylon now allows max 3 FPs per delegation
		})
		require.NoError(t, err)

		// Test case 9: Invalid delegation with 4 FPs that was valid before
		// This should now fail because Babylon's new limit is 3
		_, _, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{fpPK, consumerFPPK1, consumerFPPK2, consumerFPPK3},
			stakingValue,
			1000,
			0,
			0,
			false,
			false,
			10,
			30,
		)
		h.Error(err)
		require.ErrorIs(t, err, types.ErrTooManyFPs)

		/*
			happy case -- restaking to a Babylon fp and a consumer fp
		*/
		// add covenant signatures to this restaked BTC delegation
		h.CreateCovenantSigs(r, covenantSKs, msgBTCDel, actualDel, 10)

		// ensure the restaked BTC delegation is bonded right now
		stakingTxHash := actualDel.MustGetStakingTxHash()
		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash.String())
		h.NoError(err)
		btcTip := h.BTCLightClientKeeper.GetTipInfo(h.Ctx).Height
		status := actualDel.GetStatus(btcTip, bsParams.CovenantQuorum)
		require.Equal(t, types.BTCDelegationStatus_ACTIVE, status)
	})
}

func FuzzFinalityProviderDelegations_RestakingConsumers(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		// set all parameters
		h.GenAndApplyParams(r)

		// register a new consumer
		consumerRegister := datagen.GenRandomCosmosConsumerRegister(r)
		err := h.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, consumerRegister)
		require.NoError(t, err)

		// generate and insert new Babylon finality provider
		_, fpPK, fp := h.CreateFinalityProvider(r)

		// generate and insert new consumer finality provider
		_, consumerFPPK, consumerFP, err := h.CreateConsumerFinalityProvider(r, consumerRegister.ConsumerId)
		h.NoError(err)

		// Generate a random number of BTC delegations under this finality provider
		numBTCDels := datagen.RandomInt(r, 10) + 1
		expectedBtcDelsMap := make(map[string]*types.BTCDelegation)
		stakingValue := int64(2 * 10e8)
		for j := uint64(0); j < numBTCDels; j++ {
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			h.NoError(err)
			_, _, btcDel, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
				r,
				delSK,
				[]*btcec.PublicKey{fpPK, consumerFPPK},
				stakingValue,
				1000,
				0,
				0,
				false,
				false,
				10,
				30,
			)
			h.NoError(err)
			expectedBtcDelsMap[btcDel.BtcPk.MarshalHex()] = btcDel
		}

		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: 30}).AnyTimes()

		// Test nil request
		resp, err := h.BTCStakingKeeper.FinalityProviderDelegations(h.Ctx, nil)
		require.Nil(t, resp)
		require.Error(t, err)

		/*
			Test BTC delegator delegations under the Babylon finality provider
			or the consumer finality provider
		*/

		// Generate a page request with a limit and a nil key
		// query a page of BTC delegations and assert consistency
		limit := datagen.RandomInt(r, len(expectedBtcDelsMap)) + 1
		pagination := constructRequestWithLimit(r, limit)

		// the tested finality provider is under Babylon or consumer
		testedFP := fp
		if datagen.OneInN(r, 2) {
			testedFP = consumerFP
		}

		// Generate the initial query
		req := types.QueryFinalityProviderDelegationsRequest{
			FpBtcPkHex: testedFP.BtcPk.MarshalHex(),
			Pagination: pagination,
		}
		// Construct a mapping from the finality providers found to a boolean value
		// Will be used later to evaluate whether all the finality providers were returned
		btcDelsFound := make(map[string]bool, 0)
		for i := uint64(0); i < numBTCDels; i += limit {
			resp, err = h.BTCStakingKeeper.FinalityProviderDelegations(h.Ctx, &req)
			require.NoError(t, err)
			require.NotNil(t, resp)
			for _, btcDels := range resp.BtcDelegatorDelegations {
				require.Len(t, btcDels.Dels, 1)
				btcDel := btcDels.Dels[0]
				require.Len(t, btcDel.FpBtcPkList, 2)
				require.Equal(t, fp.BtcPk, &btcDel.FpBtcPkList[0])         // Babylon finality provider
				require.Equal(t, consumerFP.BtcPk, &btcDel.FpBtcPkList[1]) // consumer finality provider
				// Check if the pk exists in the map
				_, ok := expectedBtcDelsMap[btcDel.BtcPk.MarshalHex()]
				require.True(t, ok)
				btcDelsFound[btcDel.BtcPk.MarshalHex()] = true
			}
			// Construct the next page request
			pagination = constructRequestWithKeyAndLimit(r, resp.Pagination.NextKey, limit)
			req = types.QueryFinalityProviderDelegationsRequest{
				FpBtcPkHex: testedFP.BtcPk.MarshalHex(),
				Pagination: pagination,
			}
		}
		require.Equal(t, len(btcDelsFound), len(expectedBtcDelsMap))
	})
}
