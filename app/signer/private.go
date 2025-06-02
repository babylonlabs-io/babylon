package signer

import (
	"fmt"

	cmtconfig "github.com/cometbft/cometbft/config"
	cmtos "github.com/cometbft/cometbft/libs/os"
	"github.com/cometbft/cometbft/privval"

	checkpointingtypes "github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
)

// ConsensusKey represents the consensus keys
type ConsensusKey struct {
	Comet *privval.FilePVKey
	Bls   *BlsKey
}

// LoadConsensusKey loads the consensus keys from the node directory
// Since it loads both the FilePV and Bls from the local,
// User who runs the remote signer cannot operate this function
func LoadConsensusKey(nodeDir string) (*ConsensusKey, error) {
	filePV, err := loadFilePV(nodeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load file pv key: %w", err)
	}
	bls, err := loadBls(nodeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load bls pv key: %w", err)
	}

	return &ConsensusKey{
		Comet: &filePV.Key,
		Bls:   &bls.Key,
	}, nil
}

// InitBlsSigner initializes the bls signer
func InitBlsSigner(nodeDir string) (*checkpointingtypes.BlsSigner, error) {
	bls, err := loadBls(nodeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load bls pv key: %w", err)
	}
	blsSigner := checkpointingtypes.BlsSigner(&bls.Key)
	return &blsSigner, nil
}

// loadFilePV loads the private key from the node directory in local
func loadFilePV(homeDir string) (*privval.FilePV, error) {
	nodeCfg := cmtconfig.DefaultConfig()
	nodeCfg.SetRoot(homeDir)

	pvKeyFile := nodeCfg.PrivValidatorKeyFile()
	pvStateFile := nodeCfg.PrivValidatorStateFile()

	if err := EnsureDirs(pvKeyFile, pvStateFile); err != nil {
		return nil, fmt.Errorf("failed to ensure dirs: %w", err)
	}

	if !cmtos.FileExists(pvKeyFile) {
		return nil, fmt.Errorf("validator key file does not exist. create file using `babylond init`: %s", pvKeyFile)
	}

	filePV := privval.LoadFilePV(pvKeyFile, pvStateFile)
	return filePV, nil
}

// loadBls loads the private key from the node directory in local
func loadBls(homeDir string) (*Bls, error) {
	nodeCfg := cmtconfig.DefaultConfig()
	nodeCfg.SetRoot(homeDir)

	blsKeyFile := DefaultBlsKeyFile(homeDir)
	blsPasswordFile := DefaultBlsPasswordFile(homeDir)

	if err := EnsureDirs(blsKeyFile, blsPasswordFile); err != nil {
		return nil, fmt.Errorf("failed to ensure dirs: %w", err)
	}

	if !cmtos.FileExists(blsKeyFile) {
		return nil, fmt.Errorf("BLS key file does not exist. create file using `babylond init` or `babylond create-bls-key`: %s", blsKeyFile)
	}

	password, found := GetBlsPasswordFromEnv()
	if found && password != "" {
		bls, ok, err := TryLoadBlsFromFile(blsKeyFile, "")
		if err != nil {
			return nil, err
		}
		if ok {
			return bls, nil
		}
	}

	if !cmtos.FileExists(blsPasswordFile) {
		return nil, fmt.Errorf("BLS password file does not exist and no environment variable set: %s", blsPasswordFile)
	}

	bls, ok, err := TryLoadBlsFromFile(blsKeyFile, blsPasswordFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load bls key: %w", err)
	}
	if ok {
		return bls, nil
	}
	return nil, fmt.Errorf("failed to load bls key: %w", err)
}
