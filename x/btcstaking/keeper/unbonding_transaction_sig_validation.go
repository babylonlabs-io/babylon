package keeper

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func isAnnexedWitness(witness wire.TxWitness) bool {
	if len(witness) < 2 {
		return false
	}

	lastElement := witness[len(witness)-1]
	return len(lastElement) > 0 && lastElement[0] == txscript.TaprootAnnexTag
}

// extractAnnex attempts to extract the annex from the passed witness
func extractAnnex(witness wire.TxWitness) ([]byte, error) {
	lastElement := witness[len(witness)-1]
	return lastElement, nil
}

func parseSchnorrSigFromWitness(rawSig []byte) (*schnorr.Signature, txscript.SigHashType, error) {
	switch {
	// If the signature is exactly 64 bytes, then we know we're using the
	// implicit SIGHASH_DEFAULT sighash type.
	case len(rawSig) == schnorr.SignatureSize:
		// First, parse out the signature which is just the raw sig itself.
		sig, err := schnorr.ParseSignature(rawSig)
		if err != nil {
			return nil, 0, err
		}
		// If the sig is 64 bytes, then we'll assume that it's the
		// default sighash type, which is actually an alias for
		// SIGHASH_ALL.
		return sig, txscript.SigHashDefault, nil
	// Otherwise, if this is a signature, with a sighash looking byte
	// appended that isn't all zero, then we'll extract the sighash from
	// the end of the signature.
	case len(rawSig) == schnorr.SignatureSize+1 && rawSig[64] != 0:
		// Extract the sighash type, then snip off the last byte so we can
		// parse the signature.
		sigHashType := txscript.SigHashType(rawSig[schnorr.SignatureSize])

		rawSig = rawSig[:schnorr.SignatureSize]
		sig, err := schnorr.ParseSignature(rawSig)
		if err != nil {
			return nil, 0, err
		}

		return sig, sigHashType, nil
	// Otherwise, this is an invalid signature, so we need to bail out.
	default:
		str := fmt.Sprintf("invalid sig len: %v", len(rawSig))
		return nil, 0, fmt.Errorf(str)
	}
}

func buildOutputFetcher(
	fundingTransactions []*wire.MsgTx,
	spendStakeTx *wire.MsgTx,
) (*txscript.MultiPrevOutFetcher, error) {

	fundingTxs := make(map[chainhash.Hash]*wire.MsgTx)

	for _, tx := range fundingTransactions {
		fundingTxs[tx.TxHash()] = tx
	}

	prevOuts := make(map[wire.OutPoint]*wire.TxOut)

	// Verify that all inputs of spendStakeTx reference transactions in fundingTxs
	for _, txIn := range spendStakeTx.TxIn {
		tx, ok := fundingTxs[txIn.PreviousOutPoint.Hash]
		if !ok {
			return nil, fmt.Errorf("input references transaction %s which is not in funding transactions", txIn.PreviousOutPoint.Hash)
		}

		if txIn.PreviousOutPoint.Index >= uint32(len(tx.TxOut)) {
			return nil, fmt.Errorf("input references transaction %s which has fewer outputs than the input index %d", txIn.PreviousOutPoint.Hash, txIn.PreviousOutPoint.Index)
		}

		prevOuts[txIn.PreviousOutPoint] = tx.TxOut[txIn.PreviousOutPoint.Index]
	}

	return txscript.NewMultiPrevOutFetcher(prevOuts), nil
}

func VerifySpendStakeTxStakerSig(
	stakerPubKey *btcec.PublicKey,
	stakingOutput *wire.TxOut,
	stakingInputIdx uint32,
	fundingTransactions []*wire.MsgTx,
	spendStakeTx *wire.MsgTx) error {
	// sanity check protecting against passing non-staking outputs
	if !txscript.IsPayToTaproot(stakingOutput.PkScript) {
		return fmt.Errorf("staking output must be a pay-to-taproot output")
	}

	// sanity check to ensure the staking input index is valid
	if int(stakingInputIdx) > len(spendStakeTx.TxIn)-1 {
		return fmt.Errorf("idx %d but %d txins", stakingInputIdx, len(spendStakeTx.TxIn))
	}

	stakingInput := spendStakeTx.TxIn[stakingInputIdx]

	stakeSpendWitness := stakingInput.Witness

	annex := []byte{}

	if isAnnexedWitness(stakeSpendWitness) {
		var err error
		annex, err = extractAnnex(stakeSpendWitness)
		if err != nil {
			return fmt.Errorf("failed to extract annex: %w", err)
		}

		// Snip the annex off the end of the witness stack.
		stakeSpendWitness = stakeSpendWitness[:len(stakeSpendWitness)-1]
	}

	if len(stakeSpendWitness) < 3 {
		return fmt.Errorf("Expected at least 3 elements in the witness stack. Provided amount of elements: %d", len(stakeSpendWitness))
	}

	controlBlock, err := txscript.ParseControlBlock(
		stakeSpendWitness[len(stakeSpendWitness)-1],
	)
	if err != nil {
		return fmt.Errorf("failed to parse control block in witness: %w", err)
	}

	stakingWitnessProgram := stakingOutput.PkScript[1:]

	// Now that we know the control block is valid, we'll
	// verify the top-level taproot commitment, which
	// proves that the specified script was committed to in
	// the merkle tree.
	witnessScript := stakeSpendWitness[len(stakeSpendWitness)-2]
	err = txscript.VerifyTaprootLeafCommitment(
		controlBlock, stakingWitnessProgram, witnessScript,
	)
	if err != nil {
		return fmt.Errorf("failed to verify taproot leaf commitment in witness: %w", err)
	}

	var opts []txscript.TaprootSigHashOption
	if len(annex) > 0 {
		opts = append(opts, txscript.WithAnnex(annex))
	}

	// Staker key is always first in the script, therefore signature will be last.
	// In this true regardless of the path used to spend the staking output (timelock, unbonding, slashing)
	stakerRawSig := stakeSpendWitness[len(stakeSpendWitness)-3]

	stakerSig, sigHashType, err := parseSchnorrSigFromWitness(stakerRawSig)

	if err != nil {
		return fmt.Errorf("failed to parse schnorr signature from witness: %w", err)
	}

	prevOuts, err := buildOutputFetcher(fundingTransactions, spendStakeTx)
	if err != nil {
		return fmt.Errorf("failed to build output fetcher from provided funding transactions: %w", err)
	}

	sigHash, err := txscript.CalcTapscriptSignaturehash(
		txscript.NewTxSigHashes(spendStakeTx, prevOuts),
		sigHashType,
		spendStakeTx,
		int(stakingInputIdx),
		prevOuts,
		txscript.NewBaseTapLeaf(witnessScript),
		opts...,
	)

	if err != nil {
		return fmt.Errorf("failed to calculate tapscript signature hash: %w", err)
	}

	valid := stakerSig.Verify(sigHash, stakerPubKey)

	if !valid {
		return fmt.Errorf("failed to verify schnorr signature: %w", err)
	}

	return nil
}
