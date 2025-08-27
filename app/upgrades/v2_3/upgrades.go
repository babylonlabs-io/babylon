package v2_3

import (
	store "cosmossdk.io/store/types"

	"github.com/babylonlabs-io/babylon/v3/app/upgrades"
)

// UpgradeName defines the on-chain upgrade name for the Babylon v2.3 upgrade
const UpgradeName = "v2.3"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: upgrades.CreateUpgradeHandlerFpSoftDeleteDupAddr,
	StoreUpgrades: store.StoreUpgrades{
		Added:   []string{},
		Deleted: []string{},
	},
}
