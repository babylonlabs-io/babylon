//go:build e2e_upgrade

package app

// init is used to include v1 upgrade testnet data
// it is also used for e2e testing
func init() {
	IsE2EUpgradeBuildFlag = true
}
