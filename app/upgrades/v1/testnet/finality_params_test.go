package testnet_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/app"
	v1 "github.com/babylonlabs-io/babylon/app/upgrades/v1"
)

func TestHardCodedFinalityParamsAreValid(t *testing.T) {
	bbnApp := app.NewTmpBabylonApp()
	loadedParamas, err := v1.LoadFinalityParamsFromData(bbnApp.AppCodec())
	require.NoError(t, err)
	require.NoError(t, loadedParamas.Validate())
}
