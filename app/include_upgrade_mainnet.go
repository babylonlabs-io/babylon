//go:build mainnet

package app

import (
	"github.com/babylonlabs-io/babylon/v3/app/upgrades"
	v1 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v1"
	"github.com/babylonlabs-io/babylon/v3/app/upgrades/v1/mainnet"
	v1_1 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v1_1"
	v2 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v2"
)

// init is used to include v1 upgrade for mainnet data
func init() {
	Upgrades = []upgrades.Upgrade{v2.CreateUpgrade(false), v1_1.Upgrade, v1.CreateUpgrade(v1.UpgradeDataString{
		BtcStakingParamsStr:       mainnet.BtcStakingParamsStr,
		FinalityParamStr:          mainnet.FinalityParamStr,
		IncentiveParamStr:         mainnet.IncentiveParamStr,
		CosmWasmParamStr:          mainnet.CosmWasmParamStr,
		NewBtcHeadersStr:          mainnet.NewBtcHeadersStr,
		TokensDistributionStr:     mainnet.TokensDistributionStr,
		AllowedStakingTxHashesStr: mainnet.AllowedStakingTxHashesStr,
	}, mainnet.ParamUpgrade)}
}
