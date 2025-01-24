package datagen

import (
	"fmt"

	cfg "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/p2p"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/go-bip39"

	"github.com/babylonlabs-io/babylon/privval"
	cmtos "github.com/cometbft/cometbft/libs/os"
	cmtprivval "github.com/cometbft/cometbft/privval"
)

// InitializeNodeValidatorFiles creates private validator and p2p configuration files.
func InitializeNodeValidatorFiles(config *cfg.Config, addr sdk.AccAddress) (string, *privval.ValidatorKeys, error) {
	return InitializeNodeValidatorFilesFromMnemonic(config, "", addr)
}

func InitializeNodeValidatorFilesFromMnemonic(config *cfg.Config, mnemonic string, addr sdk.AccAddress) (nodeID string, valKeys *privval.ValidatorKeys, err error) {
	if len(mnemonic) > 0 && !bip39.IsMnemonicValid(mnemonic) {
		return "", nil, fmt.Errorf("invalid mnemonic")
	}

	nodeKey, err := p2p.LoadOrGenNodeKey(config.NodeKeyFile())
	if err != nil {
		return "", nil, err
	}

	nodeID = string(nodeKey.ID())

	cmtKeyFile := config.PrivValidatorKeyFile()
	cmtStateFile := config.PrivValidatorStateFile()
	blsKeyFile := privval.DefaultBlsKeyFile(config.RootDir)
	blsPasswordFile := privval.DefaultBlsPasswordFile(config.RootDir)
	if err := privval.EnsureDirs(cmtKeyFile, cmtStateFile, blsKeyFile, blsPasswordFile); err != nil {
		return "", nil, fmt.Errorf("failed to ensure dirs: %w", err)
	}

	var filePV *cmtprivval.FilePV
	if cmtos.FileExists(cmtKeyFile) {
		filePV = cmtprivval.LoadFilePV(cmtKeyFile, cmtStateFile)
	} else {
		var privKey ed25519.PrivKey
		if len(mnemonic) == 0 {
			privKey = ed25519.GenPrivKey()
		} else {
			privKey = ed25519.GenPrivKeyFromSecret([]byte(mnemonic))
		}
		filePV = cmtprivval.NewFilePV(privKey, cmtKeyFile, cmtStateFile)
		filePV.Key.Save()
		filePV.LastSignState.Save()
	}

	var blsPV *privval.BlsPV
	if cmtos.FileExists(blsKeyFile) {
		// if key file exists but password file does not exist -> error
		if !cmtos.FileExists(blsPasswordFile) {
			cmtos.Exit(fmt.Sprintf("BLS password file does not exist: %v", blsPasswordFile))
		}
		blsPV = privval.LoadBlsPV(blsKeyFile, blsPasswordFile)
	} else {
		blsPV = privval.GenBlsPV(blsKeyFile, blsPasswordFile, "password")
	}

	valKeys, err = privval.NewValidatorKeys(filePV.Key.PrivKey, blsPV.Key.PrivKey)
	if err != nil {
		return "", nil, err
	}

	return nodeID, valKeys, nil
}
