package app_test

import (
	"fmt"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"testing"

	tokenfactorytypes "github.com/strangelove-ventures/tokenfactory/x/tokenfactory/types"

	"cosmossdk.io/log"
	abci "github.com/cometbft/cometbft/abci/types"
	dbm "github.com/cosmos/cosmos-db"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	icatypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"

	babylonApp "github.com/babylonlabs-io/babylon/v4/app"
	testsigner "github.com/babylonlabs-io/babylon/v4/testutil/signer"
	checkpointingtypes "github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
	incentivetypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	minttypes "github.com/babylonlabs-io/babylon/v4/x/mint/types"

	"github.com/stretchr/testify/require"
)

var (
	expectedMaccPerms = map[string][]string{
		authtypes.FeeCollectorName:   nil, // fee collector account
		distrtypes.ModuleName:        nil,
		minttypes.ModuleName:         {authtypes.Minter},
		stktypes.BondedPoolName:      {authtypes.Burner, authtypes.Staking},
		stktypes.NotBondedPoolName:   {authtypes.Burner, authtypes.Staking},
		govtypes.ModuleName:          {authtypes.Burner},
		ibctransfertypes.ModuleName:  {authtypes.Minter, authtypes.Burner},
		incentivetypes.ModuleName:    nil, // this line is needed to create an account for incentive module
		tokenfactorytypes.ModuleName: {authtypes.Minter, authtypes.Burner},
		icatypes.ModuleName:          nil,
		evmtypes.ModuleName:          {authtypes.Minter, authtypes.Burner},
		erc20types.ModuleName:        {authtypes.Minter, authtypes.Burner},
		feemarkettypes.ModuleName:    nil,
		precisebanktypes.ModuleName:  {authtypes.Minter, authtypes.Burner},
	}
)

func TestBabylonBlockedAddrs(t *testing.T) {
	db := dbm.NewMemDB()

	tbs, err := testsigner.SetupTestBlsSigner()
	require.NoError(t, err)
	blsSigner := checkpointingtypes.BlsSigner(tbs)

	logger := log.NewTestLogger(t)
	appOpts, cleanup := babylonApp.TmpAppOptions()
	defer cleanup()

	app := babylonApp.NewBabylonAppWithCustomOptions(t, false, blsSigner, babylonApp.SetupOptions{
		Logger:             logger,
		DB:                 db,
		InvCheckPeriod:     0,
		SkipUpgradeHeights: map[int64]bool{},
		AppOpts:            appOpts,
	})

	for acc := range babylonApp.BlockedAddresses() {
		var addr sdk.AccAddress
		if modAddr, err := sdk.AccAddressFromBech32(acc); err == nil {
			addr = modAddr
		} else {
			addr = app.AccountKeeper.GetModuleAddress(acc)
		}

		require.True(
			t,
			app.BankKeeper.BlockedAddr(addr),
			fmt.Sprintf("ensure that blocked addresses are properly set in bank keeper: %s should be blocked", acc),
		)
	}

	_, err = app.FinalizeBlock(&abci.RequestFinalizeBlock{
		Height: 1,
	})
	require.NoError(t, err)
	_, err = app.Commit()
	require.NoError(t, err)

	logger2 := log.NewTestLogger(t)

	appOpts, cleanup = babylonApp.TmpAppOptions()
	defer cleanup()
	// Making a new app object with the db, so that initchain hasn't been called
	app2 := babylonApp.NewBabylonApp(
		logger2,
		db,
		nil,
		true,
		map[int64]bool{},
		0,
		&blsSigner,
		appOpts,
		babylonApp.EVMChainID,
		babylonApp.EVMAppOptions,
		babylonApp.EmptyWasmOpts,
	)
	_, err = app2.ExportAppStateAndValidators(false, []string{}, []string{})
	require.NoError(t, err, "ExportAppStateAndValidators should not have an error")
}

func TestGetMaccPerms(t *testing.T) {
	dup := babylonApp.GetMaccPerms()
	require.Equal(t, expectedMaccPerms, dup, "duplicated module account permissions differed from actual module account permissions")
}

func TestUpgradeStateOnGenesis(t *testing.T) {
	db := dbm.NewMemDB()

	tbs, err := testsigner.SetupTestBlsSigner()
	require.NoError(t, err)
	blsSigner := checkpointingtypes.BlsSigner(tbs)

	logger := log.NewTestLogger(t)
	appOpts, cleanup := babylonApp.TmpAppOptions()
	defer cleanup()

	app := babylonApp.NewBabylonAppWithCustomOptions(t, false, blsSigner, babylonApp.SetupOptions{
		Logger:             logger,
		DB:                 db,
		InvCheckPeriod:     0,
		SkipUpgradeHeights: map[int64]bool{},
		AppOpts:            appOpts,
	})

	// make sure the upgrade keeper has version map in state
	ctx := app.NewContext(false)
	vm, err := app.UpgradeKeeper.GetModuleVersionMap(ctx)
	require.NoError(t, err)
	for v, i := range app.ModuleManager.Modules {
		if i, ok := i.(module.HasConsensusVersion); ok {
			require.Equal(t, vm[v], i.ConsensusVersion())
		}
	}
}

func TestStakingRouterDisabled(t *testing.T) {
	db := dbm.NewMemDB()
	tbs, _ := testsigner.SetupTestBlsSigner()
	logger := log.NewTestLogger(t)
	appOpts, cleanup := babylonApp.TmpAppOptions()
	defer cleanup()

	app := babylonApp.NewBabylonAppWithCustomOptions(t, false, tbs, babylonApp.SetupOptions{
		Logger:             logger,
		DB:                 db,
		InvCheckPeriod:     0,
		SkipUpgradeHeights: map[int64]bool{},
		AppOpts:            appOpts,
	})

	msgs := []sdk.Msg{
		&stktypes.MsgCreateValidator{},
		&stktypes.MsgBeginRedelegate{},
		&stktypes.MsgCancelUnbondingDelegation{},
		&stktypes.MsgDelegate{},
		&stktypes.MsgEditValidator{},
		&stktypes.MsgUndelegate{},
		&stktypes.MsgUpdateParams{},
	}

	for _, msg := range msgs {
		msgHandler := app.MsgServiceRouter().HandlerByTypeURL(sdk.MsgTypeURL(msg))
		require.Nil(t, msgHandler)
	}
}
