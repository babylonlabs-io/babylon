package signer

import (
	"fmt"
	"os"

	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	cosmosed "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"

	cmtconfig "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/crypto"
	sdk "github.com/cosmos/cosmos-sdk/types"

	bb "github.com/babylonlabs-io/babylon/bls"
	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	checkpointingtypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
)

const TestPassword string = "password"

// SetupTestPrivSigner sets up a PrivSigner for testing
func SetupTestBlsSigner() (*bb.BlsKey, error) {
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

	blsKeyFile := bb.DefaultBlsKeyFile(nodeDir)
	blsPasswordFile := bb.DefaultBlsPasswordFile(nodeDir)

	if err := bb.EnsureDirs(blsKeyFile, blsPasswordFile); err != nil {
		return nil, fmt.Errorf("failed to ensure dirs: %w", err)
	}

	bls := bb.GenBls(blsKeyFile, blsPasswordFile, TestPassword)
	return &bls.Key, nil
}

// GenesisKeyFromPrivSigner generates a genesis key from a priv signer
func GenesisKeyFromPrivSigner(cmtPrivKey crypto.PrivKey, blsPrivKey bls12381.PrivateKey, delegatorAddress sdk.ValAddress) (*checkpointingtypes.GenesisKey, error) {
	valKeys, err := bb.NewValidatorKeys(
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
