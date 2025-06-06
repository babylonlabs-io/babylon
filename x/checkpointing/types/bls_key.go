package types

import (
	"errors"
	"fmt"

	"github.com/babylonlabs-io/babylon/v2/crypto/bls12381"
)

// Validate checks for duplicate ValidatorAddress or BlsPubKey entries.
func (vs ValidatorWithBlsKeySet) Validate() error {
	addressMap := make(map[string]struct{})
	pubKeyMap := make(map[string]struct{})

	for i, val := range vs.ValSet {
		// Check duplicate ValidatorAddress
		if _, exists := addressMap[val.ValidatorAddress]; exists {
			return fmt.Errorf("duplicate ValidatorAddress found at index %d: %s", i, val.ValidatorAddress)
		}
		addressMap[val.ValidatorAddress] = struct{}{}

		// Check duplicate BlsPubKey using string representation for map key
		key := string(val.BlsPubKey)
		if _, exists := pubKeyMap[key]; exists {
			return fmt.Errorf("duplicate BlsPubKey found at index %d", i)
		}
		pubKeyMap[key] = struct{}{}

		// check BLS key
		pk := new(bls12381.PublicKey)
		if err := pk.Unmarshal(val.BlsPubKey); err != nil {
			return err
		}

		if err := pk.ValidateBasic(); err != nil {
			return err
		}
	}

	return nil
}

// ValidateBasic stateless validate if the BlsKey is valid
func (k BlsKey) ValidateBasic() error {
	if k.Pop == nil {
		return errors.New("BLS Proof of Possession is nil")
	}
	if k.Pubkey == nil {
		return errors.New("BLS Public key is nil")
	}

	return k.Pubkey.ValidateBasic()
}
