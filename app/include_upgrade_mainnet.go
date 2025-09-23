//go:build mainnet

package app

import (
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
	v1 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v1"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades/v1/mainnet"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades/v1_1"
	v2 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v2"
	v22 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v2_2"
	v23 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v2_3"
	v4 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v4"
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

// init is used to include v2.2 upgrade for mainnet data
func init() {
	Upgrades = []upgrades.Upgrade{
		v4.Upgrade,
		v23.Upgrade, // same as v3rc3 testnet
		v22.Upgrade,
		v2.CreateUpgrade(false, WhitelistedChannelsID),
		v1_1.Upgrade,
		v1.CreateUpgrade(v1.UpgradeDataString{
			BtcStakingParamsStr:       mainnet.BtcStakingParamsStr,
			FinalityParamStr:          mainnet.FinalityParamStr,
			IncentiveParamStr:         mainnet.IncentiveParamStr,
			CosmWasmParamStr:          mainnet.CosmWasmParamStr,
			NewBtcHeadersStr:          mainnet.NewBtcHeadersStr,
			TokensDistributionStr:     mainnet.TokensDistributionStr,
			AllowedStakingTxHashesStr: mainnet.AllowedStakingTxHashesStr,
		}, mainnet.ParamUpgrade)}
}
