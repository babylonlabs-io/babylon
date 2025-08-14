package types_test

import (
	"math/rand"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/stretchr/testify/require"

	bbn "github.com/babylonlabs-io/babylon/v4/types"

	asig "github.com/babylonlabs-io/babylon/v4/crypto/schnorr-adaptor-signature"
	btctest "github.com/babylonlabs-io/babylon/v4/testutil/bitcoin"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

func FuzzBTCDelegation(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 100)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		unbondingTime := uint32(datagen.RandomInt(r, 50))
		btcDel := &types.BTCDelegation{}
		// randomise voting power
		btcDel.TotalSat = datagen.RandomInt(r, 100000)
		btcDel.BtcUndelegation = &types.BTCUndelegation{}
		btcDel.UnbondingTime = unbondingTime
		// randomise covenant sig
		hasCovenantSig := datagen.RandomInt(r, 2) == 0
		if hasCovenantSig {
			encKey, _, err := asig.GenKeyPair()
			require.NoError(t, err)
			covenantSK, _ := btcec.PrivKeyFromBytes(
				[]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			)
			covenantSig, err := asig.EncSign(covenantSK, encKey, datagen.GenRandomByteArray(r, 32))
			require.NoError(t, err)
			covPk, err := datagen.GenRandomBIP340PubKey(r)
			require.NoError(t, err)
			covSigInfo := &types.CovenantAdaptorSignatures{
				CovPk:       covPk,
				AdaptorSigs: [][]byte{covenantSig.MustMarshal()},
			}
			btcDel.CovenantSigs = []*types.CovenantAdaptorSignatures{covSigInfo}
			btcDel.BtcUndelegation.CovenantSlashingSigs = btcDel.CovenantSigs                                // doesn't matter
			btcDel.BtcUndelegation.CovenantUnbondingSigList = []*types.SignatureInfo{&types.SignatureInfo{}} // doesn't matter
		}

		// randomise start height and end height
		btcDel.StartHeight = uint32(datagen.RandomInt(r, 100)) + 1
		btcDel.EndHeight = btcDel.StartHeight + uint32(datagen.RandomInt(r, 100)) + 1

		// randomise BTC tip and w
		btcHeight := btcDel.StartHeight + uint32(datagen.RandomInt(r, 50))

		// test expected voting power
		hasVotingPower := hasCovenantSig && btcDel.StartHeight <= btcHeight && btcHeight+unbondingTime < btcDel.EndHeight
		actualVotingPower := btcDel.VotingPower(btcHeight, 1, 0)
		if hasVotingPower {
			require.Equal(t, btcDel.TotalSat, actualVotingPower)
		} else {
			require.Equal(t, uint64(0), actualVotingPower)
		}
	})
}

