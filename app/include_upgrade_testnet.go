//go:build testnet

package app

import (
	"github.com/babylonlabs-io/babylon/v2/app/upgrades"
	v1 "github.com/babylonlabs-io/babylon/v2/app/upgrades/v1"
	"github.com/babylonlabs-io/babylon/v2/app/upgrades/v1/testnet"
	v1rc5 "github.com/babylonlabs-io/babylon/v2/app/upgrades/v1rc5/testnet"
	v1rc8 "github.com/babylonlabs-io/babylon/v2/app/upgrades/v1rc8/testnet"
	v1rc9 "github.com/babylonlabs-io/babylon/v2/app/upgrades/v1rc9/testnet"
	v2 "github.com/babylonlabs-io/babylon/v2/app/upgrades/v2"
)

// init is used to include v1 upgrade testnet data
// it is also used for e2e testing
func init() {
	Upgrades = []upgrades.Upgrade{
		v1.CreateUpgrade(v2.Upgrade, v1.UpgradeDataString{
			BtcStakingParamsStr:       testnet.BtcStakingParamsStr,
			FinalityParamStr:          testnet.FinalityParamStr,
			IncentiveParamStr:         testnet.IncentiveParamStr,
			CosmWasmParamStr:          testnet.CosmWasmParamStr,
			NewBtcHeadersStr:          testnet.NewBtcHeadersStr,
			TokensDistributionStr:     testnet.TokensDistributionStr,
			AllowedStakingTxHashesStr: testnet.AllowedStakingTxHashesStr,
		}, testnet.ParamUpgrade),
		v1rc5.CreateUpgrade(),
		v1rc8.CreateUpgrade(),
		v1rc9.CreateUpgrade(),
	}
}
