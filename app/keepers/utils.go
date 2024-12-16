package keepers

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/cosmos/cosmos-sdk/client/config"
)

const defaultConfigTemplate = `# This is a TOML config file.
# For more information, see https://github.com/toml-lang/toml

###############################################################################
###                           Client Configuration                            ###
###############################################################################

# The network chain ID
chain-id = "{{ .ChainID }}"
# The keyring's backend, where the keys are stored (os|file|kwallet|pass|test|memory)
keyring-backend = "{{ .KeyringBackend }}"
# CLI output format (text|json)
output = "{{ .Output }}"
# <host>:<port> to Tendermint RPC interface for this chain
node = "{{ .Node }}"
# Transaction broadcasting mode (sync|async|block)
broadcast-mode = "{{ .BroadcastMode }}"
`

func CreateClientConfig(chainID string, backend string, homePath string) (*config.ClientConfig, error) {
	cliConf := &config.ClientConfig{
		ChainID:        chainID,
		KeyringBackend: backend,
		Output:         "text",                  // default value from config.ClientConfig
		Node:           "tcp://localhost:26657", // default value from config.ClientConfig
		BroadcastMode:  "sync",                  // default value from config.ClientConfig
	}
	err := saveClientConfig(homePath, cliConf)
	if err != nil {
		return nil, err
	}

	return cliConf, err
}

func saveClientConfig(homePath string, cliConf *config.ClientConfig) error {
	var err error
	configPath := filepath.Join(homePath, "config")
	configFilePath := filepath.Join(configPath, "client.toml")
	if err = ensureConfigPath(configPath); err != nil {
		return fmt.Errorf("couldn't make client config: %v", err)
	}

	if err = writeConfigToFile(configFilePath, cliConf); err != nil {
		return fmt.Errorf("could not write client config to the file: %v", err)
	}

	return nil
}

// ensureConfigPath creates a directory configPath if it does not exist
func ensureConfigPath(configPath string) error {
	return os.MkdirAll(filepath.Clean(configPath), 0750)
}

func writeConfigToFile(configFilePath string, config *config.ClientConfig) error {
	var buffer bytes.Buffer

	tmpl := template.New("clientConfigFileTemplate")
	configTemplate, err := tmpl.Parse(defaultConfigTemplate)
	if err != nil {
		return err
	}

	if err := configTemplate.Execute(&buffer, config); err != nil {
		return err
	}

	return os.WriteFile(configFilePath, buffer.Bytes(), 0600)
}
