package signer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	cmtcfg "github.com/cometbft/cometbft/config"
	cmtcrypto "github.com/cometbft/cometbft/crypto"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	cmtos "github.com/cometbft/cometbft/libs/os"
	"github.com/cometbft/cometbft/libs/tempfile"
	"github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/crypto/erc2335"
	checkpointingtypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
)

var _ checkpointingtypes.BlsSigner = &BlsKey{}

const (
	DefaultBlsKeyName      = "bls_key.json"         // Default file name for BLS key
	DefaultBlsPasswordName = "bls_password.txt"     // Default file name for BLS password
	BlsPasswordEnvVar      = "BABYLON_BLS_PASSWORD" // Environment variable name for BLS password
	DefaultBlsPopName      = "bls_pop.json"         // Default file name for BLS PoP
)

var (
	defaultBlsKeyFilePath  = filepath.Join(cmtcfg.DefaultConfigDir, DefaultBlsKeyName)      // Default file path for BLS key
	defaultBlsPasswordPath = filepath.Join(cmtcfg.DefaultConfigDir, DefaultBlsPasswordName) // Default file path for BLS password
	defaultBlsPopPath      = filepath.Join(cmtcfg.DefaultConfigDir, DefaultBlsPopName)      // Default file path for BLS PoP
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

// BlsPop represents a proof-of-possession for a validator.
type BlsPop struct {
	BlsPubkey bls12381.PublicKey                    `json:"bls_pub_key"`
	Pop       *checkpointingtypes.ProofOfPossession `json:"pop"`
}

// NewBls returns a new Bls.
// if private key is nil, it will panic
func NewBls(privKey bls12381.PrivateKey, keyFilePath, passwordFilePath string) *Bls {
	if privKey == nil {
		panic("BLS private key should not be nil")
	}

	if err := privKey.ValidateBasic(); err != nil {
		panic(fmt.Errorf("invalid BLS private key: %w", err))
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

// TryLoadBlsFromFile attempts to load a BLS key from the given file paths.
// It tries to use environment variable for password first, then falls back to file-based password.
// Returns nil if the key file doesn't exist or can't be accessed, panic if it can't be decrypted.
func TryLoadBlsFromFile(keyFilePath, passwordFilePath string) *Bls {
	keystore, err := erc2335.LoadKeyStore(keyFilePath)
	if err != nil {
		return nil
	}

	password, err := GetBlsPassword(passwordFilePath)
	if err != nil {
		return nil
	}

	privKey, err := erc2335.Decrypt(keystore, password)
	if err != nil {
		panic(fmt.Errorf("failed to decrypt BLS key: %w", err))
	}

	blsPrivKey := bls12381.PrivateKey(privKey)
	if err := blsPrivKey.ValidateBasic(); err != nil {
		cmtos.Exit(fmt.Sprintf("invalid BLS private key: %v", err.Error()))
	}

	return &Bls{
		Key: BlsKey{
			PubKey:       blsPrivKey.PubKey(),
			PrivKey:      blsPrivKey,
			filePath:     keyFilePath,
			passwordPath: passwordFilePath,
		},
	}
}

// GetBlsPassword retrieves the BLS password from environment variable or password file.
// Password precedence:
// 1. Environment variable (BABYLON_BLS_PASSWORD)
// 2. Password file (if path is provided and file exists)
func GetBlsPassword(passwordFilePath string) (string, error) {
	password := GetBlsPasswordFromEnv()
	if password != "" {
		return password, nil
	}

	if passwordFilePath == "" {
		return "", fmt.Errorf("BLS password not found in environment variable and no password file path provided")
	}

	if !cmtos.FileExists(passwordFilePath) {
		return "", fmt.Errorf("BLS password file does not exist: %s", passwordFilePath)
	}

	passwordBytes, err := os.ReadFile(passwordFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read BLS password file: %w", err)
	}

	return string(passwordBytes), nil
}

// GetBlsPasswordFromEnv retrieves the BLS password from the environment variable only.
// Returns empty string if not found.
func GetBlsPasswordFromEnv() string {
	return os.Getenv(BlsPasswordEnvVar)
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

	if k.passwordPath != "" {
		if err := tempfile.WriteFileAtomic(k.passwordPath, []byte(password), 0600); err != nil {
			panic(fmt.Errorf("failed to write BLS password: %w", err))
		}
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

// DefaultBlsPopFile returns the default BLS PoP file path.
func DefaultBlsPopFile(home string) string {
	return filepath.Join(home, defaultBlsPopPath)
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

// LoadBlsSignerIfExists attempts to load an existing BLS signer from the specified home directory
// Returns the signer if files exist and can be loaded, or nil if files don't exist
// Password precedence:
// 1. Environment variable (BABYLON_BLS_PASSWORD)
// 2. Custom password file path (if provided)
// 3. Default password file path
func LoadBlsSignerIfExists(homeDir string, customPasswordPath, customKeyPath string) checkpointingtypes.BlsSigner {
	blsKeyFile := determineKeyFilePath(homeDir, customKeyPath)
	if !cmtos.FileExists(blsKeyFile) {
		return nil
	}

	passwordPath := determinePasswordFilePath(homeDir, customPasswordPath)

	bls := TryLoadBlsFromFile(blsKeyFile, passwordPath)
	if bls != nil {
		return &bls.Key
	}

	return nil
}

// LoadOrGenBlsKey attempts to load an existing BLS signer or creates a new one if none exists.
// If noPassword is true, creates key without password protection.
// If password is empty and noPassword is false, will try to get from env/file or prompt.
// Password precedence:
// 1. Explicit password provided as argument
// 2. Environment variable (BABYLON_BLS_PASSWORD)
// 3. Custom or default password file
func LoadOrGenBlsKey(homeDir string, noPassword bool, password string, customPasswordPath, customKeyPath string) (checkpointingtypes.BlsSigner, error) {
	blsSigner := LoadBlsSignerIfExists(homeDir, customPasswordPath, customKeyPath)
	if blsSigner != nil {
		return blsSigner, nil
	}

	blsKeyFile := determineKeyFilePath(homeDir, customKeyPath)
	blsPasswordFile := determinePasswordFilePath(homeDir, customPasswordPath)

	if err := EnsureDirs(blsKeyFile, blsPasswordFile); err != nil {
		return nil, fmt.Errorf("failed to ensure directories for BLS key: %w", err)
	}

	password = determinePassword(noPassword, password, blsPasswordFile)

	bls := GenBls(blsKeyFile, blsPasswordFile, password)
	return &bls.Key, nil
}

// determineKeyFilePath returns the appropriate key file path
func determineKeyFilePath(homeDir, customKeyPath string) string {
	if customKeyPath != "" && customKeyPath != defaultBlsKeyFilePath {
		return customKeyPath
	}
	return DefaultBlsKeyFile(homeDir)
}

// determinePasswordFilePath returns the appropriate password file path
func determinePasswordFilePath(homeDir, customPasswordPath string) string {
	if customPasswordPath != "" && customPasswordPath != defaultBlsPasswordPath {
		return customPasswordPath
	}
	return DefaultBlsPasswordFile(homeDir)
}

// determinePassword returns the appropriate password based on the given options
func determinePassword(noPassword bool, explicitPassword, passwordFilePath string) string {
	if noPassword && passwordFilePath != "" {
		dirPath := filepath.Dir(passwordFilePath)
		if err := cmtos.EnsureDir(dirPath, 0777); err == nil {
			_ = tempfile.WriteFileAtomic(passwordFilePath, []byte(""), 0600)
		}
		return ""
	}

	if explicitPassword != "" {
		return explicitPassword
	}

	password, err := GetBlsPassword(passwordFilePath)
	if err == nil {
		return password
	}

	return NewBlsPassword()
}

// SaveBlsPop saves a proof-of-possession to a file.
func SaveBlsPop(filePath string, blsPubKey bls12381.PublicKey, pop *checkpointingtypes.ProofOfPossession) error {
	blsPop := BlsPop{
		BlsPubkey: blsPubKey,
		Pop:       pop,
	}

	// convert keystore to json
	jsonBytes, err := json.MarshalIndent(blsPop, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal bls proof-of-possession: %w", err)
	}

	// write generated erc2335 keystore to file
	if err := tempfile.WriteFileAtomic(filePath, jsonBytes, 0600); err != nil {
		return fmt.Errorf("failed to write bls proof-of-possession: %w", err)
	}
	return nil
}

// LoadBlsPop loads a proof-of-possession from a file.
func LoadBlsPop(filePath string) (BlsPop, error) {
	var bp BlsPop

	keyJSONBytes, err := os.ReadFile(filePath)
	if err != nil {
		return BlsPop{}, fmt.Errorf("failed to read bls pop file: %w", err)
	}

	if err := json.Unmarshal(keyJSONBytes, &bp); err != nil {
		return BlsPop{}, fmt.Errorf("failed to unmarshal bls pop from file: %w", err)
	}

	return bp, nil
}
