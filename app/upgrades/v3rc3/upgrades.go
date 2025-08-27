package v3rc3

import (
	store "cosmossdk.io/store/types"

	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades/v3rc3/testnet"
)

// UpgradeName defines the on-chain upgrade name for the Babylon v3rc3 upgrade
const UpgradeName = "v3rc3"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: testnet.CreateUpgradeHandler,
	StoreUpgrades: store.StoreUpgrades{
		Added:   []string{},
		Deleted: []string{},
	},
}
