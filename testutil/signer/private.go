package signer

import (
	"fmt"
	"os"

	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	cosmosed "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"

	cmtconfig "github.com/cometbft/cometbft/config"
	cmtprivval "github.com/cometbft/cometbft/privval"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/app/signer"
	"github.com/babylonlabs-io/babylon/privval"
	checkpointingtypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
)

// SetupTestPrivSigner sets up a PrivSigner for testing
func SetupTestPrivSigner() (*signer.PrivSigner, error) {
	// Create a temporary node directory
	nodeDir, err := os.MkdirTemp("", "tmp-signer")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary node directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(nodeDir)
	}()

	// generate a privSigner
	if err := GeneratePrivSigner(nodeDir); err != nil {
		return nil, fmt.Errorf("failed to generate priv signer: %w", err)
	}

	privSigner, _ := signer.InitPrivSigner(nodeDir)
	return privSigner, nil
}

// GenesisKeyFromPrivSigner generates a genesis key from a priv signer
func GenesisKeyFromPrivSigner(ps *signer.PrivSigner, delegatorAddress sdk.ValAddress) (*checkpointingtypes.GenesisKey, error) {
	valKeys, err := privval.NewValidatorKeys(
		ps.PV.Comet.PrivKey,
		ps.PV.Bls.PrivKey,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate validator keys: %w", err)
	}
	valPubkey, err := cryptocodec.FromCmtPubKeyInterface(valKeys.ValPubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to convert validator public key: %w", err)
	}
	return checkpointingtypes.NewGenesisKey(
		delegatorAddress,
		&valKeys.BlsPubkey,
		valKeys.PoP,
		&cosmosed.PubKey{Key: valPubkey.Bytes()},
	)
}

// GeneratePrivSigner generates a priv signer
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

	privval.GenBlsPV(blsKeyFile, blsPasswordFile, "password")
	return nil
}
