package signer

import (
	"fmt"

	cmtcrypto "github.com/cometbft/cometbft/crypto"

	"github.com/babylonlabs-io/babylon/v3/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/v3/x/checkpointing/types"
)

// ValidatorKeys represents a validator keys.
type ValidatorKeys struct {
	ValPubkey cmtcrypto.PubKey
	BlsPubkey bls12381.PublicKey
	PoP       *types.ProofOfPossession

	valPrivkey cmtcrypto.PrivKey
	blsPrivkey bls12381.PrivateKey
}

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

	msg := bls12381.GetPopSignMsg(blsPrivkey.PubKey(), data)
	pop := bls12381.PopProve(blsPrivkey, msg)

	return &types.ProofOfPossession{
		Ed25519Sig: data,
		BlsSig:     &pop,
	}, nil
}
