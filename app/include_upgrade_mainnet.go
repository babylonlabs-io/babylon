//go:build mainnet

package app

import (
	"github.com/babylonlabs-io/babylon/v3/app/upgrades"
	v2 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v2"
	v22 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v2_2"
	v3 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v3"
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

// init is used to include v3 upgrade for mainnet data
func init() {
	Upgrades = []upgrades.Upgrade{
		v3.Upgrade,
		v22.Upgrade,
		v2.CreateUpgrade(false, WhitelistedChannelsID),
	}
}
