package privval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/crypto/erc2335"
	cmtcfg "github.com/cometbft/cometbft/config"
	cmtos "github.com/cometbft/cometbft/libs/os"
	"github.com/cometbft/cometbft/libs/tempfile"
	"github.com/cosmos/go-bip39"
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

	filePath     string
	passwordPath string
}

// initialize node validator bls key with password
func InitializeBlsFile(config *BlsConfig, password string) (blsPubKey []byte, err error) {
	return InitializeBlsFileFromMnemonic(config, password, "")
}

// initialize node validator bls key with mnemonic and password
func InitializeBlsFileFromMnemonic(config *BlsConfig, password, mnemonic string) (blsPubKey []byte, err error) {
	if len(mnemonic) > 0 && !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid mnemonic")
	}

	blsKeyFile := config.BlsKeyFile()
	if err := os.MkdirAll(filepath.Dir(blsKeyFile), 0o777); err != nil {
		return nil, fmt.Errorf("could not create directory for bls key %q: %w", filepath.Dir(blsKeyFile), err)
	}

	blsPasswordFile := config.BlsPasswordFile()
	if err := os.MkdirAll(filepath.Dir(blsPasswordFile), 0o777); err != nil {
		return nil, fmt.Errorf("could not create directory for bls password %q: %w", filepath.Dir(blsPasswordFile), err)
	}

	// var blsPv *BlsPV
	var privKey bls12381.PrivateKey
	if len(mnemonic) == 0 {
		privKey = bls12381.GenPrivKey()
	} else {
		privKey = bls12381.GenPrivKeyFromSecret([]byte(mnemonic))
	}

	blsPv := NewBlsPV(privKey, blsKeyFile, blsPasswordFile)
	blsPv.Save(password)
	return privKey.PubKey().Bytes(), nil
}

func NewBlsPV(privKey bls12381.PrivateKey, keyFilePath, passwordFilePath string) *BlsPV {
	return &BlsPV{
		Key: BlsPVKey{
			PubKey:       privKey.PubKey(),
			PrivKey:      privKey,
			filePath:     keyFilePath,
			passwordPath: passwordFilePath,
		},
	}
}

func (pv *BlsPV) Save(password string) {
	pv.Key.Save(password)
}

func (pvKey *BlsPVKey) Save(password string) {

	passwordOutFile := pvKey.passwordPath
	if passwordOutFile == "" {
		panic("cannot save PrivValidator BLS key: password filePath not set")
	}

	// save password to file
	err := erc2335.SavePasswordToFile(password, passwordOutFile)
	if err != nil {
		panic(err)
	}

	keyOutFile := pvKey.filePath
	if keyOutFile == "" {
		panic("cannot save PrivValidator BLS key: filePath not set")
	}

	// encrypt the bls12381 key to erc2335 type
	erc2335BlsPvKey, err := erc2335.Encrypt(pvKey.PrivKey, pvKey.PubKey.Bytes(), password)
	if err != nil {
		panic(err)
	}

	// Parse the encrypted key back to Erc2335KeyStore structure
	var keystore erc2335.Erc2335KeyStore
	if err := json.Unmarshal(erc2335BlsPvKey, &keystore); err != nil {
		panic(err)
	}

	jsonBytes, err := json.MarshalIndent(keystore, "", "  ")
	if err != nil {
		panic(err)
	}

	if err := tempfile.WriteFileAtomic(keyOutFile, jsonBytes, 0600); err != nil {
		panic(err)
	}
}

func LoadBlsPV(keyFilePath, passwordFilePath string) *BlsPV {

	password, err := erc2335.LoadPaswordFromFile(passwordFilePath)
	if err != nil {
		cmtos.Exit(fmt.Sprintf("failed to read BLS password file: %v", err.Error()))
	}

	keyJSONBytes, err := os.ReadFile(keyFilePath)
	if err != nil {
		cmtos.Exit(fmt.Sprintf("failed to read BLS file: %v", err.Error()))
	}

	// decrypt bls key from erc2335 type of structure
	privKey, err := erc2335.Decrypt(keyJSONBytes, password)
	if err != nil {
		cmtos.Exit(fmt.Sprintf("failed to decrypt BLS key: %v", err.Error()))
	}

	blsPrivKey := bls12381.PrivateKey(privKey)

	return &BlsPV{
		Key: BlsPVKey{
			PubKey:   blsPrivKey.PubKey(),
			PrivKey:  blsPrivKey,
			filePath: keyFilePath,
		},
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
