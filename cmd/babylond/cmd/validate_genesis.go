package cmd

import (
	"encoding/json"
	"fmt"

	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/types/module"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/spf13/cobra"
)

const chainUpgradeGuide = "https://github.com/cosmos/cosmos-sdk/blob/a51aa517c46c70df04a06f586c67fb765e45322a/UPGRADING.md"

// ValidateGenesisCmd takes a genesis file, and makes sure that it is valid.
// 1. genesis state of each module should be valid according to each module's
// validation rule
// 2. each genesis BLS key or gentx should have a corresponding gentx or genesis
// BLS key
// modified based on "https://github.com/cosmos/cosmos-sdk/blob/6d32debf1aca4b7f1ed1429d87be1d02c315f02d/x/genutil/client/cli/validate_genesis.go"
func ValidateGenesisCmd(mbm module.BasicManager, validator genutiltypes.MessageValidator) *cobra.Command {
	return &cobra.Command{
		Use:   "validate-genesis [file]",
		Args:  cobra.RangeArgs(0, 1),
		Short: "validates the genesis file at the default location or at the location passed as an arg",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			serverCtx := server.GetServerContextFromCmd(cmd)
			clientCtx := client.GetClientContextFromCmd(cmd)

			cdc := clientCtx.Codec

			// Load default if passed no args, otherwise load passed file
			var genesis string
			if len(args) == 0 {
				genesis = serverCtx.Config.GenesisFile()
			} else {
				genesis = args[0]
			}

			genDoc, err := validateGenDoc(genesis)
			if err != nil {
				return err
			}

			var genState map[string]json.RawMessage
			if err = json.Unmarshal(genDoc.AppState, &genState); err != nil {
				return fmt.Errorf("error unmarshalling genesis doc %s: %s", genesis, err.Error())
			}

			if err = mbm.ValidateGenesis(cdc, clientCtx.TxConfig, genState); err != nil {
				return fmt.Errorf("error validating genesis file %s: %s", genesis, err.Error())
			}

			fmt.Printf("File at %s is a valid genesis file\n", genesis)
			return nil
		},
	}
}

// validateGenDoc reads a genesis file and validates that it is a correct
// Tendermint GenesisDoc. This function does not do any cosmos-related
// validation.
func validateGenDoc(importGenesisFile string) (*cmttypes.GenesisDoc, error) {
	genDoc, err := cmttypes.GenesisDocFromFile(importGenesisFile)
	if err != nil {
		return nil, fmt.Errorf("%s. Make sure that"+
			" you have correctly migrated all Tendermint consensus params, please see the"+
			" chain migration guide at %s for more info",
			err.Error(), chainUpgradeGuide,
		)
	}

	return genDoc, nil
}
