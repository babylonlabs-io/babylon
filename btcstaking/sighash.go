package btcstaking

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/babylonlabs-io/babylon/crypto/schnorr"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

/*
	Below code is necessary for replacing the Schnorr signature implementation
	from btcd with our own implementation.
	Adopted from https://github.com/btcsuite/btcd/blob/v0.24.2/txscript/sighash.go
*/

// sigHashExtFlag represents the sig hash extension flag as defined in BIP 341.
// Extensions to the base sighash algorithm will be appended to the base
// sighash digest.
type sigHashExtFlag uint8

const (
	// baseSigHashExtFlag is the base extension flag. This adds no changes
	// to the sighash digest message. This is used for segwit v1 spends,
	// a.k.a the tapscript keyspend path.
	baseSigHashExtFlag sigHashExtFlag = 0

	// tapscriptSighashExtFlag is the extension flag defined by tapscript
	// base leaf version spend define din BIP 342. This augments the base
	// sighash by including the tapscript leaf hash, the key version, and
	// the code separator position.
	tapscriptSighashExtFlag sigHashExtFlag = 1

	// blankCodeSepValue is the value of the code separator position in the
	// tapscript sighash when no code separator was found in the script.
	blankCodeSepValue = math.MaxUint32
	// sigHashMask defines the number of bits of the hash type which is used
	// to identify which outputs are signed.
	sigHashMask = 0x1f
)

// taprootSigHashOptions houses a set of functional options that may optionally
// modify how the taproot/script sighash digest algorithm is implemented.
type taprootSigHashOptions struct {
	// extFlag denotes the current message digest extension being used. For
	// top-level script spends use a value of zero, while each tapscript
	// version can define its own values as well.
	extFlag sigHashExtFlag

	// annexHash is the sha256 hash of the annex with a compact size length
	// prefix: sha256(sizeOf(annex) || annex).
	annexHash []byte

	// tapLeafHash is the hash of the tapscript leaf as defined in BIP 341.
	// This should be h_tapleaf(version || compactSizeOf(script) || script).
	tapLeafHash []byte

	// keyVersion is the key version as defined in BIP 341. This is always
	// 0x00 for all currently defined leaf versions.
	keyVersion byte

	// codeSepPos is the op code position of the last code separator. This
	// is used for the BIP 342 sighash message extension.
	codeSepPos uint32
}

// defaultTaprootSighashOptions returns the set of default sighash options for
// taproot execution.
func defaultTaprootSighashOptions() *taprootSigHashOptions {
	return &taprootSigHashOptions{}
}

// writeDigestExtensions writes out the sighash message extension defined by the
// current active sigHashExtFlags.
func (t *taprootSigHashOptions) writeDigestExtensions(w io.Writer) error {
	switch t.extFlag {
	// The base extension, used for tapscript keypath spends doesn't modify
	// the digest at all.
	case baseSigHashExtFlag:
		return nil

	// The tapscript base leaf version extension adds the leaf hash, key
	// version, and code separator position to the final digest.
	case tapscriptSighashExtFlag:
		if _, err := w.Write(t.tapLeafHash); err != nil {
			return err
		}
		if _, err := w.Write([]byte{t.keyVersion}); err != nil {
			return err
		}
		err := binary.Write(w, binary.LittleEndian, t.codeSepPos)
		if err != nil {
			return err
		}
	}

	return nil
}

// TaprootSigHashOption defines a set of functional param options that can be
// used to modify the base sighash message with optional extensions.
type TaprootSigHashOption func(*taprootSigHashOptions)

// WithBaseTapscriptVersion is a functional option that specifies that the
// sighash digest should include the extra information included as part of the
// base tapscript version.
func WithBaseTapscriptVersion(codeSepPos uint32,
	tapLeafHash []byte) TaprootSigHashOption {

	return func(o *taprootSigHashOptions) {
		o.extFlag = tapscriptSighashExtFlag
		o.tapLeafHash = tapLeafHash
		o.keyVersion = 0
		o.codeSepPos = codeSepPos
	}
}

