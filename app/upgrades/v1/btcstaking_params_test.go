package v1_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/app"
	v1 "github.com/babylonlabs-io/babylon/app/upgrades/v1"
)

func TestHardCodedBtcStakingParamsAreValid(t *testing.T) {
	bbnApp := app.NewTmpBabylonApp()
	loadedParamas, err := v1.LoadBtcStakingParamsFromData(bbnApp.AppCodec())
	require.NoError(t, err)
	require.NoError(t, loadedParamas.Validate())
}
