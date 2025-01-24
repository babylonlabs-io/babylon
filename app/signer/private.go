package signer

import (
	"fmt"

	cmtconfig "github.com/cometbft/cometbft/config"
	cmtos "github.com/cometbft/cometbft/libs/os"
	cmtprivval "github.com/cometbft/cometbft/privval"

	"github.com/babylonlabs-io/babylon/privval"

	checkpointingtypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
)

type ConsensusKey struct {
	Comet *cmtprivval.FilePVKey
	Bls   *privval.BlsPVKey
}

// LoadConsensusKey loads the private key from the node directory
// Only requires in appExport, which requires key files located in local.
func LoadConsensusKey(nodeDir string) (*ConsensusKey, error) {
	nodeCfg := cmtconfig.DefaultConfig()
	nodeCfg.SetRoot(nodeDir)

	pvKeyFile := nodeCfg.PrivValidatorKeyFile()
	pvStateFile := nodeCfg.PrivValidatorStateFile()
	blsKeyFile := privval.DefaultBlsKeyFile(nodeDir)
	blsPasswordFile := privval.DefaultBlsPasswordFile(nodeDir)

	if err := privval.EnsureDirs(pvKeyFile, pvStateFile, blsKeyFile, blsPasswordFile); err != nil {
		return nil, fmt.Errorf("failed to ensure dirs: %w", err)
	}

	if !cmtos.FileExists(pvKeyFile) {
		return nil, fmt.Errorf("validator key file does not exist. create file using `babylond init`: %s", pvKeyFile)
	}

	if !cmtos.FileExists(blsKeyFile) || !cmtos.FileExists(blsPasswordFile) {
		return nil, fmt.Errorf("BLS key file does not exist. create file using `babylond init` or `babylond create-bls-key`: %s", blsKeyFile)
	}

	filePV := cmtprivval.LoadFilePV(pvKeyFile, pvStateFile)
	blsPV := privval.LoadBlsPV(blsKeyFile, blsPasswordFile)

	return &ConsensusKey{
		Comet: &filePV.Key,
		Bls:   &blsPV.Key,
	}, nil
}

// InitBlsSigner initializes the bls signer
func InitBlsSigner(nodeDir string) (*checkpointingtypes.BlsSigner, error) {
	nodeCfg := cmtconfig.DefaultConfig()
	nodeCfg.SetRoot(nodeDir)

	blsKeyFile := privval.DefaultBlsKeyFile(nodeDir)
	blsPasswordFile := privval.DefaultBlsPasswordFile(nodeDir)

	if !cmtos.FileExists(blsKeyFile) || !cmtos.FileExists(blsPasswordFile) {
		return nil, fmt.Errorf("BLS key file does not exist. create file using `babylond init` or `babylond create-bls-key`: %s", blsKeyFile)
	}

	blsPV := privval.LoadBlsPV(blsKeyFile, blsPasswordFile)
	blsSigner := checkpointingtypes.BlsSigner(&blsPV.Key)
	return &blsSigner, nil
}
