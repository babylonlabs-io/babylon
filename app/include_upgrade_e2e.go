//go:build e2e_upgrade

package app

import (
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	v3 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v3"
	v3rc4 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v3rc4/testnet"
	costakingtypes "github.com/babylonlabs-io/babylon/v4/x/costaking/types"
)

// init is used to include v1 upgrade testnet data
// it is also used for e2e testing
func init() {
	IsE2EUpgradeBuildFlag = true
	v3.StoresToAdd = append(v3.StoresToAdd, erc20types.StoreKey, evmtypes.StoreKey, feemarkettypes.StoreKey, precisebanktypes.StoreKey, costakingtypes.StoreKey)
	v3rc4.StoresToAdd = []string{
		costakingtypes.StoreKey, erc20types.StoreKey, evmtypes.StoreKey, feemarkettypes.StoreKey, precisebanktypes.StoreKey,
	}
}
