package v1_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/app"
	v1 "github.com/babylonlabs-io/babylon/app/upgrades/v1"
	mainnetdata "github.com/babylonlabs-io/babylon/app/upgrades/v1/mainnet"
	testnetdata "github.com/babylonlabs-io/babylon/app/upgrades/v1/testnet"
)

var btcStakingDatas = []string{mainnetdata.BtcStakingParamStr, testnetdata.BtcStakingParamStr}

func TestHardCodedBtcStakingParamsAreValid(t *testing.T) {
	bbnApp := app.NewTmpBabylonApp()
	for _, strData := range btcStakingDatas {
		loadedParamas, err := v1.LoadBtcStakingParamsFromData(bbnApp.AppCodec(), strData)
		require.NoError(t, err)
		require.NoError(t, loadedParamas.Validate())
	}
}
