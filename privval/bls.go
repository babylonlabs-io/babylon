package privval

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/crypto/erc2335"
	cmtcfg "github.com/cometbft/cometbft/config"
	cmtos "github.com/cometbft/cometbft/libs/os"
	"github.com/cometbft/cometbft/libs/tempfile"
	"github.com/cosmos/cosmos-sdk/client/input"
)

const (
	DefaultBlsKeyName      = "bls_key.json"     // Default file name for BLS key
	DefaultBlsPasswordName = "bls_password.txt" // Default file name for BLS password
)

var (
	defaultBlsKeyFilePath  = filepath.Join(cmtcfg.DefaultConfigDir, DefaultBlsKeyName)      // Default file path for BLS key
	defaultBlsPasswordPath = filepath.Join(cmtcfg.DefaultConfigDir, DefaultBlsPasswordName) // Default file path for BLS password
)

// BlsPV is a wrapper around BlsPVKey
type BlsPV struct {
	// Key is a structure containing bls12381 keys,
	// paths of both key and password files,
	// and delegator address
	Key BlsPVKey
}

// BlsPVKey is a wrapper containing bls12381 keys,
// paths of both key and password files, and delegator address.
type BlsPVKey struct {
	PubKey           bls12381.PublicKey  `json:"bls_pub_key"`       // Public Key of BLS
	PrivKey          bls12381.PrivateKey `json:"bls_priv_key"`      // Private Key of BLS
	DelegatorAddress string              `json:"delegator_address"` // Delegate Address
	filePath         string              // File Path of BLS Key
	passwordPath     string              // File Path of BLS Password
}

// NewBlsPV returns a new BlsPV.
func NewBlsPV(privKey bls12381.PrivateKey, keyFilePath, passwordFilePath, delegatorAddress string) *BlsPV {
	return &BlsPV{
		Key: BlsPVKey{
			PubKey:           privKey.PubKey(),
			PrivKey:          privKey,
			DelegatorAddress: delegatorAddress,
			filePath:         keyFilePath,
			passwordPath:     passwordFilePath,
		},
	}
}

// GenBlsPV returns a new BlsPV after saving it to the file.
func GenBlsPV(keyFilePath, passwordFilePath, password, delegatorAddress string) *BlsPV {
	pv := NewBlsPV(bls12381.GenPrivKey(), keyFilePath, passwordFilePath, delegatorAddress)
	pv.Key.Save(password, delegatorAddress)
	return pv
}

// LoadBlsPV returns a BlsPV after loading the erc2335 type of structure
// from the file and decrypt it using a password.
func LoadBlsPV(keyFilePath, passwordFilePath string) *BlsPV {
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
	return &BlsPV{
		Key: BlsPVKey{
			PubKey:           blsPrivKey.PubKey(),
			PrivKey:          blsPrivKey,
			DelegatorAddress: keystore.Description,
			filePath:         keyFilePath,
			passwordPath:     passwordFilePath,
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
func (k *BlsPVKey) Save(password, addr string) {
	// encrypt the bls12381 key to erc2335 type
	erc2335BlsPvKey, err := erc2335.Encrypt(k.PrivKey, k.PubKey.Bytes(), password)
	if err != nil {
		panic(err)
	}

	// Parse the encrypted key back to Erc2335KeyStore structure
	var keystore erc2335.Erc2335KeyStore
	if err := json.Unmarshal(erc2335BlsPvKey, &keystore); err != nil {
		panic(err)
	}

	// save the delegator address to description field
	keystore.Description = addr

	// convert keystore to json
	jsonBytes, err := json.MarshalIndent(keystore, "", "  ")
	if err != nil {
		panic(err)
	}

	// write generated erc2335 keystore to file
	if err := tempfile.WriteFileAtomic(k.filePath, jsonBytes, 0600); err != nil {
		panic(err)
	}

	// save used password to file
	if err := tempfile.WriteFileAtomic(k.passwordPath, []byte(password), 0600); err != nil {
		panic(err)
	}
}

// DefaultBlsKeyFile returns the default BLS key file path.
func DefaultBlsKeyFile(home string) string {
	return filepath.Join(home, defaultBlsKeyFilePath)
}

// DefaultBlsPasswordFile returns the default BLS password file path.
func DefaultBlsPasswordFile(home string) string {
	return filepath.Join(home, defaultBlsPasswordPath)
}
