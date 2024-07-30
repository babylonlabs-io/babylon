//go:build e2e

package app

import (
	"github.com/babylonlabs-io/babylon/app/upgrades/launchsignet"
	"github.com/babylonlabs-io/babylon/app/upgrades/vanilla"
)

func init() {
	Upgrades = append(Upgrades, vanilla.Upgrade, launchsignet.Upgrade)
}
