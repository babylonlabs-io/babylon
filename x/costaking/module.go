package costaking

import (
	"context"
	"encoding/json"

	"cosmossdk.io/core/appmodule"
	"google.golang.org/grpc"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"github.com/babylonlabs-io/babylon/v4/x/costaking/client/cli"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
)

var (
	_ appmodule.HasBeginBlocker = AppModule{}
	_ appmodule.HasEndBlocker   = AppModule{}
	_ appmodule.AppModule       = AppModule{}
	_ appmodule.HasServices     = AppModule{}
	_ module.AppModuleBasic     = AppModuleBasic{}
	_ module.HasGenesisBasics   = AppModuleBasic{}
	_ module.HasGenesis         = AppModuleBasic{}
)

// ----------------------------------------------------------------------------
// AppModuleBasic
// ----------------------------------------------------------------------------

// AppModuleBasic implements the AppModuleBasic interface that defines the independent methods a Cosmos SDK module needs to implement.
type AppModuleBasic struct {
	cdc codec.BinaryCodec

	k keeper.Keeper
}

func NewAppModuleBasic(cdc codec.BinaryCodec, k keeper.Keeper) AppModuleBasic {
	return AppModuleBasic{cdc: cdc, k: k}
}

// Name returns the name of the module as a string
func (AppModuleBasic) Name() string {
	return types.ModuleName
}

// RegisterLegacyAminoCodec registers the amino codec for the module, which is used to marshal and unmarshal structs to/from []byte in order to persist them in the module's KVStore
func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	types.RegisterCodec(cdc)
}

// RegisterInterfaces registers a module's interface types and their concrete implementations as proto.Message
func (a AppModuleBasic) RegisterInterfaces(reg cdctypes.InterfaceRegistry) {
	types.RegisterInterfaces(reg)
}

// ExportGenesis implements module.HasGenesis.
func (a AppModuleBasic) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	genState, err := a.k.ExportGenesis(ctx)
	if err != nil {
		panic(err)
	}
	return cdc.MustMarshalJSON(genState)
}

// InitGenesis implements module.HasGenesis.
func (a AppModuleBasic) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, gs json.RawMessage) {
	var genState types.GenesisState

	cdc.MustUnmarshalJSON(gs, &genState)
	err := a.k.InitGenesis(ctx, genState)
	if err != nil {
		panic(err)
	}
}

// DefaultGenesis returns a default GenesisState for the module, marshalled to json.RawMessage. The default GenesisState need to be defined by the module developer and is primarily used for testing
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(types.DefaultGenesis())
}

// ValidateGenesis used to validate the GenesisState, given in its json.RawMessage form
func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, config client.TxEncodingConfig, bz json.RawMessage) error {
	var genState types.GenesisState
	if err := cdc.UnmarshalJSON(bz, &genState); err != nil {
		return err
	}
	return genState.Validate()
}

// RegisterGRPCGatewayRoutes registers the gRPC Gateway routes for the module
func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {
	err := types.RegisterQueryHandlerClient(context.Background(), mux, types.NewQueryClient(clientCtx))
	if err != nil {
		panic(err)
	}
}

// GetTxCmd returns the root Tx command for the module. The subcommands of this root command are used by end-users to generate new transactions containing messages defined in the module
func (a AppModuleBasic) GetTxCmd() *cobra.Command {
	return nil
}

// GetQueryCmd returns the root query command for the module. The subcommands of this root command are used by end-users to generate new queries to the subset of the state defined by the module
func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	return cli.GetQueryCmd()
}

// ----------------------------------------------------------------------------
// AppModule
// ----------------------------------------------------------------------------

// AppModule implements the AppModule interface that defines the inter-dependent methods that modules need to implement
type AppModule struct {
	AppModuleBasic
}

func NewAppModule(
	cdc codec.Codec,
	k keeper.Keeper,
) AppModule {
	return AppModule{
		AppModuleBasic: NewAppModuleBasic(cdc, k),
	}
}

// RegisterServices implements appmodule.HasServices.
func (am AppModule) RegisterServices(cfg grpc.ServiceRegistrar) error {
	types.RegisterMsgServer(cfg, keeper.NewMsgServerImpl(am.k))
	types.RegisterQueryServer(cfg, am.k)
	return nil
}

// BeginBlock implements appmodule.HasBeginBlocker.
func (am AppModule) BeginBlock(ctx context.Context) error {
	return BeginBlocker(ctx, am.k)
}

// EndBlock implements appmodule.HasEndBlocker.
func (am AppModule) EndBlock(ctx context.Context) error {
	return EndBlocker(ctx, am.k)
}

// ConsensusVersion is a sequence number for state-breaking change of the module. It should be incremented on each consensus-breaking change introduced by the module. To avoid wrong/empty versions, the initial version should be set to 1
func (AppModule) ConsensusVersion() uint64 { return 1 }

// IsOnePerModuleType implements the depinject.OnePerModuleType interface.
func (AppModule) IsOnePerModuleType() { // marker
}

// IsAppModule implements the appmodule.AppModule interface.
func (AppModule) IsAppModule() { // marker
}
