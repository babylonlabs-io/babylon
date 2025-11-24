package bip322

import (
	"crypto/sha256"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

const (
	bip322Tag = "BIP0322-signed-message"

	// toSpend tx constants
	toSpendVersion     = 0
	toSpendLockTime    = 0
	toSpendInputHash   = "0000000000000000000000000000000000000000000000000000000000000000"
	toSpendInputIndex  = 0xFFFFFFFF
	toSpendInputSeq    = 0
	toSpendOutputValue = 0

	// toSign tx constants
	toSignVersion     = 0
	toSignLockTime    = 0
	toSignInputSeq    = 0
	toSignOutputValue = 0
)

// GetBIP340TaggedHash builds a BIP-340 tagged hash
// More specifically, the hash is of the form
// sha256(sha256(tag) || sha256(tag) || msg)
// See https://github.com/bitcoin/bips/blob/e643d247c8bc086745f3031cdee0899803edea2f/bip-0340.mediawiki#design
// for more details
func GetBIP340TaggedHash(msg []byte) [32]byte {
	tagHash := sha256.Sum256([]byte(bip322Tag))
	sum := make([]byte, 0)
	sum = append(sum, tagHash[:]...)
	sum = append(sum, tagHash[:]...)
	sum = append(sum, msg...)
	return sha256.Sum256(sum)
}

// toSpendSignatureScript creates the signature script for the input
// of the toSpend transaction, i.e.
// `OP_0 PUSH32 [ BIP340_TAGGED_MSG ]`
// https://github.com/bitcoin/bips/blob/e643d247c8bc086745f3031cdee0899803edea2f/bip-0322.mediawiki#full
func toSpendSignatureScript(msg []byte) ([]byte, error) {
	builder := txscript.NewScriptBuilder()
	builder.AddOp(txscript.OP_0)
	data := GetBIP340TaggedHash(msg)
	builder.AddData(data[:])
	script, err := builder.Script()
	if err != nil {
		// msg depends on the input, so play it safe here and don't panic
		return nil, err
	}
	return script, nil
}

// toSignPkScript creates the public key script for the output
// of the toSign transaction, i.e.
// `OP_RETURN`
// https://github.com/bitcoin/bips/blob/e643d247c8bc086745f3031cdee0899803edea2f/bip-0322.mediawiki#full
func toSignPkScript() []byte {
	builder := txscript.NewScriptBuilder()
	builder.AddOp(txscript.OP_RETURN)
	script, err := builder.Script()
	if err != nil {
		// Panic as we're building the script entirely ourselves
		panic(err)
	}
	return script
}

// GetToSpendTx builds a toSpend transaction based on the BIP-322 spec
// https://github.com/bitcoin/bips/blob/e643d247c8bc086745f3031cdee0899803edea2f/bip-0322.mediawiki#full
// It requires as input the message that is signed and the address that produced the signature
func GetToSpendTx(msg []byte, address btcutil.Address) (*wire.MsgTx, error) {
	toSpend := wire.NewMsgTx(toSpendVersion)
	toSpend.LockTime = toSpendLockTime

	// Create a single input with dummy data based on the spec constants
	inputHash, err := chainhash.NewHashFromStr(toSpendInputHash)
	if err != nil {
		// This is a constant we have defined, so an issue here is a programming error
		panic(err)
	}
	outPoint := wire.NewOutPoint(inputHash, toSpendInputIndex)

	// The signature script containing the BIP-322 Tagged message
	script, err := toSpendSignatureScript(msg)
	if err != nil {
		return nil, err
	}
	input := wire.NewTxIn(outPoint, script, nil)
	input.Sequence = toSpendInputSeq

	// Create the output
	// The PK Script should be a pay to addr script on the provided address
	pkScript, err := txscript.PayToAddrScript(address)

	if err != nil {
		return nil, err
	}

	output := wire.NewTxOut(toSpendOutputValue, pkScript)

	toSpend.AddTxIn(input)
	toSpend.AddTxOut(output)
	return toSpend, nil
}

// GetToSignTx builds a toSign transaction based on the BIP-322 spec
// https://github.com/bitcoin/bips/blob/e643d247c8bc086745f3031cdee0899803edea2f/bip-0322.mediawiki#full
// It requires as input the toSpend transaction that it spends
// Transaction is build without any witness, so that the witness must be filled
// by the caller.
func GetToSignTx(toSpend *wire.MsgTx) *wire.MsgTx {
	toSign := wire.NewMsgTx(toSignVersion)
	toSign.LockTime = toSignLockTime

	// Specify the input outpoint
	// Given that the input is the toSpend tx we have built, the input index is 0
	inputHash := toSpend.TxHash()
	outPoint := wire.NewOutPoint(&inputHash, 0)

	input := wire.NewTxIn(outPoint, nil, nil)
	input.Sequence = toSignInputSeq

	// Create the output
	output := wire.NewTxOut(toSignOutputValue, toSignPkScript())

	toSign.AddTxIn(input)
	toSign.AddTxOut(output)
	return toSign
}

// validateSigHashType validates that the witness uses an allowed SIGHASH type
// according to BIP-322 specification.
// For Taproot (P2TR): SIGHASH_DEFAULT (implicit, no byte) or SIGHASH_ALL (0x01)
// For P2WPKH: SIGHASH_ALL (0x01)
func validateSigHashType(witness wire.TxWitness, address btcutil.Address) error {
	if len(witness) == 0 {
		return fmt.Errorf("empty witness")
	}

	// The signature is always in the first element of the witness
	sig := witness[0]
	if len(sig) == 0 {
		return fmt.Errorf("empty signature in witness")
	}

	script, err := txscript.PayToAddrScript(address)
	if err != nil {
		return err
	}
	if txscript.IsPayToTaproot(script) {
		// For Taproot:
		// - SIGHASH_DEFAULT: signature is 64 bytes (no sighash byte appended)
		// - SIGHASH_ALL: signature is 65 bytes with 0x01 as the last byte
		switch len(sig) {
		case 64:
			// SIGHASH_DEFAULT - valid
			return nil
		case 65:
			// Must be SIGHASH_ALL (0x01)
			sighash := txscript.SigHashType(sig[64])
			if sighash != txscript.SigHashAll {
				return fmt.Errorf("invalid sighash type for taproot: 0x%02x, expected SIGHASH_ALL (0x01) or SIGHASH_DEFAULT", sighash)
			}
			return nil
		default:
			return fmt.Errorf("invalid taproot signature length: %d, expected 64 (SIGHASH_DEFAULT) or 65 (with sighash byte)", len(sig))
		}
	} else if txscript.IsPayToWitnessPubKeyHash(script) {
		// For P2WPKH: signature must end with SIGHASH_ALL (0x01)
		// DER-encoded ECDSA signature format: 0x30 [total-length] 0x02 [R-length] [R] 0x02 [S-length] [S] [sighash]
		// Minimum length is ~70 bytes for ECDSA + 1 byte sighash
		if len(sig) < 9 {
			return fmt.Errorf("signature too short for P2WPKH: %d bytes", len(sig))
		}

		// The last byte should be the sighash type
		sighash := txscript.SigHashType(sig[len(sig)-1])
		if sighash != txscript.SigHashAll {
			return fmt.Errorf("invalid sighash type for P2WPKH: 0x%02x, expected SIGHASH_ALL (0x01)", sighash)
		}
		return nil
	} else {
		return fmt.Errorf("unsupported address type: %T", address)
	}
}

// VerifyP2WPKHAndP2TR validates a BIP-322 signature for either a P2WPKH or Taproot (P2TR) address.
func VerifyP2WPKHAndP2TR(
	msg []byte,
	witness wire.TxWitness,
	address btcutil.Address,
	net *chaincfg.Params,
) error {
	// First, validate that the witness uses allowed SIGHASH types
	if err := validateSigHashType(witness, address); err != nil {
		return fmt.Errorf("sighash validation failed: %w", err)
	}

	toSpend, err := GetToSpendTx(msg, address)
	if err != nil {
		return err
	}

	toSign := GetToSignTx(toSpend)

	toSign.TxIn[0].Witness = witness

	// From the rules here:
	// https://github.com/bitcoin/bips/blob/master/bip-0322.mediawiki#verification-process
	// We only need to perform verification of whether toSign spends toSpend properly
	// given that the signature is a simple one and we construct both toSpend and toSign
	inputFetcher := txscript.NewCannedPrevOutputFetcher(toSpend.TxOut[0].PkScript, 0)
	sigHashes := txscript.NewTxSigHashes(toSign, inputFetcher)
	vm, err := txscript.NewEngine(
		toSpend.TxOut[0].PkScript, toSign, 0,
		txscript.StandardVerifyFlags, txscript.NewSigCache(0), sigHashes,
		toSpend.TxOut[0].Value, inputFetcher,
	)

	if err != nil {
		return err
	}

	return vm.Execute()
}

func PubKeyToP2TrSpendAddress(p *btcec.PublicKey, net *chaincfg.Params) (*btcutil.AddressTaproot, error) {
	tapKey := txscript.ComputeTaprootKeyNoScript(p)

	address, err := btcutil.NewAddressTaproot(
		schnorr.SerializePubKey(tapKey), net,
	)
	if err != nil {
		return nil, err
	}
	return address, nil
}
