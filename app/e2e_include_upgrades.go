//go:build e2e

package app

import v1 "github.com/babylonlabs-io/babylon/app/upgrades/v1"

// init is used to include signet upgrade used for e2e testing
// this file should be removed once the upgrade testing with signet ends.
func init() {
	Upgrades = append(Upgrades, v1.Upgrade)
}
