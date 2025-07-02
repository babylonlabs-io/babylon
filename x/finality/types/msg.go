package types

import (
	"encoding/hex"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	bbntypes "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/cometbft/cometbft/crypto/merkle"
	"github.com/cometbft/cometbft/crypto/tmhash"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v3/crypto/eots"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
)

// ensure that these message types implement the sdk.Msg interface
var (
	_ sdk.Msg = &MsgResumeFinalityProposal{}
	_ sdk.Msg = &MsgUpdateParams{}
	_ sdk.Msg = &MsgAddFinalitySig{}
	_ sdk.Msg = &MsgCommitPubRandList{}
	_ sdk.Msg = &MsgUnjailFinalityProvider{}
	_ sdk.Msg = &MsgEquivocationEvidence{}
	// Ensure msgs implement ValidateBasic
	_ sdk.HasValidateBasic = &MsgAddFinalitySig{}
	_ sdk.HasValidateBasic = &MsgResumeFinalityProposal{}
	_ sdk.HasValidateBasic = &MsgCommitPubRandList{}
	_ sdk.HasValidateBasic = &MsgUnjailFinalityProvider{}
)

const ExpectedCommitmentLengthBytes = 32

func (m *MsgAddFinalitySig) MsgToSign(signingContext string) []byte {
	return msgToSignForVote(signingContext, m.BlockHeight, m.BlockAppHash)
}

func (m *MsgAddFinalitySig) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Signer); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("invalid signer address: got %s", m.Signer)
	}

	if m.FpBtcPk == nil {
		return ErrInvalidFinalitySig.Wrap("empty Finality Provider BTC PubKey")
	}

	if m.FpBtcPk.Size() != bbn.BIP340PubKeyLen {
		return ErrInvalidFinalitySig.Wrapf("invalid finality provider BTC public key length: got %d, want %d", m.FpBtcPk.Size(), bbn.BIP340PubKeyLen)
	}

	if m.PubRand == nil {
		return ErrInvalidFinalitySig.Wrap("empty Public Randomness")
	}

	if m.PubRand.Size() != bbn.SchnorrPubRandLen {
		return ErrInvalidFinalitySig.Wrapf("invalind public randomness length: got %d, want %d", m.PubRand.Size(), bbn.SchnorrPubRandLen)
	}

	if m.Proof == nil {
		return ErrInvalidFinalitySig.Wrap("empty inclusion proof")
	}

	if m.FinalitySig == nil {
		return ErrInvalidFinalitySig.Wrap("empty finality signature")
	}

	if m.FinalitySig.Size() != bbn.SchnorrEOTSSigLen {
		return ErrInvalidFinalitySig.Wrapf("invalid finality signature length: got %d, want %d", m.FinalitySig.Size(), bbn.BIP340SignatureLen)
	}

	if len(m.BlockAppHash) != tmhash.Size {
		return ErrInvalidFinalitySig.Wrapf("invalid block app hash length: got %d, want %d", len(m.BlockAppHash), tmhash.Size)
	}

	return nil
}

// VerifyFinalitySig verifies the finality signature message w.r.t. the
// public randomness commitment. The verification includes
// - verifying the proof of inclusion of the given public randomness
// - verifying the finality signature w.r.t. the given block height/hash
func VerifyFinalitySig(
	m *MsgAddFinalitySig,
	prCommit *PubRandCommit,
	signingContext string,
) error {
	// verify the index of the public randomness
	heightOfProof := prCommit.StartHeight + uint64(m.Proof.Index)
	if m.BlockHeight != heightOfProof {
		return ErrInvalidFinalitySig.Wrapf("the inclusion proof (for height %d) does not correspond to the given height (%d) in the message", heightOfProof, m.BlockHeight)
	}
	// verify the total number of randomness is same as in the commit
	if uint64(m.Proof.Total) != prCommit.NumPubRand {
		return ErrInvalidFinalitySig.Wrapf("the total number of public randomnesses in the proof (%d) does not match the number of public randomnesses committed (%d)", m.Proof.Total, prCommit.NumPubRand)
	}
	// verify the proof of inclusion for this public randomness
	unwrappedProof, err := merkle.ProofFromProto(m.Proof)
	if err != nil {
		return ErrInvalidFinalitySig.Wrapf("failed to unwrap proof: %v", err)
	}
	if err := unwrappedProof.Verify(prCommit.Commitment, *m.PubRand); err != nil {
		return ErrInvalidFinalitySig.Wrapf("the inclusion proof of the public randomness is invalid: %v", err)
	}

	// public randomness is good, verify finality signature
	msgToSign := m.MsgToSign(signingContext)
	pk, err := m.FpBtcPk.ToBTCPK()
	if err != nil {
		return err
	}
	return eots.Verify(pk, m.PubRand.ToFieldValNormalized(), msgToSign, m.FinalitySig.ToModNScalar())
}

// HashToSign returns a 32-byte hash of signing_context || (start_height || num_pub_rand || commitment)
// The signature in MsgCommitPubRandList will be on this hash
func (m *MsgCommitPubRandList) HashToSign(signingContext string) ([]byte, error) {
	hasher := tmhash.New()
	if _, err := hasher.Write([]byte(signingContext)); err != nil {
		return nil, err
	}
	if _, err := hasher.Write(sdk.Uint64ToBigEndian(m.StartHeight)); err != nil {
		return nil, err
	}
	if _, err := hasher.Write(sdk.Uint64ToBigEndian(m.NumPubRand)); err != nil {
		return nil, err
	}
	if _, err := hasher.Write(m.Commitment); err != nil {
		return nil, err
	}
	return hasher.Sum(nil), nil
}

