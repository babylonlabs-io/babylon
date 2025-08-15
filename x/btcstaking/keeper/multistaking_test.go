package keeper_test

import (
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	testutil "github.com/babylonlabs-io/babylon/v4/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
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
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyCustomParams(r, 100, 200, 0, 10)

		bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

		// generate and insert new Babylon finality provider
		_, fpPK, _ := h.CreateFinalityProvider(r)
		_, fpPK1, _ := h.CreateFinalityProvider(r)

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

		_, consumerFPPK1, _, err := h.CreateConsumerFinalityProvider(r, consumerRegister.ConsumerId)
		h.NoError(err)

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
		require.True(t, errors.Is(err, types.ErrNoBabylonFPMultiStaked), err)

		/*
			ensure BTC delegation request will fail if more than one Babylon fp is selected
		*/

		_, _, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{fpPK, fpPK1, consumerFPPK},
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
		require.True(t, errors.Is(err, types.ErrInvalidMultiStakingFPs), err)

		/*
			ensure BTC delegation request will fail if more than one consumer fp is selected
		*/

		_, _, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{fpPK, consumerFPPK, consumerFPPK1},
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
		require.True(t, errors.Is(err, types.ErrInvalidMultiStakingFPs), err)

		/*
			during multi-staking allow-list -- try multi-staking to a Babylon fp and a consumer fp but not allowed
		*/
		lcTip := uint32(30)
		_, _, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
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
			lcTip,
		)
		h.Error(err)
		h.ErrorContains(err, "it is not allowed to create new delegations with multi-staking during the multi-staking allow-list period")

		/*
			happy case -- multi-staking to a Babylon fp and a consumer fp
		*/
		heightAfterMultiStakingAllowListExpiration := int64(10)
		h = h.WithBlockHeight(heightAfterMultiStakingAllowListExpiration)
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
			lcTip,
		)
		h.NoError(err)

		// add covenant signatures to this multi-staked BTC delegation
		h.CreateCovenantSigs(r, covenantSKs, msgBTCDel, actualDel, 10)

		// ensure the multi-staked BTC delegation is bonded right now
		stakingTxHash := actualDel.MustGetStakingTxHash()
		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash.String())
		h.NoError(err)
		status, err := h.BTCStakingKeeper.BtcDelStatus(h.Ctx, actualDel, bsParams.CovenantQuorum, lcTip)
		h.NoError(err)
		require.Equal(t, types.BTCDelegationStatus_ACTIVE, status)
	})
}

