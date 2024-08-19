//go:build e2e

package app

import (
	"github.com/babylonlabs-io/babylon/app/upgrades/signetlaunch"
)

func init() {
	Upgrades = append(Upgrades, signetlaunch.Upgrade)
}
