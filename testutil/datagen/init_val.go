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

	pvKeyFile := config.PrivValidatorKeyFile()
	pvStateFile := config.PrivValidatorStateFile()

	if err := privval.IsValidFilePath(pvKeyFile, pvStateFile); err != nil {
		return "", nil, err
	}

	// bls config
	blsCfg := privval.DefaultBlsConfig()
	blsCfg.SetRoot(config.RootDir)

	blsKeyFile := blsCfg.BlsKeyFile()
	blsPasswordFile := blsCfg.BlsPasswordFile()
	if err := privval.IsValidFilePath(blsKeyFile, blsPasswordFile); err != nil {
		return "", nil, err
	}

	// load or generate private validator
	var filePV *cmtprivval.FilePV
	if cmtos.FileExists(pvKeyFile) {
		filePV = cmtprivval.LoadFilePV(pvKeyFile, pvStateFile)
	} else {
		var privKey ed25519.PrivKey
		if len(mnemonic) == 0 {
			privKey = ed25519.GenPrivKey()
		} else {
			privKey = ed25519.GenPrivKeyFromSecret([]byte(mnemonic))
		}
		filePV = cmtprivval.NewFilePV(privKey, pvKeyFile, pvStateFile)
		filePV.Key.Save()
		filePV.LastSignState.Save()
	}

	// load or generate BLS private validator
	var blsPV *privval.BlsPV
	if cmtos.FileExists(blsKeyFile) {
		// if key file exists but password file does not exist -> error
		if !cmtos.FileExists(blsPasswordFile) {
			cmtos.Exit(fmt.Sprintf("BLS password file does not exist: %v", blsPasswordFile))
		}
		blsPV = privval.LoadBlsPV(blsKeyFile, blsPasswordFile)
	} else {
		blsPV = privval.GenBlsPV(blsKeyFile, blsPasswordFile, "password", addr.String())
	}

	valKeys, err = privval.NewValidatorKeys(filePV.Key.PrivKey, blsPV.Key.PrivKey)
	if err != nil {
		return "", nil, err
	}

	return nodeID, valKeys, nil
}