func TestMultiStakingAllowList(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mock BTC light client and BTC checkpoint modules
	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
	h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

	// set all parameters
	covenantSKs, _ := h.GenAndApplyCustomParams(r, 100, 200, 0, 10)

	// Create a Babylon finality provider
	_, babylonFPPK, _ := h.CreateFinalityProvider(r)

	// Register a consumer chain and create consumer finality provider
	consumerRegister := datagen.GenRandomCosmosConsumerRegister(r)
	err := h.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, consumerRegister)
	require.NoError(t, err)
	_, consumerFPPK, _, err := h.CreateConsumerFinalityProvider(r, consumerRegister.ConsumerId)
	h.NoError(err)

	// Multi-staking FP list: one Babylon FP + one consumer FP
	fpPKs := []*btcec.PublicKey{babylonFPPK, consumerFPPK}

	// Create a staker for the previous staking transaction
	stakingValue := int64(2 * 10e8)
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	h.NoError(err)

	// Create the previous staking transaction that will be in the allow list
	// This needs to be a single FP delegation first
	lcTip := uint32(30)

	// Try to create a new multi-staking delegation
	// This should not be allowed during multi-staking allow-list period
	_, _, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK,
		fpPKs,
		stakingValue,
		1000,
		0,
		0,
		false,
		true,
		10,
		lcTip,
	)
	h.Error(err)
	h.ErrorContains(err, "it is not allowed to create new delegations with multi-staking during the multi-staking allow-list period")

	// Create the previous staking transaction that will be in the allow list
	// This needs to be a single FP delegation first
	prevStakingTxHash, prevMsgCreateBTCDel, prevDel, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK,
		[]*btcec.PublicKey{babylonFPPK}, // single Babylon FP for the original delegation
		stakingValue,
		1000,
		0,
		0,
		false,
		true,
		10,
		lcTip,
	)
	h.NoError(err)
	require.NotNil(t, prevMsgCreateBTCDel)

	// Add covenant signatures to make it active
	h.CreateCovenantSigs(r, covenantSKs, prevMsgCreateBTCDel, prevDel, 10)

	// Test 1: Create multi-staking BtcStakeExpand with txHash NOT in multi-staking allow list
	_, _, err = h.CreateBtcStakeExpansionWithBtcTipHeight(
		r,
		delSK,
		fpPKs,
		stakingValue,
		1000,
		prevDel,
		lcTip,
	)
	h.Error(err)
	h.ErrorContains(err, "not eligible for multi-staking")

	// Add the previous staking tx hash to the allow list
	prevDelTxHash, err := chainhash.NewHashFromStr(prevStakingTxHash)
	h.NoError(err)
	h.BTCStakingKeeper.IndexAllowedMultiStakingTransaction(h.Ctx, prevDelTxHash)

	// Test 2: Try to create BtcStakeExpand with prevDelTxHash in allow list
	// and increasing staked amount - should not be allowed
	_, _, err = h.CreateBtcStakeExpansionWithBtcTipHeight(
		r,
		delSK,
		fpPKs,
		stakingValue+1, // increase the staking amount
		1000,
		prevDel,
		lcTip,
	)
	h.Error(err)
	h.ErrorContains(err, "it is not allowed to modify the staking amount during the multi-staking allow-list period")

	// Test 3: Create BtcStakeExpand with prevDelTxHash in allow list
	// Create the multi-staking delegation via stake expansion
	spendingTx, fundingTx, err := h.CreateBtcStakeExpansionWithBtcTipHeight(
		r,
		delSK,
		fpPKs,
		stakingValue,
		1000,
		prevDel,
		lcTip,
	)
	require.NoError(t, err)

	bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

	expandedDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, spendingTx.TxHash().String())
	require.NoError(t, err)
	require.True(t, expandedDel.IsStakeExpansion())
	status, err := h.BTCStakingKeeper.BtcDelStatus(h.Ctx, expandedDel, bsParams.CovenantQuorum, lcTip)
	h.NoError(err)
	require.Equal(t, types.BTCDelegationStatus_PENDING, status)

	// Add covenant signatures to make it verified
	h.CreateCovenantSigs(r, covenantSKs, nil, expandedDel, 10)

	// Add witness for stake expansion tx
	prevStkTx, err := bbn.NewBTCTxFromBytes(prevDel.GetStakingTx())
	require.NoError(t, err)

	spendingTxWithWitnessBz, _ := datagen.AddWitnessToStakeExpTx(
		t,
		prevStkTx.TxOut[0],
		fundingTx.TxOut[0],
		delSK,
		covenantSKs,
		bsParams.CovenantQuorum,
		[]*btcec.PublicKey{babylonFPPK},
		uint16(1000),
		stakingValue,
		spendingTx,
		h.Net,
	)

	// build the block with the proofs
	expansionTxInclusionProof := h.BuildBTCInclusionProofForSpendingTx(r, spendingTx, lcTip)

	// Submit MsgBTCUndelegate for the original delegation to activate stake expansion
	fundingTxBz, err := bbn.SerializeBTCTx(fundingTx)
	h.NoError(err)
	msg := &types.MsgBTCUndelegate{
		Signer:                        prevDel.StakerAddr,
		StakingTxHash:                 prevStkTx.TxHash().String(),
		StakeSpendingTx:               spendingTxWithWitnessBz,
		StakeSpendingTxInclusionProof: expansionTxInclusionProof,
		FundingTransactions:           [][]byte{prevDel.GetStakingTx(), fundingTxBz},
	}
	// Ensure BTC tip is enough for the undelegate
	// Spending tx should be above BTC confirmation depth (k = 10)
	lcTip += 11
	h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: lcTip}).AnyTimes()
	_, err = h.MsgServer.BTCUndelegate(h.Ctx, msg)
	h.NoError(err)

	// Ensure the expanded delegation is active
	expandedDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, spendingTx.TxHash().String())
	require.NoError(t, err)

	status, err = h.BTCStakingKeeper.BtcDelStatus(h.Ctx, expandedDel, bsParams.CovenantQuorum, lcTip)
	h.NoError(err)
	require.Equal(t, types.BTCDelegationStatus_ACTIVE, status)

	// Test 4: Extend a multi-staking delegation with txHash NOT in allow list
	// (but original txHash was in multi-staking allow-list)
	// Register a new consumer chain and add the FP to the new delegation expansion
	consumerRegister2 := datagen.GenRandomCosmosConsumerRegister(r)
	err = h.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, consumerRegister2)
	require.NoError(t, err)
	_, consumer2FPPK, _, err := h.CreateConsumerFinalityProvider(r, consumerRegister2.ConsumerId)
	h.NoError(err)

	// Try to increase staking amt - should not be allowed
	_, _, err = h.CreateBtcStakeExpansionWithBtcTipHeight(
		r,
		delSK,
		append(fpPKs, consumer2FPPK),
		stakingValue+1,
		1000,
		expandedDel,
		lcTip,
	)
	h.Error(err)
	h.ErrorContains(err, "it is not allowed to modify the staking amount during the multi-staking allow-list period")

	// Submit the BtcStakeExpand message adding the new consumer FP
	// and keeping same staking amount
	doubleExpStakingTx, _, err := h.CreateBtcStakeExpansionWithBtcTipHeight(
		r,
		delSK,
		append(fpPKs, consumer2FPPK),
		stakingValue,
		1000,
		expandedDel,
		lcTip,
	)
	h.NoError(err)

	// Ensure the new expansion is in pending state
	doubleExpandedDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, doubleExpStakingTx.TxHash().String())
	require.NoError(t, err)
	require.True(t, expandedDel.IsStakeExpansion())
	status, err = h.BTCStakingKeeper.BtcDelStatus(h.Ctx, doubleExpandedDel, bsParams.CovenantQuorum, lcTip)
	h.NoError(err)
	require.Equal(t, types.BTCDelegationStatus_PENDING, status)
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
		heightAfterMultiStakingAllowListExpiration := int64(10)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil).WithBlockHeight(heightAfterMultiStakingAllowListExpiration)

		// set all parameters
		h.GenAndApplyCustomParams(r, 100, 200, 0, 2)

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
	heightAfterMultiStakingAllowListExpiration := int64(10)
	h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil).WithBlockHeight(heightAfterMultiStakingAllowListExpiration)

	// set all parameters
	covenantSKs, _ := h.GenAndApplyCustomParams(r, 100, 200, 0, 2)

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
