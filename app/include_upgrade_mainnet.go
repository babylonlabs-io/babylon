//go:build mainnet

package app

import (
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
	v1 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v1"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades/v1/mainnet"
	v1_1 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v1_1"
	v2 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v2"
)

var WhitelistedChannelsID = map[string]struct{}{
	"channel-0": struct{}{},
	"channel-1": struct{}{},
	"channel-2": struct{}{},
	"channel-3": struct{}{},
	"channel-4": struct{}{},
	"channel-5": struct{}{},
	"channel-6": struct{}{},
}

// init is used to include v1 upgrade for mainnet data
func init() {
	Upgrades = []upgrades.Upgrade{v2.CreateUpgrade(false, WhitelistedChannelsID), v1_1.Upgrade, v1.CreateUpgrade(v1.UpgradeDataString{
		BtcStakingParamsStr:       mainnet.BtcStakingParamsStr,
		FinalityParamStr:          mainnet.FinalityParamStr,
		IncentiveParamStr:         mainnet.IncentiveParamStr,
		CosmWasmParamStr:          mainnet.CosmWasmParamStr,
		NewBtcHeadersStr:          mainnet.NewBtcHeadersStr,
		TokensDistributionStr:     mainnet.TokensDistributionStr,
		AllowedStakingTxHashesStr: mainnet.AllowedStakingTxHashesStr,
	}, mainnet.ParamUpgrade)}
}
