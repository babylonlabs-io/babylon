package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"
	reflectionv1 "cosmossdk.io/api/cosmos/reflection/v1"
	"cosmossdk.io/client/v2/autocli"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/log"
	"cosmossdk.io/x/circuit"
	circuittypes "cosmossdk.io/x/circuit/types"
	"cosmossdk.io/x/evidence"
	evidencetypes "cosmossdk.io/x/evidence/types"
	"cosmossdk.io/x/feegrant"
	feegrantmodule "cosmossdk.io/x/feegrant/module"
	"cosmossdk.io/x/upgrade"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	wasmapp "github.com/CosmWasm/wasmd/app"
	"github.com/CosmWasm/wasmd/x/wasm"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/babylonchain/babylon/app/upgrades"
	bbn "github.com/babylonchain/babylon/types"
	abci "github.com/cometbft/cometbft/abci/types"
	cmtos "github.com/cometbft/cometbft/libs/os"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	nodeservice "github.com/cosmos/cosmos-sdk/client/grpc/node"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	runtimeservices "github.com/cosmos/cosmos-sdk/runtime/services"
	"github.com/cosmos/cosmos-sdk/server/api"
	"github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	authsims "github.com/cosmos/cosmos-sdk/x/auth/simulation"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	authzmodule "github.com/cosmos/cosmos-sdk/x/authz/module"
	"github.com/cosmos/cosmos-sdk/x/bank"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/consensus"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/mint"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/cosmos/cosmos-sdk/x/params"
	paramsclient "github.com/cosmos/cosmos-sdk/x/params/client"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/gogoproto/proto"
	"github.com/cosmos/ibc-go/modules/capability"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	ibcwasm "github.com/cosmos/ibc-go/modules/light-clients/08-wasm"
	ibcwasmkeeper "github.com/cosmos/ibc-go/modules/light-clients/08-wasm/keeper"
	ibcwasmtypes "github.com/cosmos/ibc-go/modules/light-clients/08-wasm/types"
	ibcfee "github.com/cosmos/ibc-go/v8/modules/apps/29-fee"
	ibcfeetypes "github.com/cosmos/ibc-go/v8/modules/apps/29-fee/types"
	"github.com/cosmos/ibc-go/v8/modules/apps/transfer"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	ibc "github.com/cosmos/ibc-go/v8/modules/core" // ibc module puts types under `ibchost` rather than `ibctypes`
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"
	ibctm "github.com/cosmos/ibc-go/v8/modules/light-clients/07-tendermint"
	"github.com/spf13/cast"

	appkeepers "github.com/babylonchain/babylon/app/keepers"
	appparams "github.com/babylonchain/babylon/app/params"
	"github.com/babylonchain/babylon/client/docs"
	"github.com/babylonchain/babylon/x/btccheckpoint"
	btccheckpointtypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	"github.com/babylonchain/babylon/x/btclightclient"
	btclightclienttypes "github.com/babylonchain/babylon/x/btclightclient/types"
	"github.com/babylonchain/babylon/x/btcstaking"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/babylonchain/babylon/x/checkpointing"
	checkpointingtypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/babylonchain/babylon/x/epoching"
	epochingkeeper "github.com/babylonchain/babylon/x/epoching/keeper"
	epochingtypes "github.com/babylonchain/babylon/x/epoching/types"
	"github.com/babylonchain/babylon/x/finality"
	finalitytypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/babylonchain/babylon/x/incentive"
	incentivetypes "github.com/babylonchain/babylon/x/incentive/types"
	"github.com/babylonchain/babylon/x/monitor"
	monitortypes "github.com/babylonchain/babylon/x/monitor/types"
	"github.com/babylonchain/babylon/x/zoneconcierge"
	zckeeper "github.com/babylonchain/babylon/x/zoneconcierge/keeper"
	zctypes "github.com/babylonchain/babylon/x/zoneconcierge/types"
)

const (
	appName = "BabylonApp"

	// Custom prefix for application environmental variables.
	// From cosmos version 0.46 is is possible to have custom prefix for application
	// environmental variables - https://github.com/cosmos/cosmos-sdk/pull/10950
	BabylonAppEnvPrefix = ""

	// According to https://github.com/CosmWasm/wasmd#genesis-configuration chains
	// using smart contracts should configure proper gas limits per block.
	// https://medium.com/cosmwasm/cosmwasm-for-ctos-iv-native-integrations-713140bf75fc
	// suggests 50M as reasonable limits. Me may want to adjust it later.
	DefaultGasLimit int64 = 50000000

	DefaultVoteExtensionsEnableHeight = 1
)

