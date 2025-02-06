package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"cosmossdk.io/client/v2/autocli"
	confixcmd "cosmossdk.io/tools/confix/cmd"
	"github.com/CosmWasm/wasmd/x/wasm"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	cmtcfg "github.com/cometbft/cometbft/config"
	cmtcli "github.com/cometbft/cometbft/libs/cli"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtxconfig "github.com/cosmos/cosmos-sdk/x/auth/tx/config"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"

	appsigner "github.com/babylonlabs-io/babylon/app/signer"

	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"

	"github.com/babylonlabs-io/babylon/app"
	"github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/cmd/babylond/cmd/genhelpers"
	checkpointingtypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
)

// NewRootCmd creates a new root command for babylond. It is called once in the
// main function.
func NewRootCmd() *cobra.Command {
	// we "pre"-instantiate the application for getting the injected/configured encoding configuration
	// note, this is not necessary when using app wiring, as depinject can be directly used
	tempApp := app.NewTmpBabylonApp()

	initClientCtx := client.Context{}.
		WithCodec(tempApp.AppCodec()).
		WithInterfaceRegistry(tempApp.InterfaceRegistry()).
		WithTxConfig(tempApp.TxConfig()).
		WithLegacyAmino(tempApp.LegacyAmino()).
		WithInput(os.Stdin).
		WithAccountRetriever(types.AccountRetriever{}).
		WithHomeDir(app.DefaultNodeHome).
		WithViper("") // In app, we don't use any prefix for env variables.

	rootCmd := &cobra.Command{
		Use:   "babylond",
		Short: "Start the Babylon app",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// set the default command outputs
			cmd.SetOut(cmd.OutOrStdout())
			cmd.SetErr(cmd.ErrOrStderr())

			initClientCtx = initClientCtx.WithCmdContext(cmd.Context())
			initClientCtx, err := client.ReadPersistentCommandFlags(initClientCtx, cmd.Flags())
			if err != nil {
				return fmt.Errorf("failed to read command flags: %w", err)
			}

			initClientCtx, err = config.ReadFromClientConfig(initClientCtx)
			if err != nil {
				return fmt.Errorf("failed to read client config: %w", err)
			}

			if !initClientCtx.Offline {
				var modes []signing.SignMode
				modes = append(modes, tx.DefaultSignModes...)
				modes = append(modes, signing.SignMode_SIGN_MODE_TEXTUAL)

				txConfigOpts := tx.ConfigOptions{
					EnabledSignModes:           modes,
					TextualCoinMetadataQueryFn: authtxconfig.NewGRPCCoinMetadataQueryFn(initClientCtx),
				}
				txConfig, err := tx.NewTxConfigWithOptions(
					initClientCtx.Codec,
					txConfigOpts,
				)
				if err != nil {
					return fmt.Errorf("failed to create tx config: %w", err)
				}

				initClientCtx = initClientCtx.WithTxConfig(txConfig)
			}

			if err := client.SetCmdClientContextHandler(initClientCtx, cmd); err != nil {
				return fmt.Errorf("failed to set cmd client context handler: %w", err)
			}

			customAppTemplate, customAppConfig := initAppConfig()
			customCometConfig := initCometConfig()

			err = server.InterceptConfigsPreRunHandler(cmd, customAppTemplate, customAppConfig, customCometConfig)

			if err != nil {
				return fmt.Errorf("failed to intercept configs: %w", err)
			}

			return nil
		},
	}

	initRootCmd(rootCmd, tempApp.TxConfig(), tempApp.BasicModuleManager)

	// add keyring to autocli opts
	autoCliOpts := tempApp.AutoCliOpts()
	initClientCtx, _ = config.ReadFromClientConfig(initClientCtx)
	autoCliOpts.Keyring, _ = keyring.NewAutoCLIKeyring(initClientCtx.Keyring)
	autoCliOpts.ClientCtx = initClientCtx
	autoCliOpts.TxConfigOpts = tx.ConfigOptions{
		EnabledSignModes:           tx.DefaultSignModes,
		TextualCoinMetadataQueryFn: authtxconfig.NewGRPCCoinMetadataQueryFn(initClientCtx),
	}

	EnhanceRootCommandWithoutTxStaking(autoCliOpts, rootCmd)

	return rootCmd
}

