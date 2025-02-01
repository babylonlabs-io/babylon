package datagen

import (
	"fmt"

	cfg "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/p2p"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/go-bip39"

	appsigner "github.com/babylonlabs-io/babylon/app/signer"
	cmtos "github.com/cometbft/cometbft/libs/os"
	"github.com/cometbft/cometbft/privval"
)

// InitializeNodeValidatorFiles creates private validator and p2p configuration files.
func InitializeNodeValidatorFiles(config *cfg.Config, addr sdk.AccAddress) (string, *appsigner.ValidatorKeys, error) {
	return InitializeNodeValidatorFilesFromMnemonic(config, "", addr)
}

func InitializeNodeValidatorFilesFromMnemonic(config *cfg.Config, mnemonic string, addr sdk.AccAddress) (nodeID string, valKeys *appsigner.ValidatorKeys, err error) {
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
	blsKeyFile := appsigner.DefaultBlsKeyFile(config.RootDir)
	blsPasswordFile := appsigner.DefaultBlsPasswordFile(config.RootDir)
	if err := appsigner.EnsureDirs(cmtKeyFile, cmtStateFile, blsKeyFile, blsPasswordFile); err != nil {
		return "", nil, fmt.Errorf("failed to ensure dirs: %w", err)
	}

	var filePV *privval.FilePV
	if cmtos.FileExists(cmtKeyFile) {
		filePV = privval.LoadFilePV(cmtKeyFile, cmtStateFile)
	} else {
		var privKey ed25519.PrivKey
		if len(mnemonic) == 0 {
			privKey = ed25519.GenPrivKey()
		} else {
			privKey = ed25519.GenPrivKeyFromSecret([]byte(mnemonic))
		}
		filePV = privval.NewFilePV(privKey, cmtKeyFile, cmtStateFile)
		filePV.Key.Save()
		filePV.LastSignState.Save()
	}

	var bls *appsigner.Bls
	if cmtos.FileExists(blsKeyFile) {
		// if key file exists but password file does not exist -> error
		if !cmtos.FileExists(blsPasswordFile) {
			cmtos.Exit(fmt.Sprintf("BLS password file does not exist: %v", blsPasswordFile))
		}
		bls = appsigner.LoadBls(blsKeyFile, blsPasswordFile)
	} else {
		bls = appsigner.GenBls(blsKeyFile, blsPasswordFile, "password")
	}

	valKeys, err = appsigner.NewValidatorKeys(filePV.Key.PrivKey, bls.Key.PrivKey)
	if err != nil {
		return "", nil, err
	}

	return nodeID, valKeys, nil
}
