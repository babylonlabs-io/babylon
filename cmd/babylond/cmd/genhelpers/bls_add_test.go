package genhelpers_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/server/config"
	"github.com/cosmos/cosmos-sdk/types/module"

	"github.com/cosmos/cosmos-sdk/x/genutil"

	appv1alpha1 "cosmossdk.io/api/cosmos/app/v1alpha1"
	authmodulev1 "cosmossdk.io/api/cosmos/auth/module/v1"
	"cosmossdk.io/core/appconfig"
	"cosmossdk.io/depinject"
	"cosmossdk.io/log"
	cmtconfig "github.com/cometbft/cometbft/config"
	tmjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/libs/tempfile"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/testutil/configurator"
	"github.com/cosmos/cosmos-sdk/testutil/network"
	genutiltest "github.com/cosmos/cosmos-sdk/x/genutil/client/testutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/app"
	"github.com/babylonlabs-io/babylon/cmd/babylond/cmd/genhelpers"
	"github.com/babylonlabs-io/babylon/privval"
	"github.com/babylonlabs-io/babylon/testutil/cli"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	"github.com/babylonlabs-io/babylon/x/checkpointing/types"
)

func newConfig() depinject.Config {
	return configurator.NewAppConfig(
		func(config *configurator.Config) {
			config.ModuleConfigs["auth"] = &appv1alpha1.ModuleConfig{
				Name: "auth",
				Config: appconfig.WrapAny(&authmodulev1.Module{
					Bech32Prefix: "bbn", // overwrite prefix here
					ModuleAccountPermissions: []*authmodulev1.ModuleAccountPermission{
						{Account: "fee_collector"},
						{Account: "distribution"},
						{Account: "mint", Permissions: []string{"minter"}},
						{Account: "bonded_tokens_pool", Permissions: []string{"burner", "staking"}},
						{Account: "not_bonded_tokens_pool", Permissions: []string{"burner", "staking"}},
						{Account: "gov", Permissions: []string{"burner"}},
						{Account: "nft"},
					},
				}),
			}
		},
		configurator.ParamsModule(),
		configurator.BankModule(),
		configurator.GenutilModule(),
		configurator.StakingModule(),
		configurator.ConsensusModule(),
		configurator.TxModule(),
	)
}

// test adding genesis BLS keys without gentx
// error is expected
func Test_CmdCreateAddWithoutGentx(t *testing.T) {
	home := t.TempDir()
	logger := log.NewNopLogger()
	cmtcfg, err := genutiltest.CreateDefaultCometConfig(home)
	require.NoError(t, err)

	db := dbm.NewMemDB()
	signer, err := app.SetupTestPrivSigner()
	require.NoError(t, err)
	bbn := app.NewBabylonAppWithCustomOptions(t, false, signer, app.SetupOptions{
		Logger:             logger,
		DB:                 db,
		InvCheckPeriod:     0,
		SkipUpgradeHeights: map[int64]bool{},
		AppOpts:            app.TmpAppOptions(),
	})
	gentxModule := bbn.BasicModuleManager[genutiltypes.ModuleName].(genutil.AppModuleBasic)
	appCodec := bbn.AppCodec()

	err = genutiltest.ExecInitCmd(module.NewBasicManager(genutil.AppModuleBasic{}), home, appCodec)
	require.NoError(t, err)

	serverCtx := server.NewContext(viper.New(), cmtcfg, logger)
	clientCtx := client.Context{}.WithCodec(appCodec).WithHomeDir(home)
	cfg := serverCtx.Config
	cfg.SetRoot(clientCtx.HomeDir)

	ctx := context.Background()
	ctx = context.WithValue(ctx, client.ClientContextKey, &clientCtx)
	ctx = context.WithValue(ctx, server.ServerContextKey, serverCtx)

	genKey := datagen.GenerateGenesisKey()
	jsonBytes, err := tmjson.MarshalIndent(genKey, "", "  ")
	require.NoError(t, err)
	genKeyFileName := filepath.Join(home, fmt.Sprintf("gen-bls-%s.json", genKey.ValidatorAddress))
	err = tempfile.WriteFileAtomic(genKeyFileName, jsonBytes, 0600)
	require.NoError(t, err)
	addGenBlsCmd := genhelpers.CmdAddBls(gentxModule.GenTxValidator)
	addGenBlsCmd.SetArgs(
		[]string{genKeyFileName},
	)
	err = addGenBlsCmd.ExecuteContext(ctx)
	require.Error(t, err)
}

