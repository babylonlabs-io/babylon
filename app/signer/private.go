package signer

import (
	"path/filepath"

	cmtconfig "github.com/cometbft/cometbft/config"

	"github.com/babylonlabs-io/babylon/privval"
	cmtprivval "github.com/cometbft/cometbft/privval"
)

type PrivSigner struct {
	WrappedPV *privval.WrappedFilePV
}

func InitPrivSigner(nodeDir string) (*PrivSigner, error) {
	nodeCfg := cmtconfig.DefaultConfig()
	blsCfg := privval.DefaultBlsConfig()

	pvKeyFile := filepath.Join(nodeDir, nodeCfg.PrivValidatorKeyFile())
	pvStateFile := filepath.Join(nodeDir, nodeCfg.PrivValidatorStateFile())
	blsKeyFile := filepath.Join(nodeDir, blsCfg.BlsKeyFile())
	blsPasswordFile := filepath.Join(nodeDir, blsCfg.BlsPasswordFile())

	if err := privval.IsValidFilePath(pvKeyFile, pvStateFile, blsKeyFile, blsPasswordFile); err != nil {
		return nil, err
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
