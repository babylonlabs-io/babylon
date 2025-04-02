package ecdsa

import (
	"bytes"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

const (
	MAGIC_MESSAGE_PREFIX = "Bitcoin Signed Message:\n"
)

// magicHash encodes the given msg into byte array, then calculates its sha256d hash
// ref: https://github.com/okx/js-wallet-sdk/blob/a57c2acbe6ce917c0aa4e951d96c4e562ad58444/packages/coin-bitcoin/src/message.ts#L28-L34
func magicHash(msg string) chainhash.Hash {
	buf := bytes.NewBuffer(nil)
	// we have to use wire.WriteVarString which encodes the string length into the byte array in Bitcoin's own way
	// message prefix
	// NOTE: we have control over the buffer so errors should not happen
	if err := wire.WriteVarString(buf, 0, MAGIC_MESSAGE_PREFIX); err != nil {
		panic(err)
	}
	// message
	if err := wire.WriteVarString(buf, 0, msg); err != nil {
		panic(err)
	}
	bytes := buf.Bytes()

	return chainhash.DoubleHashH(bytes)
}

func Sign(sk *btcec.PrivateKey, msg string) []byte {
	msgHash := magicHash(msg)
	return ecdsa.SignCompact(sk, msgHash[:], true)
}

func Verify(pk *btcec.PublicKey, msg string, sigBytes []byte) error {
	msgHash := magicHash(msg)
	recoveredPK, wasCompressed, err := ecdsa.RecoverCompact(sigBytes, msgHash[:])
	if err != nil {
		return err
	}
	if !wasCompressed {
		return fmt.Errorf("unsupported signature: uncompressed public key")
	}
	var s btcec.ModNScalar
	if overflow := s.SetByteSlice(sigBytes[33:65]); overflow {
		return fmt.Errorf("invalid signature: S >= group order")
	}
	if s.IsOverHalfOrder() {
		return fmt.Errorf("invalid signature: S >= group order/2")
	}

	var serializedPK []byte
	var serializedRecoveredPK []byte

	if wasCompressed {
		serializedPK = pk.SerializeCompressed()
		serializedRecoveredPK = recoveredPK.SerializeCompressed()
	} else {
		serializedPK = pk.SerializeUncompressed()
		serializedRecoveredPK = recoveredPK.SerializeUncompressed()
	}

	if !bytes.Equal(serializedPK, serializedRecoveredPK) {
		return fmt.Errorf("the recovered PK does not match the given PK")
	}
	return nil
}

func RecoverPublicKey(msg string, sigBytes []byte) (*btcec.PublicKey, bool, error) {
	msgHash := magicHash(msg)
	recoveredPK, wasCompressed, err := ecdsa.RecoverCompact(sigBytes, msgHash[:])
	if err != nil {
		return nil, false, err
	}

	var s btcec.ModNScalar
	if overflow := s.SetByteSlice(sigBytes[33:65]); overflow {
		return nil, false, fmt.Errorf("invalid signature: S >= group order")
	}
	if s.IsOverHalfOrder() {
		return nil, false, fmt.Errorf("invalid signature: S >= group order/2")
	}

	return recoveredPK, wasCompressed, nil
}
