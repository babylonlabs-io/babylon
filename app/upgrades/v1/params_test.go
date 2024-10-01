package v1_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/app"
	v1 "github.com/babylonlabs-io/babylon/app/upgrades/v1"
)

func TestHardCodedBtcStakingParamsAreValid(t *testing.T) {
	bbnApp := app.NewTmpBabylonApp()
	for _, upgradeData := range UpgradeV1Data {
		loadedParamas, err := v1.LoadBtcStakingParamsFromData(bbnApp.AppCodec(), upgradeData.BTCStakingParam)
		require.NoError(t, err)
		require.NoError(t, loadedParamas.Validate())
	}
}

func TestHardCodedFinalityParamsAreValid(t *testing.T) {
	bbnApp := app.NewTmpBabylonApp()
	for _, upgradeData := range UpgradeV1Data {
		loadedParamas, err := v1.LoadFinalityParamsFromData(bbnApp.AppCodec(), upgradeData.FinalityParam)
		require.NoError(t, err)
		require.NoError(t, loadedParamas.Validate())
	}
}
