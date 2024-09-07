package config_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/test/e2e/bcd_consumer_integration/cosmwasmclient/config"
	"github.com/stretchr/testify/require"
)

// TestWasmQueryConfig ensures that the default Babylon query config is valid
func TestWasmQueryConfig(t *testing.T) {
	defaultConfig := config.DefaultWasmQueryConfig()
	err := defaultConfig.Validate()
	require.NoError(t, err)
}
