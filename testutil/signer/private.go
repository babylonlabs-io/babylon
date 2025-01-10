package signer

import (
	"os"
	"path/filepath"

	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	cosmosed "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"

	"github.com/babylonlabs-io/babylon/app/signer"
	"github.com/babylonlabs-io/babylon/privval"
	checkpointingtypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
	cmtconfig "github.com/cometbft/cometbft/config"
	cmtprivval "github.com/cometbft/cometbft/privval"
)

// SetupTestPrivSigner sets up a PrivSigner for testing
func SetupTestPrivSigner() (*signer.PrivSigner, error) {
	// Create a temporary node directory
	nodeDir, err := os.MkdirTemp("", "tmp-signer")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = os.RemoveAll(nodeDir)
	}()

	// generate a privSigner
	if err := GeneratePrivSigner(nodeDir); err != nil {
		return nil, err
	}

	privSigner, _ := signer.InitPrivSigner(nodeDir)
	return privSigner, nil
}

func GenesisKeyFromPrivSigner(ps *signer.PrivSigner) (*checkpointingtypes.GenesisKey, error) {
	valKeys, err := privval.NewValidatorKeys(ps.WrappedPV.GetValPrivKey(), ps.WrappedPV.GetBlsPrivKey())
	if err != nil {
		return nil, err
	}
	valPubkey, err := cryptocodec.FromCmtPubKeyInterface(valKeys.ValPubkey)
	if err != nil {
		return nil, err
	}
	return checkpointingtypes.NewGenesisKey(
		ps.WrappedPV.GetAddress(),
		&valKeys.BlsPubkey,
		valKeys.PoP,
		&cosmosed.PubKey{Key: valPubkey.Bytes()},
	)
}

func GeneratePrivSigner(nodeDir string) error {
	nodeCfg := cmtconfig.DefaultConfig()
	blsCfg := privval.DefaultBlsConfig()

	pvKeyFile := filepath.Join(nodeDir, nodeCfg.PrivValidatorKeyFile())
	pvStateFile := filepath.Join(nodeDir, nodeCfg.PrivValidatorStateFile())
	blsKeyFile := filepath.Join(nodeDir, blsCfg.BlsKeyFile())
	blsPasswordFile := filepath.Join(nodeDir, blsCfg.BlsPasswordFile())

	if err := privval.IsValidFilePath(pvKeyFile, pvStateFile, blsKeyFile, blsPasswordFile); err != nil {
		return err
	}

	cometPV := cmtprivval.GenFilePV(pvKeyFile, pvStateFile)
	cometPV.Key.Save()
	cometPV.LastSignState.Save()

	privval.GenBlsPV(blsKeyFile, blsPasswordFile, "password", "")
	return nil
}
