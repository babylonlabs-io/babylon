package signer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	cmtcfg "github.com/cometbft/cometbft/config"
	cmtcrypto "github.com/cometbft/cometbft/crypto"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	cmtos "github.com/cometbft/cometbft/libs/os"
	"github.com/cometbft/cometbft/libs/tempfile"
	"github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v3/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/v3/crypto/erc2335"
	checkpointingtypes "github.com/babylonlabs-io/babylon/v3/x/checkpointing/types"
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

// loadBlsPrivKeyFromFile loads a BLS private key from a file.
// Password should be determined before calling this function.
func loadBlsPrivKeyFromFile(keyFilePath, password string) (bls12381.PrivateKey, error) {
	keystore, err := erc2335.LoadKeyStore(keyFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load BLS key file: %w", err)
	}

	privKey, err := erc2335.Decrypt(keystore, password)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt BLS key: %w", err)
	}

	blsPrivKey := bls12381.PrivateKey(privKey)
	if err := blsPrivKey.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("invalid BLS private key: %w", err)
	}

	return blsPrivKey, nil
}

// TryLoadBlsFromFile attempts to load a BLS key from the given file paths.
// It tries to use environment variable for password first, then falls back to file-based password.
// Returns error if the key file exists, but cannot get password or the key cannot
// be decrypted
func TryLoadBlsFromFile(keyFilePath, passwordFilePath string) (*Bls, bool, error) {
	password, err := GetBlsPassword(passwordFilePath)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get password: %w", err)
	}

	blsPrivKey, err := loadBlsPrivKeyFromFile(keyFilePath, password)
	if err != nil {
		return nil, false, fmt.Errorf("failed to load BLS key: %w", err)
	}

	return &Bls{
		Key: BlsKey{
			PubKey:       blsPrivKey.PubKey(),
			PrivKey:      blsPrivKey,
			filePath:     keyFilePath,
			passwordPath: passwordFilePath,
		},
	}, true, nil
}