var (
	// EmptyWasmOpts defines a type alias for a list of wasm options.
	EmptyWasmOpts []wasmkeeper.Option

	// DefaultNodeHome default home directories for the application daemon
	DefaultNodeHome string
	// fee collector account, module accounts and their permissions
	maccPerms = map[string][]string{
		authtypes.FeeCollectorName:     nil, // fee collector account
		distrtypes.ModuleName:          nil,
		minttypes.ModuleName:           {authtypes.Minter},
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
		govtypes.ModuleName:            {authtypes.Burner},
		ibctransfertypes.ModuleName:    {authtypes.Minter, authtypes.Burner},
		ibcfeetypes.ModuleName:         nil,
		incentivetypes.ModuleName:      nil, // this line is needed to create an account for incentive module
	}

	// software upgrades and forks
	Upgrades = []upgrades.Upgrade{}
	Forks    = []upgrades.Fork{}
)

func init() {
	// Note: If this changes, the home directory under x/checkpointing/client/cli/tx.go needs to change as well
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	DefaultNodeHome = filepath.Join(userHomeDir, ".babylond")
}

var (
	_ runtime.AppI            = (*BabylonApp)(nil)
	_ servertypes.Application = (*BabylonApp)(nil)
)

// BabylonApp extends an ABCI application, but with most of its parameters exported.
// They are exported for convenience in creating helper functions, as object
// capabilities aren't needed for testing.
type BabylonApp struct {
	*baseapp.BaseApp
	*appkeepers.AppKeepers

	legacyAmino *codec.LegacyAmino
	appCodec    codec.Codec
	txConfig    client.TxConfig

	interfaceRegistry types.InterfaceRegistry
	invCheckPeriod    uint

	// the module manager
	ModuleManager      *module.Manager
	BasicModuleManager module.BasicManager

	// simulation manager
	sm *module.SimulationManager

	// module configurator
	configurator module.Configurator
}

