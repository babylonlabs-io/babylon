package signer

import (
	"fmt"

	cmtconfig "github.com/cometbft/cometbft/config"

	"github.com/babylonlabs-io/babylon/privval"
	cmtos "github.com/cometbft/cometbft/libs/os"
	cmtprivval "github.com/cometbft/cometbft/privval"
)

type PrivSigner struct {
	PV *privval.WrappedFilePV
}

func InitPrivSigner(nodeDir string) (*PrivSigner, error) {
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

	cometPV := cmtprivval.LoadFilePV(pvKeyFile, pvStateFile)
	blsPV := privval.LoadBlsPV(blsKeyFile, blsPasswordFile)

	return &PrivSigner{
		PV: privval.NewWrappedFilePV(cometPV.Key, blsPV.Key),
	}, nil
}