func FuzzBTCDelegation_SlashingTx(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		net := &chaincfg.SimNetParams

		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)

		// multi-staked to a random number of finality providers
		numMultiStakedFPs := int(datagen.RandomInt(r, 10) + 1)
		fpSKs, fpPKs, err := datagen.GenRandomBTCKeyPairs(r, numMultiStakedFPs)
		require.NoError(t, err)
		fpBTCPKs := bbn.NewBIP340PKsFromBTCPKs(fpPKs)

		// a random finality provider gets slashed
		slashedFPIdx := int(datagen.RandomInt(r, numMultiStakedFPs))
		fpSK := fpSKs[slashedFPIdx]

		// (3, 5) covenant committee
		covenantSKs, covenantPKs, err := datagen.GenRandomBTCKeyPairs(r, 5)
		require.NoError(t, err)
		covenantQuorum := uint32(3)
		bsParams := &types.Params{
			CovenantPks:    bbn.NewBIP340PKsFromBTCPKs(covenantPKs),
			CovenantQuorum: covenantQuorum,
		}

		stakingTimeBlocks := uint32(5)
		stakingValue := int64(2 * 10e8)
		slashingAddress, err := datagen.GenRandomBTCAddress(r, &chaincfg.SimNetParams)
		require.NoError(t, err)
		slashingPkScript, err := txscript.PayToAddrScript(slashingAddress)
		require.NoError(t, err)

		slashingRate := sdkmath.LegacyNewDecWithPrec(int64(datagen.RandomInt(r, 41)+10), 2)
		unbondingTime := uint16(100) + 1
		slashingChangeLockTime := unbondingTime

		// only the quorum of signers provided the signatures
		covenantSigners := covenantSKs
		// construct the BTC delegation with everything
		btcDel, err := datagen.GenRandomBTCDelegation(
			r,
			t,
			&chaincfg.SimNetParams,
			fpBTCPKs,
			delSK,
			"",
			covenantSigners,
			covenantPKs,
			covenantQuorum,
			slashingPkScript,
			stakingTimeBlocks,
			1000,
			1000+stakingTimeBlocks,
			uint64(stakingValue),
			slashingRate,
			slashingChangeLockTime,
		)
		require.NoError(t, err)

		stakingInfo, err := btcDel.GetStakingInfo(bsParams, net)
		require.NoError(t, err)

		// TESTING
		orderedCovenantPKs := bbn.SortBIP340PKs(bsParams.CovenantPks)
		covSigsForFP, err := types.GetOrderedCovenantSignatures(slashedFPIdx, btcDel.CovenantSigs, bsParams)
		require.NoError(t, err)
		fpPK := fpSK.PubKey()
		encKey, err := asig.NewEncryptionKeyFromBTCPK(fpPK)
		require.NoError(t, err)
		slashingSpendInfo, err := stakingInfo.SlashingPathSpendInfo()
		require.NoError(t, err)
		for i := range covSigsForFP {
			if covSigsForFP[i] == nil {
				continue
			}
			err := btcDel.SlashingTx.EncVerifyAdaptorSignature(
				stakingInfo.StakingOutput,
				slashingSpendInfo.GetPkScriptPath(),
				orderedCovenantPKs[i].MustToBTCPK(),
				encKey,
				covSigsForFP[i],
			)
			require.NoError(t, err)
		}

		// build slashing tx with witness for spending the staking tx
		slashingTxWithWitness, err := btcDel.BuildSlashingTxWithWitness(bsParams, net, fpSK)
		require.NoError(t, err)

		// assert execution
		btctest.AssertSlashingTxExecution(t, stakingInfo.StakingOutput, slashingTxWithWitness)
	})
}

