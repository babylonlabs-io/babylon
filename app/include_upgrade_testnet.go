//go:build testnet

package app

import (
	"github.com/babylonlabs-io/babylon/app/upgrades"
	v1 "github.com/babylonlabs-io/babylon/app/upgrades/v1"
	"github.com/babylonlabs-io/babylon/app/upgrades/v1/testnet"
)

// init is used to include v1 upgrade testnet data
// it is also used for e2e testing
func init() {
	Upgrades = []upgrades.Upgrade{v1.CreateUpgrade(v1.UpgradeDataString{
		BtcStakingParamStr:    testnet.BtcStakingParamStr,
		FinalityParamStr:      testnet.FinalityParamStr,
		CosmWasmParamStr:      testnet.CosmWasmParamStr,
		NewBtcHeadersStr:      testnet.NewBtcHeadersStr,
		SignedFPsStr:          testnet.SignedFPsStr,
		TokensDistributionStr: testnet.TokensDistributionStr,
	})}
}
