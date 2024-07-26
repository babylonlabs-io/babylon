package privval

import (
	"errors"

	cmtcrypto "github.com/cometbft/cometbft/crypto"

	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/x/checkpointing/types"
)

type ValidatorKeys struct {
	ValPubkey cmtcrypto.PubKey
	BlsPubkey bls12381.PublicKey
	PoP       *types.ProofOfPossession

	valPrivkey cmtcrypto.PrivKey
	blsPrivkey bls12381.PrivateKey
}

func NewValidatorKeys(valPrivkey cmtcrypto.PrivKey, blsPrivKey bls12381.PrivateKey) (*ValidatorKeys, error) {
	pop, err := BuildPoP(valPrivkey, blsPrivKey)
	if err != nil {
		return nil, err
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
		return nil, errors.New("validator private key is empty")
	}
	if blsPrivkey == nil {
		return nil, errors.New("BLS private key is empty")
	}
	data, err := valPrivKey.Sign(blsPrivkey.PubKey().Bytes())
	if err != nil {
		return nil, err
	}
	pop := bls12381.Sign(blsPrivkey, data)
	return &types.ProofOfPossession{
		Ed25519Sig: data,
		BlsSig:     &pop,
	}, nil
}
