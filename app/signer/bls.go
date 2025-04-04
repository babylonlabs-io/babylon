package signer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
// Returns error if the key file exists, but cannot get password or the key cannot
// be decrypted
func TryLoadBlsFromFile(keyFilePath, passwordFilePath string) (*Bls, error) {
	keystore, err := erc2335.LoadKeyStore(keyFilePath)
	if err != nil {
		//nolint:nilerr
		return nil, nil
	}

	password, err := GetBlsPassword(passwordFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get password")
	}

	privKey, err := erc2335.Decrypt(keystore, password)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt BLS key: %w", err)
	}

	blsPrivKey := bls12381.PrivateKey(privKey)
	if err := blsPrivKey.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("invalid BLS private key: %w", err)
	}

	return &Bls{
		Key: BlsKey{
			PubKey:       blsPrivKey.PubKey(),
			PrivKey:      blsPrivKey,
			filePath:     keyFilePath,
			passwordPath: passwordFilePath,
		},
	}, nil
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

// GetBlsKeyPassword is a unified function to handle BLS key password retrieval with proper priority:
// 1. Environment variable BABYLON_BLS_PASSWORD
// 2. Explicit password provided as argument
// 3. Password file (ONLY if path is explicitly provided and file exists)
//
// If noPassword is true, returns empty string without checking other sources.
// Returns password and a boolean indicating if a password was found from one of the sources.
func GetBlsKeyPassword(noPassword bool, explicitPassword, passwordFilePath string) (string, bool) {
	// If using no-password mode, return empty password immediately
	if noPassword {
		return "", true
	}

	// 1. Try environment variable first
	password := GetBlsPasswordFromEnv()
	if password != "" {
		return password, true
	}

	// 2. Use explicit password if provided
	if explicitPassword != "" {
		return explicitPassword, true
	}

	// 3. Try password file ONLY if path is explicitly provided and file exists
	// Important: Don't default to any location if passwordFilePath is empty
	if passwordFilePath != "" && cmtos.FileExists(passwordFilePath) {
		passwordBytes, err := os.ReadFile(passwordFilePath)
		if err == nil {
			return string(passwordBytes), true
		}
	}

	// No password found from any source
	return "", false
}

// ValidatePasswordMethods checks if multiple password methods are being used simultaneously.
// Returns an error if more than one method is provided.
// noPassword is a special mode that overrides all other methods if set to true.
func ValidatePasswordMethods(noPassword bool, explicitPassword string, passwordFilePath string) error {
	// Collect the active password methods for a more descriptive error message
	var activeMethods []string

	// Check each password method
	if noPassword {
		activeMethods = append(activeMethods, "--no-bls-password flag")
	}

	if envPassword := GetBlsPasswordFromEnv(); envPassword != "" {
		activeMethods = append(activeMethods, fmt.Sprintf("%s environment variable", BlsPasswordEnvVar))
	}

	if explicitPassword != "" {
		activeMethods = append(activeMethods, "--insecure-bls-password flag")
	}

	if passwordFilePath != "" && cmtos.FileExists(passwordFilePath) {
		activeMethods = append(activeMethods, fmt.Sprintf("--bls-password-file flag (file: %s)", passwordFilePath))
	}

	// If more than one method is provided, return an error with the specific active methods
	if len(activeMethods) > 1 {
		return fmt.Errorf("multiple password methods detected (%d): %s - please provide only one method",
			len(activeMethods), strings.Join(activeMethods, ", "))
	}

	return nil
}

// LoadBlsSignerIfExists attempts to load an existing BLS signer from the specified home directory
// Returns the signer if files exist and can be loaded, or nil if files don't exist
// Password precedence:
// 1. Environment variable (BABYLON_BLS_PASSWORD)
// 2. Explicit password (if provided)
// 3. Custom password file path (ONLY if explicitly provided and file exists)
// 4. Prompt for password
// Error will be returned if the key exists but cannot decrypt it
func LoadBlsSignerIfExists(homeDir string, noPassword bool, explicitPassword, customPasswordPath, customKeyPath string) (checkpointingtypes.BlsSigner, error) {
	blsKeyFile := determineKeyFilePath(homeDir, customKeyPath)
	if !cmtos.FileExists(blsKeyFile) {
		return nil, nil
	}

	passwordPath := determinePasswordFilePath(customPasswordPath)

	// Try to load keystore first
	keystore, err := erc2335.LoadKeyStore(blsKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load BLS key file: %w", err)
	}

	// Try to get password from all sources
	password, found := GetBlsKeyPassword(noPassword, explicitPassword, passwordPath)
	if !found && !noPassword {
		// No password found from standard sources, use unlock prompt
		password = GetBlsUnlockPassword()
	}

	if password == "" && !noPassword {
		return nil, fmt.Errorf("could not obtain BLS password from any source")
	}

	privKey, err := erc2335.Decrypt(keystore, password)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt BLS key: %w", err)
	}

	blsPrivKey := bls12381.PrivateKey(privKey)
	if err := blsPrivKey.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("invalid BLS private key: %w", err)
	}

	return &BlsKey{
		PubKey:       blsPrivKey.PubKey(),
		PrivKey:      blsPrivKey,
		filePath:     blsKeyFile,
		passwordPath: passwordPath,
	}, nil
}

