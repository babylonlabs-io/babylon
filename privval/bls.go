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
	DefaultBlsKeyName      = "bls_key.json"
	DefaultBlsPasswordName = "bls_password.txt"
)

type BlsPV struct {
	Key BlsPVKey
}

type BlsPVKey struct {
	PubKey  bls12381.PublicKey  `json:"bls_pub_key"`
	PrivKey bls12381.PrivateKey `json:"bls_priv_key"`

	DelegatorAddress string

	filePath     string
	passwordPath string
}

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

func GenBlsPV(keyFilePath, passwordFilePath, password, delegatorAddress string) *BlsPV {
	pv := NewBlsPV(bls12381.GenPrivKey(), keyFilePath, passwordFilePath, delegatorAddress)
	pv.Key.Save(password, delegatorAddress)
	return pv
}

func LoadBlsPV(keyFilePath, passwordFilePath string) *BlsPV {
	password, err := erc2335.LoadPaswordFromFile(passwordFilePath)
	if err != nil {
		cmtos.Exit(fmt.Sprintf("failed to read BLS password file: %v", err.Error()))
	}

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

func GetBlsPassword() string {
	inBuf := bufio.NewReader(os.Stdin)
	password, err := input.GetString("Enter your bls password", inBuf)
	if err != nil {
		cmtos.Exit("failed to get BLS password")
	}
	return password
}

// Save bls key using password
// Check both paths of bls key and password inside function
func (k *BlsPVKey) Save(password, addr string) {
	// check file path is valid
	if err := IsValidFilePath(k.filePath, k.passwordPath); err != nil {
		panic(err)
	}

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
	err = erc2335.SavePasswordToFile(password, k.passwordPath)
	if err != nil {
		panic(err)
	}
}

// -------------------------------------------------------------------------------
// ---------------------------- BLS Config ---------------------------------------
// -------------------------------------------------------------------------------

type BlsConfig struct {
	RootDir         string `mapstructure:"home"`
	BlsKeyPath      string `mapstructure:"bls_key_file"`
	BlsPasswordPath string `mapstructure:"bls_password_file"`
}

func DefaultBlsConfig() BlsConfig {
	return BlsConfig{
		BlsKeyPath:      filepath.Join(cmtcfg.DefaultConfigDir, DefaultBlsKeyName),
		BlsPasswordPath: filepath.Join(cmtcfg.DefaultConfigDir, DefaultBlsPasswordName),
	}
}

func (cfg *BlsConfig) SetRoot(root string) *BlsConfig {
	cfg.RootDir = root
	return cfg
}

func (cfg BlsConfig) BlsKeyFile() string {
	return rootify(cfg.BlsKeyPath, cfg.RootDir)
}

func (cfg BlsConfig) BlsPasswordFile() string {
	return rootify(cfg.BlsPasswordPath, cfg.RootDir)
}

// helper function to make config creation independent of root dir
// copied from https://github.com/cometbft/cometbft/blob/v0.38.15/config/config.go
func rootify(path, root string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}
