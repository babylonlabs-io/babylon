package app

import (
	"testing"

	"github.com/stretchr/testify/require"

	erc20types "github.com/cosmos/evm/x/erc20/types"
)

// BabylonApp DefaultGenesis ERC20 NativePrecompiles test
func TestBabylonApp_DefaultGenesis_ERC20NativePrecompiles(t *testing.T) {
	// Test that BabylonApp.DefaultGenesis() correctly sets NativePrecompiles
	app := NewTmpBabylonApp()
	genesis := app.DefaultGenesis()

	// Verify ERC20 module genesis is present
	require.Contains(t, genesis, erc20types.ModuleName, "ERC20 module should be in genesis")

	// Unmarshal ERC20 genesis state
	var erc20GenState erc20types.GenesisState
	err := app.appCodec.UnmarshalJSON(genesis[erc20types.ModuleName], &erc20GenState)
	require.NoError(t, err, "Should be able to unmarshal ERC20 genesis state")

	// Verify NativePrecompiles contains WTokenContractMainnet
	require.Contains(t, erc20GenState.NativePrecompiles, WTokenContractMainnet,
		"NativePrecompiles should contain WTokenContractMainnet")
	require.Len(t, erc20GenState.NativePrecompiles, 1,
		"Should have exactly one native precompile")

	// Verify it's the correct contract address
	require.Equal(t, WTokenContractMainnet, erc20GenState.NativePrecompiles[0],
		"First native precompile should be WTokenContractMainnet")

	// Verify the field structure (not nested under Params)
	require.IsType(t, []string{}, erc20GenState.NativePrecompiles,
		"NativePrecompiles should be []string type")

	// Verify the genesis state validates
	require.NoError(t, erc20GenState.Validate(),
		"ERC20 genesis state should be valid")
}
