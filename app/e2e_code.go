//go:build e2e

// This file contains code specific to end-to-end (e2e) testing for the Babylon application.
// It includes the signet upgrade and enables integration.

package app

import (
	"github.com/babylonlabs-io/babylon/app/upgrades/signetlaunch"
	zctypes "github.com/babylonlabs-io/babylon/x/zonecaching/types"
)

// init is used to include signet upgrade used for e2e testing
// this file should be removed once the upgrade testing with signet ends.
func init() {
	Upgrades = append(Upgrades, signetlaunch.Upgrade)
	zctypes.EnableIntegration = true
}
