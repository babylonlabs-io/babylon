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

		// set all parameters (Babylon defaults to MaxMultiStakedFps=2)
		covenantSKs, _ := h.GenAndApplyParams(r)
		bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

		// generate and insert new Babylon finality provider
		_, fpPK, _ := h.CreateFinalityProvider(r)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)

		/*
			Test basic consumer functionality
		*/
		// Should fail: registering FP with non-existing consumer
		_, _, _, err = h.CreateConsumerFinalityProvider(r, "non-existing-consumer")
		h.Error(err)

		// Register a consumer with limit = 5 (higher than Babylon's 2)
		consumer := datagen.GenRandomCosmosConsumerRegister(r)
		consumer.ConsumerMaxMultiStakedFps = 5
		err = h.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, consumer)
		require.NoError(t, err)
		_, consumerFPPK, _, err := h.CreateConsumerFinalityProvider(r, consumer.ConsumerId)
		h.NoError(err)

		stakingValue := int64(2 * 10e8)

		/*
			Test Babylon's enforcement (effective limit = min(2, 5) = 2)
		*/
		// ✅ PASS: 2 FPs (1 Babylon + 1 consumer) = 2 ≤ 2
		_, msgBTCDel, actualDel, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
			r, delSK, []*btcec.PublicKey{fpPK, consumerFPPK}, stakingValue,
			1000, 0, 0, false, false, 10, 30,
		)
		h.NoError(err)

		// ❌ FAIL: Multiple Babylon FPs not allowed
		_, fpPK2, _ := h.CreateFinalityProvider(r)
		_, _, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
			r, delSK, []*btcec.PublicKey{fpPK, fpPK2}, stakingValue,
			1000, 0, 0, false, false, 10, 30,
		)
		h.Error(err)
		require.ErrorIs(t, err, types.ErrTooManyBabylonFPs)

		// Complete the successful delegation
		h.CreateCovenantSigs(r, covenantSKs, msgBTCDel, actualDel, 10)
		stakingTxHash := actualDel.MustGetStakingTxHash()
		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash.String())
		h.NoError(err)
		btcTip := h.BTCLightClientKeeper.GetTipInfo(h.Ctx).Height
		status := actualDel.GetStatus(btcTip, bsParams.CovenantQuorum)
		require.Equal(t, types.BTCDelegationStatus_ACTIVE, status)
	})
}

func FuzzRestaking_BabylonParamValidation(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		h.GenAndApplyParams(r)

		// Create Babylon FP and delegator
		_, fpPK, _ := h.CreateFinalityProvider(r)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)

		// Register consumers with HIGH limits (won't be the bottleneck)
		consumer1 := datagen.GenRandomCosmosConsumerRegister(r)
		consumer1.ConsumerMaxMultiStakedFps = 10
		err = h.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, consumer1)
		require.NoError(t, err)
		_, consumerFPPK1, _, err := h.CreateConsumerFinalityProvider(r, consumer1.ConsumerId)
		h.NoError(err)

		consumer2 := datagen.GenRandomCosmosConsumerRegister(r)
		consumer2.ConsumerMaxMultiStakedFps = 10
		err = h.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, consumer2)
		require.NoError(t, err)
		_, consumerFPPK2, _, err := h.CreateConsumerFinalityProvider(r, consumer2.ConsumerId)
		h.NoError(err)

		stakingValue := int64(2 * 10e8)

		/*
			Test 1: Babylon limit = 2 (default)
			Effective limit = min(2, 10, 10) = 2
		*/
		babylonParams := h.BTCStkConsumerKeeper.GetParams(h.Ctx)
		require.Equal(t, uint32(2), babylonParams.MaxMultiStakedFps)

		// ❌ FAIL: 3 FPs > Babylon's limit of 2
		_, _, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
			r, delSK, []*btcec.PublicKey{fpPK, consumerFPPK1, consumerFPPK2}, stakingValue,
			1000, 0, 0, false, false, 10, 30,
		)
		h.Error(err)
		require.ErrorIs(t, err, types.ErrTooManyFPs)

		/*
			Test 2: Increase Babylon limit to 4
			Effective limit = min(4, 10, 10) = 4
		*/
		err = h.BTCStkConsumerKeeper.SetParams(h.Ctx, btcstkconsumertypes.Params{
			PermissionedIntegration: false,
			MaxMultiStakedFps:       4,
		})
		require.NoError(t, err)

		// ✅ PASS: 3 FPs ≤ Babylon's new limit of 4
		_, _, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
			r, delSK, []*btcec.PublicKey{fpPK, consumerFPPK1, consumerFPPK2}, stakingValue,
			1000, 0, 0, false, false, 10, 30,
		)
		h.NoError(err)

		/*
			Test 3: Reduce Babylon limit to 2 again
			Effective limit = min(2, 10, 10) = 2
		*/
		err = h.BTCStkConsumerKeeper.SetParams(h.Ctx, btcstkconsumertypes.Params{
			PermissionedIntegration: false,
			MaxMultiStakedFps:       2,
		})
		require.NoError(t, err)

		// ❌ FAIL: 3 FPs > Babylon's reduced limit of 2
		_, _, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
			r, delSK, []*btcec.PublicKey{fpPK, consumerFPPK1, consumerFPPK2}, stakingValue,
			1000, 0, 0, false, false, 10, 30,
		)
		h.Error(err)
		require.ErrorIs(t, err, types.ErrTooManyFPs)
	})
}

