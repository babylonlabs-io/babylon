package types

import (
	"encoding/hex"
	"fmt"

	btcctypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
)

func NewInclusionProof(txKey *btcctypes.TransactionKey, proof []byte) *InclusionProof {
	return &InclusionProof{
		Key:   txKey,
		Proof: proof,
	}
}

func NewInclusionProofFromHex(inclusionProofHex string) (*InclusionProof, error) {
	inclusionProofBytes, err := hex.DecodeString(inclusionProofHex)
	if err != nil {
		return nil, err
	}
	var inclusionProof InclusionProof
	if err := inclusionProof.Unmarshal(inclusionProofBytes); err != nil {
		return nil, err
	}
	return &inclusionProof, nil
}

func (ip *InclusionProof) ValidateBasic() error {
	if ip.Key == nil {
		return fmt.Errorf("key in InclusionProof is nil")
	}
	if ip.Proof == nil {
		return fmt.Errorf("proof in InclussionProof is nil")
	}

	return nil
}
