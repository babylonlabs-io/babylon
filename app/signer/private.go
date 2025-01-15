package signer

import (
	"fmt"

	cmtconfig "github.com/cometbft/cometbft/config"

	"github.com/babylonlabs-io/babylon/privval"
	cmtprivval "github.com/cometbft/cometbft/privval"
)

type PrivSigner struct {
	WrappedPV *privval.WrappedFilePV
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

	cometPV := cmtprivval.LoadFilePV(pvKeyFile, pvStateFile)
	blsPV := privval.LoadBlsPV(blsKeyFile, blsPasswordFile)

	wrappedPV := &privval.WrappedFilePV{
		Key: privval.WrappedFilePVKey{
			CometPVKey: cometPV.Key,
			BlsPVKey:   blsPV.Key,
		},
		LastSignState: cometPV.LastSignState,
	}

	return &PrivSigner{
		WrappedPV: wrappedPV,
	}, nil
}
