package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	// Force-load the tracer engines to trigger registration due to Go-Ethereum v1.10.15 changes
	_ "github.com/ethereum/go-ethereum/eth/tracers/js"
	_ "github.com/ethereum/go-ethereum/eth/tracers/native"

	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"
	reflectionv1 "cosmossdk.io/api/cosmos/reflection/v1"
	"cosmossdk.io/client/v2/autocli"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"cosmossdk.io/x/circuit"
	circuittypes "cosmossdk.io/x/circuit/types"
	"cosmossdk.io/x/evidence"
	evidencetypes "cosmossdk.io/x/evidence/types"
	"cosmossdk.io/x/feegrant"
	feegrantmodule "cosmossdk.io/x/feegrant/module"
	"cosmossdk.io/x/upgrade"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/CosmWasm/wasmd/x/wasm"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/babylonlabs-io/babylon/v4/x/feemarketwrapper"
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
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/server/api"
	"github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/auth"
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
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/params"
	paramsclient "github.com/cosmos/cosmos-sdk/x/params/client"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	evmante "github.com/cosmos/evm/ante"
	evmencoding "github.com/cosmos/evm/encoding"
	srvflags "github.com/cosmos/evm/server/flags"
	evmutils "github.com/cosmos/evm/utils"
	"github.com/cosmos/evm/x/erc20"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	"github.com/cosmos/evm/x/precisebank"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evm "github.com/cosmos/evm/x/vm"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/cosmos/gogoproto/proto"
	pfmrouter "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v10/packetforward"
	pfmroutertypes "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v10/packetforward/types"
	ratelimiter "github.com/cosmos/ibc-apps/modules/rate-limiting/v10"
	ratelimittypes "github.com/cosmos/ibc-apps/modules/rate-limiting/v10/types"
	ibcwasm "github.com/cosmos/ibc-go/modules/light-clients/08-wasm/v10"
	ibcwasmkeeper "github.com/cosmos/ibc-go/modules/light-clients/08-wasm/v10/keeper"
	ibcwasmtypes "github.com/cosmos/ibc-go/modules/light-clients/08-wasm/v10/types"
	ica "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts"
	icatypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/types"
	"github.com/cosmos/ibc-go/v10/modules/apps/transfer"
	ibctransfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	ibc "github.com/cosmos/ibc-go/v10/modules/core" // ibc module puts types under `ibchost` rather than `ibctypes`
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
	ibctm "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/spf13/cast"

	"github.com/babylonlabs-io/babylon/v4/app/ante"
	appkeepers "github.com/babylonlabs-io/babylon/v4/app/keepers"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
	"github.com/babylonlabs-io/babylon/v4/client/docs"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/btccheckpoint"
	btccheckpointtypes "github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/types"
	"github.com/babylonlabs-io/babylon/v4/x/btclightclient"
	btclightclienttypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking"
	btcstakingtypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer"
	btcstkconsumertypes "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
	"github.com/babylonlabs-io/babylon/v4/x/checkpointing"
	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/prepare"
	checkpointingtypes "github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/vote_extensions"
	"github.com/babylonlabs-io/babylon/v4/x/coostaking"
	coostakingtypes "github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
	"github.com/babylonlabs-io/babylon/v4/x/epoching"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	"github.com/babylonlabs-io/babylon/v4/x/finality"
	finalitytypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
	"github.com/babylonlabs-io/babylon/v4/x/incentive"
	incentivekeeper "github.com/babylonlabs-io/babylon/v4/x/incentive/keeper"
	incentivetypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	"github.com/babylonlabs-io/babylon/v4/x/mint"
	minttypes "github.com/babylonlabs-io/babylon/v4/x/mint/types"
	"github.com/babylonlabs-io/babylon/v4/x/monitor"
	monitortypes "github.com/babylonlabs-io/babylon/v4/x/monitor/types"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge"
	zckeeper "github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/keeper"
	zctypes "github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	"github.com/strangelove-ventures/tokenfactory/x/tokenfactory"
	tokenfactorytypes "github.com/strangelove-ventures/tokenfactory/x/tokenfactory/types"
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
		authtypes.FeeCollectorName:                  {authtypes.Burner}, // fee collector account, needs Burner role for feemarket burning of BaseFee
		distrtypes.ModuleName:                       nil,
		minttypes.ModuleName:                        {authtypes.Minter},
		stakingtypes.BondedPoolName:                 {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName:              {authtypes.Burner, authtypes.Staking},
		govtypes.ModuleName:                         {authtypes.Burner},
		ibctransfertypes.ModuleName:                 {authtypes.Minter, authtypes.Burner},
		incentivetypes.ModuleName:                   nil, // this line is needed to create an account for incentive module
		coostakingtypes.ModuleName:                  nil, // this line is needed to create an account for coostaking module
		tokenfactorytypes.ModuleName:                {authtypes.Minter, authtypes.Burner},
		icatypes.ModuleName:                         nil,
		evmtypes.ModuleName:                         {authtypes.Minter, authtypes.Burner},
		feemarkettypes.ModuleName:                   nil,
		erc20types.ModuleName:                       {authtypes.Minter, authtypes.Burner}, // Allows erc20 module to mint/burn for token pairs
		precisebanktypes.ModuleName:                 {authtypes.Minter, authtypes.Burner},
		incentivetypes.ModAccCommissionCollectorBSN: nil, // Babylon BSN rewards commission collector
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
	clientCtx   client.Context

	interfaceRegistry types.InterfaceRegistry
	invCheckPeriod    uint

	// the module manager
	ModuleManager      *module.Manager
	BasicModuleManager module.BasicManager

	// simulation manager
	sm *module.SimulationManager

	// module configurator
	configurator module.Configurator

	// pending tx listeners
	pendingTxListeners []evmante.PendingTxListener
}

