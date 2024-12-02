package v1_test

import (
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/app"
	v1 "github.com/babylonlabs-io/babylon/app/upgrades/v1"
	testutilk "github.com/babylonlabs-io/babylon/testutil/keeper"
)

func TestHardCodedBtcStakingParamsAreValid(t *testing.T) {
	for _, upgradeData := range UpgradeV1Data {
		db := dbm.NewMemDB()
		stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())
		k, ctx := testutilk.BTCStakingKeeperWithStore(t, db, stateStore, nil, nil, nil)

		params, err := v1.LoadBtcStakingParamsFromData(upgradeData.BtcStakingParamsStr)
		require.NoError(t, err)

		for _, p := range params {
			// using set Params here makes sure the parameters in the upgrade string are consistent
			err = k.SetParams(ctx, p)
			require.NoError(t, err)
		}
	}
}

func TestHardCodedFinalityParamsAreValid(t *testing.T) {
	bbnApp := app.NewTmpBabylonApp()
	for _, upgradeData := range UpgradeV1Data {
		params, err := v1.LoadFinalityParamsFromData(bbnApp.AppCodec(), upgradeData.FinalityParamStr)
		require.NoError(t, err)
		require.NoError(t, params.Validate())
	}
}

func TestHardCodedWasmParamsAreValid(t *testing.T) {
	bbnApp := app.NewTmpBabylonApp()

	for _, upgradeData := range UpgradeV1Data {
		params, err := v1.LoadCosmWasmParamsFromData(bbnApp.AppCodec(), upgradeData.CosmWasmParamStr)
		require.NoError(t, err)
		require.NotNil(t, params)
		require.Equal(t, params.InstantiateDefaultPermission, wasmtypes.AccessTypeEverybody)
	}
}
