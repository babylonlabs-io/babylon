package privval

import (
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
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	checkpointingtypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
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
	// wonjoon/feat: separate cometPvKey and blsPvKey
	CometPVKey privval.FilePVKey
	BlsPVKey   BlsPVKey

	// wonjoon/todo: remove
	DelegatorAddress string `json:"acc_address"`
}

// wonjoon/todo: remove or refactoring
func (pvKey WrappedFilePVKey) Save() {
	pvKey.CometPVKey.Save()
	pvKey.BlsPVKey.Save("")
}

type WrappedFilePV struct {
	Key           WrappedFilePVKey
	LastSignState privval.FilePVLastSignState
}

// wonjoon/todo: refactoring
func NewWrappedFilePV(
	cometPrivKey cmtcrypto.PrivKey,
	blsPrivKey bls12381.PrivateKey,
	cometKeyFilePath, cometStateFilePath string,
	// blsKeyFilePath string,
) *WrappedFilePV {
	filePV := privval.NewFilePV(cometPrivKey, cometKeyFilePath, cometStateFilePath)
	return &WrappedFilePV{
		Key: WrappedFilePVKey{
			CometPVKey: filePV.Key,
			BlsPVKey: NewBlsPV(
				blsPrivKey,
				DefaultBlsConfig().BlsKeyFile(), // blsKeyFilePath,
			).Key,
		},
		LastSignState: filePV.LastSignState,
	}
}

// GenWrappedFilePV generates a new validator with randomly generated private key
// and sets the filePaths, but does not call Save().
// wonjoon/todo: refactoring
func GenWrappedFilePV(
	cometKeyFilePath, cometStateFilePath string,
	//blsKeyFilePath string,
) *WrappedFilePV {
	return NewWrappedFilePV(
		ed25519.GenPrivKey(),
		bls12381.GenPrivKey(),
		cometKeyFilePath,
		cometStateFilePath,
		//blsKeyFilePath,
	)
}

// LoadWrappedFilePV loads a FilePV from the filePaths.  The FilePV handles double
// signing prevention by persisting data to the stateFilePath.  If either file path
// does not exist, the program will exit.
// wonjoon/todo: refactoring
func LoadWrappedFilePV(
	cometKeyFilePath, cometStateFilePath string,
	//  blsKeyFilePath, blsPassword string
) *WrappedFilePV {
	return loadWrappedFilePV(
		cometKeyFilePath,
		cometStateFilePath,
		DefaultBlsConfig().BlsKeyFile(), // blsKeyFilePath,
		"",                              // blsPassword,
		true,
	)
}

// LoadWrappedFilePVEmptyState loads a FilePV from the given keyFilePath, with an empty LastSignState.
// If the keyFilePath does not exist, the program will exit.
// wonjoon/todo: refactoring
func LoadWrappedFilePVEmptyState(
	cometKeyFilePath, cometStateFilePath string,
	//  blsKeyFilePath, blsPassword string
) *WrappedFilePV {
	return loadWrappedFilePV(
		cometKeyFilePath,
		cometStateFilePath,
		DefaultBlsConfig().BlsKeyFile(), // blsKeyFilePath,
		"",                              // blsPassword,
		false,
	)
}

// If loadState is true, we load from the stateFilePath. Otherwise, we use an empty LastSignState.
func loadWrappedFilePV(cometKeyFilePath, cometStateFilePath, blsKeyFilePath, blsPassword string, loadState bool) *WrappedFilePV {

	// comet
	var cometPv *privval.FilePV
	if loadState {
		cometPv = privval.LoadFilePV(cometKeyFilePath, cometStateFilePath)
	} else {
		cometPv = privval.LoadFilePVEmptyState(cometKeyFilePath, cometStateFilePath)
	}

	// bls
	blsPv := LoadBlsPV(blsKeyFilePath, blsPassword)

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
// wonjoon/todo: refactoring
func LoadOrGenWrappedFilePV(
	cometKeyFilePath, cometStateFilePath string,
	// blsKeyFilePath, blsPassword string,
) *WrappedFilePV {
	cometPv := privval.LoadOrGenFilePV(cometKeyFilePath, cometStateFilePath)
	blsPv := LoadOrGenBlsPV(
		DefaultBlsConfig().BlsKeyFile(), // blsKeyFilePath,
		"",                              // blsPassword,
	)
	return &WrappedFilePV{
		Key: WrappedFilePVKey{
			CometPVKey: cometPv.Key,
			BlsPVKey:   blsPv.Key,
		},
		LastSignState: cometPv.LastSignState,
	}
}

// ExportGenBls writes a {address, bls_pub_key, pop, and pub_key} into a json file
func (pv *WrappedFilePV) ExportGenBls(filePath string) (outputFileName string, err error) {
	// file check
	if !cmtos.FileExists(filePath) {
		return outputFileName, errors.New("export file path does not exist")
	}

	// ---- Should be removed ---//
	valAddress := pv.GetAddress()
	if valAddress.Empty() {
		return outputFileName, errors.New("validator address should not be empty")
	}
	//-------------------------//

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
	pv.Key.Save()
}

// GetPubKey returns the public key of the validator.
// Implements PrivValidator.
func (pv *WrappedFilePV) GetPubKey() (cmtcrypto.PubKey, error) {
	return pv.Key.CometPVKey.PubKey, nil
}

func (pv *WrappedFilePV) GetValPrivKey() cmtcrypto.PrivKey {
	return pv.Key.CometPVKey.PrivKey
}

func (pv *WrappedFilePV) GetBlsPrivKey() bls12381.PrivateKey {
	return pv.Key.BlsPVKey.PrivKey
}

func (pv *WrappedFilePV) SignMsgWithBls(msg []byte) (bls12381.Signature, error) {
	blsPrivKey := pv.GetBlsPrivKey()
	if blsPrivKey == nil {
		return nil, checkpointingtypes.ErrBlsPrivKeyDoesNotExist
	}
	return bls12381.Sign(blsPrivKey, msg), nil
}

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

// ---- Should be removed ---//₩
// Save persists the FilePV to disk.
func (pv *WrappedFilePV) Save() {
	pv.Key.Save()
	pv.LastSignState.Save()
}

//-------------------------//

// Reset resets all fields in the FilePV.
// NOTE: Unsafe!
func (pv *WrappedFilePV) Reset() {
	var sig []byte
	pv.LastSignState.Height = 0
	pv.LastSignState.Round = 0
	pv.LastSignState.Step = 0
	pv.LastSignState.Signature = sig
	pv.LastSignState.SignBytes = nil
	pv.Save()
}

// Clean removes PVKey file and PVState file
func (pv *WrappedFilePV) Clean(keyFilePath, stateFilePath string) {
	_ = os.RemoveAll(filepath.Dir(keyFilePath))
	_ = os.RemoveAll(filepath.Dir(stateFilePath))
}

// String returns a string representation of the FilePV.
func (pv *WrappedFilePV) String() string {
	return fmt.Sprintf(
		"PrivValidator{%v LH:%v, LR:%v, LS:%v}",
		pv.GetAddress(),
		pv.LastSignState.Height,
		pv.LastSignState.Round,
		pv.LastSignState.Step,
	)
}
