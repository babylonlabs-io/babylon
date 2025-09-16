//go:build testnet

package app

import (
<<<<<<< HEAD
	"github.com/babylonlabs-io/babylon/v3/app/upgrades"
	v2 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v2"
	v22 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v2_2"
	v2rc4 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v2rc4/testnet"
	v3 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v3"
	v3rc2 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v3rc2/testnet"
	v3rc3 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v3rc3"
=======
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
	v2 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v2"
	v22 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v2_2"
	v2rc4 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v2rc4/testnet"
	v3 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v3"
	v3rc2 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v3rc2/testnet"
	v3rc3 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v3rc3"
	v3rc4 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v3rc4/testnet"
>>>>>>> 5cbf5d53 (Add : Upgrade handler for epoching spam prevention (#1663) (#1703))
)

// init is used to include v1 upgrade testnet data
// it is also used for e2e testing
func init() {
	Upgrades = []upgrades.Upgrade{
		v3rc4.Upgrade,
		v3rc3.Upgrade, // same as v2_3 mainnet
		v3rc2.Upgrade,
		v3.CreateUpgrade(false, 10, 264773, 2419200), // TODO: to be updated
		v2rc4.Upgrade,
		v2.CreateUpgrade(true, map[string]struct{}{}),
		v22.Upgrade,
	}
}