func (m *MsgCommitPubRandList) VerifySig(signingContext string) error {
	msgHash, err := m.HashToSign(signingContext)
	if err != nil {
		return err
	}
	if m.FpBtcPk == nil {
		return fmt.Errorf("empty FP BTC PubKey")
	}
	pk, err := m.FpBtcPk.ToBTCPK()
	if err != nil {
		return err
	}
	if m.Sig == nil {
		return fmt.Errorf("empty signature")
	}
	schnorrSig, err := m.Sig.ToBTCSig()
	if err != nil {
		return err
	}
	if !schnorrSig.Verify(msgHash, pk) {
		return fmt.Errorf("failed to verify signature")
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgCommitPubRandList
func (m *MsgCommitPubRandList) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(m.Signer)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid signer address (%s)", err)
	}

	if m.FpBtcPk == nil {
		return ErrInvalidPubRand.Wrap("empty FP BTC PubKey")
	}

	// Checks if the commitment is exactly 32 bytes
	if len(m.Commitment) != ExpectedCommitmentLengthBytes {
		return ErrInvalidPubRand.Wrapf("commitment must be %d bytes, got %d", ExpectedCommitmentLengthBytes, len(m.Commitment))
	}

	// To avoid public randomness reset,
	// check for overflow when doing (StartHeight + NumPubRand)
	if m.StartHeight >= (m.StartHeight + m.NumPubRand) {
		return ErrOverflowInBlockHeight.Wrapf(
			"public rand commit start block height: %d is equal or higher than (start height + num pub rand) %d",
			m.StartHeight, m.StartHeight+m.NumPubRand,
		)
	}

	if m.Sig == nil {
		return ErrInvalidPubRand.Wrap("empty signature")
	}

	return nil
}

// ValidateBasic performs stateless validation on MsgUnjailFinalityProvider
func (m *MsgUnjailFinalityProvider) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Signer); err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid signer address (%s)", err)
	}
	if m.FpBtcPk == nil {
		return ErrInvalidUnjailFinalityProvider.Wrap("empty FP BTC PubKey")
	}

	return nil
}

func (m *MsgResumeFinalityProposal) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid authority address (%s)", err)
	}
	if m.HaltingHeight == 0 {
		return ErrInvalidResumeFinality.Wrap("halting height is zero")
	}
	if len(m.FpPksHex) == 0 {
		return ErrInvalidResumeFinality.Wrap("no fp pk hex set")
	}

	fps := make(map[string]struct{})
	for _, fpPkHex := range m.FpPksHex {
		_, err := bbntypes.NewBIP340PubKeyFromHex(fpPkHex)
		if err != nil {
			return ErrInvalidResumeFinality.Wrapf("failed to parse FP BTC PK Hex (%s) into BIP-340", fpPkHex)
		}

		_, found := fps[fpPkHex]
		if found {
			return ErrInvalidResumeFinality.Wrapf("duplicated FP BTC PK Hex (%s)", fpPkHex)
		}
		fps[fpPkHex] = struct{}{}
	}

	return nil
}

// ToEvidence converts MsgEquivocationEvidence to Evidence
func (m *MsgEquivocationEvidence) ParseToEvidence() (*Evidence, error) {
	if _, err := sdk.AccAddressFromBech32(m.Signer); err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid signer address (%s)", err)
	}

	fpBtcPk, err := bbntypes.NewBIP340PubKeyFromHex(m.FpBtcPkHex)
	if err != nil {
		return nil, ErrInvalidEquivocationEvidence.Wrapf("invalid FP BTC PK: %s", err)
	}

	pubRand, err := bbntypes.NewSchnorrPubRandFromHex(m.PubRandHex)
	if err != nil {
		return nil, ErrInvalidEquivocationEvidence.Wrapf("invalid PubRand: %s", err)
	}

	canonicalAppHash, err := hex.DecodeString(m.CanonicalAppHashHex)
	if err != nil || len(canonicalAppHash) != 32 {
		return nil, ErrInvalidEquivocationEvidence.Wrap("malformed CanonicalAppHash")
	}

	forkAppHash, err := hex.DecodeString(m.ForkAppHashHex)
	if err != nil || len(forkAppHash) != 32 {
		return nil, ErrInvalidEquivocationEvidence.Wrap("malformed ForkAppHash")
	}

	canonicalFinalitySig, err := bbntypes.NewSchnorrEOTSSigFromHex(m.CanonicalFinalitySigHex)
	if err != nil {
		return nil, ErrInvalidEquivocationEvidence.Wrapf("invalid CanonicalFinalitySig: %s", err)
	}

	forkFinalitySig, err := bbntypes.NewSchnorrEOTSSigFromHex(m.ForkFinalitySigHex)
	if err != nil {
		return nil, ErrInvalidEquivocationEvidence.Wrapf("invalid ForkFinalitySig: %s", err)
	}

	return &Evidence{
		FpBtcPk:              fpBtcPk,
		BlockHeight:          m.BlockHeight,
		PubRand:              pubRand,
		CanonicalAppHash:     canonicalAppHash,
		ForkAppHash:          forkAppHash,
		CanonicalFinalitySig: canonicalFinalitySig,
		ForkFinalitySig:      forkFinalitySig,
		SigningContext:       m.SigningContext,
	}, nil
}