// NewBabylonApp returns a reference to an initialized BabylonApp.
func NewBabylonApp(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	skipUpgradeHeights map[int64]bool,
	invCheckPeriod uint,
	blsSigner *checkpointingtypes.BlsSigner,
	appOpts servertypes.AppOptions,
	evmChainID uint64,
	evmAppOptions EVMOptionsFn,
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

	encCfg := evmencoding.MakeConfig(evmChainID)
	interfaceRegistry := encCfg.InterfaceRegistry
	appCodec := encCfg.Codec
	legacyAmino := encCfg.Amino
	txConfig := encCfg.TxConfig

	bApp := baseapp.NewBaseApp(appName, logger, db, txConfig.TxDecoder(), baseAppOptions...)
	bApp.SetCommitMultiStoreTracer(traceStore)
	bApp.SetVersion(version.Version)
	bApp.SetInterfaceRegistry(interfaceRegistry)
	bApp.SetTxEncoder(txConfig.TxEncoder())
	bApp.SetMempool(getAppMempool(appOpts))

	// Add after encoder has been set:
	if err := evmAppOptions(evmChainID); err != nil {
		// Initialize the EVM application configuration
		panic(fmt.Errorf("failed to initialize EVM app configuration: %w", err))
	}

	wasmConfig, err := wasm.ReadNodeConfig(appOpts)
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
		&btcConfig,
		encCfg,
		bApp,
		maccPerms,
		homePath,
		invCheckPeriod,
		skipUpgradeHeights,
		*blsSigner,
		appOpts,
		wasmConfig,
		wasmOpts,
		BlockedAddresses(),
	)

	// Create IBC Tendermint Light Client Stack
	clientKeeper := app.IBCKeeper.ClientKeeper
	tmLightClientModule := ibctm.NewLightClientModule(appCodec, clientKeeper.GetStoreProvider())
	clientKeeper.AddRoute(ibctm.ModuleName, &tmLightClientModule)

	// TODO: Do we we need this ?
	// wasmLightClientModule := ibcwasm.NewLightClientModule(app.WasmClientKeeper, clientKeeper.GetStoreProvider())
	// clientKeeper.AddRoute(ibcwasmtypes.ModuleName, &wasmLightClientModule)

	/****  Module Options ****/

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
		feegrantmodule.NewAppModule(appCodec, app.AccountKeeper, app.BankKeeper, app.FeeGrantKeeper, app.interfaceRegistry),
		gov.NewAppModule(appCodec, &app.GovKeeper, app.AccountKeeper, app.BankKeeper, app.GetSubspace(govtypes.ModuleName)),
		mint.NewAppModule(appCodec, app.MintKeeper, app.AccountKeeper),
		slashing.NewAppModule(appCodec, app.SlashingKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper, app.GetSubspace(slashingtypes.ModuleName), app.interfaceRegistry),
		distr.NewAppModule(appCodec, app.DistrKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper, app.GetSubspace(distrtypes.ModuleName)),
		staking.NewAppModule(appCodec, app.StakingKeeper, app.AccountKeeper, app.BankKeeper, app.GetSubspace(stakingtypes.ModuleName)),
		upgrade.NewAppModule(app.UpgradeKeeper, app.AccountKeeper.AddressCodec()),
		evidence.NewAppModule(app.EvidenceKeeper),
		params.NewAppModule(app.ParamsKeeper), //nolint:staticcheck
		consensus.NewAppModule(appCodec, app.ConsensusParamsKeeper),
		circuit.NewAppModule(appCodec, app.CircuitKeeper),
		authzmodule.NewAppModule(appCodec, app.AuthzKeeper, app.AccountKeeper, app.BankKeeper, app.interfaceRegistry),
		// non sdk modules
		wasm.NewAppModule(appCodec, &app.WasmKeeper, app.StakingKeeper, app.AccountKeeper, app.BankKeeper, app.MsgServiceRouter(), app.GetSubspace(wasmtypes.ModuleName)),
		ibc.NewAppModule(app.IBCKeeper),
		transfer.NewAppModule(app.TransferKeeper),
		ibctm.AppModule{},
		ibcwasm.NewAppModule(app.IBCWasmKeeper),
		pfmrouter.NewAppModule(app.PFMRouterKeeper, app.GetSubspace(pfmroutertypes.ModuleName)),
		tokenfactory.NewAppModule(app.TokenFactoryKeeper, app.AccountKeeper, app.BankKeeper, app.GetSubspace(tokenfactorytypes.ModuleName)),
		ratelimiter.NewAppModule(appCodec, app.RatelimitKeeper),
		ica.NewAppModule(app.ICAControllerKeeper, app.ICAHostKeeper),
		// Babylon modules - btc timestamping
		epoching.NewAppModule(appCodec, app.EpochingKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper),
		btclightclient.NewAppModule(appCodec, app.BTCLightClientKeeper),
		btccheckpoint.NewAppModule(appCodec, app.BtcCheckpointKeeper),
		checkpointing.NewAppModule(appCodec, app.CheckpointingKeeper),
		monitor.NewAppModule(appCodec, app.MonitorKeeper),
		// Babylon modules - integration
		btcstkconsumer.NewAppModule(appCodec, app.BTCStkConsumerKeeper),
		zoneconcierge.NewAppModule(appCodec, app.ZoneConciergeKeeper, app.AccountKeeper, app.BankKeeper),
		// Babylon modules - btc staking
		btcstaking.NewAppModule(appCodec, app.BTCStakingKeeper),
		finality.NewAppModule(appCodec, app.FinalityKeeper),
		// Babylon modules - tokenomics
		incentive.NewAppModule(appCodec, app.IncentiveKeeper, app.AccountKeeper, app.BankKeeper),
		coostaking.NewAppModule(appCodec, app.CoostakingKeeper),
		// Cosmos EVM modules
		evm.NewAppModule(app.EVMKeeper, app.AccountKeeper, app.AccountKeeper.AddressCodec()),
		feemarketwrapper.NewAppModule(app.FeemarketKeeper, app.GetTKey(feemarkettypes.TransientKey)),
		erc20.NewAppModule(app.Erc20Keeper, app.AccountKeeper),
		precisebank.NewAppModule(app.PreciseBankKeeper, app.BankKeeper, app.AccountKeeper),
	)

	// BasicModuleManager defines the module BasicManager which is in charge of setting up basic,
	// non-dependant module elements, such as codec registration and genesis verification.
	// By default, it is composed of all the modules from the module manager.
	// Additionally, app module basics can be overwritten by passing them as an argument.
	app.BasicModuleManager = module.NewBasicManagerFromManager(
		app.ModuleManager,
		map[string]module.AppModuleBasic{
			genutiltypes.ModuleName: genutil.NewAppModuleBasic(checkpointingtypes.GenTxMessageValidatorWrappedCreateValidator),
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
		authtypes.ModuleName,
	)

	// During begin block slashing happens after distr.BeginBlocker so that
	// there is nothing left over in the validator fee pool, so as to keep the
	// CanWithdrawInvariant invariant.
	// NOTE: staking module is required if HistoricalEntries param > 0
	app.ModuleManager.SetOrderBeginBlockers(
		upgradetypes.ModuleName,
		// NOTE: incentive module's BeginBlock has to be after mint but before distribution
		// so that it can intercept a part of new inflation to reward BTC staking stakeholders.
		// Coostaking module goes right after incentives but before distribution to also take
		// a cut of the inflation for Coostaking (BABY + BTC) staking.
		minttypes.ModuleName, incentivetypes.ModuleName, coostakingtypes.ModuleName, distrtypes.ModuleName,
		// Cosmos EVM
		erc20types.ModuleName,
		feemarkettypes.ModuleName,
		evmtypes.ModuleName, // NOTE: EVM BeginBlocker must come after FeeMarket BeginBlocker
		slashingtypes.ModuleName,
		evidencetypes.ModuleName, stakingtypes.ModuleName,
		authtypes.ModuleName, banktypes.ModuleName, govtypes.ModuleName, genutiltypes.ModuleName,
		authz.ModuleName, feegrant.ModuleName,
		paramstypes.ModuleName, vestingtypes.ModuleName, consensusparamtypes.ModuleName, circuittypes.ModuleName,
		// Token factory
		tokenfactorytypes.ModuleName,
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
		wasmtypes.ModuleName,
		ratelimittypes.ModuleName,
		// Integration related modules
		zctypes.ModuleName,
		btcstkconsumertypes.ModuleName,
		// BTC staking related modules
		btcstakingtypes.ModuleName,
		finalitytypes.ModuleName,
	)
	// TODO: there will be an architecture design on whether to modify slashing/evidence, specifically
	// - how many validators can we slash in a single epoch and
	// - whether and when to jail slashed validators
	// app.mm.OrderBeginBlockers = append(app.mm.OrderBeginBlockers[:4], app.mm.OrderBeginBlockers[4+1:]...) // remove slashingtypes.ModuleName
	// app.mm.OrderBeginBlockers = append(app.mm.OrderBeginBlockers[:4], app.mm.OrderBeginBlockers[4+1:]...) // remove evidencetypes.ModuleName

	app.ModuleManager.SetOrderEndBlockers(govtypes.ModuleName, stakingtypes.ModuleName,
		authtypes.ModuleName, banktypes.ModuleName, distrtypes.ModuleName,
		slashingtypes.ModuleName, minttypes.ModuleName,
		genutiltypes.ModuleName, evidencetypes.ModuleName, authz.ModuleName,
		feegrant.ModuleName,
		paramstypes.ModuleName, upgradetypes.ModuleName, vestingtypes.ModuleName, consensusparamtypes.ModuleName,
		// Token factory
		tokenfactorytypes.ModuleName,
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
		wasmtypes.ModuleName,
		ratelimittypes.ModuleName,
		// Integration related modules
		zctypes.ModuleName,
		btcstkconsumertypes.ModuleName,
		// BTC staking related modules
		btcstakingtypes.ModuleName,
		finalitytypes.ModuleName,
		// tokenomics related modules
		incentivetypes.ModuleName,  // EndBlock of incentive module does not matter
		coostakingtypes.ModuleName, // EndBlock of coostaking module does not matter
		// Cosmos EVM
		evmtypes.ModuleName,
		erc20types.ModuleName,
		feemarkettypes.ModuleName,
	)
	// Babylon does not want EndBlock processing in staking
	app.ModuleManager.OrderEndBlockers = append(app.ModuleManager.OrderEndBlockers[:1], app.ModuleManager.OrderEndBlockers[1+1:]...) // remove stakingtypes.ModuleName

	for _, m := range app.ModuleManager.OrderEndBlockers {
		if strings.EqualFold(m, stakingtypes.ModuleName) {
			panic("staking module endblocker is active")
		}
	}

	// NOTE: The genutils module must occur after staking so that pools are
	// properly initialized with tokens from genesis accounts.
	// so that other modules that want to create or claim capabilities afterwards in InitChain
	// can do so safely.
	genesisModuleOrder := []string{
		authtypes.ModuleName, banktypes.ModuleName, distrtypes.ModuleName,
		// module that subscribes to staking hooks.
		// staking init genesis calls AfterDelegationModified
		coostakingtypes.ModuleName,
		stakingtypes.ModuleName,
		slashingtypes.ModuleName, govtypes.ModuleName, minttypes.ModuleName,
		// Cosmos EVM modules
		// NOTE: feemarket module needs to be initialized before genutil module
		evmtypes.ModuleName,
		feemarkettypes.ModuleName,
		erc20types.ModuleName,
		precisebanktypes.ModuleName,
		genutiltypes.ModuleName, evidencetypes.ModuleName, authz.ModuleName,
		feegrant.ModuleName,
		paramstypes.ModuleName, upgradetypes.ModuleName, vestingtypes.ModuleName, consensusparamtypes.ModuleName, circuittypes.ModuleName,
		// Token factory
		tokenfactorytypes.ModuleName,
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
		ratelimittypes.ModuleName,
		wasmtypes.ModuleName,
		icatypes.ModuleName,
		pfmroutertypes.ModuleName,
		// Integration related modules
		zctypes.ModuleName,
		btcstkconsumertypes.ModuleName,
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

	app.configurator = module.NewConfigurator(app.appCodec, app.MsgServiceRouter(), app.GRPCQueryRouter())
	app.RegisterServicesWithoutStaking()
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

	maxGasWanted := cast.ToUint64(appOpts.Get(srvflags.EVMMaxTxGasWanted))
	evmHandlerOpts := NewEVMAnteHandlerOptionsFromApp(app, txConfig, maxGasWanted)
	if err := evmHandlerOpts.Validate(); err != nil {
		panic(err)
	}

	// initialize AnteHandler for the app
	anteHandler := ante.NewAnteHandler(
		appOpts,
		evmHandlerOpts.Options(),
		&app.AccountKeeper,
		app.BankKeeper,
		&app.FeeGrantKeeper,
		txConfig.SignModeHandler(),
		app.IBCKeeper,
		&wasmConfig,
		&app.WasmKeeper,
		&app.CircuitKeeper,
		&app.EpochingKeeper,
		&btcConfig,
		&app.BtcCheckpointKeeper,
		runtime.NewKVStoreService(app.AppKeepers.GetKey(wasmtypes.StoreKey)),
	)

	// set proposal extension
	proposalHandler := prepare.NewProposalHandler(
		logger, &app.CheckpointingKeeper, bApp.Mempool(), bApp, app.EncCfg, bbn.NewEthSignerExtractionAdapter(mempool.NewDefaultSignerExtractionAdapter()))
	proposalHandler.SetHandlers(bApp)

	// set vote extension
	voteExtHandler := vote_extensions.NewVoteExtensionHandler(logger, &app.CheckpointingKeeper)
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
		incentivekeeper.NewRefundTxDecorator(&app.IncentiveKeeper, app.GetTKey(feemarkettypes.TransientKey)),
		zckeeper.NewIBCHeaderDecorator(&app.ZoneConciergeKeeper),
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
	// FIXME (https://github.com/babylonlabs-io/babylon-private/issues/266): This is a temporary fix
	protoFiles, _ := proto.MergedRegistry()
	// if err != nil {
	// 	panic(err)
	// }
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

// RegisterServicesWithoutStaking calls the module manager
// registration services without the staking module.
func (app *BabylonApp) RegisterServicesWithoutStaking() {
	// removes the staking module from the register services
	stkModTemp := app.ModuleManager.Modules[stakingtypes.ModuleName]
	delete(app.ModuleManager.Modules, stakingtypes.ModuleName)

	if err := app.ModuleManager.RegisterServices(app.configurator); err != nil {
		panic(err)
	}

	app.RegisterStakingQueryAndMigrations()

	// adds the staking module back it back
	app.ModuleManager.Modules[stakingtypes.ModuleName] = stkModTemp
}

// RegisterStakingQueryAndMigrations registrates in the configurator
// the x/staking query server and its migrations
func (app *BabylonApp) RegisterStakingQueryAndMigrations() {
	cfg, stkK := app.configurator, app.StakingKeeper
	stkq := stakingkeeper.NewQuerier(stkK)

	stakingtypes.RegisterQueryServer(cfg.QueryServer(), stkq)

	ls := app.GetSubspace(stakingtypes.ModuleName)
	m := stakingkeeper.NewMigrator(stkK, ls)
	if err := cfg.RegisterMigration(stakingtypes.ModuleName, 1, m.Migrate1to2); err != nil {
		panic(fmt.Sprintf("failed to migrate x/%s from version 1 to 2: %v", stakingtypes.ModuleName, err))
	}
	if err := cfg.RegisterMigration(stakingtypes.ModuleName, 2, m.Migrate2to3); err != nil {
		panic(fmt.Sprintf("failed to migrate x/%s from version 2 to 3: %v", stakingtypes.ModuleName, err))
	}
	if err := cfg.RegisterMigration(stakingtypes.ModuleName, 3, m.Migrate3to4); err != nil {
		panic(fmt.Sprintf("failed to migrate x/%s from version 3 to 4: %v", stakingtypes.ModuleName, err))
	}
	if err := cfg.RegisterMigration(stakingtypes.ModuleName, 4, m.Migrate4to5); err != nil {
		panic(fmt.Sprintf("failed to migrate x/%s from version 4 to 5: %v", stakingtypes.ModuleName, err))
	}
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
	genesis := a.BasicModuleManager.DefaultGenesis(a.appCodec)
	// Add EVM genesis configuration
	evmGenState := evmtypes.DefaultGenesisState()
	evmGenState.Params.ActiveStaticPrecompiles = evmtypes.AvailableStaticPrecompiles
	evmGenState.Params.EvmDenom = appparams.BaseCoinUnit
	genesis[evmtypes.ModuleName] = a.appCodec.MustMarshalJSON(evmGenState)

	// Add ERC20 genesis configuration
	erc20GenState := erc20types.DefaultGenesisState()
	genesis[erc20types.ModuleName] = a.appCodec.MustMarshalJSON(erc20GenState)

	feemarketGenState := feemarkettypes.DefaultGenesisState()
	feemarketGenState.Params.NoBaseFee = false
	feemarketGenState.Params.BaseFee = math.LegacyMustNewDecFromStr("0.01")
	feemarketGenState.Params.MinGasPrice = feemarketGenState.Params.BaseFee
	genesis[feemarkettypes.ModuleName] = a.appCodec.MustMarshalJSON(feemarketGenState)

	return genesis
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
			return
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
	blockedAddrs := make(map[string]bool)
	for acc := range GetMaccPerms() {
		blockedAddrs[authtypes.NewModuleAddress(acc).String()] = true
	}

	// allow the following addresses to receive funds
	delete(blockedAddrs, appparams.AccGov.String())

	// Block precompiled contracts
	blockedPrecompilesHex := evmtypes.AvailableStaticPrecompiles
	for _, addr := range vm.PrecompiledAddressesPrague {
		blockedPrecompilesHex = append(blockedPrecompilesHex, addr.Hex())
	}

	for _, precompile := range blockedPrecompilesHex {
		blockedAddrs[evmutils.Bech32StringFromHexAddress(precompile)] = true
	}
	return blockedAddrs
}

// getAppMempool returns the corresponding application mempool according to the
// mempool.MaxTx value (default = 0).
// - mempool.MaxTx = 0: uncapped PriorityNonce mempool (default)
// - mempool.MaxTx > 0: capped PriorityNonce mempool
// - mempool.MaxTx < 0: no-op mempool
func getAppMempool(appOpts servertypes.AppOptions) mempool.Mempool {
	var (
		mp         mempool.Mempool
		maxTxs     = cast.ToInt(appOpts.Get(server.FlagMempoolMaxTxs))
		mempoolCfg = mempool.DefaultPriorityNonceMempoolConfig()
	)
	mempoolCfg.SignerExtractor = bbn.NewEthSignerExtractionAdapter(mempool.NewDefaultSignerExtractionAdapter())
	mempoolCfg.MaxTx = maxTxs
	mp = mempool.NewPriorityMempool(mempoolCfg)
	if maxTxs < 0 {
		mp = mempool.NoOpMempool{}
	}
	return mp
}

func (app *BabylonApp) RegisterPendingTxListener(listener func(common.Hash)) {
	app.pendingTxListeners = append(app.pendingTxListeners, listener)
}

func (app *BabylonApp) SetClientCtx(clientCtx client.Context) {
	app.clientCtx = clientCtx
}
