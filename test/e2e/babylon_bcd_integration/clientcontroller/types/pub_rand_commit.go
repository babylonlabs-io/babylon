package types

import (
	"fmt"

	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cometbft/cometbft/crypto/merkle"
)

type PubRandCommit struct {
	StartHeight uint64 `json:"start_height"`
	NumPubRand  uint64 `json:"num_pub_rand"`
	Commitment  []byte `json:"commitment"`
}

// Validate checks if the PubRandCommit structure is valid
// returns an error if not.
func (prc *PubRandCommit) Validate() error {
	if prc.NumPubRand < 1 {
		return fmt.Errorf("NumPubRand must be >= 1, got %d", prc.NumPubRand)
	}
	return nil
}

// GetPubRandCommitAndProofs commits a list of public randomness and returns
// the commitment (i.e., Merkle root) and all Merkle proofs
func GetPubRandCommitAndProofs(pubRandList []*btcec.FieldVal) ([]byte, []*merkle.Proof) {
	prBytesList := make([][]byte, 0, len(pubRandList))
	for _, pr := range pubRandList {
		prBytesList = append(prBytesList, bbn.NewSchnorrPubRandFromFieldVal(pr).MustMarshal())
	}
	return merkle.ProofsFromByteSlices(prBytesList)
}