// EnhanceRootCommandWithoutTxStaking excludes staking tx commands
func EnhanceRootCommandWithoutTxStaking(autoCliOpts autocli.AppOptions, rootCmd *cobra.Command) {
	if err := autoCliOpts.EnhanceRootCommand(rootCmd); err != nil {
		panic(fmt.Errorf("failed to enhance root command: %w", err))
	}

	txCmd := FindSubCommand(rootCmd, "tx")
	if txCmd == nil {
		panic("failed to find tx subcommand")
	}

	stkTxCmd := FindSubCommand(txCmd, "staking")
	if stkTxCmd == nil {
		panic("failed to find tx staking subcommand")
	}
	txCmd.RemoveCommand(stkTxCmd)
}

// FindSubCommand finds a sub-command of the provided command whose Use
// string is or begins with the provided subCmdName.
// It verifies the command's aliases as well.
func FindSubCommand(cmd *cobra.Command, subCmdName string) *cobra.Command {
	for _, subCmd := range cmd.Commands() {
		use := subCmd.Use
		if use == subCmdName || strings.HasPrefix(use, subCmdName+" ") {
			return subCmd
		}

		for _, alias := range subCmd.Aliases {
			if alias == subCmdName || strings.HasPrefix(alias, subCmdName+" ") {
				return subCmd
			}
		}
	}
	return nil
}

// initCometConfig helps to override default Comet Config values.
// return tmcfg.DefaultConfig if no custom configuration is required for the application.
func initCometConfig() *cmtcfg.Config {
	cfg := cmtcfg.DefaultConfig()

	// these values put a higher strain on node memory
	// cfg.P2P.MaxNumInboundPeers = 100
	// cfg.P2P.MaxNumOutboundPeers = 40

	return cfg
}

// initAppConfig helps to override default appConfig template and configs.
// return "", nil if no custom configuration is required for the application.
func initAppConfig() (string, interface{}) {
	// The following code snippet is just for reference.

	// Optionally allow the chain developer to overwrite the SDK's default
	// server config.
	babylonConfig := DefaultBabylonAppConfig()
	babylonTemplate := DefaultBabylonTemplate()
	return babylonTemplate, babylonConfig
}

func initRootCmd(rootCmd *cobra.Command, txConfig client.TxEncodingConfig, basicManager module.BasicManager) {
	cfg := sdk.GetConfig()
	cfg.Seal()

	gentxModule := basicManager[genutiltypes.ModuleName].(genutil.AppModuleBasic)

	rootCmd.AddCommand(
		InitCmd(basicManager, app.DefaultNodeHome),
		genhelpers.CollectGenTxsCmd(banktypes.GenesisBalancesIterator{}, app.DefaultNodeHome, gentxModule.GenTxValidator, authcodec.NewBech32Codec(params.Bech32PrefixValAddr)),
		genutilcli.MigrateGenesisCmd(genutilcli.MigrationMap),
		genhelpers.GenTxCmd(basicManager, txConfig, banktypes.GenesisBalancesIterator{}, app.DefaultNodeHome, authcodec.NewBech32Codec(params.Bech32PrefixValAddr)),
		ValidateGenesisCmd(basicManager, gentxModule.GenTxValidator),
		PrepareGenesisCmd(app.DefaultNodeHome, basicManager),
		AddGenesisAccountCmd(app.DefaultNodeHome),
		cmtcli.NewCompletionCmd(rootCmd, true),
		TestnetCmd(basicManager, banktypes.GenesisBalancesIterator{}),
		genhelpers.CmdGenHelpers(gentxModule.GenTxValidator),
		MigrateBlsKeyCmd(),
		CreateBlsKeyCmd(),
		ModuleSizeCmd(),
		DebugCmd(),
		confixcmd.ConfigCommand(),
	)

	server.AddCommands(rootCmd, app.DefaultNodeHome, newApp, appExport, addModuleInitFlags)

	// add keybase, auxiliary RPC, query, and tx child commands
	rootCmd.AddCommand(
		server.StatusCommand(),
		queryCommand(),
		txCommand(),
		keys.Commands(),
	)
}

func addModuleInitFlags(startCmd *cobra.Command) {
	crisis.AddModuleInitFlags(startCmd)
	wasm.AddModuleInitFlags(startCmd)

	startCmd.Flags().String(flags.FlagKeyringBackend, flags.DefaultKeyringBackend, "Select keyring's backend (os|file|kwallet|pass|test)")
	startCmd.Flags().String(flags.FlagChainID, "", "genesis file chain-id, if left blank will be randomly created")
}

func queryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "query",
		Aliases:                    []string{"q"},
		Short:                      "Querying subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		rpc.QueryEventForTxCmd(),
		rpc.ValidatorCommand(),
		server.QueryBlockCmd(),
		authcmd.QueryTxsByEventsCmd(),
		server.QueryBlocksCmd(),
		authcmd.QueryTxCmd(),
		server.QueryBlockResultsCmd(),
	)

	cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")

	return cmd
}

func txCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "tx",
		Short:                      "Transactions subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		authcmd.GetSignCommand(),
		authcmd.GetSignBatchCommand(),
		authcmd.GetMultiSignCommand(),
		authcmd.GetMultiSignBatchCmd(),
		authcmd.GetValidateSignaturesCommand(),
		authcmd.GetBroadcastCommand(),
		authcmd.GetEncodeCommand(),
		authcmd.GetDecodeCommand(),
		authcmd.GetSimulateCmd(),
	)

	cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")

	return cmd
}

// newApp is an appCreator
func newApp(logger log.Logger, db dbm.DB, traceStore io.Writer, appOpts servertypes.AppOptions) servertypes.Application {
	baseappOptions := server.DefaultBaseappOptions(appOpts)

	skipUpgradeHeights := make(map[int64]bool)
	for _, h := range cast.ToIntSlice(appOpts.Get(server.FlagUnsafeSkipUpgrades)) {
		skipUpgradeHeights[int64(h)] = true
	}

	homeDir := cast.ToString(appOpts.Get(flags.FlagHome))

	// auto migrate when build tag is set to "e2e_upgrade"
	automigrate_e2e_upgrade(logger, homeDir)

	blsSigner, err := appsigner.InitBlsSigner(homeDir)
	if err != nil {
		panic(fmt.Errorf("failed to initialize priv signer: %w", err))
	}

	var wasmOpts []wasmkeeper.Option
	if cast.ToBool(appOpts.Get("telemetry.enabled")) {
		wasmOpts = append(wasmOpts, wasmkeeper.WithVMCacheMetrics(prometheus.DefaultRegisterer))
	}

	return app.NewBabylonApp(
		logger, db, traceStore, true, skipUpgradeHeights,
		cast.ToUint(appOpts.Get(server.FlagInvCheckPeriod)),
		blsSigner,
		appOpts,
		wasmOpts,
		baseappOptions...,
	)
}

// appExport creates a new app (optionally at a given height)
// and exports state.
func appExport(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	height int64,
	forZeroHeight bool,
	jailAllowedAddrs []string,
	appOpts servertypes.AppOptions,
	modulesToExport []string,
) (servertypes.ExportedApp, error) {
	var babylonApp *app.BabylonApp
	homePath, ok := appOpts.Get(flags.FlagHome).(string)
	if !ok || homePath == "" {
		return servertypes.ExportedApp{}, errors.New("application home not set")
	}

	ck, err := appsigner.LoadConsensusKey(homePath)
	if err != nil {
		panic(fmt.Errorf("failed to initialize priv signer: %w", err))
	}

	blsSigner := checkpointingtypes.BlsSigner(ck.Bls)

	if height != -1 {
		babylonApp = app.NewBabylonApp(logger, db, traceStore, false, map[int64]bool{}, uint(1), &blsSigner, appOpts, app.EmptyWasmOpts)

		if err = babylonApp.LoadHeight(height); err != nil {
			return servertypes.ExportedApp{}, fmt.Errorf("failed to load height: %w", err)
		}
	} else {
		babylonApp = app.NewBabylonApp(logger, db, traceStore, true, map[int64]bool{}, uint(1), &blsSigner, appOpts, app.EmptyWasmOpts)
	}

	return babylonApp.ExportAppStateAndValidators(forZeroHeight, jailAllowedAddrs, modulesToExport)
}

// automigrate_e2e_upgrade_test runs when the build tag is set to "e2e_upgrade".
// It always checks if the key structure is the previous version
// and migrates into a separate version of the divided key files
func automigrate_e2e_upgrade(logger log.Logger, homeDir string) {
	if app.IsE2EUpgradeBuildFlag {
		logger.Debug(
			"***************************************************************************\n" +
				"NOTE: In testnet mode, it will automatically migrate the key file\n" +
				"if priv_validator_key.json contains both the comet and bls keys,\n" +
				"used in previous version.\n" +
				"Do not run it in a production environment, as it may cause problems.\n" +
				"***************************************************************************\n",
		)
		if err := migrate(homeDir, "password"); err != nil {
			logger.Debug(err.Error())
		}
	}
}
