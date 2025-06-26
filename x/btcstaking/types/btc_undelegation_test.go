package types_test

import (
	"math/rand"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/stretchr/testify/require"

	asig "github.com/babylonlabs-io/babylon/v3/crypto/schnorr-adaptor-signature"
	btctest "github.com/babylonlabs-io/babylon/v3/testutil/bitcoin"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
)

func FuzzBTCUndelegation_SlashingTx(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		net := &chaincfg.SimNetParams

		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)

		// restaked to a random number of finality providers
		numRestakedFPs := int(datagen.RandomInt(r, 10) + 1)
		fpSKs, fpPKs, err := datagen.GenRandomBTCKeyPairs(r, numRestakedFPs)
		fpBTCPKs := bbn.NewBIP340PKsFromBTCPKs(fpPKs)
		require.NoError(t, err)

		// a random finality provider gets slashed
		slashedFPIdx := int(datagen.RandomInt(r, numRestakedFPs))
		fpSK, fpPK := fpSKs[slashedFPIdx], fpPKs[slashedFPIdx]
		decKey, err := asig.NewDecryptionKeyFromBTCSK(fpSK)
		require.NoError(t, err)
		encKey, err := asig.NewEncryptionKeyFromBTCPK(fpPK)
		require.NoError(t, err)

		// (3, 5) covenant committee
		covenantSKs, covenantPKs, err := datagen.GenRandomBTCKeyPairs(r, 5)
		require.NoError(t, err)
		covenantQuorum := uint32(3)
		bsParams := &types.Params{
			CovenantPks:    bbn.NewBIP340PKsFromBTCPKs(covenantPKs),
			CovenantQuorum: covenantQuorum,
		}
		covenantSigners := covenantSKs

		stakingTimeBlocks := uint32(5)
		stakingValue := int64(2 * 10e8)
		slashingAddress, err := datagen.GenRandomBTCAddress(r, &chaincfg.SimNetParams)
		require.NoError(t, err)

		slashingPkScript, err := txscript.PayToAddrScript(slashingAddress)
		require.NoError(t, err)

		slashingRate := sdkmath.LegacyNewDecWithPrec(int64(datagen.RandomInt(r, 41)+10), 2)
		unbondingTime := uint16(100) + 1
		slashingChangeLockTime := unbondingTime

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
			stakingTimeBlocks+1000,
			uint64(stakingValue),
			slashingRate,
			slashingChangeLockTime,
		)
		require.NoError(t, err)

		unbondingInfo, err := btcDel.GetUnbondingInfo(bsParams, net)
		require.NoError(t, err)

		// TESTING
		orderedCovenantPKs := bbn.SortBIP340PKs(bsParams.CovenantPks)
		covSigsForFP, err := types.GetOrderedCovenantSignatures(slashedFPIdx, btcDel.BtcUndelegation.CovenantSlashingSigs, bsParams)
		require.NoError(t, err)
		slashingSpendInfo, err := unbondingInfo.SlashingPathSpendInfo()
		require.NoError(t, err)
		for i := range covSigsForFP {
			if covSigsForFP[i] == nil {
				continue
			}
			err := btcDel.BtcUndelegation.SlashingTx.EncVerifyAdaptorSignature(
				unbondingInfo.UnbondingOutput,
				slashingSpendInfo.GetPkScriptPath(),
				orderedCovenantPKs[i].MustToBTCPK(),
				encKey,
				covSigsForFP[i],
			)
			require.NoError(t, err)

			covSig, err := covSigsForFP[i].Decrypt(decKey)
			require.NoError(t, err)
			err = btcDel.BtcUndelegation.SlashingTx.VerifySignature(
				unbondingInfo.UnbondingOutput,
				slashingSpendInfo.GetPkScriptPath(),
				orderedCovenantPKs[i].MustToBTCPK(),
				bbn.NewBIP340SignatureFromBTCSig(covSig),
			)
			require.NoError(t, err)
		}

		// build slashing tx with witness for spending the unbonding tx
		unbondingSlashingTxWithWitness, err := btcDel.BuildUnbondingSlashingTxWithWitness(bsParams, net, fpSK)
		require.NoError(t, err)

		// assert the execution
		btctest.AssertSlashingTxExecution(t, unbondingInfo.UnbondingOutput, unbondingSlashingTxWithWitness)
	})
}
