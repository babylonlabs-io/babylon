package signer

import (
	"fmt"
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

	blsCfg := privval.DefaultBlsConfig()
	blsCfg.SetRoot(nodeCfg.RootDir)
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

	if !cmtos.FileExists(blsKeyFile) {
		return nil, fmt.Errorf("BLS key file does not exist: %v", blsKeyFile)
	}

	if !cmtos.FileExists(blsPasswordFile) {
		return nil, fmt.Errorf("BLS password file does not exist: %v", blsPasswordFile)
	}

	blsPv := privval.LoadBlsPV(blsKeyFile, blsPasswordFile)
	cmtPv := cmtprivval.LoadFilePV(pvKeyFile, pvStateFile)
	wrappedPvKey := privval.WrappedFilePVKey{
		CometPVKey: cmtPv.Key,
		BlsPVKey:   blsPv.Key,
	}
	wrappedPV := &privval.WrappedFilePV{
		Key:           wrappedPvKey,
		LastSignState: cmtPv.LastSignState,
	}

	return &PrivSigner{
		WrappedPV: wrappedPV,
	}, nil
}

func InitTestPrivSigner(nodeDir string) (*PrivSigner, error) {
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

	blsCfg := privval.DefaultBlsConfig()
	blsCfg.SetRoot(nodeCfg.RootDir)
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

	wrappedPV := privval.LoadOrGenWrappedFilePV(pvKeyFile, pvStateFile, blsKeyFile, blsPasswordFile)

	return &PrivSigner{
		WrappedPV: wrappedPV,
	}, nil
}
