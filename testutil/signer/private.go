package signer

import (
	"fmt"
	"os"

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
	nodeCfg.SetRoot(nodeDir)

	cmtKeyFile := nodeCfg.PrivValidatorKeyFile()
	cmtStateFile := nodeCfg.PrivValidatorStateFile()
	blsKeyFile := privval.DefaultBlsKeyFile(nodeDir)
	blsPasswordFile := privval.DefaultBlsPasswordFile(nodeDir)

	if err := privval.EnsureDirs(cmtKeyFile, cmtStateFile, blsKeyFile, blsPasswordFile); err != nil {
		return fmt.Errorf("failed to ensure dirs: %w", err)
	}

	cometPV := cmtprivval.GenFilePV(cmtKeyFile, cmtStateFile)
	cometPV.Key.Save()
	cometPV.LastSignState.Save()

	privval.GenBlsPV(blsKeyFile, blsPasswordFile, "password", "")
	return nil
}
