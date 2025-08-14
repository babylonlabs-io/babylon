package keeper

import (
	"testing"

	"cosmossdk.io/store"
	storetypes "cosmossdk.io/store/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	accountk "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/app"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
)

func AccountKeeper(
	t testing.TB,
	db dbm.DB,
	stateStore store.CommitMultiStore,
) accountk.AccountKeeper {
	storeKey := storetypes.NewKVStoreKey(authtypes.StoreKey)

	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)

	k := accountk.NewAccountKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		authtypes.ProtoBaseAccount,
		app.GetMaccPerms(),
		authcodec.NewBech32Codec(appparams.Bech32PrefixAccAddr),
		appparams.Bech32PrefixAccAddr,
		appparams.AccGov.String(),
	)

	return k
}
