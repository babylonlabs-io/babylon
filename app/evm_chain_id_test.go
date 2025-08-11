package app_test

import (
<<<<<<< HEAD
	"github.com/babylonlabs-io/babylon/v4/app"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/cmd/babylond/cmd"
=======
	"github.com/babylonlabs-io/babylon/v3/app"
	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	"github.com/babylonlabs-io/babylon/v3/cmd/babylond/cmd"
>>>>>>> ff15aa5 (feat: bump cosmos evm from v1.0.0-rc0 to v0.3.1 (#1492))
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"github.com/stretchr/testify/require"
	"testing"
)

// TestEVMChainIDConfiguration verifies that the EVM chain ID is properly configured
func TestEVMChainIDConfiguration(t *testing.T) {
	// Verify the app params define the correct EVM chain ID
	require.Equal(t, uint64(6901), uint64(appparams.EVMChainID),
		"EVMChainID constant should be 6901")

	// Verify the default Babylon app config sets the correct EVM chain ID
	babylonConfig := cmd.DefaultBabylonAppConfig()
	require.Equal(t, uint64(appparams.EVMChainID), babylonConfig.EVM.EVMChainID,
		"Default Babylon app config should set EVM chain ID to %d", appparams.EVMChainID)

	// Verify EVMAppOptions is called with the correct chain ID
	// This tests that the global EVM configuration is set correctly
	err := app.EVMAppOptions(appparams.EVMChainID)
	require.NoError(t, err, "EVMAppOptions should succeed with the correct chain ID")

	// Verify the global EVM chain config is set correctly after EVMAppOptions
	globalChainConfig := evmtypes.GetEthChainConfig()
	require.NotNil(t, globalChainConfig, "Global EVM chain config should be set")
	require.Equal(t, uint64(appparams.EVMChainID), globalChainConfig.ChainID.Uint64(),
		"Global EVM chain config should have chain ID %d", appparams.EVMChainID)

	t.Logf("EVM chain ID validation passed: all configurations use chain ID %d", appparams.EVMChainID)
}