// RawTxInTapscriptSignature computes a raw schnorr signature for a signature
// generated from a tapscript leaf. This differs from the
// RawTxInTaprootSignature which is used to generate signatures for top-level
// taproot key spends.
//
// TODO(roasbeef): actually add code-sep to interface? not really used
// anywhere....
func RawTxInTapscriptSignature(tx *wire.MsgTx, sigHashes *txscript.TxSigHashes, idx int,
	amt int64, pkScript []byte, tapLeaf txscript.TapLeaf, hashType txscript.SigHashType,
	privKey *btcec.PrivateKey) ([]byte, error) {

	// First, we'll start by compute the top-level taproot sighash.
	tapLeafHash := tapLeaf.TapHash()
	sigHash, err := calcTaprootSignatureHashRaw(
		sigHashes, hashType, tx, idx,
		txscript.NewCannedPrevOutputFetcher(pkScript, amt),
		WithBaseTapscriptVersion(blankCodeSepValue, tapLeafHash[:]),
	)
	if err != nil {
		return nil, err
	}

	// With the sighash constructed, we can sign it with the specified
	// private key.
	signature, err := schnorr.Sign(privKey, sigHash)
	if err != nil {
		return nil, err
	}

	// Finally, append the sighash type to the final sig if it's not the
	// default sighash value (in which case appending it is disallowed).
	if hashType != txscript.SigHashDefault {
		return append(signature.Serialize(), byte(hashType)), nil
	}

	// The default sighash case where we'll return _just_ the signature.
	return signature.Serialize(), nil
}

// isValidTaprootSigHash returns true if the passed sighash is a valid taproot
// sighash.
func isValidTaprootSigHash(hashType txscript.SigHashType) bool {
	switch hashType {
	case txscript.SigHashDefault, txscript.SigHashAll, txscript.SigHashNone, txscript.SigHashSingle:
		fallthrough
	case 0x81, 0x82, 0x83:
		return true

	default:
		return false
	}
}

