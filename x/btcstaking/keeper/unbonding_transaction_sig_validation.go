package keeper

import (
	"fmt"
	bbn "github.com/babylonlabs-io/babylon/v4/types"

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
func extractAnnex(witness wire.TxWitness) []byte {
	lastElement := witness[len(witness)-1]
	return lastElement
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
		return nil, 0, fmt.Errorf("invalid sig len: %d", len(rawSig))
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

// VerifySpendStakeTxStakerSig verifies that staker signature which is included
// in the spend stake transaction witness is valid.
// Funding transactions are necessary, as taproot signature commits to values and
// pkScripts of all the inputs to the spend stake transaction.
func VerifySpendStakeTxStakerSig(
	stakerPubKeys []*btcec.PublicKey,
	stakingOutput *wire.TxOut,
	stakingInputIdx uint32,
	fundingTransactions []*wire.MsgTx,
	spendStakeTx *wire.MsgTx,
) error {
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
		annex = extractAnnex(stakeSpendWitness)
		// Snip the annex off the end of the witness stack.
		stakeSpendWitness = stakeSpendWitness[:len(stakeSpendWitness)-1]
	}

	// After snipping potential annex, there will be at least 3 elements in the witness stack
	// ... <StakerSig> <ScriptPath> <ControlBlock>
	// this is true regardless of the path used to spend the staking output (timelock, unbonding, slashing)
	if len(stakeSpendWitness) < 3 {
		return fmt.Errorf("Expected at least 3 elements in the witness stack. Provided amount of elements: %d", len(stakeSpendWitness))
	}

	controlBlock, err := txscript.ParseControlBlock(
		stakeSpendWitness[len(stakeSpendWitness)-1],
	)
	if err != nil {
		return fmt.Errorf("failed to parse control block in witness: %w", err)
	}

	// StakingOutput is always a pay-to-taproot output, and its pkScript have 34
	// bytes program.
	// 1st byte is OP_1, which defines program version
	// 2nd byte is OP_DATA_32, which pushes 32 bytes of data
	// last 32 bytes is actual taproot program
	stakingWitnessProgram := stakingOutput.PkScript[2:]

	// Now that we know the control block is valid, we'll
	// verify the top-level taproot commitment, which
	// proves that the specified script was committed to in
	// the merkle tree.
	witnessScript := stakeSpendWitness[len(stakeSpendWitness)-2]

	if err := txscript.VerifyTaprootLeafCommitment(
		controlBlock,
		stakingWitnessProgram,
		witnessScript,
	); err != nil {
		return fmt.Errorf("failed to verify taproot leaf commitment in witness: %w", err)
	}

	// Staker key is always first in the script, therefore signature will be last.
	// It is true regardless of the path used to spend the staking output (timelock, unbonding, slashing)
	// index of stakerRawSigs started from len(stakeSpendWitness)-2-stakerCount to len(stakeSpendWitness)-3
	// NOTE: if single sig btc delegation, length of the stakerRawSigs is 1, otherwise (M-of-N multisig btc delegation),
	// the length of stakerRawSigs is same with the length of stakerPubKeys (N)
	stakerCount := len(stakerPubKeys)
	stakerRawSigs := stakeSpendWitness[len(stakeSpendWitness)-2-stakerCount : len(stakeSpendWitness)-2]

	for i, stakerRawSig := range stakerRawSigs {
		// stakerRawSig could be an empty byte to match the total length of stakerPubKeys
		if len(stakerRawSig) == 0 {
			continue
		}
		stakerSig, sigHashType, err := parseSchnorrSigFromWitness(stakerRawSig)
		if err != nil {
			return fmt.Errorf("failed to parse schnorr signature from witness: %w", err)
		}
		prevOuts, err := buildOutputFetcher(fundingTransactions, spendStakeTx)
		if err != nil {
			return fmt.Errorf("failed to build output fetcher from provided funding transactions: %w", err)
		}

		var opts []txscript.TaprootSigHashOption
		if len(annex) > 0 {
			opts = append(opts, txscript.WithAnnex(annex))
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

		// sort stakerPubKeys in reverse lexicographical order before verify it with the given signature
		// NOTE: staker signatures are also sorted in reverse lexicographical order
		stakerBIP340PKs := bbn.NewBIP340PKsFromBTCPKs(stakerPubKeys)
		sortedStakerBIP340PKs := bbn.SortBIP340PKs(stakerBIP340PKs)
		sortedStakerBTCPks, err := bbn.NewBTCPKsFromBIP340PKs(sortedStakerBIP340PKs)
		if err != nil {
			return fmt.Errorf("failed to build bip340 pks: %w", err)
		}

		valid := stakerSig.Verify(sigHash, sortedStakerBTCPks[i])

		if !valid {
			return fmt.Errorf("failed to verify %d schnorr signature of %d: %w", i, stakerCount, err)
		}
	}

	return nil
}
