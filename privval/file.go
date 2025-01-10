package privval

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	cmtcrypto "github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/crypto/ed25519"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	cmtos "github.com/cometbft/cometbft/libs/os"
	"github.com/cometbft/cometbft/libs/tempfile"
	"github.com/cometbft/cometbft/privval"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/crypto/codec"

	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/crypto/erc2335"
	checkpointingtypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// copied from github.com/cometbft/cometbft/privval/file.go"
//
//nolint:unused
const (
	stepNone      int8 = 0 // Used to distinguish the initial state
	stepPropose   int8 = 1
	stepPrevote   int8 = 2
	stepPrecommit int8 = 3
)

// copied from github.com/cometbft/cometbft/privval/file.go"
//
//nolint:unused
func voteToStep(vote *cmtproto.Vote) int8 {
	switch vote.Type {
	case cmtproto.PrevoteType:
		return stepPrevote
	case cmtproto.PrecommitType:
		return stepPrecommit
	default:
		panic(fmt.Sprintf("Unknown vote type: %v", vote.Type))
	}
}

// WrappedFilePVKey wraps FilePVKey with BLS keys.
type WrappedFilePVKey struct {
	CometPVKey       privval.FilePVKey
	BlsPVKey         BlsPVKey
	DelegatorAddress string
}

// WrappedFilePV wraps FilePV with WrappedFilePVKey.
type WrappedFilePV struct {
	Key           WrappedFilePVKey
	LastSignState privval.FilePVLastSignState
}

// GenWrappedFilePV generates a new validator with randomly generated private key
// and sets the filePaths, but does not call Save().
func GenWrappedFilePV(cmtKeyFilePath, cmtStateFilePath, blsKeyFilePath, blsPasswordFilePath string) *WrappedFilePV {
	cometPv := privval.NewFilePV(ed25519.GenPrivKey(), cmtKeyFilePath, cmtStateFilePath)
	blsPv := NewBlsPV(bls12381.GenPrivKey(), blsKeyFilePath, blsPasswordFilePath)
	return &WrappedFilePV{
		Key: WrappedFilePVKey{
			CometPVKey: cometPv.Key,
			BlsPVKey:   blsPv.Key,
		},
		LastSignState: cometPv.LastSignState,
	}
}

func GenWrappedFilePVWithMnemonic(mnemonic, cmtKeyFilePath, cmtStateFilePath, blsKeyFilePath, blsPasswordFilePath string) *WrappedFilePV {
	cometPv := privval.NewFilePV(ed25519.GenPrivKeyFromSecret([]byte(mnemonic)), cmtKeyFilePath, cmtStateFilePath)
	blsPv := NewBlsPV(bls12381.GenPrivKeyFromSecret([]byte(mnemonic)), blsKeyFilePath, blsPasswordFilePath)
	return &WrappedFilePV{
		Key: WrappedFilePVKey{
			CometPVKey: cometPv.Key,
			BlsPVKey:   blsPv.Key,
		},
		LastSignState: cometPv.LastSignState,
	}
}

// LoadOrGenWrappedFilePV loads a FilePV from the given filePaths
// or else generates a new one and saves it to the filePaths.
func LoadOrGenWrappedFilePV(cmtKeyFilePath, cmtStateFilePath, blsKeyFilePath, blsPasswordFilePath string) *WrappedFilePV {

	var blsPV *BlsPV

	if !cmtos.FileExists(blsKeyFilePath) {
		var blsPassword string
		var err error
		if cmtos.FileExists(blsPasswordFilePath) {
			blsPassword, err = erc2335.LoadPaswordFromFile(blsPasswordFilePath)
			if err != nil {
				cmtos.Exit(fmt.Sprintf("failed to read BLS password file: %v", err.Error()))
			}
		} else {
			blsPassword = erc2335.CreateRandomPassword()
		}

		blsPV = NewBlsPV(bls12381.GenPrivKey(), blsKeyFilePath, blsPassword)
		blsPV.Save(blsPassword)
	} else {
		blsPV = LoadBlsPV(blsKeyFilePath, blsPasswordFilePath)
	}

	var cometPV *privval.FilePV
	if cmtos.FileExists(cmtKeyFilePath) {
		cometPV = privval.LoadFilePV(cmtKeyFilePath, cmtStateFilePath)
	} else {
		cometPV = privval.GenFilePV(cmtKeyFilePath, cmtStateFilePath)
		cometPV.Key.Save()
	}

	wrappedFilePV := &WrappedFilePV{
		Key: WrappedFilePVKey{
			CometPVKey: cometPV.Key,
			BlsPVKey:   blsPV.Key,
		},
		LastSignState: cometPV.LastSignState,
	}

	return wrappedFilePV
}

