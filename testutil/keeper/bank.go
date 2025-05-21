package keeper

import (
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storetypes "cosmossdk.io/store/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	bankk "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
)

func BankKeeper(
	t testing.TB,
	db dbm.DB,
	stateStore store.CommitMultiStore,
	accountKeeper banktypes.AccountKeeper,
) bankk.Keeper {
	storeKey := storetypes.NewKVStoreKey(banktypes.StoreKey)

	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	k := bankk.NewBaseKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		accountKeeper,
		map[string]bool{},
		appparams.AccGov.String(),
		log.NewNopLogger(),
	)

	return k
}
