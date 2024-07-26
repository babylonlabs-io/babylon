package types

import (
	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/cometbft/cometbft/crypto/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

// IsValid verifies the validity of PoP
// 1. verify(sig=bls_sig, pubkey=blsPubkey, msg=pop.ed25519_sig)?
// 2. verify(sig=pop.ed25519_sig, pubkey=valPubkey, msg=blsPubkey)?
// BLS_pk ?= decrypt(key = Ed25519_pk, data = decrypt(key = BLS_pk, data = PoP))
func (pop ProofOfPossession) IsValid(blsPubkey bls12381.PublicKey, valPubkey cryptotypes.PubKey) bool {
	ok, _ := bls12381.Verify(*pop.BlsSig, blsPubkey, pop.Ed25519Sig)
	if !ok {
		return false
	}
	ed25519PK := ed25519.PubKey(valPubkey.Bytes())
	return ed25519PK.VerifySignature(blsPubkey, pop.Ed25519Sig)
}
