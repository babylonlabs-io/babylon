package v3rc3

import (
	store "cosmossdk.io/store/types"

	"github.com/babylonlabs-io/babylon/v2/app/upgrades"
)

// UpgradeName defines the on-chain upgrade name for the Babylon v3rc3 upgrade
const UpgradeName = "v3rc3"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: upgrades.CreateUpgradeHandlerFpSoftDeleteDupAddr,
	StoreUpgrades: store.StoreUpgrades{
		Added:   []string{},
		Deleted: []string{},
	},
}
