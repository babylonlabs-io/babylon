package types

import (
	asig "github.com/babylonlabs-io/babylon/crypto/schnorr-adaptor-signature"
	bbn "github.com/babylonlabs-io/babylon/types"
)

func (ud *BTCUndelegation) HasCovenantQuorumOnSlashing(quorum uint32) bool {
	return len(ud.CovenantSlashingSigs) >= int(quorum)
}

func (ud *BTCUndelegation) HasCovenantQuorumOnUnbonding(quorum uint32) bool {
	return len(ud.CovenantUnbondingSigList) >= int(quorum)
}

// IsSignedByCovMemberOnUnbonding checks whether the given covenant PK has signed the unbonding tx
func (ud *BTCUndelegation) IsSignedByCovMemberOnUnbonding(covPK *bbn.BIP340PubKey) bool {
	for _, sigInfo := range ud.CovenantUnbondingSigList {
		if sigInfo.Pk.Equals(covPK) {
			return true
		}
	}
	return false
}

// IsSignedByCovMemberOnSlashing checks whether the given covenant PK has signed the slashing tx
func (ud *BTCUndelegation) IsSignedByCovMemberOnSlashing(covPK *bbn.BIP340PubKey) bool {
	for _, sigInfo := range ud.CovenantSlashingSigs {
		if sigInfo.CovPk.Equals(covPK) {
			return true
		}
	}
	return false
}

func (ud *BTCUndelegation) IsSignedByCovMember(covPk *bbn.BIP340PubKey) bool {
	return ud.IsSignedByCovMemberOnUnbonding(covPk) && ud.IsSignedByCovMemberOnSlashing(covPk)
}

func (ud *BTCUndelegation) HasCovenantQuorums(covenantQuorum uint32) bool {
	return ud.HasCovenantQuorumOnUnbonding(covenantQuorum) &&
		ud.HasCovenantQuorumOnSlashing(covenantQuorum)
}

func (ud *BTCUndelegation) GetCovSlashingAdaptorSig(
	covBTCPK *bbn.BIP340PubKey,
	valIdx int,
	quorum uint32,
) (*asig.AdaptorSignature, error) {
	if !ud.HasCovenantQuorums(quorum) {
		return nil, ErrInvalidDelegationState.Wrap("BTC undelegation does not have a covenant quorum yet")
	}
	for _, covASigs := range ud.CovenantSlashingSigs {
		if covASigs.CovPk.Equals(covBTCPK) {
			if valIdx >= len(covASigs.AdaptorSigs) {
				return nil, ErrFpNotFound.Wrap("validator index is out of scope")
			}
			sigBytes := covASigs.AdaptorSigs[valIdx]
			return asig.NewAdaptorSignatureFromBytes(sigBytes)
		}
	}

	return nil, ErrInvalidCovenantPK.Wrap("covenant PK is not found")
}

// AddCovenantSigs adds a Schnorr signature on the unbonding tx, and
// a list of adaptor signatures on the unbonding slashing tx, each encrypted
// by a finality provider's PK this BTC delegation restakes to, from the given
// covenant
// It is up to the caller to ensure that given adaptor signatures are valid or
// that they were not added before
func (ud *BTCUndelegation) addCovenantSigs(
	covPk *bbn.BIP340PubKey,
	unbondingSig *bbn.BIP340Signature,
	slashingSigs []asig.AdaptorSignature,
) {
	covUnbondingSigInfo := &SignatureInfo{Pk: covPk, Sig: unbondingSig}
	ud.CovenantUnbondingSigList = append(ud.CovenantUnbondingSigList, covUnbondingSigInfo)

	adaptorSigs := make([][]byte, 0, len(slashingSigs))
	for _, s := range slashingSigs {
		adaptorSigs = append(adaptorSigs, s.MustMarshal())
	}
	slashingSigsInfo := &CovenantAdaptorSignatures{CovPk: covPk, AdaptorSigs: adaptorSigs}
	ud.CovenantSlashingSigs = append(ud.CovenantSlashingSigs, slashingSigsInfo)
}
