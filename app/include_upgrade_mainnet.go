//go:build mainnet

package app

import (
	"github.com/babylonlabs-io/babylon/v2/app/upgrades"
	v1 "github.com/babylonlabs-io/babylon/v2/app/upgrades/v1"
	"github.com/babylonlabs-io/babylon/v2/app/upgrades/v1/mainnet"
	v2 "github.com/babylonlabs-io/babylon/v2/app/upgrades/v2"
)

// init is used to include v1 upgrade for mainnet data
func init() {
	Upgrades = []upgrades.Upgrade{v2.Upgrade, v1.CreateUpgrade(v1.UpgradeDataString{
		BtcStakingParamsStr:       mainnet.BtcStakingParamsStr,
		FinalityParamStr:          mainnet.FinalityParamStr,
		IncentiveParamStr:         mainnet.IncentiveParamStr,
		CosmWasmParamStr:          mainnet.CosmWasmParamStr,
		NewBtcHeadersStr:          mainnet.NewBtcHeadersStr,
		TokensDistributionStr:     mainnet.TokensDistributionStr,
		AllowedStakingTxHashesStr: mainnet.AllowedStakingTxHashesStr,
	}, mainnet.ParamUpgrade)}
}
