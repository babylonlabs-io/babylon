package keeper_test

import (
	"errors"
	"math/rand"
	"testing"

	"github.com/babylonchain/babylon/testutil/datagen"
	bbn "github.com/babylonchain/babylon/types"
	"github.com/babylonchain/babylon/x/btcstaking/types"
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
		ckptKeeper := types.NewMockCheckpointingKeeper(ctrl)
		h := NewHelper(t, btclcKeeper, btccKeeper, ckptKeeper)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)
		changeAddress, err := datagen.GenRandomBTCAddress(r, h.Net)
		h.NoError(err)

		bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)
		wValue := h.BTCCheckpointKeeper.GetParams(h.Ctx).CheckpointFinalizationTimeout

		// generate and insert new Babylon finality provider
		_, fpPK, _ := h.CreateFinalityProvider(r)
		/*
			ensure that registering a consumer chain finality provider with non-existing
			chain ID will fail
		*/
		_, _, _, err = h.CreateConsumerChainFinalityProvider(r, "non-existing chain ID")
		h.Error(err)

		/*
			register a new consumer chain and create a new finality provider under it
			ensure it's correctly generated
		*/
		chainRegister := datagen.GenRandomChainRegister(r)
		h.BTCStkConsumerKeeper.SetChainRegister(h.Ctx, chainRegister)
		_, czFPPK, czFP, err := h.CreateConsumerChainFinalityProvider(r, chainRegister.ChainId)
		h.NoError(err)
		czFPBTCPK := bbn.NewBIP340PubKeyFromBTCPK(czFPPK)
		czFP2, err := h.BTCStkConsumerKeeper.GetConsumerFinalityProvider(h.Ctx, chainRegister.ChainId, czFPBTCPK)
		h.NoError(err)
		require.Equal(t, czFP, czFP2)

		/*
			ensure BTC delegation request will fail if some fp PK does not exist
		*/
		stakingValue := int64(2 * 10e8)
		_, randomFPPK, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		_, _, _, _, _, err = h.CreateDelegation(
			r,
			[]*btcec.PublicKey{fpPK, randomFPPK},
			changeAddress.EncodeAddress(),
			stakingValue,
			1000,
		)
		h.Error(err)
		require.True(t, errors.Is(err, types.ErrFpNotFound))

		/*
			ensure BTC delegation request will fail if no PK corresponds to a Babylon fp
		*/
		_, _, _, _, _, err = h.CreateDelegation(
			r,
			[]*btcec.PublicKey{czFPPK},
			changeAddress.EncodeAddress(),
			stakingValue,
			1000,
		)
		h.Error(err)
		require.True(t, errors.Is(err, types.ErrNoBabylonFPRestaked), err)

		/*
			happy case -- restaking to a Babylon fp and a consumer chain fp
		*/
		_, _, _, msgBTCDel, actualDel, err := h.CreateDelegation(
			r,
			[]*btcec.PublicKey{fpPK, czFPPK},
			changeAddress.EncodeAddress(),
			stakingValue,
			1000,
		)
		h.NoError(err)

		// add covenant signatures to this restaked BTC delegation
		h.CreateCovenantSigs(r, covenantSKs, msgBTCDel, actualDel)

		// ensure the restaked BTC delegation is bonded right now
		stakingTxHash := actualDel.MustGetStakingTxHash()
		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash.String())
		h.NoError(err)
		btcTip := h.BTCLightClientKeeper.GetTipInfo(h.Ctx).Height
		status := actualDel.GetStatus(btcTip, wValue, bsParams.CovenantQuorum)
		require.Equal(t, types.BTCDelegationStatus_ACTIVE, status)
	})
}

func FuzzFinalityProviderDelegations_RestakingConsumerChains(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		ckptKeeper := types.NewMockCheckpointingKeeper(ctrl)
		h := NewHelper(t, btclcKeeper, btccKeeper, ckptKeeper)

		// set all parameters
		h.GenAndApplyParams(r)
		changeAddress, err := datagen.GenRandomBTCAddress(r, h.Net)
		h.NoError(err)

		// register a new consumer chain
		chainRegister := datagen.GenRandomChainRegister(r)
		h.BTCStkConsumerKeeper.SetChainRegister(h.Ctx, chainRegister)

		// generate and insert new Babylon finality provider
		_, fpPK, fp := h.CreateFinalityProvider(r)

		// generate and insert new consumer chain finality provider
		_, czFPPK, czFP, err := h.CreateConsumerChainFinalityProvider(r, chainRegister.ChainId)
		h.NoError(err)

		// Generate a random number of BTC delegations under this finality provider
		numBTCDels := datagen.RandomInt(r, 10) + 1
		expectedBtcDelsMap := make(map[string]*types.BTCDelegation)
		stakingValue := int64(2 * 10e8)
		for j := uint64(0); j < numBTCDels; j++ {
			_, _, _, _, btcDel, err := h.CreateDelegation(
				r,
				[]*btcec.PublicKey{fpPK, czFPPK},
				changeAddress.EncodeAddress(),
				stakingValue,
				1000,
			)
			h.NoError(err)
			expectedBtcDelsMap[btcDel.BtcPk.MarshalHex()] = btcDel
		}

		// Test nil request
		resp, err := h.BTCStakingKeeper.FinalityProviderDelegations(h.Ctx, nil)
		require.Nil(t, resp)
		require.Error(t, err)

		/*
			Test BTC delegator delegations under the Babylon finality provider
			or the consumer chain finality provider
		*/

		// Generate a page request with a limit and a nil key
		// query a page of BTC delegations and assert consistency
		limit := datagen.RandomInt(r, len(expectedBtcDelsMap)) + 1
		pagination := constructRequestWithLimit(r, limit)

		// the tested finality provider is under Babylon or consumer chain
		testedFP := fp
		if datagen.OneInN(r, 2) {
			testedFP = czFP
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
				require.Equal(t, fp.BtcPk, &btcDel.FpBtcPkList[0])   // Babylon finality provider
				require.Equal(t, czFP.BtcPk, &btcDel.FpBtcPkList[1]) // consumer chain finality provider
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