// LoadOrGenBlsKey attempts to load an existing BLS signer or creates a new one if none exists.
// If noPassword is true, creates key without password protection.
// Password precedence:
// 1. Environment variable (BABYLON_BLS_PASSWORD)
// 2. Explicit password provided as argument
// 3. Password file (ONLY if explicitly provided and file exists)
// 4. Interactive prompt (with confirmation for new keys)
func LoadOrGenBlsKey(homeDir string, noPassword bool, explicitPassword string, customPasswordPath, customKeyPath string) (checkpointingtypes.BlsSigner, error) {
	// Validate that only one password method is provided
	if err := ValidatePasswordMethods(noPassword, explicitPassword, customPasswordPath); err != nil {
		return nil, err
	}

	blsKeyFile := determineKeyFilePath(homeDir, customKeyPath)
	blsPasswordFile := determinePasswordFilePath(customPasswordPath)

	keyExists := cmtos.FileExists(blsKeyFile)

	if keyExists {
		blsSigner, err := LoadBlsSignerIfExists(homeDir, noPassword, explicitPassword, customPasswordPath, customKeyPath)
		if err != nil {
			return nil, fmt.Errorf("BLS key file exists at %s but could not be loaded: %w. If you need to generate a new key, please manually delete the existing file first", blsKeyFile, err)
		}
		if blsSigner != nil {
			return blsSigner, nil
		}
	}

	var genPassword string
	if noPassword {
		genPassword = ""
	} else {
		var found bool
		genPassword, found = GetBlsKeyPassword(false, explicitPassword, blsPasswordFile)

		// If we couldn't get a password from env/flag/file, use NewBlsPassword for confirmation flow
		if !found {
			fmt.Println("Enter a secure password for your BLS key (you'll be asked to confirm)")
			genPassword = NewBlsPassword()
		}
	}

	// Create password file if explicitly requested (non-empty path) and not in no-password mode
	passwordFilePath := ""
	if blsPasswordFile != "" && !noPassword {
		passwordFilePath = blsPasswordFile
	}

	bls := GenBls(blsKeyFile, passwordFilePath, genPassword)
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
// ONLY if a custom password path is explicitly provided
func determinePasswordFilePath(customPasswordPath string) string {
	if customPasswordPath != "" {
		return customPasswordPath
	}
	// Return empty string instead of defaulting to the default password file
	return ""
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

// ShowBlsKey decrypts and returns public key information from an existing BLS key file
// Password precedence follows the same order as other functions:
// 1. Environment variable (BABYLON_BLS_PASSWORD)
// 2. Explicit password provided as argument
// 3. Password file (ONLY if explicitly provided and file exists)
// 4. Interactive prompt (single prompt)
func ShowBlsKey(homeDir string, noPassword bool, explicitPassword string, customPasswordPath, customKeyPath string) (map[string]interface{}, error) {
	// Validate that only one password method is provided
	if err := ValidatePasswordMethods(noPassword, explicitPassword, customPasswordPath); err != nil {
		return nil, err
	}

	blsKeyFile := determineKeyFilePath(homeDir, customKeyPath)
	if !cmtos.FileExists(blsKeyFile) {
		return nil, fmt.Errorf("BLS key file does not exist at %s", blsKeyFile)
	}

	// Determine password file path without defaulting if empty
	passwordPath := determinePasswordFilePath(customPasswordPath)

	// Load keystore first to check if we can open the file
	keystore, err := erc2335.LoadKeyStore(blsKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load BLS key file: %w", err)
	}

	// Gather information about password sources tried
	var passwordSourcesInfo []string

	// Check environment variable first
	envVarSet := GetBlsPasswordFromEnv() != ""
	if envVarSet {
		passwordSourcesInfo = append(passwordSourcesInfo, "environment variable")
	}

	// Check explicit password flag
	explicitPasswordProvided := explicitPassword != ""
	if explicitPasswordProvided {
		passwordSourcesInfo = append(passwordSourcesInfo, "explicit password flag")
	}

	// Check password file
	passwordFileExists := passwordPath != "" && cmtos.FileExists(passwordPath)
	if passwordFileExists {
		passwordSourcesInfo = append(passwordSourcesInfo, fmt.Sprintf("password file (%s)", passwordPath))
	}

	// Get password from all possible sources
	pass, found := GetBlsKeyPassword(noPassword, explicitPassword, passwordPath)

	var promptUsed bool
	if !found && !noPassword {
		// If no password found from standard sources, use unlock prompt
		passwordSourcesInfo = append(passwordSourcesInfo, "interactive prompt")
		promptUsed = true
		pass = GetBlsUnlockPassword()
	}

	if pass == "" && !noPassword {
		return nil, fmt.Errorf("could not obtain BLS password from any source. Sources tried: %s", strings.Join(passwordSourcesInfo, ", "))
	}

	// Try to decrypt the key with the obtained password
	privKey, err := erc2335.Decrypt(keystore, pass)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt BLS key (incorrect password): %w", err)
	}

	blsPrivKey := bls12381.PrivateKey(privKey)
	if err := blsPrivKey.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("invalid BLS private key: %w", err)
	}

	blsPubKey := blsPrivKey.PubKey()

	var passwordType string
	switch {
	case noPassword:
		passwordType = "none (unencrypted key)"
	case GetBlsPasswordFromEnv() != "":
		passwordType = "environment variable"
	case explicitPassword != "":
		passwordType = "provided as parameter"
	case passwordFileExists && found:
		passwordType = fmt.Sprintf("from file (%s)", passwordPath)
	case promptUsed:
		passwordType = "from prompt"
	default:
		passwordType = "unknown source"
	}

	result := map[string]interface{}{
		"pubkey":           blsPubKey.Bytes(),
		"pubkey_hex":       fmt.Sprintf("%x", blsPubKey.Bytes()),
		"keystore_path":    blsKeyFile,
		"password_path":    passwordPath,
		"password_type":    passwordType,
		"password_sources": passwordSourcesInfo,
	}

	return result, nil
}

