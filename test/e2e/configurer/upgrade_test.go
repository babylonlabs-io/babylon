package configurer

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	v3 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v3"

	"github.com/babylonlabs-io/babylon/v3/app"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer/config"
	"github.com/stretchr/testify/require"
)

func TestParseGovPropFromFile(t *testing.T) {
	cdc := app.NewTmpBabylonApp().AppCodec()

	pwd, err := os.Getwd()
	require.NoError(t, err)
	upgradePath := filepath.Join(pwd, "../", config.UpgradeV3FilePath)

	_, msgProp, err := parseGovPropFromFile(cdc, upgradePath)
	require.NoError(t, err)

	require.Equal(t, msgProp.Plan.Name, v3.UpgradeName)
}

func TestWriteGovPropToFile(t *testing.T) {
	cdc := app.NewTmpBabylonApp().AppCodec()

	pwd, err := os.Getwd()
	require.NoError(t, err)
	upgradePath := filepath.Join(pwd, "../", config.UpgradeV3FilePath)

	prop, msgProp, err := parseGovPropFromFile(cdc, upgradePath)
	require.NoError(t, err)

	r := rand.New(rand.NewSource(time.Now().Unix()))
	newPropHeight := r.Int63()
	msgProp.Plan.Height = newPropHeight

	tempFilePath := filepath.Join(t.TempDir(), filepath.Base(config.UpgradeV3FilePath))

	err = writeGovPropToFile(cdc, tempFilePath, *prop, *msgProp)
	require.NoError(t, err)

	_, updatedProp, err := parseGovPropFromFile(cdc, tempFilePath)
	require.NoError(t, err)

	require.Equal(t, updatedProp.Plan.Name, msgProp.Plan.Name)
	require.Equal(t, updatedProp.Plan.Height, newPropHeight)
}
