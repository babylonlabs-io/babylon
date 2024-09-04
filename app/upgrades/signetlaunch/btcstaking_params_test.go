package signetlaunch_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/app"
	v1 "github.com/babylonlabs-io/babylon/app/upgrades/signetlaunch"
	"github.com/stretchr/testify/require"
)

func TestHardCodedParamsAreValid(t *testing.T) {
	bbnApp := app.NewTmpBabylonApp()
	loadedParamas, err := v1.LoadBtcStakingParamsFromData(bbnApp.AppCodec())
	require.NoError(t, err)
	require.NoError(t, loadedParamas.Validate())
}
