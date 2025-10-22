//go:build testnet

package app

import (
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
	v1_1 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v1_1"
	v1rc5 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v1rc5/testnet"
	v1rc8 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v1rc8/testnet"
	v1rc9 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v1rc9/testnet"
	v2 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v2"
	v22 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v2_2"
	v23 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v2_3"
	v2rc4 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v2rc4/testnet"
	v4 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v4"
	v5 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v5"
)

// init is used to include v1 upgrade testnet data
// it is also used for e2e testing
func init() {
	Upgrades = []upgrades.Upgrade{
		v5.Upgrade,
		v4.Upgrade,
		v23.Upgrade,
		v22.Upgrade,
		v2rc4.Upgrade,
		v2.CreateUpgrade(true, map[string]struct{}{}),
		v1_1.Upgrade,
		v1rc5.CreateUpgrade(),
		v1rc8.CreateUpgrade(),
		v1rc9.CreateUpgrade(),
		v22.Upgrade,
	}
}
