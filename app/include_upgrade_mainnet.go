//go:build mainnet

package app

import (
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades/v1_1"
	v2 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v2"
	v22 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v2_2"
	v23 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v2_3"
	v4 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v4"
	v41 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v4_1"
	v42 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v4_2"
	v5 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v5"
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
		v5.Upgrade,
		v42.Upgrade,
		v41.Upgrade,
		v4.Upgrade,
		v23.Upgrade, // same as v3rc3 testnet
		v22.Upgrade,
		v2.CreateUpgrade(false, WhitelistedChannelsID),
		v1_1.Upgrade,
	}
}