func FuzzRestaking_ConsumerParamValidation(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		h.GenAndApplyParams(r)

		// Set Babylon limit HIGH so it won't be the bottleneck
		err := h.BTCStkConsumerKeeper.SetParams(h.Ctx, btcstkconsumertypes.Params{
			PermissionedIntegration: false,
			MaxMultiStakedFps:       10,
		})
		require.NoError(t, err)

		// Create Babylon FP and delegator
		_, fpPK, _ := h.CreateFinalityProvider(r)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)

		// Register consumers with RESTRICTIVE limits
		restrictiveConsumer := datagen.GenRandomCosmosConsumerRegister(r)
		restrictiveConsumer.ConsumerMaxMultiStakedFps = 2 // This will be the bottleneck
		err = h.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, restrictiveConsumer)
		require.NoError(t, err)
		_, restrictiveFPPK, _, err := h.CreateConsumerFinalityProvider(r, restrictiveConsumer.ConsumerId)
		h.NoError(err)

		permissiveConsumer := datagen.GenRandomCosmosConsumerRegister(r)
		permissiveConsumer.ConsumerMaxMultiStakedFps = 5
		err = h.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, permissiveConsumer)
		require.NoError(t, err)
		_, permissiveFPPK, _, err := h.CreateConsumerFinalityProvider(r, permissiveConsumer.ConsumerId)
		h.NoError(err)

		stakingValue := int64(2 * 10e8)

		/*
			Test consumer limit enforcement
			Effective limit = min(10, 2, 5) = 2 (restrictive consumer is bottleneck)
		*/

		// ✅ PASS: 2 FPs = restrictive consumer's limit
		_, _, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
			r, delSK, []*btcec.PublicKey{fpPK, restrictiveFPPK}, stakingValue,
			1000, 0, 0, false, false, 10, 30,
		)
		h.NoError(err)

		// ❌ FAIL: 3 FPs > restrictive consumer's limit of 2
		_, _, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
			r, delSK, []*btcec.PublicKey{fpPK, restrictiveFPPK, permissiveFPPK}, stakingValue,
			1000, 0, 0, false, false, 10, 30,
		)
		h.Error(err)
		require.ErrorIs(t, err, types.ErrTooManyFPs)

		// ❌ FAIL: Multiple FPs from same consumer not allowed
		_, restrictiveFPPK2, _, err := h.CreateConsumerFinalityProvider(r, restrictiveConsumer.ConsumerId)
		h.NoError(err)
		_, _, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
			r, delSK, []*btcec.PublicKey{fpPK, restrictiveFPPK, restrictiveFPPK2}, stakingValue,
			1000, 0, 0, false, false, 10, 30,
		)
		h.Error(err)
		require.ErrorIs(t, err, types.ErrTooManyFPsFromSameConsumer)
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
