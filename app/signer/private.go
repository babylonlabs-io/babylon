package signer

import (
	"path/filepath"

	cmtconfig "github.com/cometbft/cometbft/config"
	cmtos "github.com/cometbft/cometbft/libs/os"

	"github.com/babylonlabs-io/babylon/privval"
	cmtprivval "github.com/cometbft/cometbft/privval"
)

type PrivSigner struct {
	WrappedPV *privval.WrappedFilePV
}

func InitPrivSigner(nodeDir string) (*PrivSigner, error) {

	// cometPv
	nodeCfg := cmtconfig.DefaultConfig()
	pvKeyFile := filepath.Join(nodeDir, nodeCfg.PrivValidatorKeyFile())
	err := cmtos.EnsureDir(filepath.Dir(pvKeyFile), 0777)
	if err != nil {
		return nil, err
	}
	pvStateFile := filepath.Join(nodeDir, nodeCfg.PrivValidatorStateFile())
	err = cmtos.EnsureDir(filepath.Dir(pvStateFile), 0777)
	if err != nil {
		return nil, err
	}
	cometPv := cmtprivval.LoadOrGenFilePV(pvKeyFile, pvStateFile)

	// blsPv
	blsCfg := privval.DefaultBlsConfig()
	blsCfg.RootDir = nodeCfg.RootDir
	blsKeyFile := filepath.Join(nodeDir, blsCfg.BlsKeyFile())
	err = cmtos.EnsureDir(filepath.Dir(blsKeyFile), 0777)
	if err != nil {
		return nil, err
	}
	blsPasswordFile := filepath.Join(nodeDir, blsCfg.BlsPasswordFile())
	err = cmtos.EnsureDir(filepath.Dir(blsPasswordFile), 0777)
	if err != nil {
		return nil, err
	}

	password := privval.LoadOrGenBlsPassword(blsPasswordFile)
	blsPv := privval.LoadOrGenBlsPV(blsKeyFile, password)

	return &PrivSigner{
		WrappedPV: &privval.WrappedFilePV{
			Key: privval.WrappedFilePVKey{
				CometPVKey: cometPv.Key,
				BlsPVKey:   blsPv.Key,
			},
			LastSignState: cometPv.LastSignState,
		},
	}, nil
}
