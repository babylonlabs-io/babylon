package genhelpers_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	dbm "github.com/cosmos/cosmos-db"

	"cosmossdk.io/log"
	appparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutiltest "github.com/cosmos/cosmos-sdk/x/genutil/client/testutil"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/app"
	"github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/cmd/babylond/cmd"
	"github.com/babylonlabs-io/babylon/cmd/babylond/cmd/genhelpers"
	"github.com/babylonlabs-io/babylon/privval"
	"github.com/babylonlabs-io/babylon/testutil/signer"
	"github.com/babylonlabs-io/babylon/x/checkpointing/types"
)

func Test_CmdGenTx(t *testing.T) {
	home := t.TempDir()
	logger := log.NewNopLogger()
	cfg, err := genutiltest.CreateDefaultCometConfig(home)
	require.NoError(t, err)

	signer, err := signer.SetupTestPrivSigner()
	require.NoError(t, err)
	bbn := app.NewBabylonAppWithCustomOptions(t, false, signer, app.SetupOptions{
		Logger:             logger,
		DB:                 dbm.NewMemDB(),
		InvCheckPeriod:     0,
		SkipUpgradeHeights: map[int64]bool{},
		AppOpts:            app.TmpAppOptions(),
	})

	err = genutiltest.ExecInitCmd(bbn.BasicModuleManager, home, bbn.AppCodec())
	require.NoError(t, err)

	serverCtx := server.NewContext(viper.New(), cfg, logger)
	clientCtx := client.Context{}.
		WithCodec(bbn.AppCodec()).
		WithHomeDir(home).
		WithTxConfig(bbn.TxConfig())

	bbn.TxConfig()

	ctx := context.Background()
	ctx = context.WithValue(ctx, server.ServerContextKey, serverCtx)
	ctx = context.WithValue(ctx, client.ClientContextKey, &clientCtx)

	// create keyring to get the validator address
	kb, err := keyring.New(sdk.KeyringServiceName(), keyring.BackendTest, home, os.Stdin, clientCtx.Codec)
	require.NoError(t, err)
	keyringAlgos, _ := kb.SupportedAlgorithms()
	algo, err := keyring.NewSigningAlgoFromString(string(hd.Secp256k1Type), keyringAlgos)
	require.NoError(t, err)

	keyName := "legoo"
	addr, _, err := testutil.GenerateSaveCoinKey(kb, keyName, "", true, algo)
	require.NoError(t, err)

	baseFlags := []string{
		fmt.Sprintf("--%s=%s", flags.FlagHome, home),
		fmt.Sprintf("--%s=%s", flags.FlagKeyringBackend, keyring.BackendTest),
	}

	argsGen := append([]string{
		keyName,
		fmt.Sprintf("%d%s", 10000, appparams.BaseCoinUnit),
	}, baseFlags...)
	// add funds to validator in genesis
	addGenAcc := cmd.AddGenesisAccountCmd(home)
	addGenAcc.SetArgs(argsGen)
	err = addGenAcc.ExecuteContext(ctx)
	require.NoError(t, err)

	genTxCmd := genhelpers.GenTxCmd(bbn.BasicModuleManager, bbn.TxConfig(), banktypes.GenesisBalancesIterator{}, app.DefaultNodeHome, authcodec.NewBech32Codec(params.Bech32PrefixValAddr))
	genTxCmd.SetArgs(append(argsGen, fmt.Sprintf("--%s=%s", flags.FlagChainID, "test-chain-id")))

	// execute the gentx cmd
	err = genTxCmd.ExecuteContext(ctx)
	require.NoError(t, err)

	// verifies if the BLS was sucessfully created with gentx
	keyPath, statePath := genhelpers.PathsNodeCfg(home)
	filePV := privval.LoadWrappedFilePV(keyPath, statePath)
	outputFilePath := filepath.Join(filepath.Dir(keyPath), fmt.Sprintf("gen-bls-%s.json", sdk.ValAddress(addr).String()))
	require.NoError(t, err)
	genKey, err := types.LoadGenesisKeyFromFile(outputFilePath)
	require.NoError(t, err)
	require.Equal(t, sdk.ValAddress(addr).String(), genKey.ValidatorAddress)

	require.True(t, filePV.Key.BlsPubKey.Equal(*genKey.BlsKey.Pubkey))
	require.Equal(t, filePV.Key.PubKey.Bytes(), genKey.ValPubkey.Bytes())
	require.True(t, genKey.BlsKey.Pop.IsValid(*genKey.BlsKey.Pubkey, genKey.ValPubkey))
}