// calcTaprootSignatureHashRaw computes the sighash as specified in BIP 143.
// If an invalid sighash type is passed in, an error is returned.
func calcTaprootSignatureHashRaw(sigHashes *txscript.TxSigHashes, hType txscript.SigHashType,
	tx *wire.MsgTx, idx int,
	prevOutFetcher txscript.PrevOutputFetcher,
	sigHashOpts ...TaprootSigHashOption) ([]byte, error) {

	opts := defaultTaprootSighashOptions()
	for _, sigHashOpt := range sigHashOpts {
		sigHashOpt(opts)
	}

	// If a valid sighash type isn't passed in, then we'll exit early.
	if !isValidTaprootSigHash(hType) {
		// TODO(roasbeef): use actual errr here
		return nil, fmt.Errorf("invalid taproot sighash type: %v", hType)
	}

	// As a sanity check, ensure the passed input index for the transaction
	// is valid.
	if idx > len(tx.TxIn)-1 {
		return nil, fmt.Errorf("idx %d but %d txins", idx, len(tx.TxIn))
	}

	// We'll utilize this buffer throughout to incrementally calculate
	// the signature hash for this transaction.
	var sigMsg bytes.Buffer

	// The final sighash always has a value of 0x00 prepended to it, which
	// is called the sighash epoch.
	sigMsg.WriteByte(0x00)

	// First, we write the hash type encoded as a single byte.
	if err := sigMsg.WriteByte(byte(hType)); err != nil {
		return nil, err
	}

	// Next we'll write out the transaction specific data which binds the
	// outer context of the sighash.
	err := binary.Write(&sigMsg, binary.LittleEndian, tx.Version)
	if err != nil {
		return nil, err
	}
	err = binary.Write(&sigMsg, binary.LittleEndian, tx.LockTime)
	if err != nil {
		return nil, err
	}

	// If sighash isn't anyone can pay, then we'll include all the
	// pre-computed midstate digests in the sighash.
	if hType&txscript.SigHashAnyOneCanPay != txscript.SigHashAnyOneCanPay {
		sigMsg.Write(sigHashes.HashPrevOutsV1[:])
		sigMsg.Write(sigHashes.HashInputAmountsV1[:])
		sigMsg.Write(sigHashes.HashInputScriptsV1[:])
		sigMsg.Write(sigHashes.HashSequenceV1[:])
	}

	// If this is sighash all, or its taproot alias (sighash default),
	// then we'll also include the pre-computed digest of all the outputs
	// of the transaction.
	if hType&txscript.SigHashSingle != txscript.SigHashSingle &&
		hType&txscript.SigHashSingle != txscript.SigHashNone {

		sigMsg.Write(sigHashes.HashOutputsV1[:])
	}

	// Next, we'll write out the relevant information for this specific
	// input.
	//
	// The spend type is computed as the (ext_flag*2) + annex_present. We
	// use this to bind the extension flag (that BIP 342 uses), as well as
	// the annex if its present.
	input := tx.TxIn[idx]
	witnessHasAnnex := opts.annexHash != nil
	spendType := byte(opts.extFlag) * 2
	if witnessHasAnnex {
		spendType += 1
	}

	if err := sigMsg.WriteByte(spendType); err != nil {
		return nil, err
	}

	// If anyone can pay is active, then we'll write out just the specific
	// information about this input, given we skipped writing all the
	// information of all the inputs above.
	if hType&txscript.SigHashAnyOneCanPay == txscript.SigHashAnyOneCanPay {
		// We'll start out with writing this input specific information by
		// first writing the entire previous output.
		err = wire.WriteOutPoint(&sigMsg, 0, 0, &input.PreviousOutPoint)
		if err != nil {
			return nil, err
		}

		// Next, we'll write out the previous output (amt+script) being
		// spent itself.
		prevOut := prevOutFetcher.FetchPrevOutput(input.PreviousOutPoint)
		if err := wire.WriteTxOut(&sigMsg, 0, 0, prevOut); err != nil {
			return nil, err
		}

		// Finally, we'll write out the input sequence itself.
		err = binary.Write(&sigMsg, binary.LittleEndian, input.Sequence)
		if err != nil {
			return nil, err
		}
	} else {
		err := binary.Write(&sigMsg, binary.LittleEndian, uint32(idx))
		if err != nil {
			return nil, err
		}
	}

	// Now that we have the input specific information written, we'll
	// include the anex, if we have it.
	if witnessHasAnnex {
		sigMsg.Write(opts.annexHash)
	}

	// Finally, if this is sighash single, then we'll write out the
	// information for this given output.
	if hType&sigHashMask == txscript.SigHashSingle {
		// If this output doesn't exist, then we'll return with an error
		// here as this is an invalid sighash type for this input.
		if idx >= len(tx.TxOut) {
			// TODO(roasbeef): real error here
			return nil, fmt.Errorf("invalid sighash type for input")
		}

		// Now that we know this is a valid sighash input combination,
		// we'll write out the information specific to this input.
		// We'll write the wire serialization of the output and compute
		// the sha256 in a single step.
		shaWriter := sha256.New()
		txOut := tx.TxOut[idx]
		if err := wire.WriteTxOut(shaWriter, 0, 0, txOut); err != nil {
			return nil, err
		}

		// With the digest obtained, we'll write this out into our
		// signature message.
		if _, err := sigMsg.Write(shaWriter.Sum(nil)); err != nil {
			return nil, err
		}
	}

	// Now that we've written out all the base information, we'll write any
	// message extensions (if they exist).
	if err := opts.writeDigestExtensions(&sigMsg); err != nil {
		return nil, err
	}

	// The final sighash is computed as: hash_TagSigHash(0x00 || sigMsg).
	// We wrote the 0x00 above so we don't need to append here and incur
	// extra allocations.
	sigHash := chainhash.TaggedHash(chainhash.TagTapSighash, sigMsg.Bytes())
	return sigHash[:], nil
}
