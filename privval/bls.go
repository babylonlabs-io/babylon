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

const DefaultBlsKeyName = "bls_key.json"

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
	privKey, err := erc2335.DecryptBLS(keyJSONBytes, password)
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

func (pv *BlsPV) Save(password string) {
	pv.Key.Save(password)
}

func (k *BlsPVKey) Save(password string) {
	outFile := k.filePath
	if outFile == "" {
		panic("cannot save PrivValidator BLS key: filePath not set")
	}

	// encrypt the bls12381 key to erc2335 type
	erc2335BlsPvKey, err := erc2335.EncryptBLS(k.PrivKey, k.PubKey.Bytes(), password)
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

func InitializeNodeValidatorBlsFiles(config *BlsConfig, password string) (blsPubKey []byte, err error) {
	return InitializeNodeValidatorBlsFilesFromMnemonic(config, "", password)
}

func InitializeNodeValidatorBlsFilesFromMnemonic(config *BlsConfig, mnemonic, password string) (blsPubKey []byte, err error) {
	if len(mnemonic) > 0 && !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid mnemonic")
	}

	blsKeyFile := config.BlsKeyFile()
	if err := cmtos.EnsureDir(filepath.Dir(blsKeyFile), 0777); err != nil {
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
	RootDir    string `mapstructure:"home"`
	BlsKeyPath string `mapstructure:"bls_key_file"`
}

func DefaultBlsConfig() BlsConfig {
	return BlsConfig{
		RootDir:    cmtcfg.DefaultDataDir,
		BlsKeyPath: filepath.Join(cmtcfg.DefaultDataDir, DefaultBlsKeyName),
	}
}

func SetBlsConfig(rootDir, blsKeyPath string) BlsConfig {
	return BlsConfig{
		RootDir:    rootDir,
		BlsKeyPath: blsKeyPath,
	}
}

func (cfg BlsConfig) BlsKeyFile() string {
	if filepath.IsAbs(cfg.BlsKeyPath) {
		return cfg.BlsKeyPath
	}
	return filepath.Join(cfg.RootDir, cfg.BlsKeyPath)
}
