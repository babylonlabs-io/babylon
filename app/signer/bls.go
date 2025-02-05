package signer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	cmtcfg "github.com/cometbft/cometbft/config"
	cmtcrypto "github.com/cometbft/cometbft/crypto"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	cmtos "github.com/cometbft/cometbft/libs/os"
	"github.com/cometbft/cometbft/libs/tempfile"
	"github.com/cosmos/cosmos-sdk/client/input"
	"github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/crypto/erc2335"
	checkpointingtypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
)

var _ checkpointingtypes.BlsSigner = &BlsKey{}

const (
	DefaultBlsKeyName      = "bls_key.json"     // Default file name for BLS key
	DefaultBlsPasswordName = "bls_password.txt" // Default file name for BLS password
)

var (
	defaultBlsKeyFilePath  = filepath.Join(cmtcfg.DefaultConfigDir, DefaultBlsKeyName)      // Default file path for BLS key
	defaultBlsPasswordPath = filepath.Join(cmtcfg.DefaultConfigDir, DefaultBlsPasswordName) // Default file path for BLS password
)

// Bls is a wrapper around BlsKey
type Bls struct {
	// Key is a structure containing bls12381 keys,
	// paths of both key and password files,
	// and delegator address
	Key BlsKey
}

// BlsKey is a wrapper containing bls12381 keys,
// paths of both key and password files, and delegator address.
type BlsKey struct {
	PubKey       bls12381.PublicKey  `json:"bls_pub_key"`  // Public Key of BLS
	PrivKey      bls12381.PrivateKey `json:"bls_priv_key"` // Private Key of BLS
	filePath     string              // File Path of BLS Key
	passwordPath string              // File Path of BLS Password
}

// NewBls returns a new Bls.
// if private key is nil, it will panic
func NewBls(privKey bls12381.PrivateKey, keyFilePath, passwordFilePath string) *Bls {
	if privKey == nil {
		panic("BLS private key should not be nil")
	}
	return &Bls{
		Key: BlsKey{
			PubKey:       privKey.PubKey(),
			PrivKey:      privKey,
			filePath:     keyFilePath,
			passwordPath: passwordFilePath,
		},
	}
}

// GenBls returns a new Bls after saving it to the file.
func GenBls(keyFilePath, passwordFilePath, password string) *Bls {
	pv := NewBls(bls12381.GenPrivKey(), keyFilePath, passwordFilePath)
	pv.Key.Save(password)
	return pv
}

// LoadBls returns a Bls after loading the erc2335 type of structure
// from the file and decrypt it using a password.
func LoadBls(keyFilePath, passwordFilePath string) *Bls {
	passwordBytes, err := os.ReadFile(passwordFilePath)
	if err != nil {
		cmtos.Exit(fmt.Sprintf("failed to read BLS password file: %v", err.Error()))
	}
	password := string(passwordBytes)

	keystore, err := erc2335.LoadKeyStore(keyFilePath)
	if err != nil {
		cmtos.Exit(fmt.Sprintf("failed to read erc2335 keystore: %v", err.Error()))
	}

	// decrypt bls key from erc2335 type of structure
	privKey, err := erc2335.Decrypt(keystore, password)
	if err != nil {
		cmtos.Exit(fmt.Sprintf("failed to decrypt BLS key: %v", err.Error()))
	}

	blsPrivKey := bls12381.PrivateKey(privKey)
	return &Bls{
		Key: BlsKey{
			PubKey:       blsPrivKey.PubKey(),
			PrivKey:      blsPrivKey,
			filePath:     keyFilePath,
			passwordPath: passwordFilePath,
		},
	}
}

// NewBlsPassword returns a password from the user prompt.
func NewBlsPassword() string {
	inBuf := bufio.NewReader(os.Stdin)
	password, err := input.GetString("Enter your bls password", inBuf)
	if err != nil {
		cmtos.Exit("failed to get BLS password")
	}
	return password
}

