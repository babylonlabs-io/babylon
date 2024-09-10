package types

import (
	fmt "fmt"

	"github.com/babylonlabs-io/babylon/crypto/eots"
	"github.com/cometbft/cometbft/crypto/merkle"
	"github.com/cometbft/cometbft/crypto/tmhash"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ensure that these message types implement the sdk.Msg interface
var (
	_ sdk.Msg = &MsgUpdateParams{}
	_ sdk.Msg = &MsgAddFinalitySig{}
	_ sdk.Msg = &MsgCommitPubRandList{}
)

func (m *MsgAddFinalitySig) MsgToSign() []byte {
	return msgToSignForVote(m.BlockHeight, m.BlockAppHash)
}

// VerifyFinalitySig verifies the finality signature message w.r.t. the
// public randomness commitment. The verification includes
// - verifying the proof of inclusion of the given public randomness
// - verifying the finality signature w.r.t. the given block height/hash
func VerifyFinalitySig(m *MsgAddFinalitySig, prCommit *PubRandCommit) error {
	fmt.Printf("VerifyFinalitySig: Starting verification for block height %d\n", m.BlockHeight)

	if m.Proof != nil {
		fmt.Println("VerifyFinalitySig: m.Proof exists")
	} else {
		fmt.Println("VerifyFinalitySig: m.Proof is nil")
	}

	// verify the index of the public randomness
	heightOfProof := prCommit.StartHeight + uint64(m.Proof.Index)
	fmt.Printf("VerifyFinalitySig: Proof height: %d, Message block height: %d\n", heightOfProof, m.BlockHeight)
	if m.BlockHeight != heightOfProof {
		return ErrInvalidFinalitySig.Wrapf("the inclusion proof (for height %d) does not correspond to the given height (%d) in the message", heightOfProof, m.BlockHeight)
	}

	// verify the total number of randomness is same as in the commit
	fmt.Printf("VerifyFinalitySig: Proof total: %d, Commit NumPubRand: %d\n", m.Proof.Total, prCommit.NumPubRand)
	if uint64(m.Proof.Total) != prCommit.NumPubRand {
		return ErrInvalidFinalitySig.Wrapf("the total number of public randomnesses in the proof (%d) does not match the number of public randomnesses committed (%d)", m.Proof.Total, prCommit.NumPubRand)
	}

	// verify the proof of inclusion for this public randomness
	unwrappedProof, err := merkle.ProofFromProto(m.Proof)
	if err != nil {
		fmt.Printf("VerifyFinalitySig: Failed to unwrap proof: %v\n", err)
		return ErrInvalidFinalitySig.Wrapf("failed to unwrap proof: %v", err)
	}

	fmt.Println("VerifyFinalitySig: Verifying inclusion proof")
	if err := unwrappedProof.Verify(prCommit.Commitment, *m.PubRand); err != nil {
		fmt.Printf("VerifyFinalitySig: Inclusion proof verification failed: %v\n", err)
		return ErrInvalidFinalitySig.Wrapf("the inclusion proof of the public randomness is invalid: %v", err)
	}

	// public randomness is good, verify finality signature
	msgToSign := m.MsgToSign()
	pk, err := m.FpBtcPk.ToBTCPK()
	if err != nil {
		fmt.Printf("VerifyFinalitySig: Failed to convert FpBtcPk to BTCPK: %v\n", err)
		return err
	}

	fmt.Println("VerifyFinalitySig: Verifying finality signature")
	err = eots.Verify(pk, m.PubRand.ToFieldVal(), msgToSign, m.FinalitySig.ToModNScalar())
	if err != nil {
		fmt.Printf("VerifyFinalitySig: Finality signature verification failed: %v\n", err)
	} else {
		fmt.Println("VerifyFinalitySig: Verification successful")
	}
	return err
}

// HashToSign returns a 32-byte hash of (start_height || num_pub_rand || commitment)
// The signature in MsgCommitPubRandList will be on this hash
func (m *MsgCommitPubRandList) HashToSign() ([]byte, error) {
	hasher := tmhash.New()
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

func (m *MsgCommitPubRandList) VerifySig() error {
	msgHash, err := m.HashToSign()
	if err != nil {
		return err
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
