package signer

import (
	"fmt"

	cmtconfig "github.com/cometbft/cometbft/config"
	cmtos "github.com/cometbft/cometbft/libs/os"
	cmtprivval "github.com/cometbft/cometbft/privval"

	"github.com/babylonlabs-io/babylon/privval"

	checkpointingtypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
)

// ConsensusKey represents the consensus keys
type ConsensusKey struct {
	Comet *cmtprivval.FilePVKey
	Bls   *privval.BlsPVKey
}

// LoadConsensusKey loads the consensus keys from the node directory
// Since it loads both the FilePV and BlsPV from the local,
// User who runs the remote signer cannot operate this function
func LoadConsensusKey(nodeDir string) (*ConsensusKey, error) {
	filePV, err := loadFilePV(nodeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load file pv key: %w", err)
	}
	blsPV, err := loadBlsPV(nodeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load bls pv key: %w", err)
	}

	return &ConsensusKey{
		Comet: &filePV.Key,
		Bls:   &blsPV.Key,
	}, nil
}

// InitBlsSigner initializes the bls signer
func InitBlsSigner(nodeDir string) (*checkpointingtypes.BlsSigner, error) {
	blsPv, err := loadBlsPV(nodeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load bls pv key: %w", err)
	}
	blsSigner := checkpointingtypes.BlsSigner(&blsPv.Key)
	return &blsSigner, nil
}

// loadFilePV loads the private key from the node directory in local
func loadFilePV(homeDir string) (*cmtprivval.FilePV, error) {
	nodeCfg := cmtconfig.DefaultConfig()
	nodeCfg.SetRoot(homeDir)

	pvKeyFile := nodeCfg.PrivValidatorKeyFile()
	pvStateFile := nodeCfg.PrivValidatorStateFile()

	if err := privval.EnsureDirs(pvKeyFile, pvStateFile); err != nil {
		return nil, fmt.Errorf("failed to ensure dirs: %w", err)
	}

	if !cmtos.FileExists(pvKeyFile) {
		return nil, fmt.Errorf("validator key file does not exist. create file using `babylond init`: %s", pvKeyFile)
	}

	filePV := cmtprivval.LoadFilePV(pvKeyFile, pvStateFile)
	return filePV, nil
}

// loadBlsPV loads the private key from the node directory in local
func loadBlsPV(homeDir string) (*privval.BlsPV, error) {
	nodeCfg := cmtconfig.DefaultConfig()
	nodeCfg.SetRoot(homeDir)

	blsKeyFile := privval.DefaultBlsKeyFile(homeDir)
	blsPasswordFile := privval.DefaultBlsPasswordFile(homeDir)

	if err := privval.EnsureDirs(blsKeyFile, blsPasswordFile); err != nil {
		return nil, fmt.Errorf("failed to ensure dirs: %w", err)
	}

	if !cmtos.FileExists(blsKeyFile) || !cmtos.FileExists(blsPasswordFile) {
		return nil, fmt.Errorf("BLS key file does not exist. create file using `babylond init` or `babylond create-bls-key`: %s", blsKeyFile)
	}

	blsPV := privval.LoadBlsPV(blsKeyFile, blsPasswordFile)
	return blsPV, nil
}
