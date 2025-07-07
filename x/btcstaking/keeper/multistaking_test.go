package keeper_test

import (
	"errors"
	"math/rand"
	"testing"
	"time"

	testutil "github.com/babylonlabs-io/babylon/v3/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func FuzzMultiStaking_MultiStakedBTCDelegation(f *testing.F) {
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
			register a new consumer and create a new finality provider under it
			ensure it's correctly generated
		*/
		consumerRegister := datagen.GenRandomCosmosConsumerRegister(r)
		err = h.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, consumerRegister)
		require.NoError(t, err)
		_, consumerFPPK, consumerFP, err := h.CreateConsumerFinalityProvider(r, consumerRegister.ConsumerId)
		h.NoError(err)
		consumerFPBTCPK := bbn.NewBIP340PubKeyFromBTCPK(consumerFPPK)
		consumerFP2, err := h.BTCStakingKeeper.GetFinalityProvider(h.Ctx, consumerFPBTCPK.MustMarshal())
		h.NoError(err)
		// on finality provider creation, the commission update time is set to the
		// current block time. The consumerFP is randomly generated with update time = 0,
		// so we need to update it to the block time to make it equal
		consumerFP.CommissionInfo.UpdateTime = h.Ctx.BlockTime().UTC()
		require.Equal(t, consumerFP, consumerFP2)

		/*
			ensure BTC delegation request will fail if some fp PK does not exist
		*/
		stakingValue := int64(2 * 10e8)
		_, randomFPPK, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		_, _, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{fpPK, randomFPPK},
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
		require.True(t, errors.Is(err, types.ErrFpNotFound))

		/*
			ensure BTC delegation request will fail if no PK corresponds to a Babylon fp
		*/
		_, _, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{consumerFPPK},
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
		require.True(t, errors.Is(err, types.ErrNoBabylonFPRestaked), err)

		/*
			happy case -- multi-staking to a Babylon fp and a consumer fp
		*/

		_, msgBTCDel, actualDel, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
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

		// add covenant signatures to this multi-staked BTC delegation
		h.CreateCovenantSigs(r, covenantSKs, msgBTCDel, actualDel, 10)

		// ensure the multi-staked BTC delegation is bonded right now
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

func TestNoActivationEventForRollupConsumer(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mock BTC light client and BTC checkpoint modules
	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
	h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

	// set all parameters
	covenantSKs, _ := h.GenAndApplyParams(r)

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
		register a new consumer and create a new finality provider under it
		ensure it's correctly generated
	*/
	randomAddress := datagen.GenRandomAddress()
	consumerRegister := datagen.GenRandomRollupRegister(r, randomAddress.String())
	err = h.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, consumerRegister)
	require.NoError(t, err)
	_, consumerFPPK, consumerFP, err := h.CreateConsumerFinalityProvider(r, consumerRegister.ConsumerId)
	h.NoError(err)

	// on finality provider creation, the commission update time is set to the
	// current block time. The consumerFP is randomly generated with update time = 0,
	// so we need to update it to the block time to make it equal
	consumerFP.CommissionInfo.UpdateTime = h.Ctx.BlockTime().UTC()
	stakingValue := int64(2 * 10e8)

	_, msgBTCDel, actualDel, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
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

	// add covenant signatures to this multi-staked BTC delegation
	h.CreateCovenantSigs(r, covenantSKs, msgBTCDel, actualDel, 10)

	// ensure the multi-staked BTC delegation is bonded right now
	stakingTxHash := actualDel.MustGetStakingTxHash()
	actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash.String())
	h.NoError(err)
	require.NotNil(t, actualDel)

	// No activation packets should be present for rollup consumers
	ibcPackets := h.BTCStakingKeeper.GetAllBTCStakingConsumerIBCPackets(h.Ctx)
	require.Empty(t, ibcPackets)
}