// NewBabylonApp returns a reference to an initialized BabylonApp.
func NewBabylonApp(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	skipUpgradeHeights map[int64]bool,
	invCheckPeriod uint,
	privSigner *appkeepers.PrivSigner,
	appOpts servertypes.AppOptions,
	wasmOpts []wasmkeeper.Option,
	baseAppOptions ...func(*baseapp.BaseApp),
) *BabylonApp {
	// we could also take it from global object which should be initialised in rootCmd
	// but this way it makes babylon app more testable
	btcConfig := bbn.ParseBtcOptionsFromConfig(appOpts)
	homePath := cast.ToString(appOpts.Get(flags.FlagHome))
	if homePath == "" {
		homePath = DefaultNodeHome
	}

	encCfg := appparams.DefaultEncodingConfig()
	interfaceRegistry := encCfg.InterfaceRegistry
	appCodec := encCfg.Codec
	legacyAmino := encCfg.Amino
	txConfig := encCfg.TxConfig
	std.RegisterLegacyAminoCodec(legacyAmino)
	std.RegisterInterfaces(interfaceRegistry)

	bApp := baseapp.NewBaseApp(appName, logger, db, txConfig.TxDecoder(), baseAppOptions...)
	bApp.SetCommitMultiStoreTracer(traceStore)
	bApp.SetVersion(version.Version)
	bApp.SetInterfaceRegistry(interfaceRegistry)
	bApp.SetTxEncoder(txConfig.TxEncoder())

	wasmConfig, err := wasm.ReadWasmConfig(appOpts)
	if err != nil {
		panic(fmt.Sprintf("error while reading wasm config: %s", err))
	}

	app := &BabylonApp{
		AppKeepers:        &appkeepers.AppKeepers{},
		BaseApp:           bApp,
		legacyAmino:       legacyAmino,
		appCodec:          appCodec,
		txConfig:          txConfig,
		interfaceRegistry: interfaceRegistry,
		invCheckPeriod:    invCheckPeriod,
	}

	app.AppKeepers.InitKeepers(
		logger,
		appCodec,
		&btcConfig,
		encCfg,
		bApp,
		maccPerms,
		homePath,
		invCheckPeriod,
		skipUpgradeHeights,
		privSigner,
		appOpts,
		wasmConfig,
		wasmOpts,
		BlockedAddresses(),
	)

	/****  Module Options ****/

	// NOTE: we may consider parsing `appOpts` inside module constructors. For the moment
	// we prefer to be more strict in what arguments the modules expect.
	var skipGenesisInvariants = cast.ToBool(appOpts.Get(crisis.FlagSkipGenesisInvariants))

	// NOTE: Any module instantiated in the module manager that is later modified
	// must be passed by reference here.
	app.ModuleManager = module.NewManager(
		genutil.NewAppModule(
			app.AccountKeeper,
			app.StakingKeeper,
			app,
			txConfig,
		),
		auth.NewAppModule(appCodec, app.AccountKeeper, authsims.RandomGenesisAccounts, app.GetSubspace(authtypes.ModuleName)),
		vesting.NewAppModule(app.AccountKeeper, app.BankKeeper),
		bank.NewAppModule(appCodec, app.BankKeeper, app.AccountKeeper, app.GetSubspace(banktypes.ModuleName)),
		crisis.NewAppModule(app.CrisisKeeper, skipGenesisInvariants, app.GetSubspace(crisistypes.ModuleName)),
		feegrantmodule.NewAppModule(appCodec, app.AccountKeeper, app.BankKeeper, app.FeeGrantKeeper, app.interfaceRegistry),
		gov.NewAppModule(appCodec, &app.GovKeeper, app.AccountKeeper, app.BankKeeper, app.GetSubspace(govtypes.ModuleName)),
		mint.NewAppModule(appCodec, app.MintKeeper, app.AccountKeeper, nil, app.GetSubspace(minttypes.ModuleName)),
		slashing.NewAppModule(appCodec, app.SlashingKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper, app.GetSubspace(slashingtypes.ModuleName), app.interfaceRegistry),
		distr.NewAppModule(appCodec, app.DistrKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper, app.GetSubspace(distrtypes.ModuleName)),
		staking.NewAppModule(appCodec, app.StakingKeeper, app.AccountKeeper, app.BankKeeper, app.GetSubspace(stakingtypes.ModuleName)),
		upgrade.NewAppModule(app.UpgradeKeeper, app.AccountKeeper.AddressCodec()),
		evidence.NewAppModule(app.EvidenceKeeper),
		params.NewAppModule(app.ParamsKeeper),
		consensus.NewAppModule(appCodec, app.ConsensusParamsKeeper),
		circuit.NewAppModule(appCodec, app.CircuitKeeper),
		authzmodule.NewAppModule(appCodec, app.AuthzKeeper, app.AccountKeeper, app.BankKeeper, app.interfaceRegistry),
		// non sdk modules
		capability.NewAppModule(appCodec, *app.CapabilityKeeper, false),
		wasm.NewAppModule(appCodec, &app.WasmKeeper, app.StakingKeeper, app.AccountKeeper, app.BankKeeper, app.MsgServiceRouter(), app.GetSubspace(wasmtypes.ModuleName)),
		ibc.NewAppModule(app.IBCKeeper),
		transfer.NewAppModule(app.TransferKeeper),
		ibcfee.NewAppModule(app.IBCFeeKeeper),
		ibctm.AppModule{},
		ibcwasm.NewAppModule(app.IBCWasmKeeper),
		// Babylon modules - btc timestamping
		epoching.NewAppModule(appCodec, app.EpochingKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper),
		btclightclient.NewAppModule(appCodec, app.BTCLightClientKeeper),
		btccheckpoint.NewAppModule(appCodec, app.BtcCheckpointKeeper),
		checkpointing.NewAppModule(appCodec, app.CheckpointingKeeper),
		monitor.NewAppModule(appCodec, app.MonitorKeeper),
		zoneconcierge.NewAppModule(appCodec, app.ZoneConciergeKeeper, app.AccountKeeper, app.BankKeeper),
		// Babylon modules - btc staking
		btcstaking.NewAppModule(appCodec, app.BTCStakingKeeper),
		finality.NewAppModule(appCodec, app.FinalityKeeper),
		// Babylon modules - tokenomics
		incentive.NewAppModule(appCodec, app.IncentiveKeeper, app.AccountKeeper, app.BankKeeper),
	)

	// BasicModuleManager defines the module BasicManager which is in charge of setting up basic,
	// non-dependant module elements, such as codec registration and genesis verification.
	// By default, it is composed of all the modules from the module manager.
	// Additionally, app module basics can be overwritten by passing them as an argument.
	app.BasicModuleManager = module.NewBasicManagerFromManager(
		app.ModuleManager,
		map[string]module.AppModuleBasic{
			genutiltypes.ModuleName: genutil.NewAppModuleBasic(genutiltypes.DefaultMessageValidator),
			govtypes.ModuleName: gov.NewAppModuleBasic(
				[]govclient.ProposalHandler{
					paramsclient.ProposalHandler,
				},
			),
		})
	app.BasicModuleManager.RegisterLegacyAminoCodec(legacyAmino)
	app.BasicModuleManager.RegisterInterfaces(interfaceRegistry)

	// NOTE: upgrade module is required to be prioritized
	app.ModuleManager.SetOrderPreBlockers(
		upgradetypes.ModuleName,
	)

	// During begin block slashing happens after distr.BeginBlocker so that
	// there is nothing left over in the validator fee pool, so as to keep the
	// CanWithdrawInvariant invariant.
	// NOTE: staking module is required if HistoricalEntries param > 0
	// NOTE: capability module's beginblocker must come before any modules using capabilities (e.g. IBC)
	app.ModuleManager.SetOrderBeginBlockers(
		upgradetypes.ModuleName, capabilitytypes.ModuleName,
		// NOTE: incentive module's BeginBlock has to be after mint but before distribution
		// so that it can intercept a part of new inflation to reward BTC staking/timestamping stakeholders
		minttypes.ModuleName, incentivetypes.ModuleName, distrtypes.ModuleName,
		slashingtypes.ModuleName,
		evidencetypes.ModuleName, stakingtypes.ModuleName,
		authtypes.ModuleName, banktypes.ModuleName, govtypes.ModuleName, crisistypes.ModuleName, genutiltypes.ModuleName,
		authz.ModuleName, feegrant.ModuleName,
		paramstypes.ModuleName, vestingtypes.ModuleName, consensusparamtypes.ModuleName, circuittypes.ModuleName,
		// Babylon modules
		epochingtypes.ModuleName,
		btclightclienttypes.ModuleName,
		btccheckpointtypes.ModuleName,
		checkpointingtypes.ModuleName,
		monitortypes.ModuleName,
		// IBC-related modules
		ibcexported.ModuleName,
		ibcwasmtypes.ModuleName,
		ibctransfertypes.ModuleName,
		zctypes.ModuleName,
		ibcfeetypes.ModuleName,
		wasmtypes.ModuleName,
		// BTC staking related modules
		btcstakingtypes.ModuleName,
		finalitytypes.ModuleName,
	)
	// TODO: there will be an architecture design on whether to modify slashing/evidence, specifically
	// - how many validators can we slash in a single epoch and
	// - whether and when to jail slashed validators
	// app.mm.OrderBeginBlockers = append(app.mm.OrderBeginBlockers[:4], app.mm.OrderBeginBlockers[4+1:]...) // remove slashingtypes.ModuleName
	// app.mm.OrderBeginBlockers = append(app.mm.OrderBeginBlockers[:4], app.mm.OrderBeginBlockers[4+1:]...) // remove evidencetypes.ModuleName

	app.ModuleManager.SetOrderEndBlockers(crisistypes.ModuleName, govtypes.ModuleName, stakingtypes.ModuleName,
		capabilitytypes.ModuleName, authtypes.ModuleName, banktypes.ModuleName, distrtypes.ModuleName,
		slashingtypes.ModuleName, minttypes.ModuleName,
		genutiltypes.ModuleName, evidencetypes.ModuleName, authz.ModuleName,
		feegrant.ModuleName,
		paramstypes.ModuleName, upgradetypes.ModuleName, vestingtypes.ModuleName, consensusparamtypes.ModuleName,
		// Babylon modules
		epochingtypes.ModuleName,
		btclightclienttypes.ModuleName,
		btccheckpointtypes.ModuleName,
		checkpointingtypes.ModuleName,
		monitortypes.ModuleName,
		// IBC-related modules
		ibcexported.ModuleName,
		ibcwasmtypes.ModuleName,
		ibctransfertypes.ModuleName,
		zctypes.ModuleName,
		ibcfeetypes.ModuleName,
		wasmtypes.ModuleName,
		// BTC staking related modules
		btcstakingtypes.ModuleName,
		finalitytypes.ModuleName,
		// tokenomics related modules
		incentivetypes.ModuleName, // EndBlock of incentive module does not matter
	)
	// Babylon does not want EndBlock processing in staking
	app.ModuleManager.OrderEndBlockers = append(app.ModuleManager.OrderEndBlockers[:2], app.ModuleManager.OrderEndBlockers[2+1:]...) // remove stakingtypes.ModuleName

	// NOTE: The genutils module must occur after staking so that pools are
	// properly initialized with tokens from genesis accounts.
	// NOTE: Capability module must occur first so that it can initialize any capabilities
	// so that other modules that want to create or claim capabilities afterwards in InitChain
	// can do so safely.
	genesisModuleOrder := []string{
		capabilitytypes.ModuleName, authtypes.ModuleName, banktypes.ModuleName, distrtypes.ModuleName, stakingtypes.ModuleName,
		slashingtypes.ModuleName, govtypes.ModuleName, minttypes.ModuleName, crisistypes.ModuleName,
		genutiltypes.ModuleName, evidencetypes.ModuleName, authz.ModuleName,
		feegrant.ModuleName,
		paramstypes.ModuleName, upgradetypes.ModuleName, vestingtypes.ModuleName, consensusparamtypes.ModuleName, circuittypes.ModuleName,
		// Babylon modules
		btclightclienttypes.ModuleName,
		epochingtypes.ModuleName,
		btccheckpointtypes.ModuleName,
		checkpointingtypes.ModuleName,
		monitortypes.ModuleName,
		// IBC-related modules
		ibcexported.ModuleName,
		ibcwasmtypes.ModuleName,
		ibctransfertypes.ModuleName,
		zctypes.ModuleName,
		ibcfeetypes.ModuleName,
		wasmtypes.ModuleName,
		// BTC staking related modules
		btcstakingtypes.ModuleName,
		finalitytypes.ModuleName,
		// tokenomics-related modules
		incentivetypes.ModuleName,
	}
	app.ModuleManager.SetOrderInitGenesis(genesisModuleOrder...)
	app.ModuleManager.SetOrderExportGenesis(genesisModuleOrder...)

	// Uncomment if you want to set a custom migration order here.
	// app.mm.SetOrderMigrations(custom order)

	app.ModuleManager.RegisterInvariants(app.CrisisKeeper)
	app.configurator = module.NewConfigurator(app.appCodec, app.MsgServiceRouter(), app.GRPCQueryRouter())
	if err := app.ModuleManager.RegisterServices(app.configurator); err != nil {
		panic(err)
	}

	autocliv1.RegisterQueryServer(app.GRPCQueryRouter(), runtimeservices.NewAutoCLIQueryService(app.ModuleManager.Modules))

	reflectionSvc, err := runtimeservices.NewReflectionService()
	if err != nil {
		panic(err)
	}
	reflectionv1.RegisterReflectionServiceServer(app.GRPCQueryRouter(), reflectionSvc)

	// add test gRPC service for testing gRPC queries in isolation
	testdata.RegisterQueryServer(app.GRPCQueryRouter(), testdata.QueryImpl{})

	// create the simulation manager and define the order of the modules for deterministic simulations
	//
	// NOTE: this is not required apps that don't use the simulator for fuzz testing
	// transactions
	overrideModules := map[string]module.AppModuleSimulation{
		authtypes.ModuleName: auth.NewAppModule(app.appCodec, app.AccountKeeper, authsims.RandomGenesisAccounts, app.GetSubspace(authtypes.ModuleName)),
	}
	app.sm = module.NewSimulationManagerFromAppModules(app.ModuleManager.Modules, overrideModules)

	app.sm.RegisterStoreDecoders()

	// initialize stores
	app.MountKVStores(app.GetKVStoreKeys())
	app.MountTransientStores(app.GetTransientStoreKeys())
	app.MountMemoryStores(app.GetMemoryStoreKeys())

	// initialize AnteHandler, which includes
	// - authAnteHandler
	// - custom wasm ante handler NewLimitSimulationGasDecorator and NewCountTXDecorator
	// - Extra decorators introduced in Babylon, such as DropValidatorMsgDecorator that delays validator-related messages
	//
	// We are using constructor from wasmapp as it introduces custom wasm ante handle decorators
	// early in chain of ante handlers.
	authAnteHandler, err := wasmapp.NewAnteHandler(
		wasmapp.HandlerOptions{
			HandlerOptions: ante.HandlerOptions{
				AccountKeeper:   app.AccountKeeper,
				BankKeeper:      app.BankKeeper,
				SignModeHandler: txConfig.SignModeHandler(),
				FeegrantKeeper:  app.FeeGrantKeeper,
				SigGasConsumer:  ante.DefaultSigVerificationGasConsumer,
			},
			IBCKeeper:             app.IBCKeeper,
			WasmConfig:            &wasmConfig,
			TXCounterStoreService: runtime.NewKVStoreService(app.AppKeepers.GetKey(wasmtypes.StoreKey)),
			WasmKeeper:            &app.WasmKeeper,
			CircuitKeeper:         &app.CircuitKeeper,
		},
	)

	if err != nil {
		panic(err)
	}

	anteHandler := sdk.ChainAnteDecorators(
		NewWrappedAnteHandler(authAnteHandler),
		epochingkeeper.NewDropValidatorMsgDecorator(app.EpochingKeeper),
		NewBtcValidationDecorator(btcConfig, &app.BtcCheckpointKeeper),
	)

	// set proposal extension
	proposalHandler := checkpointing.NewProposalHandler(
		logger, &app.CheckpointingKeeper, bApp.Mempool(), bApp)
	proposalHandler.SetHandlers(bApp)

	// set vote extension
	voteExtHandler := checkpointing.NewVoteExtensionHandler(logger, &app.CheckpointingKeeper)
	voteExtHandler.SetHandlers(bApp)

	app.SetInitChainer(app.InitChainer)
	app.SetPreBlocker(func(ctx sdk.Context, req *abci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {
		// execute the existing PreBlocker
		res, err := app.PreBlocker(ctx, req)
		if err != nil {
			return res, err
		}
		// execute checkpointing module's PreBlocker
		// NOTE: this does not change the consensus parameter in `res`
		ckptPreBlocker := proposalHandler.PreBlocker()
		if _, err := ckptPreBlocker(ctx, req); err != nil {
			return res, err
		}
		return res, nil
	})
	app.SetBeginBlocker(app.BeginBlocker)
	app.SetEndBlocker(app.EndBlocker)
	app.SetAnteHandler(anteHandler)

	// set postHandler
	postHandler := sdk.ChainPostDecorators(
		zckeeper.NewIBCHeaderDecorator(app.ZoneConciergeKeeper),
	)
	app.SetPostHandler(postHandler)

	// must be before Loading version
	// requires the snapshot store to be created and registered as a BaseAppOption
	// see cmd/wasmd/root.go: 206 - 214 approx
	if manager := app.SnapshotManager(); manager != nil {
		err := manager.RegisterExtensions(
			wasmkeeper.NewWasmSnapshotter(app.CommitMultiStore(), &app.WasmKeeper),
		)
		if err != nil {
			panic(fmt.Errorf("failed to register snapshot extension: %s", err))
		}

		err = manager.RegisterExtensions(
			ibcwasmkeeper.NewWasmSnapshotter(app.CommitMultiStore(), &app.IBCWasmKeeper),
		)
		if err != nil {
			panic(fmt.Errorf("failed to register snapshot extension: %s", err))
		}
	}

	// At startup, after all modules have been registered, check that all proto
	// annotations are correct.
	protoFiles, err := proto.MergedRegistry()
	if err != nil {
		panic(err)
	}
	err = msgservice.ValidateProtoAnnotations(protoFiles)
	if err != nil {
		// Once we switch to using protoreflect-based antehandlers, we might
		// want to panic here instead of logging a warning.
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
	}

	// set upgrade handler and store loader for supporting software upgrade
	app.setupUpgradeHandlers()
	app.setupUpgradeStoreLoaders()

	if loadLatest {
		if err := app.LoadLatestVersion(); err != nil {
			cmtos.Exit(err.Error())
		}

		ctx := app.BaseApp.NewUncachedContext(true, cmtproto.Header{})

		// Initialize pinned codes in wasmvm as they are not persisted there
		if err := app.WasmKeeper.InitializePinnedCodes(ctx); err != nil {
			cmtos.Exit(fmt.Sprintf("failed initialize pinned codes %s", err))
		}
	}

	return app
}

// GetBaseApp returns the BaseApp of BabylonApp
// required by ibctesting
func (app *BabylonApp) GetBaseApp() *baseapp.BaseApp {
	return app.BaseApp
}

// Name returns the name of the App
func (app *BabylonApp) Name() string { return app.BaseApp.Name() }

// PreBlocker application updates every pre block
func (app *BabylonApp) PreBlocker(ctx sdk.Context, _ *abci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {
	return app.ModuleManager.PreBlock(ctx)
}

// BeginBlockForks is intended to be ran in a chain upgrade.
func (app *BabylonApp) BeginBlockForks(ctx sdk.Context) {
	for _, fork := range Forks {
		if ctx.BlockHeight() == fork.UpgradeHeight {
			fork.BeginForkLogic(ctx, app.AppKeepers)
			return
		}
	}
}

// BeginBlocker application updates every begin block
func (app *BabylonApp) BeginBlocker(ctx sdk.Context) (sdk.BeginBlock, error) {
	app.BeginBlockForks(ctx)
	return app.ModuleManager.BeginBlock(ctx)
}

// EndBlocker application updates every end block
func (app *BabylonApp) EndBlocker(ctx sdk.Context) (sdk.EndBlock, error) {
	return app.ModuleManager.EndBlock(ctx)
}

// InitChainer application update at chain initialization
func (app *BabylonApp) InitChainer(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	var genesisState GenesisState
	if err := json.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
		panic(err)
	}
	err := app.UpgradeKeeper.SetModuleVersionMap(ctx, app.ModuleManager.GetVersionMap())
	if err != nil {
		panic(err)
	}

	if _, ok := app.ModuleManager.Modules[epochingtypes.ModuleName].(module.HasGenesis); !ok {
		panic("FAULTY")
	}

	res, err := app.ModuleManager.InitGenesis(ctx, app.appCodec, genesisState)
	if err != nil {
		panic(err)
	}

	return res, nil
}

// LoadHeight loads a particular height
func (app *BabylonApp) LoadHeight(height int64) error {
	return app.LoadVersion(height)
}

// ModuleAccountAddrs returns all the app's module account addresses.
func (app *BabylonApp) ModuleAccountAddrs() map[string]bool {
	modAccAddrs := make(map[string]bool)
	for acc := range maccPerms {
		modAccAddrs[authtypes.NewModuleAddress(acc).String()] = true
	}

	return modAccAddrs
}

// LegacyAmino returns BabylonApp's amino codec.
//
// NOTE: This is solely to be used for testing purposes as it may be desirable
// for modules to register their own custom testing types.
func (app *BabylonApp) LegacyAmino() *codec.LegacyAmino {
	return app.legacyAmino
}

// AppCodec returns BabylonApp's app codec.
//
// NOTE: This is solely to be used for testing purposes as it may be desirable
// for modules to register their own custom testing types.
func (app *BabylonApp) AppCodec() codec.Codec {
	return app.appCodec
}

// InterfaceRegistry returns babylonApp's InterfaceRegistry
func (app *BabylonApp) InterfaceRegistry() types.InterfaceRegistry {
	return app.interfaceRegistry
}

func (app *BabylonApp) EncodingConfig() *appparams.EncodingConfig {
	return &appparams.EncodingConfig{
		InterfaceRegistry: app.InterfaceRegistry(),
		Codec:             app.AppCodec(),
		TxConfig:          app.TxConfig(),
		Amino:             app.LegacyAmino(),
	}
}

// SimulationManager implements the SimulationApp interface
func (app *BabylonApp) SimulationManager() *module.SimulationManager {
	return app.sm
}

// RegisterAPIRoutes registers all application module routes with the provided
// API server.
func (app *BabylonApp) RegisterAPIRoutes(apiSvr *api.Server, apiConfig config.APIConfig) {
	clientCtx := apiSvr.ClientCtx
	// Register new tx routes from grpc-gateway.
	authtx.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// Register new tendermint queries routes from grpc-gateway.
	cmtservice.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// Register node gRPC service for grpc-gateway.
	nodeservice.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// Register grpc-gateway routes for all modules.
	app.BasicModuleManager.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// register swagger API from root so that other applications can override easily
	if apiConfig.Swagger {
		docs.RegisterOpenAPIService(apiSvr.Router)
	}
}

// RegisterTxService implements the Application.RegisterTxService method.
func (app *BabylonApp) RegisterTxService(clientCtx client.Context) {
	authtx.RegisterTxService(app.BaseApp.GRPCQueryRouter(), clientCtx, app.BaseApp.Simulate, app.interfaceRegistry)
}

// RegisterTendermintService implements the Application.RegisterTendermintService method.
func (app *BabylonApp) RegisterTendermintService(clientCtx client.Context) {
	cmtservice.RegisterTendermintService(
		clientCtx,
		app.BaseApp.GRPCQueryRouter(),
		app.interfaceRegistry,
		app.Query,
	)
}

func (app *BabylonApp) RegisterNodeService(clientCtx client.Context, cfg config.Config) {
	nodeservice.RegisterNodeService(clientCtx, app.GRPCQueryRouter(), cfg)
}

// DefaultGenesis returns a default genesis from the registered AppModuleBasic's.
func (a *BabylonApp) DefaultGenesis() map[string]json.RawMessage {
	return a.BasicModuleManager.DefaultGenesis(a.appCodec)
}

func (app *BabylonApp) TxConfig() client.TxConfig {
	return app.txConfig
}

// AutoCliOpts returns the autocli options for the app.
func (app *BabylonApp) AutoCliOpts() autocli.AppOptions {
	modules := make(map[string]appmodule.AppModule, 0)
	for _, m := range app.ModuleManager.Modules {
		if moduleWithName, ok := m.(module.HasName); ok {
			moduleName := moduleWithName.Name()
			if appModule, ok := moduleWithName.(appmodule.AppModule); ok {
				modules[moduleName] = appModule
			}
		}
	}

	return autocli.AppOptions{
		Modules:               modules,
		ModuleOptions:         runtimeservices.ExtractAutoCLIOptions(app.ModuleManager.Modules),
		AddressCodec:          authcodec.NewBech32Codec(appparams.Bech32PrefixAccAddr),
		ValidatorAddressCodec: authcodec.NewBech32Codec(appparams.Bech32PrefixValAddr),
		ConsensusAddressCodec: authcodec.NewBech32Codec(appparams.Bech32PrefixConsAddr),
	}
}

// configure store loader that checks if version == upgradeHeight and applies store upgrades
func (app *BabylonApp) setupUpgradeStoreLoaders() {
	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(fmt.Sprintf("failed to read upgrade info from disk %s", err))
	}

	if app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		return
	}

	for _, upgrade := range Upgrades {
		if upgradeInfo.Name == upgrade.UpgradeName {
			storeUpgrades := upgrade.StoreUpgrades
			app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))
		}
	}
}

func (app *BabylonApp) setupUpgradeHandlers() {
	for _, upgrade := range Upgrades {
		app.UpgradeKeeper.SetUpgradeHandler(
			upgrade.UpgradeName,
			upgrade.CreateUpgradeHandler(
				app.ModuleManager,
				app.configurator,
				app.BaseApp,
				app.AppKeepers,
			),
		)
	}
}

// GetMaccPerms returns a copy of the module account permissions
func GetMaccPerms() map[string][]string {
	dupMaccPerms := make(map[string][]string)
	for k, v := range maccPerms {
		dupMaccPerms[k] = v
	}
	return dupMaccPerms
}

// BlockedAddresses returns all the app's blocked account addresses.
func BlockedAddresses() map[string]bool {
	modAccAddrs := make(map[string]bool)
	for acc := range GetMaccPerms() {
		modAccAddrs[authtypes.NewModuleAddress(acc).String()] = true
	}

	// allow the following addresses to receive funds
	delete(modAccAddrs, authtypes.NewModuleAddress(govtypes.ModuleName).String())

	return modAccAddrs
}