func TestGetStatus(t *testing.T) {
	covenantQuorum := 1
	tcs := []struct {
		title string

		btcDel    types.BTCDelegation
		btcHeight uint32
		expStatus types.BTCDelegationStatus
	}{
		{
			"unbonded",
			types.BTCDelegation{
				BtcUndelegation: &types.BTCUndelegation{
					DelegatorUnbondingInfo: &types.DelegatorUnbondingInfo{},
				},
			},
			1,
			types.BTCDelegationStatus_UNBONDED,
		},
		{
			"pending, missing cov sigs",
			types.BTCDelegation{
				CovenantSigs: make([]*types.CovenantAdaptorSignatures, 0),
				BtcUndelegation: &types.BTCUndelegation{
					CovenantUnbondingSigList: make([]*types.SignatureInfo, covenantQuorum),
					CovenantSlashingSigs:     make([]*types.CovenantAdaptorSignatures, covenantQuorum),
				},
			},
			1,
			types.BTCDelegationStatus_PENDING,
		},
		{
			"pending, missing cov sigs for unbonding",
			types.BTCDelegation{
				CovenantSigs: make([]*types.CovenantAdaptorSignatures, covenantQuorum),
				BtcUndelegation: &types.BTCUndelegation{
					CovenantUnbondingSigList: make([]*types.SignatureInfo, 0),
					CovenantSlashingSigs:     make([]*types.CovenantAdaptorSignatures, covenantQuorum),
				},
			},
			1,
			types.BTCDelegationStatus_PENDING,
		},
		{
			"pending, missing cov sigs for slashing",
			types.BTCDelegation{
				CovenantSigs: make([]*types.CovenantAdaptorSignatures, covenantQuorum),
				BtcUndelegation: &types.BTCUndelegation{
					CovenantUnbondingSigList: make([]*types.SignatureInfo, covenantQuorum),
					CovenantSlashingSigs:     make([]*types.CovenantAdaptorSignatures, 0),
				},
			},
			1,
			types.BTCDelegationStatus_PENDING,
		},
		{
			"verified: without inclusion proof",
			types.BTCDelegation{
				CovenantSigs: make([]*types.CovenantAdaptorSignatures, covenantQuorum),
				BtcUndelegation: &types.BTCUndelegation{
					CovenantUnbondingSigList: make([]*types.SignatureInfo, covenantQuorum),
					CovenantSlashingSigs:     make([]*types.CovenantAdaptorSignatures, covenantQuorum),
				},
			},
			1,
			types.BTCDelegationStatus_VERIFIED,
		},
		{
			"active: with inclusion proof",
			types.BTCDelegation{
				CovenantSigs: make([]*types.CovenantAdaptorSignatures, covenantQuorum),
				BtcUndelegation: &types.BTCUndelegation{
					CovenantUnbondingSigList: make([]*types.SignatureInfo, covenantQuorum),
					CovenantSlashingSigs:     make([]*types.CovenantAdaptorSignatures, covenantQuorum),
				},
				StartHeight: 1,
				EndHeight:   2,
			},
			1,
			types.BTCDelegationStatus_ACTIVE,
		},
		{
			"unbonded: btc height is less than start height",
			types.BTCDelegation{
				CovenantSigs: make([]*types.CovenantAdaptorSignatures, covenantQuorum),
				BtcUndelegation: &types.BTCUndelegation{
					CovenantUnbondingSigList: make([]*types.SignatureInfo, covenantQuorum),
					CovenantSlashingSigs:     make([]*types.CovenantAdaptorSignatures, covenantQuorum),
				},
				StartHeight: 10,
				EndHeight:   25,
			},
			8,
			types.BTCDelegationStatus_UNBONDED,
		},
		{
			"active: correct delegation",
			types.BTCDelegation{
				CovenantSigs: make([]*types.CovenantAdaptorSignatures, covenantQuorum),
				BtcUndelegation: &types.BTCUndelegation{
					CovenantUnbondingSigList: make([]*types.SignatureInfo, covenantQuorum),
					CovenantSlashingSigs:     make([]*types.CovenantAdaptorSignatures, covenantQuorum),
				},
				StartHeight:   10,
				EndHeight:     25,
				UnbondingTime: 5,
			},
			15,
			types.BTCDelegationStatus_ACTIVE,
		},
		{
			"expired: btcHeight+d.UnbondingTime == d.EndHeight",
			types.BTCDelegation{
				CovenantSigs: make([]*types.CovenantAdaptorSignatures, covenantQuorum),
				BtcUndelegation: &types.BTCUndelegation{
					CovenantUnbondingSigList: make([]*types.SignatureInfo, covenantQuorum),
					CovenantSlashingSigs:     make([]*types.CovenantAdaptorSignatures, covenantQuorum),
				},
				StartHeight:   10,
				EndHeight:     25,
				UnbondingTime: 5,
			},
			20,
			types.BTCDelegationStatus_EXPIRED,
		},
		{
			"expired: btcHeight+d.UnbondingTime > d.EndHeight",
			types.BTCDelegation{
				CovenantSigs: make([]*types.CovenantAdaptorSignatures, covenantQuorum),
				BtcUndelegation: &types.BTCUndelegation{
					CovenantUnbondingSigList: make([]*types.SignatureInfo, covenantQuorum),
					CovenantSlashingSigs:     make([]*types.CovenantAdaptorSignatures, covenantQuorum),
				},
				StartHeight:   10,
				EndHeight:     25,
				UnbondingTime: 6,
			},
			20,
			types.BTCDelegationStatus_EXPIRED,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			actStatus := tc.btcDel.GetStatus(tc.btcHeight, uint32(covenantQuorum), 0)
			require.Equal(t, tc.expStatus, actStatus)
		})
	}
}
