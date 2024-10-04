package v1_test

import (
	"testing"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/app"
	v1 "github.com/babylonlabs-io/babylon/app/upgrades/v1"
)

func TestHardCodedBtcStakingParamsAreValid(t *testing.T) {
	bbnApp := app.NewTmpBabylonApp()
	for _, upgradeData := range UpgradeV1Data {
		params, err := v1.LoadBtcStakingParamsFromData(bbnApp.AppCodec(), upgradeData.BtcStakingParamStr)
		require.NoError(t, err)
		require.NoError(t, params.Validate())
	}
}

func TestHardCodedFinalityParamsAreValid(t *testing.T) {
	bbnApp := app.NewTmpBabylonApp()
	for _, upgradeData := range UpgradeV1Data {
		params, err := v1.LoadFinalityParamsFromData(bbnApp.AppCodec(), upgradeData.FinalityParamStr)
		require.NoError(t, err)
		require.NoError(t, params.Validate())
	}
}

func TestHardCodedWasmParamsAreValid(t *testing.T) {
	bbnApp := app.NewTmpBabylonApp()

	for _, upgradeData := range UpgradeV1Data {
		params, err := v1.LoadCosmWasmParamsFromData(bbnApp.AppCodec(), upgradeData.CosmWasmParamStr)
		require.NoError(t, err)
		require.NotNil(t, params)
		require.Equal(t, params.InstantiateDefaultPermission, wasmtypes.AccessTypeEverybody)
	}
}
