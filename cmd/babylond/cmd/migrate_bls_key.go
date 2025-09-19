package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/babylonlabs-io/babylon/v4/app"

	appsigner "github.com/babylonlabs-io/babylon/v4/app/signer"
	"github.com/babylonlabs-io/babylon/v4/crypto/bls12381"
	cmtcfg "github.com/cometbft/cometbft/config"
	cmtcrypto "github.com/cometbft/cometbft/crypto"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	cmtos "github.com/cometbft/cometbft/libs/os"
	"github.com/cometbft/cometbft/privval"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"
)

// PrevWrappedFilePV is a struct for prev version of priv_validator_key.json
type PrevWrappedFilePV struct {
	PrivKey    cmtcrypto.PrivKey   `json:"priv_key"`
	BlsPrivKey bls12381.PrivateKey `json:"bls_priv_key"`
}

// MigrateBlsKeyCmd returns a command to migrate the bls keys
func MigrateBlsKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate-bls-key",
		Short: "Migrate the contents of the priv_validator_key.json file into separate files of bls and comet",
		Long: strings.TrimSpace(`Migration splits the contents of the priv_validator_key.json file, 
		which contained both the bls and comet keys used in previous versions, into separate files.

BLS keys are stored along with the Ed25519 validator key in priv_validator_key.json in the previous version,
which should exist before running the command (via babylond init or babylond testnet).

NOTE: Before proceeding with the migration, ensure you back up the priv_validator_key.json file to a secure location.
This will help prevent potential loss of critical validator information in case of issues during the migration process.

Example:
$ babylond migrate-bls-key --home ./
`,
		),

		RunE: func(cmd *cobra.Command, args []string) error {
			homeDir, _ := cmd.Flags().GetString(flags.FlagHome)
			return migrate(homeDir, blsPassword(cmd))
		},
	}

	cmd.Flags().String(flags.FlagHome, app.DefaultNodeHome, "The node home directory")
	cmd.Flags().Bool(flagNoBlsPassword, false, "The BLS key will use an empty password if the flag is set.")
	return cmd
}

// migrate splits the contents of the priv_validator_key.json file,
// which contained both the bls and comet keys used in previous versions, into separate files.
// After saving keys to separate files, it verifies if the migrated keys match
func migrate(homeDir, password string) error {
	cmtcfg := cmtcfg.DefaultConfig()
	cmtcfg.SetRoot(homeDir)

	filepath := cmtcfg.PrivValidatorKeyFile()

	if !cmtos.FileExists(filepath) {
		return fmt.Errorf("priv_validator_key.json of previous version not found in %s", filepath)
	}

	pv, err := loadPrevWrappedFilePV(filepath)
	if err != nil {
		return fmt.Errorf("failed to load previous version of priv_validator_key.json in %s", filepath)
	}

	prevCmtPrivKey := pv.PrivKey
	prevBlsPrivKey := pv.BlsPrivKey

	if prevCmtPrivKey == nil || prevBlsPrivKey == nil {
		return fmt.Errorf("priv_validator_key.json of previous version does not contain both the comet and bls keys")
	}

	cmtKeyFilePath := cmtcfg.PrivValidatorKeyFile()
	cmtStateFilePath := cmtcfg.PrivValidatorStateFile()
	blsKeyFilePath := appsigner.DefaultBlsKeyFile(homeDir)
	blsPasswordFilePath := appsigner.DefaultBlsPasswordFile(homeDir)

	cmtPv := privval.NewFilePV(prevCmtPrivKey, cmtKeyFilePath, cmtStateFilePath)
	bls := appsigner.NewBls(prevBlsPrivKey, blsKeyFilePath, blsPasswordFilePath)

	// save key to files after verification
	cmtPv.Save()
	bls.Key.Save(password)

	if err := verifySeparateFiles(
		cmtKeyFilePath, cmtStateFilePath, blsKeyFilePath, blsPasswordFilePath,
		prevCmtPrivKey, prevBlsPrivKey,
	); err != nil {
		return fmt.Errorf("failed to verify separate files: %w", err)
	}

	return nil
}

// loadPrevWrappedFilePV loads a prev version of priv_validator_key.json
func loadPrevWrappedFilePV(filePath string) (*PrevWrappedFilePV, error) {
	keyJSONBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("Error reading PrivValidator key from %v: %v\n", filePath, err)
	}
	pvKey := PrevWrappedFilePV{}
	err = cmtjson.Unmarshal(keyJSONBytes, &pvKey)
	if err != nil {
		return nil, fmt.Errorf("Error reading PrivValidator key from %v: %v\n", filePath, err)
	}
	return &pvKey, nil
}

// verifySeparateFiles checks if the migrated keys match
// after saving keys to separate files
func verifySeparateFiles(
	cmtKeyFilePath, cmtStateFilePath, blsKeyFilePath, blsPasswordFilePath string,
	prevCmtPrivKey cmtcrypto.PrivKey,
	prevBlsPrivKey bls12381.PrivateKey,
) error {
	cmtPv := privval.LoadFilePV(cmtKeyFilePath, cmtStateFilePath)
	bls, _, _ := appsigner.TryLoadBlsFromFile(blsKeyFilePath, blsPasswordFilePath)

	if bytes.Equal(prevCmtPrivKey.Bytes(), cmtPv.Key.PrivKey.Bytes()) && bytes.Equal(prevBlsPrivKey, bls.Key.PrivKey) {
		return nil
	}
	return fmt.Errorf("migrated keys do not match")
}