// test adding genesis BLS keys with gentx
// error is expected if adding duplicate
func Test_CmdAddBlsWithGentx(t *testing.T) {
	db := dbm.NewMemDB()
	signer, err := app.SetupTestPrivSigner()
	require.NoError(t, err)
	bbn := app.NewBabylonAppWithCustomOptions(t, false, signer, app.SetupOptions{
		Logger:             log.NewNopLogger(),
		DB:                 db,
		InvCheckPeriod:     0,
		SkipUpgradeHeights: map[int64]bool{},
		AppOpts:            app.TmpAppOptions(),
	})

	gentxModule := bbn.BasicModuleManager[genutiltypes.ModuleName].(genutil.AppModuleBasic)
	config.SetConfigTemplate(config.DefaultConfigTemplate)
	cfg, _ := network.DefaultConfigWithAppConfig(newConfig())
	cfg.NumValidators = 1
	testNetwork, err := network.New(t, t.TempDir(), cfg)
	require.NoError(t, err)
	defer testNetwork.Cleanup()

	_, err = testNetwork.WaitForHeight(1)
	require.NoError(t, err)

	targetCfg := cmtconfig.DefaultConfig()
	targetCfg.SetRoot(filepath.Join(testNetwork.Validators[0].Dir, "simd"))
	targetGenesisFile := targetCfg.GenesisFile()
	targetCtx := testNetwork.Validators[0].ClientCtx
	for i := 0; i < cfg.NumValidators; i++ {
		v := testNetwork.Validators[i]
		// build and create genesis BLS key
		genBlsCmd := genhelpers.CmdCreateBls()
		nodeCfg := cmtconfig.DefaultConfig()
		homeDir := filepath.Join(v.Dir, "simd")
		nodeCfg.SetRoot(homeDir)
		keyPath := nodeCfg.PrivValidatorKeyFile()
		statePath := nodeCfg.PrivValidatorStateFile()
		filePV := privval.GenWrappedFilePV(keyPath, statePath)
		filePV.SetAccAddress(v.Address)
		_, err = cli.ExecTestCLICmd(v.ClientCtx, genBlsCmd, []string{fmt.Sprintf("--%s=%s", flags.FlagHome, homeDir)})
		require.NoError(t, err)
		genKeyFileName := filepath.Join(filepath.Dir(keyPath), fmt.Sprintf("gen-bls-%s.json", v.ValAddress))
		genKey, err := types.LoadGenesisKeyFromFile(genKeyFileName)
		require.NoError(t, err)
		require.NotNil(t, genKey)

		// add genesis BLS key to the target context
		addBlsCmd := genhelpers.CmdAddBls(gentxModule.GenTxValidator)
		_, err = cli.ExecTestCLICmd(targetCtx, addBlsCmd, []string{genKeyFileName})
		require.NoError(t, err)
		appState, _, err := genutiltypes.GenesisStateFromGenFile(targetGenesisFile)
		require.NoError(t, err)
		// test duplicate
		_, err = cli.ExecTestCLICmd(targetCtx, addBlsCmd, []string{genKeyFileName})
		require.Error(t, err)

		checkpointingGenState := types.GetGenesisStateFromAppState(v.ClientCtx.Codec, appState)
		require.NotEmpty(t, checkpointingGenState.GenesisKeys)
		gks := checkpointingGenState.GetGenesisKeys()
		require.Equal(t, genKey, gks[i])
		filePV.Clean(keyPath, statePath)
	}
}
