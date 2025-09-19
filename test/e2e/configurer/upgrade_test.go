package configurer

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v4/app"
	v1 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v1"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/config"
	"github.com/stretchr/testify/require"
)

func TestParseGovPropFromFile(t *testing.T) {
	cdc := app.NewTmpBabylonApp().AppCodec()

	pwd, err := os.Getwd()
	require.NoError(t, err)
	upgradePath := filepath.Join(pwd, "../", config.UpgradeSignetLaunchFilePath)

	_, msgProp, err := parseGovPropFromFile(cdc, upgradePath)
	require.NoError(t, err)

	require.Equal(t, msgProp.Plan.Name, v1.UpgradeName)
}

func TestWriteGovPropToFile(t *testing.T) {
	cdc := app.NewTmpBabylonApp().AppCodec()

	pwd, err := os.Getwd()
	require.NoError(t, err)
	upgradePath := filepath.Join(pwd, "../", config.UpgradeSignetLaunchFilePath)

	prop, msgProp, err := parseGovPropFromFile(cdc, upgradePath)
	require.NoError(t, err)

	r := rand.New(rand.NewSource(time.Now().Unix()))
	newPropHeight := r.Int63()
	msgProp.Plan.Height = newPropHeight

	tempFilePath := filepath.Join(t.TempDir(), filepath.Base(config.UpgradeSignetLaunchFilePath))

	err = writeGovPropToFile(cdc, tempFilePath, *prop, *msgProp)
	require.NoError(t, err)

	_, updatedProp, err := parseGovPropFromFile(cdc, tempFilePath)
	require.NoError(t, err)

	require.Equal(t, updatedProp.Plan.Name, msgProp.Plan.Name)
	require.Equal(t, updatedProp.Plan.Height, newPropHeight)
}
