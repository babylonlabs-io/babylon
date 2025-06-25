package signer

import (
	"fmt"
	"os"

	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	cosmosed "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"

	cmtconfig "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/crypto"
	sdk "github.com/cosmos/cosmos-sdk/types"

	appsigner "github.com/babylonlabs-io/babylon/v3/app/signer"
	"github.com/babylonlabs-io/babylon/v3/crypto/bls12381"
	checkpointingtypes "github.com/babylonlabs-io/babylon/v3/x/checkpointing/types"
)

const TestPassword string = "password"

// SetupTestBlsSigner sets up a BLS signer for testing
func SetupTestBlsSigner() (*appsigner.BlsKey, error) {
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

	blsKeyFile := appsigner.DefaultBlsKeyFile(nodeDir)
	blsPasswordFile := appsigner.DefaultBlsPasswordFile(nodeDir)

	if err := appsigner.EnsureDirs(blsKeyFile, blsPasswordFile); err != nil {
		return nil, fmt.Errorf("failed to ensure dirs: %w", err)
	}

	bls := appsigner.GenBls(blsKeyFile, blsPasswordFile, TestPassword)
	return &bls.Key, nil
}

// GenesisKeyFromPrivSigner generates a genesis key from a priv signer
func GenesisKeyFromPrivSigner(cmtPrivKey crypto.PrivKey, blsPrivKey bls12381.PrivateKey, delegatorAddress sdk.ValAddress) (*checkpointingtypes.GenesisKey, error) {
	valKeys, err := appsigner.NewValidatorKeys(
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
