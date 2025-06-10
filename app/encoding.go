package app

import (
	"os"

	"cosmossdk.io/log"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/client/flags"
	simsutils "github.com/cosmos/cosmos-sdk/testutil/sims"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/testutil/signer"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	checkpointingtypes "github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
)

// TmpAppOptions returns an app option with tmp dir and btc network. It is up to
// the caller to clean up the temp dir.
func TmpAppOptions() (simsutils.AppOptionsMap, func()) {
	dir, err := os.MkdirTemp("", "babylon-tmp-app")
	if err != nil {
		panic(err)
	}
	appOpts := simsutils.AppOptionsMap{
		flags.FlagHome:       dir,
		"btc-config.network": string(bbn.BtcSimnet),
	}

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return appOpts, cleanup
}

// NewTmpBabylonApp returns a new BabylonApp
func NewTmpBabylonApp() *BabylonApp {
	tbs, _ := signer.SetupTestBlsSigner()
	blsSigner := checkpointingtypes.BlsSigner(tbs)

	appOpts, cleanup := TmpAppOptions()
	defer cleanup()

	return NewBabylonApp(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		map[int64]bool{},
		0,
		&blsSigner,
		appOpts,
		EVMChainID,
		NoOpEVMOptions,
		[]wasmkeeper.Option{})
}

// GetEncodingConfig returns a *registered* encoding config
// Note that the only way to register configuration is through the app creation
func GetEncodingConfig() *appparams.EncodingConfig {
	tmpApp := NewTmpBabylonApp()
	return &appparams.EncodingConfig{
		InterfaceRegistry: tmpApp.InterfaceRegistry(),
		Codec:             tmpApp.AppCodec(),
		TxConfig:          tmpApp.TxConfig(),
		Amino:             tmpApp.LegacyAmino(),
	}
}
