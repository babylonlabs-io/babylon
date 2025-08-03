//go:build e2e_upgrade

package app

import (
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"github.com/babylonlabs-io/babylon/v3/app/upgrades"
	v2 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v2"
	v22 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v2_2"
	v2rc4 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v2rc4/testnet"
	v3 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v3"
)

// init is used to include v1 upgrade testnet data
// it is also used for e2e testing
func init() {
	IsE2EUpgradeBuildFlag = true
	v3.StoresToAdd = append(v3.StoresToAdd, erc20types.StoreKey, evmtypes.StoreKey, feemarkettypes.StoreKey, precisebanktypes.StoreKey)
	Upgrades = []upgrades.Upgrade{
		v3.CreateUpgrade(false, 10, 260000, 2419200), // TODO: to be updated
		v2rc4.Upgrade,
		v2.CreateUpgrade(true, map[string]struct{}{}),
		v22.Upgrade,
	}
}
