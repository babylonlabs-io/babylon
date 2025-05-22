package keeper_test

import (
	"bytes"
	"math"
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v3/btcstaking"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/keeper"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

var (
	covenantSk, _         = btcec.NewPrivateKey()
	finalityProviderSK, _ = btcec.NewPrivateKey()
	chainParams           = chaincfg.MainNetParams
	stakingTime           = uint16(math.MaxUint16)
	stakingAmount         = btcutil.Amount(1000000)
)

func signTxWithOneScriptSpendInputFromTapLeafInternal(
	txToSign *wire.MsgTx,
	fundingOutput *wire.TxOut,
	privKey *btcec.PrivateKey,
	tapLeaf txscript.TapLeaf,
	inputIdx uint32,
	outputFetcher *txscript.MultiPrevOutFetcher,
) (*schnorr.Signature, error) {
	sigHashes := txscript.NewTxSigHashes(txToSign, outputFetcher)

	sig, err := txscript.RawTxInTapscriptSignature(
		txToSign, sigHashes, int(inputIdx), fundingOutput.Value,
		fundingOutput.PkScript, tapLeaf, txscript.SigHashDefault,
		privKey,
	)

	if err != nil {
		return nil, err
	}

	parsedSig, err := schnorr.ParseSignature(sig)

	if err != nil {
		return nil, err
	}

	return parsedSig, nil
}

func buildOutputPointMap(txs []*wire.MsgTx) map[wire.OutPoint]*wire.TxOut {
	outputs := make(map[wire.OutPoint]*wire.TxOut)

	for _, tx := range txs {
		for i, txOut := range tx.TxOut {
			outputs[wire.OutPoint{Hash: tx.TxHash(), Index: uint32(i)}] = txOut
		}
	}

	return outputs
}

func outputIsEqual(output1 *wire.TxOut, output2 *wire.TxOut) bool {
	return bytes.Equal(output1.PkScript, output2.PkScript) && output1.Value == output2.Value
}

// Property: Arbitrary unbonding transaction signature verification
func FuzzSigVerification(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		stakerSK, stakerPubKey, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)

		numFundingTransaction := datagen.RandomInRange(r, 5, 10)
		stakingTxIndex := datagen.RandomInRange(r, 0, numFundingTransaction)
		stakingOutputIndex := datagen.RandomInRange(r, 0, 2)

		fundingTxs := make([]*wire.MsgTx, numFundingTransaction)
		for i := 0; i < numFundingTransaction; i++ {
			numOutputs := datagen.RandomInRange(r, 2, 4)
			fundingTxs[i] = datagen.GenRandomTxWithOutputs(r, numOutputs)
		}

		stakingInfo, err := btcstaking.BuildStakingInfo(
			stakerPubKey,
			[]*btcec.PublicKey{finalityProviderSK.PubKey()},
			[]*btcec.PublicKey{covenantSk.PubKey()},
			1,
			stakingTime,
			stakingAmount,
			&chainParams,
		)
		require.NoError(t, err)

		fundingTxs[stakingTxIndex].TxOut[stakingOutputIndex] = stakingInfo.StakingOutput

		r.Shuffle(len(fundingTxs), func(i, j int) {
			fundingTxs[i], fundingTxs[j] = fundingTxs[j], fundingTxs[i]
		})

		spendStakeTx := wire.NewMsgTx(1)
		spendStakeTx.AddTxOut(stakingInfo.StakingOutput)

		outputPointMap := buildOutputPointMap(fundingTxs)

		var stakingInputIdx uint32 = 0
		for _, tx := range fundingTxs {
			for j, txOut := range tx.TxOut {
				spendStakeTx.AddTxIn(
					wire.NewTxIn(
						&wire.OutPoint{Hash: tx.TxHash(), Index: uint32(j)},
						nil,
						nil,
					),
				)

				if outputIsEqual(txOut, stakingInfo.StakingOutput) {
					stakingInputIdx = uint32(len(spendStakeTx.TxIn) - 1)
				}
			}
		}

		unbondingSpendInfo, err := stakingInfo.UnbondingPathSpendInfo()
		require.NoError(t, err)

		inputFetcher := txscript.NewMultiPrevOutFetcher(outputPointMap)

		stakerSig, err := signTxWithOneScriptSpendInputFromTapLeafInternal(
			spendStakeTx,
			stakingInfo.StakingOutput,
			stakerSK,
			unbondingSpendInfo.RevealedLeaf,
			stakingInputIdx,
			inputFetcher,
		)
		require.NoError(t, err)
		require.NotNil(t, stakerSig)

		covenantSig, err := signTxWithOneScriptSpendInputFromTapLeafInternal(
			spendStakeTx,
			stakingInfo.StakingOutput,
			covenantSk,
			unbondingSpendInfo.RevealedLeaf,
			stakingInputIdx,
			inputFetcher,
		)

		require.NoError(t, err)
		require.NotNil(t, covenantSig)

		witness, err := unbondingSpendInfo.CreateUnbondingPathWitness(
			[]*schnorr.Signature{covenantSig},
			stakerSig,
		)
		require.NoError(t, err)
		require.NotNil(t, witness)

		spendStakeTx.TxIn[stakingInputIdx].Witness = witness

		err = keeper.VerifySpendStakeTxStakerSig(
			stakerPubKey,
			stakingInfo.StakingOutput,
			stakingInputIdx,
			fundingTxs,
			spendStakeTx,
		)
		require.NoError(t, err)
	})
}
