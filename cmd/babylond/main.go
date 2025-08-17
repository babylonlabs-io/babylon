package main

import (
	"os"

	"cosmossdk.io/log"

	"github.com/babylonlabs-io/babylon/v4/app"
	"github.com/babylonlabs-io/babylon/v4/cmd/babylond/cmd"
	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"

	"github.com/babylonlabs-io/babylon/v4/app/params"
)

func main() {
	params.SetAddressPrefixes()
	rootCmd := cmd.NewRootCmd()

	if err := svrcmd.Execute(rootCmd, app.BabylonAppEnvPrefix, app.DefaultNodeHome); err != nil {
		log.NewLogger(rootCmd.OutOrStderr()).Error("failure when running app", "err", err)
		os.Exit(1)
	}
}