func LoadWrappedFilePV(keyFilePath, stateFilePath, blsKeyFilePath, blsPasswordFilePath string) *WrappedFilePV {

	if !cmtos.FileExists(blsKeyFilePath) {
		cmtos.Exit(fmt.Sprintf("BLS key file does not exist: %v", blsKeyFilePath))
	}

	blsPv := LoadBlsPV(blsKeyFilePath, blsPasswordFilePath)

	if !cmtos.FileExists(keyFilePath) {
		cmtos.Exit(fmt.Sprintf("validator key file does not exist: %v", keyFilePath))
	}

	cometPv := privval.LoadFilePV(keyFilePath, stateFilePath)

	return &WrappedFilePV{
		Key: WrappedFilePVKey{
			CometPVKey:       cometPv.Key,
			BlsPVKey:         blsPv.Key,
			DelegatorAddress: ReadDelegatorAddressFromFile(blsKeyFilePath),
		},
		LastSignState: cometPv.LastSignState,
	}
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
	if pv.Key.DelegatorAddress == "" {
		return sdk.ValAddress{}
	}
	addr, err := sdk.AccAddressFromBech32(pv.Key.DelegatorAddress)
	if err != nil {
		cmtos.Exit(err.Error())
	}
	return sdk.ValAddress(addr)
}

func (pv *WrappedFilePV) SetAccAddress(addr sdk.AccAddress) {
	pv.Key.DelegatorAddress = addr.String()
	// pv.Key.Save()
	SaveDelegatorAddressToFile(pv.Key.DelegatorAddress, pv.Key.BlsPVKey.filePath)
}

func SaveDelegatorAddressToFile(delegatorAddress, filePath string) {

	var data map[string]interface{}
	if err := ReadJSON(filePath, &data); err != nil {
		cmtos.Exit(fmt.Sprintf("Failed to read JSON file: %v\n", err))
	}

	data["description"] = delegatorAddress
	if err := WriteJSON(filePath, data); err != nil {
		cmtos.Exit(fmt.Sprintf("Failed to write to JSON file: %v\n", err))
	}
}

func ReadDelegatorAddressFromFile(filePath string) string {

	var data map[string]interface{}
	if err := ReadJSON(filePath, &data); err != nil {
		cmtos.Exit(fmt.Sprintf("Failed to read JSON file: %v\n", err))
	}
	return data["description"].(string)
}

func WriteJSON(filePath string, v interface{}) error {
	jsonBytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, jsonBytes, 0644)
}

func ReadJSON(filePath string, v interface{}) error {
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			*v.(*map[string]interface{}) = make(map[string]interface{})
			return nil
		}
		return err
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(v)
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

// Clean removes PVKey file and PVState file
func (pv *WrappedFilePV) Clean(paths ...string) {
	for _, path := range paths {
		_ = os.RemoveAll(filepath.Dir(path))
	}
}

func (pv *WrappedFilePV) Save(password string) {
	pv.Key.CometPVKey.Save()
	pv.Key.BlsPVKey.Save(password)
	pv.LastSignState.Save()
}
