package configurer_test

import (
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/babylonlabs-io/babylon/app/upgrades/signetlaunch"
	"github.com/babylonlabs-io/babylon/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/test/e2e/configurer/config"
	"github.com/stretchr/testify/require"
)

func TestParseGovPropFromFile(t *testing.T) {
	pwd, err := os.Getwd()
	require.NoError(t, err)
	upgradePath := filepath.Join(pwd, "../", config.UpgradeSignetLaunchFilePath)

	prop, err := configurer.ParseGovPropFromFile(upgradePath)
	require.NoError(t, err)

	require.Equal(t, prop.Plan.Name, v1.Upgrade.UpgradeName)
	require.Equal(t, prop.Plan.Height, int64(25))
}
