//go:build mainnet

package app

import (
	"github.com/babylonlabs-io/babylon/app/upgrades"
	v1 "github.com/babylonlabs-io/babylon/app/upgrades/v1"
	"github.com/babylonlabs-io/babylon/app/upgrades/v1/mainnet"
)

// init is used to include v1 upgrade for mainnet data
func init() {
	Upgrades = []upgrades.Upgrade{v1.CreateUpgrade(v1.UpgradeDataString{
		BtcStakingParamsStr:       mainnet.BtcStakingParamsStr,
		FinalityParamStr:          mainnet.FinalityParamStr,
		IncentiveParamStr:         mainnet.IncentiveParamStr,
		CosmWasmParamStr:          mainnet.CosmWasmParamStr,
		NewBtcHeadersStr:          mainnet.NewBtcHeadersStr,
		TokensDistributionStr:     mainnet.TokensDistributionStr,
		AllowedStakingTxHashesStr: mainnet.AllowedStakingTxHashesStr,
	}, nil)}
}
