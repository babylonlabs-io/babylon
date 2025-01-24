package privval

import (
	"fmt"

	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/x/checkpointing/keeper"
	checkpointingtypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
	cmtcrypto "github.com/cometbft/cometbft/crypto"
	cmtprivval "github.com/cometbft/cometbft/privval"
)

var _ keeper.BlsSigner = &WrappedFilePV{}

// WrappedFilePV is a wrapper around cmtprivval.FilePV
type WrappedFilePV struct {
	Comet cmtprivval.FilePVKey
	Bls   BlsPVKey
}

// NewWrappedFilePV creates a new WrappedFilePV
func NewWrappedFilePV(comet cmtprivval.FilePVKey, bls BlsPVKey) *WrappedFilePV {
	return &WrappedFilePV{
		Comet: comet,
		Bls:   bls,
	}
}

// SignMsgWithBls signs a message with BLS, implementing the BlsSigner interface
func (pv *WrappedFilePV) SignMsgWithBls(msg []byte) (bls12381.Signature, error) {
	if pv.Bls.PrivKey == nil {
		return nil, fmt.Errorf("BLS private key does not exist: %w", checkpointingtypes.ErrBlsPrivKeyDoesNotExist)
	}
	return bls12381.Sign(pv.Bls.PrivKey, msg), nil
}

// GetBlsPubkey returns the public key of the BLS, implementing the BlsSigner interface
func (pv *WrappedFilePV) GetBlsPubkey() (bls12381.PublicKey, error) {
	if pv.Bls.PrivKey == nil {
		return nil, fmt.Errorf("Error while getting BLS public key: %w", checkpointingtypes.ErrBlsPrivKeyDoesNotExist)
	}
	return pv.Bls.PrivKey.PubKey(), nil
}

// GetValidatorPubkey returns the public key of the validator, implementing the BlsSigner interface
func (pv *WrappedFilePV) GetValidatorPubkey() cmtcrypto.PubKey {
	return pv.Comet.PrivKey.PubKey()
}
