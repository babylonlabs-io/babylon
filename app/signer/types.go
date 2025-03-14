package signer

import (
	"fmt"

	cmtcrypto "github.com/cometbft/cometbft/crypto"

	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/x/checkpointing/types"
)

// ValidatorKeys represents a validator keys.
type ValidatorKeys struct {
	ValPubkey cmtcrypto.PubKey
	BlsPubkey bls12381.PublicKey
	PoP       *types.ProofOfPossession

	valPrivkey cmtcrypto.PrivKey
	blsPrivkey bls12381.PrivateKey
}

// // BlsPop represents a proof-of-possession for a validator.
// type BlsPop struct {
// 	BlsPubkey bls12381.PublicKey
// 	PoP       *types.ProofOfPossession
// }

// NewValidatorKeys creates a new instance including validator keys.
func NewValidatorKeys(valPrivkey cmtcrypto.PrivKey, blsPrivKey bls12381.PrivateKey) (*ValidatorKeys, error) {
	pop, err := BuildPoP(valPrivkey, blsPrivKey)
	if err != nil {
		return nil, fmt.Errorf("failed to build PoP: %w", err)
	}
	return &ValidatorKeys{
		ValPubkey:  valPrivkey.PubKey(),
		BlsPubkey:  blsPrivKey.PubKey(),
		valPrivkey: valPrivkey,
		blsPrivkey: blsPrivKey,
		PoP:        pop,
	}, nil
}

// BuildPoP builds a proof-of-possession by PoP=sign(key = BLS_sk, data = sign(key = Ed25519_sk, data = BLS_pk))
// where valPrivKey is Ed25519_sk and blsPrivkey is BLS_sk
func BuildPoP(valPrivKey cmtcrypto.PrivKey, blsPrivkey bls12381.PrivateKey) (*types.ProofOfPossession, error) {
	if valPrivKey == nil {
		return nil, fmt.Errorf("validator private key is empty")
	}
	if blsPrivkey == nil {
		return nil, fmt.Errorf("BLS private key is empty")
	}
	data, err := valPrivKey.Sign(blsPrivkey.PubKey().Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to sign Ed25519 key: %w", err)
	}
	pop := bls12381.Sign(blsPrivkey, data)
	return &types.ProofOfPossession{
		Ed25519Sig: data,
		BlsSig:     &pop,
	}, nil
}

// // SaveBlsPop saves a proof-of-possession to a file.
// func SaveBlsPop(filePath string, blsPubKey bls12381.PublicKey, pop *types.ProofOfPossession) error {
// 	blsPop := BlsPop{
// 		BlsPubkey: blsPubKey,
// 		PoP:       pop,
// 	}

// 	// convert keystore to json
// 	jsonBytes, err := json.MarshalIndent(blsPop, "", "  ")
// 	if err != nil {
// 		return fmt.Errorf("failed to marshal bls proof-of-possession: %w", err)
// 	}

// 	// write generated erc2335 keystore to file
// 	if err := tempfile.WriteFileAtomic(filePath, jsonBytes, 0600); err != nil {
// 		return fmt.Errorf("failed to write bls proof-of-possession: %w", err)
// 	}
// 	return nil
// }

// // LoadBlsPop loads a proof-of-possession from a file.
// func LoadBlsPop(filePath string) (BlsPop, error) {
// 	var bp BlsPop

// 	keyJSONBytes, err := os.ReadFile(filePath)
// 	if err != nil {
// 		return BlsPop{}, fmt.Errorf("failed to read bls pop file: %w", err)
// 	}

// 	if err := json.Unmarshal(keyJSONBytes, &bp); err != nil {
// 		return BlsPop{}, fmt.Errorf("failed to unmarshal bls pop from file: %w", err)
// 	}

// 	return bp, nil
// }
