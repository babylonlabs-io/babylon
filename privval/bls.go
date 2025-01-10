package privval

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/crypto/erc2335"
	cmtcfg "github.com/cometbft/cometbft/config"
	cmtos "github.com/cometbft/cometbft/libs/os"
	"github.com/cometbft/cometbft/libs/tempfile"
	"github.com/cosmos/cosmos-sdk/client/input"
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
	PubKey   bls12381.PublicKey
	PrivKey  bls12381.PrivateKey
	filePath string
}

// todo: where
func NewBlsPV(privKey bls12381.PrivateKey, keyFilePath string) *BlsPV {
	return &BlsPV{
		Key: BlsPVKey{
			PubKey:   privKey.PubKey(),
			PrivKey:  privKey,
			filePath: keyFilePath,
		},
	}
}

func GenBlsPV(keyFilePath string) *BlsPV {
	return NewBlsPV(bls12381.GenPrivKey(), keyFilePath)
}

func LoadBlsPV(keyFilePath, password string) *BlsPV {

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

func LoadOrGenBlsPV(keyFilePath, password string) *BlsPV {

	var pv *BlsPV
	if cmtos.FileExists(keyFilePath) {
		pv = LoadBlsPV(keyFilePath, password)
	} else {
		pv = GenBlsPV(keyFilePath)
		pv.Save(password)
	}
	return pv
}

func LoadOrGenBlsPassword(passwordFilePath string) string {
	password, isExist, err := LoadBlsPassword(passwordFilePath)
	if err != nil {
		cmtos.Exit(fmt.Sprintf("failed to read BLS password file: %v", err.Error()))
	}

	if !isExist {
		newPassword, err := GenBlsPassword()
		if err != nil {
			cmtos.Exit(fmt.Sprintf("failed to generate BLS password: %v", err.Error()))
		}
		err = erc2335.SavePasswordToFile(newPassword, passwordFilePath)
		if err != nil {
			cmtos.Exit(fmt.Sprintf("failed to save BLS password file: %v", err.Error()))
		}
		log.Print("BLS password saved to ", passwordFilePath)
		return newPassword
	}
	return password
}

func LoadBlsPassword(passwordFilePath string) (string, bool, error) {
	if cmtos.FileExists(passwordFilePath) {
		password, err := os.ReadFile(passwordFilePath)
		if err != nil {
			return "", true, fmt.Errorf("failed to read BLS password file: %v", err.Error())
		}
		return string(password), true, nil
	}
	return "", false, nil
}

func GenRandomBlsPassword() string {
	return erc2335.CreateRandomPassword()
}

func GenBlsPassword() (string, error) {
	inBuf := bufio.NewReader(os.Stdin)
	return input.GetString("Enter your bls password", inBuf)
}

func (pv *BlsPV) Save(password string) {
	pv.Key.Save(password)
}

func (pvKey *BlsPVKey) Save(password string) {
	outFile := pvKey.filePath
	if outFile == "" {
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

	if err := tempfile.WriteFileAtomic(outFile, jsonBytes, 0600); err != nil {
		panic(err)
	}
}

// initialize node validator bls key with random password
func InitializeNodeValidatorBlsFiles(config *BlsConfig) (blsPubKey []byte, err error) {
	password := erc2335.CreateRandomPassword()
	return InitializeNodeValidatorBlsFilesFromMnemonic(config, "", password)
}

// initialize node validator bls key with specific password
// if you want to generate a random password,
// use 'CreateRandomBlsPassword' to generate a random password and pass it to this function
func InitializeNodeValidatorBlsFilesWithPassword(config *BlsConfig, password string) (blsPubKey []byte, err error) {
	return InitializeNodeValidatorBlsFilesFromMnemonic(config, "", password)
}

// initialize node validator bls key with mnemonic and specific password
// if you want to generate a random password,
// use 'CreateRandomBlsPassword' to generate a random password and pass it to this function
func InitializeNodeValidatorBlsFilesFromMnemonic(config *BlsConfig, mnemonic, password string) (blsPubKey []byte, err error) {
	if len(mnemonic) > 0 && !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid mnemonic")
	}

	blsKeyFile := config.BlsKeyFile()
	if err := os.MkdirAll(filepath.Dir(blsKeyFile), 0o777); err != nil {
		return nil, fmt.Errorf("could not create directory %q: %w", filepath.Dir(blsKeyFile), err)
	}

	var blsPv *BlsPV
	if len(mnemonic) == 0 {
		blsPv = LoadOrGenBlsPV(blsKeyFile, password)
	} else {
		privKey := bls12381.GenPrivKeyFromSecret([]byte(mnemonic))
		blsPv = NewBlsPV(privKey, blsKeyFile)
		blsPv.Save(password)
	}

	return blsPv.Key.PubKey.Bytes(), nil
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
		RootDir:         cmtcfg.DefaultConfigDir,
		BlsKeyPath:      filepath.Join(cmtcfg.DefaultConfigDir, DefaultBlsKeyName),
		BlsPasswordPath: filepath.Join(cmtcfg.DefaultConfigDir, DefaultBlsPasswordName),
	}
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