// Save saves the bls12381 key to the file.
// The file stores an erc2335 structure containing the encrypted bls private key.
func (k *BlsKey) Save(password string) {
	// encrypt the bls12381 key to erc2335 type
	erc2335BlsKey, err := erc2335.Encrypt(k.PrivKey, k.PubKey.Bytes(), password)
	if err != nil {
		panic(fmt.Errorf("failed to encrypt BLS key: %w", err))
	}

	// Parse the encrypted key back to Erc2335KeyStore structure
	var keystore erc2335.Erc2335KeyStore
	if err := json.Unmarshal(erc2335BlsKey, &keystore); err != nil {
		panic(fmt.Errorf("failed to unmarshal BLS key: %w", err))
	}

	// convert keystore to json
	jsonBytes, err := json.MarshalIndent(keystore, "", "  ")
	if err != nil {
		panic(fmt.Errorf("failed to marshal BLS key: %w", err))
	}

	// write generated erc2335 keystore to file
	if err := tempfile.WriteFileAtomic(k.filePath, jsonBytes, 0600); err != nil {
		panic(fmt.Errorf("failed to write BLS key: %w", err))
	}

	// save used password to file
	if err := tempfile.WriteFileAtomic(k.passwordPath, []byte(password), 0600); err != nil {
		panic(fmt.Errorf("failed to write BLS password: %w", err))
	}
}

// ExportGenBls writes a {address, bls_pub_key, pop, and pub_key} into a json file
func ExportGenBls(valAddress sdk.ValAddress, cmtPrivKey cmtcrypto.PrivKey, blsPrivKey bls12381.PrivateKey, filePath string) (outputFileName string, err error) {
	if !cmtos.FileExists(filePath) {
		return outputFileName, fmt.Errorf("input file %s does not exists", filePath)
	}

	validatorKey, err := NewValidatorKeys(cmtPrivKey, blsPrivKey)
	if err != nil {
		return outputFileName, fmt.Errorf("failed to create validator keys: %w", err)
	}

	pubkey, err := codec.FromCmtPubKeyInterface(validatorKey.ValPubkey)
	if err != nil {
		return outputFileName, fmt.Errorf("failed to convert validator public key: %w", err)
	}

	genbls, err := checkpointingtypes.NewGenesisKey(valAddress, &validatorKey.BlsPubkey, validatorKey.PoP, pubkey)
	if err != nil {
		return outputFileName, fmt.Errorf("failed to create genesis key: %w", err)
	}

	jsonBytes, err := cmtjson.MarshalIndent(genbls, "", "  ")
	if err != nil {
		return outputFileName, fmt.Errorf("failed to marshal genesis key: %w", err)
	}

	outputFileName = filepath.Join(filePath, fmt.Sprintf("gen-bls-%s.json", valAddress.String()))
	if err := tempfile.WriteFileAtomic(outputFileName, jsonBytes, 0600); err != nil {
		return outputFileName, fmt.Errorf("failed to write file: %w", err)
	}
	return outputFileName, nil
}

// DefaultBlsKeyFile returns the default BLS key file path.
func DefaultBlsKeyFile(home string) string {
	return filepath.Join(home, defaultBlsKeyFilePath)
}

// DefaultBlsPasswordFile returns the default BLS password file path.
func DefaultBlsPasswordFile(home string) string {
	return filepath.Join(home, defaultBlsPasswordPath)
}

// SignMsgWithBls signs a message with BLS, implementing the BlsSigner interface
func (k *BlsKey) SignMsgWithBls(msg []byte) (bls12381.Signature, error) {
	if k.PrivKey == nil {
		return nil, fmt.Errorf("BLS private key does not exist: %w", checkpointingtypes.ErrBlsPrivKeyDoesNotExist)
	}
	return bls12381.Sign(k.PrivKey, msg), nil
}

// BlsPubKey returns the public key of the BLS, implementing the BlsSigner interface
func (k *BlsKey) BlsPubKey() (bls12381.PublicKey, error) {
	if k.PrivKey == nil {
		return nil, checkpointingtypes.ErrBlsPrivKeyDoesNotExist
	}
	if k.PubKey == nil {
		return nil, checkpointingtypes.ErrBlsKeyDoesNotExist
	}
	return k.PubKey, nil
}
