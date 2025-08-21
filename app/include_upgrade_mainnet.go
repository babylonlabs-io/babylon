//go:build mainnet

package app

import (
	"github.com/babylonlabs-io/babylon/v3/app/upgrades"
	v2 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v2"
	v22 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v2_2"
	v3 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v3"
)

var WhitelistedChannelsID = map[string]struct{}{
	"channel-0": {},
	"channel-1": {},
	"channel-2": {},
	"channel-3": {},
	"channel-4": {},
	"channel-5": {},
	"channel-6": {},
}

// init is used to include v3 upgrade for mainnet data
func init() {
	Upgrades = []upgrades.Upgrade{
		v3.CreateUpgrade(true, 5, 915000, 2419200), // to be updated
		v22.Upgrade,
		v2.CreateUpgrade(false, WhitelistedChannelsID),
	}
}
