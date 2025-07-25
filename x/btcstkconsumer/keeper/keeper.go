package keeper

import (
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
)

type Keeper struct {
	cdc          codec.BinaryCodec
	storeService store.KVStoreService

	accountKeeper types.AccountKeeper
	bankKeeper    types.BankKeeper
	clientKeeper  types.ClientKeeper
	wasmKeeper    types.WasmKeeper

	// the address capable of executing a MsgUpdateParams message. Typically, this
	// should be the x/gov module account.
	authority string

	Schema collections.Schema
	// Collections for KV store management
	ParamsCollection collections.Item[types.Params]
	ConsumerRegistry collections.Map[string, types.ConsumerRegister]
}

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	accountKeeper types.AccountKeeper,
	bankKeeper types.BankKeeper,
	clientKeeper types.ClientKeeper,
	wasmKeeper types.WasmKeeper,
	authority string,
) Keeper {
	if _, err := sdk.AccAddressFromBech32(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address: %s", authority))
	}

	sb := collections.NewSchemaBuilder(storeService)

	k := Keeper{
		cdc:           cdc,
		storeService:  storeService,
		bankKeeper:    bankKeeper,
		accountKeeper: accountKeeper,
		clientKeeper:  clientKeeper,
		wasmKeeper:    wasmKeeper,
		authority:     authority,

		// Initialize collections
		ParamsCollection: collections.NewItem[types.Params](
			sb,
			types.ParamsKey,
			"params",
			codec.CollValue[types.Params](cdc),
		),
		ConsumerRegistry: collections.NewMap[string, types.ConsumerRegister](
			sb,
			types.ConsumerRegisterKey,
			"consumer_registry",
			collections.StringKey,
			codec.CollValue[types.ConsumerRegister](cdc),
		),
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema

	return k
}

// GetAuthority returns the module's authority.
func (k Keeper) GetAuthority() string {
	return k.authority
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}
