package ratelimit

import (
	"context"

	ratelimit "github.com/cosmos/ibc-apps/modules/rate-limiting/v10"
	"github.com/cosmos/ibc-apps/modules/rate-limiting/v10/keeper"

	"cosmossdk.io/core/appmodule"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
)

var (
	_ module.AppModule          = AppModule{}
	_ appmodule.HasBeginBlocker = AppModule{}
)

// ----------------------------------------------------------------------------
// AppModuleBasic
// ----------------------------------------------------------------------------

// AppModule implements the AppModule interface for the capability module.
type AppModule struct {
	ratelimit.AppModule

	keeper keeper.Keeper
}

func NewAppModule(
	cdc codec.Codec,
	keeper keeper.Keeper,
) AppModule {
	return AppModule{
		AppModule: ratelimit.NewAppModule(cdc, keeper),
		keeper:    keeper,
	}
}

// BeginBlock executes all ABCI BeginBlock logic respective to the capability module.
func (am AppModule) BeginBlock(context context.Context) error {
	ctx := sdk.UnwrapSDKContext(context)
	am.keeper.BeginBlocker(ctx)
	return nil
}