// GetBlsPassword retrieves the BLS password from environment variable or password file.
// Password sources:
// 1. Environment variable (BABYLON_BLS_PASSWORD)
// 2. Password file (if path is provided and file exists)
func GetBlsPassword(passwordFilePath string) (string, error) {
	password, found := GetBlsPasswordFromEnv()
	if found {
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
func GetBlsPasswordFromEnv() (string, bool) {
	return os.LookupEnv(BlsPasswordEnvVar)
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
	if err := tempfile.WriteFileAtomic(k.filePath, jsonBytes, 0400); err != nil {
		panic(fmt.Errorf("failed to write BLS key: %w", err))
	}

	// write password to file
	if k.passwordPath != "" {
		if err := tempfile.WriteFileAtomic(k.passwordPath, []byte(password), 0400); err != nil {
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

// GetBlsKeyPassword is a unified function to handle BLS key password retrieval.
// Returns password if a password was found from one of the sources.
// 1. If noPassword is true, returns empty string without checking other sources.
// 2. Environment variable BABYLON_BLS_PASSWORD
// 3. Password file (ONLY if path is explicitly provided and file exists)
// 4. Interactive prompt, if no other sources are found
//   - For new keys: confirmation flow with matching passwords
//   - For existing keys: single prompt without confirmation
func GetBlsKeyPassword(noPassword bool, passwordFilePath string, isNewKey bool) (string, error) {
	if err := ValidatePasswordMethods(noPassword, passwordFilePath); err != nil {
		return "", err
	}

	// If using no-password mode, return empty password immediately
	if noPassword {
		return "", nil
	}

	// Try getting password from environment
	password, found := GetBlsPasswordFromEnv()
	if found {
		return password, nil
	}

	passwordBytes, err := os.ReadFile(passwordFilePath)
	if err == nil {
		return string(passwordBytes), nil
	}

	// Use appropriate interactive prompt based on whether this is a new key or existing key
	if isNewKey {
		return NewBlsPassword(), nil
	}
	return GetBlsUnlockPasswordFromPrompt(), nil
}

// ValidatePasswordMethods checks if multiple password methods are being used simultaneously.
// Returns an error if more than one method is provided.
func ValidatePasswordMethods(noPassword bool, passwordFilePath string) error {
	_, envFound := GetBlsPasswordFromEnv()
	fileSet := passwordFilePath != ""

	if (noPassword && (envFound || fileSet)) || (envFound && fileSet) {
		return fmt.Errorf("multiple password methods detected (no-password: %v, env var: %v, password file: %v) - please provide only one method",
			noPassword, envFound, fileSet)
	}
	return nil
}

// LoadBlsSignerIfExists attempts to load an existing BLS signer from the specified home directory
// Returns the signer if files exist and can be loaded, or nil if files don't exist
// Possible password sources:
// 1. Empty password passed from flag
// 2. Environment variable (BABYLON_BLS_PASSWORD)
// 3. Custom password file path (ONLY if explicitly provided and file exists)
// 4. Interactive prompt for password
// Error will be returned if the key exists but cannot decrypt it
func LoadBlsSignerIfExists(homeDir string, noPassword bool, customPasswordPath, customKeyPath string) (checkpointingtypes.BlsSigner, error) {
	blsKeyFile := determineKeyFilePath(homeDir, customKeyPath)
	if !cmtos.FileExists(blsKeyFile) {
		return nil, nil
	}

	// Try to get password from all sources
	password, err := GetBlsKeyPassword(noPassword, customPasswordPath, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get password: %w", err)
	}

	blsPrivKey, err := loadBlsPrivKeyFromFile(blsKeyFile, password)
	if err != nil {
		return nil, fmt.Errorf("failed to load BLS key: %w", err)
	}

	return &BlsKey{
		PubKey:       blsPrivKey.PubKey(),
		PrivKey:      blsPrivKey,
		filePath:     blsKeyFile,
		passwordPath: customPasswordPath,
	}, nil
}

// LoadOrGenBlsKey attempts to load an existing BLS signer or creates a new one if none exists.
// If noPassword is true, creates key without password protection.
// Password sources:
// 1. Environment variable (BABYLON_BLS_PASSWORD)
// 2. Password file (ONLY if explicitly provided and file exists)
// 3. Interactive prompt (with confirmation for new keys)
func LoadOrGenBlsKey(homeDir string, noPassword bool, customPasswordPath, customKeyPath string) (checkpointingtypes.BlsSigner, error) {
	blsKeyFile := determineKeyFilePath(homeDir, customKeyPath)
	keyExists := cmtos.FileExists(blsKeyFile)

	if keyExists {
		blsSigner, err := LoadBlsSignerIfExists(homeDir, noPassword, customPasswordPath, customKeyPath)
		if err != nil {
			return nil, fmt.Errorf("BLS key file exists at %s but could not be loaded: %w. If you need to generate a new key, please manually delete the existing file first", blsKeyFile, err)
		}
		if blsSigner != nil {
			return blsSigner, nil
		}
	}

	genPassword, err := GetBlsKeyPassword(noPassword, customPasswordPath, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get password: %w", err)
	}
	bls := GenBls(blsKeyFile, customPasswordPath, genPassword)
	return &bls.Key, nil
}

// determineKeyFilePath returns the appropriate key file path
func determineKeyFilePath(homeDir, customKeyPath string) string {
	if customKeyPath != "" && customKeyPath != defaultBlsKeyFilePath {
		return customKeyPath
	}
	return DefaultBlsKeyFile(homeDir)
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

// ShowBlsKey displays information about a BLS key
// Takes a password that was determined by the password determination logic.
func ShowBlsKey(homeDir string, password string) (map[string]interface{}, error) {
	blsKeyFile := determineKeyFilePath(homeDir, "")
	if !cmtos.FileExists(blsKeyFile) {
		return nil, fmt.Errorf("BLS key file does not exist at %s", blsKeyFile)
	}

	blsPrivKey, err := loadBlsPrivKeyFromFile(blsKeyFile, password)
	if err != nil {
		return nil, fmt.Errorf("failed to load BLS key: %w", err)
	}

	blsPubKey := blsPrivKey.PubKey()

	result := map[string]interface{}{
		"pubkey":        blsPubKey.Bytes(),
		"pubkey_hex":    fmt.Sprintf("%x", blsPubKey.Bytes()),
		"keystore_path": blsKeyFile,
	}

	return result, nil
}

// LoadBlsPrivKey loads a BLS private key from a file.
func LoadBlsPrivKey(homeDir, password string) (bls12381.PrivateKey, error) {
	blsKeyFile := determineKeyFilePath(homeDir, "")

	if !cmtos.FileExists(blsKeyFile) {
		return nil, fmt.Errorf("BLS key file does not exist at %s", blsKeyFile)
	}

	return loadBlsPrivKeyFromFile(blsKeyFile, password)
}

// CreateBlsKey creates a new BLS key
// Takes a password that was determined by the password determination logic.
func CreateBlsKey(homeDir string, password string, passwordFilePath string, cmd *cobra.Command) error {
	blsKeyFile := determineKeyFilePath(homeDir, "")

	// Check if BLS key already exists
	if cmtos.FileExists(blsKeyFile) {
		return fmt.Errorf("BLS key already exists at %s. If you need to generate a new key, please manually delete the existing file first", blsKeyFile)
	}

	// Ensure the key file directory exists
	if err := EnsureDirs(blsKeyFile); err != nil {
		return fmt.Errorf("failed to ensure key directory exists: %w", err)
	}

	// If a password file is specified, ensure its directory exists too
	if passwordFilePath != "" {
		if err := EnsureDirs(passwordFilePath); err != nil {
			return fmt.Errorf("failed to ensure password file directory exists: %w", err)
		}
	}

	// Generate key with provided password
	bls := NewBls(bls12381.GenPrivKey(), blsKeyFile, passwordFilePath)
	bls.Key.Save(password)

	// Print appropriate message based on password source
	if password == "" {
		cmd.Printf("BLS key generated successfully without password protection.\n")
		if passwordFilePath != "" {
			cmd.Printf("An empty password file has been created at for backward compatibility.\n")
		}
	} else {
		cmd.Printf("\n⚠️ IMPORTANT: Your BLS key has been created with password protection. ⚠️\n")
		cmd.Printf("You can provide this password when starting the node using one of these methods:\n")
		cmd.Printf("1. (Recommended) Set the %s environment variable: \nexport %s=<your_password>\n", BlsPasswordEnvVar, BlsPasswordEnvVar)

		if passwordFilePath != "" {
			cmd.Printf("2. The password has been stored in the specified password file. You can use it when starting the node by providing the path to the password file\n")
			cmd.Printf("babylond start --bls-password-file=<path_to_password_file>\n")
		} else {
			cmd.Printf("2. (Not recommended) Create a password file and provide its path when starting the node by specifying the path to the password file.\n")
			cmd.Printf("babylond start --bls-password-file=<path_to_file>\n")
		}
		cmd.Println("3. If you did not specify the password in one of the above methods interactive password prompt will be displayed.")
		cmd.Printf("\nRemember to securely store your password. If you lose it, you won't be able to access your BLS key.\n")
	}

	return nil
}

// UpdateBlsPassword updates the password for a BLS key
// Takes a password that was determined by the password determination logic.
func UpdateBlsPassword(homeDir string, blsPrivKey bls12381.PrivateKey, password string, passwordFilePath string, cmd *cobra.Command) error {
	blsKeyFile := determineKeyFilePath(homeDir, "")

	// Check if BLS key already exists
	if !cmtos.FileExists(blsKeyFile) {
		return fmt.Errorf("BLS key does not exist at %s", blsKeyFile)
	}

	// Create backup of BLS key file before removing it
	backupBlsKeyFile := blsKeyFile + ".bk"
	if err := cmtos.CopyFile(blsKeyFile, backupBlsKeyFile); err != nil {
		return fmt.Errorf("failed to create backup of BLS key file: %w", err)
	}

	// Remove BLS key file
	if err := os.Remove(blsKeyFile); err != nil {
		return fmt.Errorf("failed to remove BLS key file: %w", err)
	}

	// If a password file is specified, ensure its directory exists too
	if passwordFilePath != "" {
		if err := EnsureDirs(passwordFilePath); err != nil {
			return fmt.Errorf("failed to ensure password file directory exists: %w", err)
		}
	}

	// Generate key with provided password
	bls := NewBls(blsPrivKey, blsKeyFile, passwordFilePath)
	bls.Key.Save(password)

	// Remove backup of BLS key file
	if err := os.Remove(backupBlsKeyFile); err != nil {
		return fmt.Errorf("failed to remove backup of BLS key file: %w", err)
	}

	// Print appropriate message based on password source
	if password == "" {
		cmd.Printf("The password for BLS key updated successfully without password protection.\n")
		if passwordFilePath != "" {
			cmd.Printf("An empty password file has been created at for backward compatibility.\n")
		}
	} else {
		cmd.Printf("\n⚠️ IMPORTANT: The password for BLS key has been updated with password protection. ⚠️\n")
		cmd.Printf("You can provide this password when starting the node using one of these methods:\n")
		cmd.Printf("1. (Recommended) Set the %s environment variable: \nexport %s=<new_password>\n", BlsPasswordEnvVar, BlsPasswordEnvVar)

		if passwordFilePath != "" {
			cmd.Printf("2. The new password has been stored in the specified password file. You can use it when starting the node by providing the path to the password file\n")
			cmd.Printf("babylond start --bls-password-file=<path_to_password_file>\n")
		} else {
			cmd.Printf("2. (Not recommended) Create a new password file and provide its path when starting the node by specifying the path to the new password file.\n")
			cmd.Printf("babylond start --bls-password-file=<path_to_file>\n")
		}
		cmd.Println("3. If you did not specify the new password in one of the above methods interactive password prompt will be displayed.")
		cmd.Printf("\nRemember to securely store your new password. If you lose it, you won't be able to access your BLS key.\n")
	}

	return nil
}