// CreateBlsKey creates a new BLS key with proper password handling and user feedback.
// This is a common function used by both InitCmd and CreateBlsKeyCmd.
func CreateBlsKey(homeDir string, noBlsPassword bool, explicitPassword, passwordFile string) error {
	blsKeyFile := determineKeyFilePath(homeDir, "")

	// Check if BLS key already exists
	if cmtos.FileExists(blsKeyFile) {
		return fmt.Errorf("BLS key already exists at %s. If you need to generate a new key, please manually delete the existing file first", blsKeyFile)
	}

	// Validate that only one password method is provided
	if err := ValidatePasswordMethods(noBlsPassword, explicitPassword, passwordFile); err != nil {
		return err
	}

	// Determine appropriate password file path
	blsPasswordFile := ""
	if passwordFile != "" {
		blsPasswordFile = passwordFile
	}

	// Ensure the key file directory exists
	if err := EnsureDirs(blsKeyFile); err != nil {
		return fmt.Errorf("failed to ensure key directory exists: %w", err)
	}

	// If a password file is specified, ensure its directory exists too
	if blsPasswordFile != "" {
		if err := EnsureDirs(blsPasswordFile); err != nil {
			return fmt.Errorf("failed to ensure password file directory exists: %w", err)
		}
	}

	if noBlsPassword {
		// Generate BLS key without password protection
		bls := NewBls(bls12381.GenPrivKey(), blsKeyFile, blsPasswordFile)
		bls.Key.Save("")
		fmt.Printf("BLS key generated successfully without password protection.\n")
		if blsPasswordFile != "" {
			fmt.Printf("An empty password file has been created at %s for backward compatibility.\n", blsPasswordFile)
		}
		return nil
	}

	// Get password with proper priority order (env var, flag, file)
	password, found := GetBlsKeyPassword(false, explicitPassword, blsPasswordFile)

	// If no password was found from other sources, use NewBlsPassword for confirmation flow
	if !found {
		password = NewBlsPassword()
	}

	// Generate key with password protection
	bls := NewBls(bls12381.GenPrivKey(), blsKeyFile, blsPasswordFile)
	bls.Key.Save(password)

	fmt.Printf("\nIMPORTANT: Your BLS key has been created with password protection.\n")
	fmt.Printf("You must provide this password when starting the node using one of these methods:\n")
	fmt.Printf("1. (Recommended) Set the %s environment variable:\n", BlsPasswordEnvVar)
	fmt.Printf("export %s=<your_password>\n", BlsPasswordEnvVar)

	if blsPasswordFile != "" {
		fmt.Printf("2. The password has been stored in %s. You can use it when starting the node:\n", blsPasswordFile)
		fmt.Printf("babylond start --bls-password-file=%s\n", blsPasswordFile)
	} else {
		fmt.Printf("2. (Not recommended) Create a password file and provide its path when starting the node:\n")
		fmt.Printf("babylond start --bls-password-file=<path_to_file>\n")
	}

	fmt.Printf("\nRemember to securely store your password. If you lose it, you won't be able to access your BLS key.\n")

	return nil
}
