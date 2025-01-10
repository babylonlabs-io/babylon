package privval

import (
	"errors"
	"fmt"
	"path/filepath"

	cmtcrypto "github.com/cometbft/cometbft/crypto"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	cmtos "github.com/cometbft/cometbft/libs/os"
	"github.com/cometbft/cometbft/libs/tempfile"
	"github.com/cometbft/cometbft/privval"
	"github.com/cosmos/cosmos-sdk/crypto/codec"

	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	checkpointingtypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// WrappedFilePVKey wraps FilePVKey with BLS keys.
type WrappedFilePVKey struct {
	CometPVKey privval.FilePVKey
	BlsPVKey   BlsPVKey
}

// WrappedFilePV wraps FilePV with WrappedFilePVKey.
type WrappedFilePV struct {
	Key           WrappedFilePVKey
	LastSignState privval.FilePVLastSignState
}

// ExportGenBls writes a {address, bls_pub_key, pop, and pub_key} into a json file
func (pv *WrappedFilePV) ExportGenBls(filePath string) (outputFileName string, err error) {
	if !cmtos.FileExists(filePath) {
		return outputFileName, errors.New("export file path does not exist")
	}

	valAddress := pv.GetAddress()
	if valAddress.Empty() {
		return outputFileName, errors.New("validator address should not be empty")
	}

	validatorKey, err := NewValidatorKeys(pv.GetValPrivKey(), pv.GetBlsPrivKey())
	if err != nil {
		return outputFileName, err
	}

	pubkey, err := codec.FromCmtPubKeyInterface(validatorKey.ValPubkey)
	if err != nil {
		return outputFileName, err
	}

	genbls, err := checkpointingtypes.NewGenesisKey(valAddress, &validatorKey.BlsPubkey, validatorKey.PoP, pubkey)
	if err != nil {
		return outputFileName, err
	}

	jsonBytes, err := cmtjson.MarshalIndent(genbls, "", "  ")
	if err != nil {
		return outputFileName, err
	}

	outputFileName = filepath.Join(filePath, fmt.Sprintf("gen-bls-%s.json", valAddress.String()))
	err = tempfile.WriteFileAtomic(outputFileName, jsonBytes, 0600)
	return outputFileName, err
}

// GetAddress returns the delegator address of the validator.
// Implements PrivValidator.
func (pv *WrappedFilePV) GetAddress() sdk.ValAddress {
	if pv.Key.BlsPVKey.DelegatorAddress == "" {
		return sdk.ValAddress{}
	}
	addr, err := sdk.AccAddressFromBech32(pv.Key.BlsPVKey.DelegatorAddress)
	if err != nil {
		cmtos.Exit(err.Error())
	}
	return sdk.ValAddress(addr)
}

// GetPubKey returns the public key of the validator.
func (pv *WrappedFilePV) GetPubKey() (cmtcrypto.PubKey, error) {
	return pv.Key.CometPVKey.PubKey, nil
}

// GetValPrivKey returns the private key of the validator.
func (pv *WrappedFilePV) GetValPrivKey() cmtcrypto.PrivKey {
	return pv.Key.CometPVKey.PrivKey
}

// GetBlsPrivKey returns the private key of the BLS.
func (pv *WrappedFilePV) GetBlsPrivKey() bls12381.PrivateKey {
	return pv.Key.BlsPVKey.PrivKey
}

// GetBlsPubkey returns the public key of the BLS
func (pv *WrappedFilePV) GetBlsPubkey() (bls12381.PublicKey, error) {
	blsPrivKey := pv.GetBlsPrivKey()
	if blsPrivKey == nil {
		return nil, checkpointingtypes.ErrBlsPrivKeyDoesNotExist
	}
	return blsPrivKey.PubKey(), nil
}

func (pv *WrappedFilePV) GetValidatorPubkey() (cmtcrypto.PubKey, error) {
	return pv.GetPubKey()
}

// SignMsgWithBls signs a message with BLS
func (pv *WrappedFilePV) SignMsgWithBls(msg []byte) (bls12381.Signature, error) {
	blsPrivKey := pv.GetBlsPrivKey()
	if blsPrivKey == nil {
		return nil, checkpointingtypes.ErrBlsPrivKeyDoesNotExist
	}
	return bls12381.Sign(blsPrivKey, msg), nil
}
