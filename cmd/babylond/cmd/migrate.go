package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/babylonlabs-io/babylon/app"
	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/privval"
	cmtcfg "github.com/cometbft/cometbft/config"
	cmtcrypto "github.com/cometbft/cometbft/crypto"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	cmtos "github.com/cometbft/cometbft/libs/os"
	cmtprivval "github.com/cometbft/cometbft/privval"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"
)

// PrevWrappedFilePV is a struct for prev version of priv_validator_key.json
type PrevWrappedFilePV struct {
	PrivKey    cmtcrypto.PrivKey   `json:"priv_key"`
	BlsPrivKey bls12381.PrivateKey `json:"bls_priv_key"`
}

func MigrateBlsKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate-bls-key",
		Short: "Migrate the contents of the priv_validator_key.json file into separate files of bls and comet",
		Long: strings.TrimSpace(`migrate splits the contents of the priv_validator_key.json file, 
		which contained both the bls and comet keys used in previous versions, into separate files.

BLS keys are stored along with other validator keys in priv_validator_key.json in previous version,
which should exist before running the command (via babylond init or babylond testnet).

NOTE: Before proceeding with the migration, ensure you back up the priv_validator_key.json file to a secure location.
This will help prevent potential loss of critical validator information in case of issues during the migration process.

Example:
$ babylond migrate-bls-key --home ./
`,
		),

		RunE: func(cmd *cobra.Command, args []string) error {
			homeDir, _ := cmd.Flags().GetString(flags.FlagHome)
			password, _ := cmd.Flags().GetString(flagBlsPassword)
			return migrate(homeDir, password)
		},
	}

	cmd.Flags().String(flags.FlagHome, app.DefaultNodeHome, "The node home directory")
	cmd.Flags().String(flagBlsPassword, "", "The password for the BLS key. If a flag is set, the non-empty password should be provided. If a flag is not set, the password will be read from the prompt.")
	return cmd
}

// migrate splits the contents of the priv_validator_key.json file,
// which contained both the bls and comet keys used in previous versions, into separate files
func migrate(homeDir, password string) error {
	cmtcfg := cmtcfg.DefaultConfig()
	cmtcfg.SetRoot(homeDir)

	filepath := cmtcfg.PrivValidatorKeyFile()

	if !cmtos.FileExists(filepath) {
		return fmt.Errorf("priv_validator_key.json of previous version not found")
	}

	pv, err := loadPrevWrappedFilePV(filepath)
	if err != nil {
		return err
	}

	prevCmtPrivKey := pv.PrivKey
	prevBlsPrivKey := pv.BlsPrivKey

	if prevCmtPrivKey == nil || prevBlsPrivKey == nil {
		return fmt.Errorf("priv_validator_key.json of previous version does not contain both the comet and bls keys")
	}

	if password == "" {
		password = privval.NewBlsPassword()
	}

	cmtPv := cmtprivval.NewFilePV(prevCmtPrivKey, cmtcfg.PrivValidatorKeyFile(), cmtcfg.PrivValidatorStateFile())
	blsPv := privval.NewBlsPV(prevBlsPrivKey, privval.DefaultBlsKeyFile(homeDir), privval.DefaultBlsPasswordFile(homeDir))

	// before saving keys to files, verify that the migrated keys match
	if err := verifyAfterMigration(
		prevCmtPrivKey,
		cmtPv.Key.PrivKey,
		prevBlsPrivKey,
		blsPv.Key.PrivKey,
	); err != nil {
		return fmt.Errorf("failed to verify after migration: %w", err)
	}

	// save key to files after verification
	cmtPv.Save()
	blsPv.Key.Save(password)
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

// verifyAfterMigration checks if the migrated keys match
func verifyAfterMigration(prevCmtPrivKey, newCmtPrivKey cmtcrypto.PrivKey, prevBlsPrivKey, newBlsPrivKey bls12381.PrivateKey) error {
	if bytes.Equal(prevCmtPrivKey.Bytes(), newCmtPrivKey.Bytes()) && bytes.Equal(prevBlsPrivKey, newBlsPrivKey) {
		return nil
	}
	return fmt.Errorf("migrated keys do not match")
}
