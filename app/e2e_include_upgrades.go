//go:build e2e

package app

import (
	"github.com/babylonlabs-io/babylon/app/upgrades/signetlaunch"
)

// init is used to include signet upgrade used for e2e testing
// this file should be removed once the upgrade testing with signet ends.
func init() {
	Upgrades = append(Upgrades, signetlaunch.Upgrade)
}
