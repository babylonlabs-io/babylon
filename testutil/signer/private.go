package signer

import (
	"fmt"
	"os"

	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	cosmosed "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"

	cmtconfig "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/crypto"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/privval"
	checkpointingtypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
)

const TestPassword string = "password"

// SetupTestPrivSigner sets up a PrivSigner for testing
func SetupTestBlsSigner() (*privval.BlsPVKey, error) {
	// Create a temporary node directory
	nodeDir, err := os.MkdirTemp("", "tmp-signer")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary node directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(nodeDir)
	}()

	nodeCfg := cmtconfig.DefaultConfig()
	nodeCfg.SetRoot(nodeDir)

	blsKeyFile := privval.DefaultBlsKeyFile(nodeDir)
	blsPasswordFile := privval.DefaultBlsPasswordFile(nodeDir)

	if err := privval.EnsureDirs(blsKeyFile, blsPasswordFile); err != nil {
		return nil, fmt.Errorf("failed to ensure dirs: %w", err)
	}

	blsPv := privval.GenBlsPV(blsKeyFile, blsPasswordFile, TestPassword)
	return &blsPv.Key, nil
}

// GenesisKeyFromPrivSigner generates a genesis key from a priv signer
func GenesisKeyFromPrivSigner(cmtPrivKey crypto.PrivKey, blsPrivKey bls12381.PrivateKey, delegatorAddress sdk.ValAddress) (*checkpointingtypes.GenesisKey, error) {
	valKeys, err := privval.NewValidatorKeys(
		cmtPrivKey,
		blsPrivKey,
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
