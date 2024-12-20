package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	asig "github.com/babylonlabs-io/babylon/crypto/schnorr-adaptor-signature"
	bbn "github.com/babylonlabs-io/babylon/types"
)

func (fp *FinalityProvider) IsSlashed() bool {
	return fp.SlashedBabylonHeight > 0
}

func (fp *FinalityProvider) IsJailed() bool {
	return fp.Jailed
}

// Address returns the bech32 fp address
func (fp *FinalityProvider) Address() sdk.AccAddress {
	return sdk.MustAccAddressFromBech32(fp.Addr)
}

func (fp *FinalityProvider) ValidateBasic() error {
	// ensure fields are non-empty and well-formatted
	if _, err := sdk.AccAddressFromBech32(fp.Addr); err != nil {
		return fmt.Errorf("invalid finality provider address: %s - %w", fp.Addr, err)
	}
	if fp.BtcPk == nil {
		return fmt.Errorf("empty BTC public key")
	}
	if _, err := fp.BtcPk.ToBTCPK(); err != nil {
		return fmt.Errorf("BtcPk is not correctly formatted: %w", err)
	}
	if fp.Pop == nil {
		return fmt.Errorf("empty proof of possession")
	}
	if err := fp.Pop.ValidateBasic(); err != nil {
		return fmt.Errorf("PoP is not valid: %w", err)
	}

	return nil
}

func ExistsDup(btcPKs []bbn.BIP340PubKey) bool {
	seen := make(map[string]struct{})

	for _, btcPK := range btcPKs {
		pkStr := string(btcPK)
		if _, found := seen[pkStr]; found {
			return true
		} else {
			seen[pkStr] = struct{}{}
		}
	}

	return false
}

func NewSignatureInfo(pk *bbn.BIP340PubKey, sig *bbn.BIP340Signature) *SignatureInfo {
	return &SignatureInfo{
		Pk:  pk,
		Sig: sig,
	}
}

// GetOrderedCovenantSignatures returns the ordered covenant adaptor signatures
// encrypted by the finality provider's PK at the given index from the given list of
// covenant signatures
// the order of covenant adaptor signatures will follow the reverse lexicographical order
// of signing public keys, in order to be used as tx witness
func GetOrderedCovenantSignatures(fpIdx int, covSigsList []*CovenantAdaptorSignatures, params *Params) ([]*asig.AdaptorSignature, error) {
	// construct the map where
	// - key is the covenant PK, and
	// - value is this covenant member's adaptor signature encrypted
	//   by the given finality provider's PK
	covSigsMap := map[string]*asig.AdaptorSignature{}
	for _, covSigs := range covSigsList {
		// find the adaptor signature at the corresponding finality provider's index
		if fpIdx >= len(covSigs.AdaptorSigs) {
			return nil, fmt.Errorf("finality provider index is out of the scope")
		}
		covSigBytes := covSigs.AdaptorSigs[fpIdx]
		// decode the adaptor signature bytes
		covSig, err := asig.NewAdaptorSignatureFromBytes(covSigBytes)
		if err != nil {
			return nil, err
		}
		// append to map
		covSigsMap[covSigs.CovPk.MarshalHex()] = covSig
	}

	// sort covenant PKs in reverse lexicographical order
	orderedCovenantPKs := bbn.SortBIP340PKs(params.CovenantPks)

	// get ordered list of covenant signatures w.r.t. the order of sorted covenant PKs
	// Note that only a quorum number of covenant signatures needs to be provided
	orderedCovSigs := []*asig.AdaptorSignature{}
	for _, covPK := range orderedCovenantPKs {
		if covSig, ok := covSigsMap[covPK.MarshalHex()]; ok {
			orderedCovSigs = append(orderedCovSigs, covSig)
		} else {
			orderedCovSigs = append(orderedCovSigs, nil)
		}
	}

	return orderedCovSigs, nil
}
