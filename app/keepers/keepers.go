package keepers

import (
	"path/filepath"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	circuitkeeper "cosmossdk.io/x/circuit/keeper"
	circuittypes "cosmossdk.io/x/circuit/types"
	evidencekeeper "cosmossdk.io/x/evidence/keeper"
	evidencetypes "cosmossdk.io/x/evidence/types"
	"cosmossdk.io/x/feegrant"
	feegrantkeeper "cosmossdk.io/x/feegrant/keeper"
	upgradekeeper "cosmossdk.io/x/upgrade/keeper"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/CosmWasm/wasmd/x/wasm"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	appparams "github.com/babylonlabs-io/babylon/v2/app/params"
	bbn "github.com/babylonlabs-io/babylon/v2/types"
	owasm "github.com/babylonlabs-io/babylon/v2/wasmbinding"
	btccheckpointkeeper "github.com/babylonlabs-io/babylon/v2/x/btccheckpoint/keeper"
	btccheckpointtypes "github.com/babylonlabs-io/babylon/v2/x/btccheckpoint/types"
	btclightclientkeeper "github.com/babylonlabs-io/babylon/v2/x/btclightclient/keeper"
	btclightclienttypes "github.com/babylonlabs-io/babylon/v2/x/btclightclient/types"
	btcstakingkeeper "github.com/babylonlabs-io/babylon/v2/x/btcstaking/keeper"
	btcstakingtypes "github.com/babylonlabs-io/babylon/v2/x/btcstaking/types"
	checkpointingkeeper "github.com/babylonlabs-io/babylon/v2/x/checkpointing/keeper"
	checkpointingtypes "github.com/babylonlabs-io/babylon/v2/x/checkpointing/types"
	epochingkeeper "github.com/babylonlabs-io/babylon/v2/x/epoching/keeper"
	epochingtypes "github.com/babylonlabs-io/babylon/v2/x/epoching/types"
	finalitykeeper "github.com/babylonlabs-io/babylon/v2/x/finality/keeper"
	finalitytypes "github.com/babylonlabs-io/babylon/v2/x/finality/types"
	incentivekeeper "github.com/babylonlabs-io/babylon/v2/x/incentive/keeper"
	incentivetypes "github.com/babylonlabs-io/babylon/v2/x/incentive/types"
	mintkeeper "github.com/babylonlabs-io/babylon/v2/x/mint/keeper"
	minttypes "github.com/babylonlabs-io/babylon/v2/x/mint/types"
	monitorkeeper "github.com/babylonlabs-io/babylon/v2/x/monitor/keeper"
	monitortypes "github.com/babylonlabs-io/babylon/v2/x/monitor/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/runtime"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	consensusparamkeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"github.com/cosmos/cosmos-sdk/x/params"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	paramproposal "github.com/cosmos/cosmos-sdk/x/params/types/proposal"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
<<<<<<< HEAD
	pfmrouter "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v8/packetforward"
	pfmrouterkeeper "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v8/packetforward/keeper"
	pfmroutertypes "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v8/packetforward/types"
	ratelimiter "github.com/cosmos/ibc-apps/modules/rate-limiting/v8"
	ratelimitkeeper "github.com/cosmos/ibc-apps/modules/rate-limiting/v8/keeper"
	ratelimittypes "github.com/cosmos/ibc-apps/modules/rate-limiting/v8/types"
	ibccallbacks "github.com/cosmos/ibc-go/modules/apps/callbacks"
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	ibcwasmkeeper "github.com/cosmos/ibc-go/modules/light-clients/08-wasm/keeper"
	ibcwasmtypes "github.com/cosmos/ibc-go/modules/light-clients/08-wasm/types"
	icacontroller "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller"
	icacontrollerkeeper "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller/keeper"
	icacontrollertypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller/types"
	icahost "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host"
	icahostkeeper "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/keeper"
	icahosttypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/types"
	"github.com/cosmos/ibc-go/v8/modules/apps/transfer"
	ibctransferkeeper "github.com/cosmos/ibc-go/v8/modules/apps/transfer/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	porttypes "github.com/cosmos/ibc-go/v8/modules/core/05-port/types" // ibc module puts types under `ibchost` rather than `ibctypes`
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"
	ibckeeper "github.com/cosmos/ibc-go/v8/modules/core/keeper"
=======
	ibccallbacks "github.com/cosmos/ibc-go/v10/modules/apps/callbacks"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"
	tokenfactorybindings "github.com/strangelove-ventures/tokenfactory/x/tokenfactory/bindings"
>>>>>>> 09820f4 (add custom tokenfactory bindings (#1398))
	tokenfactorykeeper "github.com/strangelove-ventures/tokenfactory/x/tokenfactory/keeper"
	tokenfactorytypes "github.com/strangelove-ventures/tokenfactory/x/tokenfactory/types"
)

// Enable all default present capabilities.
var tokenFactoryCapabilities = []string{
	tokenfactorytypes.EnableBurnFrom,
	tokenfactorytypes.EnableForceTransfer,
	tokenfactorytypes.EnableSetMetadata,
	// CommunityPoolFeeFunding sends tokens to the community pool when a new fee is charged (if one is set in params).
	// This is useful for ICS chains, or networks who wish to just have the fee tokens burned (not gas fees, just the extra on top).
	tokenfactorytypes.EnableCommunityPoolFeeFunding,
}

// Capabilities of the IBC wasm contracts
func WasmCapabilities() []string {
	// The last arguments can contain custom message handlers, and custom query handlers,
	// if we want to allow any custom callbacks
	return []string{
		"iterator",
		"staking",
		"stargate",
		"cosmwasm_1_1",
		"cosmwasm_1_2",
		"cosmwasm_1_3",
		"cosmwasm_1_4",
		"cosmwasm_2_0",
		"babylon",
	}
}

type AppKeepers struct {
	// keepers
	AccountKeeper         authkeeper.AccountKeeper
	BankKeeper            bankkeeper.Keeper
	CapabilityKeeper      *capabilitykeeper.Keeper
	StakingKeeper         *stakingkeeper.Keeper
	SlashingKeeper        slashingkeeper.Keeper
	MintKeeper            mintkeeper.Keeper
	DistrKeeper           distrkeeper.Keeper
	GovKeeper             govkeeper.Keeper
	UpgradeKeeper         *upgradekeeper.Keeper
	ParamsKeeper          paramskeeper.Keeper
	AuthzKeeper           authzkeeper.Keeper
	EvidenceKeeper        evidencekeeper.Keeper
	FeeGrantKeeper        feegrantkeeper.Keeper
	ConsensusParamsKeeper consensusparamkeeper.Keeper
	CircuitKeeper         circuitkeeper.Keeper

	// Token factory
	TokenFactoryKeeper tokenfactorykeeper.Keeper

	// Babylon modules
	EpochingKeeper       epochingkeeper.Keeper
	BTCLightClientKeeper btclightclientkeeper.Keeper
	BtcCheckpointKeeper  btccheckpointkeeper.Keeper
	CheckpointingKeeper  checkpointingkeeper.Keeper
	MonitorKeeper        monitorkeeper.Keeper

	// IBC-related modules
	IBCKeeper           *ibckeeper.Keeper           // IBC Keeper must be a pointer in the app, so we can SetRouter on it correctly
	TransferKeeper      ibctransferkeeper.Keeper    // for cross-chain fungible token transfers
	IBCWasmKeeper       ibcwasmkeeper.Keeper        // for IBC wasm light clients
	PFMRouterKeeper     *pfmrouterkeeper.Keeper     // Packet Forwarding Middleware
	ICAHostKeeper       *icahostkeeper.Keeper       // Interchain Accounts host
	ICAControllerKeeper *icacontrollerkeeper.Keeper // Interchain Accounts controller
	RatelimitKeeper     ratelimitkeeper.Keeper

	// BTC staking related modules
	BTCStakingKeeper btcstakingkeeper.Keeper
	FinalityKeeper   finalitykeeper.Keeper

	// wasm smart contract module
	WasmKeeper wasmkeeper.Keeper

	// tokenomics-related modules
	IncentiveKeeper incentivekeeper.Keeper

	// make scoped keepers public for test purposes
	ScopedIBCKeeper           capabilitykeeper.ScopedKeeper
	ScopedTransferKeeper      capabilitykeeper.ScopedKeeper
	ScopedZoneConciergeKeeper capabilitykeeper.ScopedKeeper
	ScopedWasmKeeper          capabilitykeeper.ScopedKeeper

	// keys to access the substores
	keys    map[string]*storetypes.KVStoreKey
	tkeys   map[string]*storetypes.TransientStoreKey
	memKeys map[string]*storetypes.MemoryStoreKey

	EncCfg *appparams.EncodingConfig
}

func (ak *AppKeepers) InitKeepers(
	logger log.Logger,
	btcConfig *bbn.BtcConfig,
	encodingConfig *appparams.EncodingConfig,
	bApp *baseapp.BaseApp,
	maccPerms map[string][]string,
	homePath string,
	invCheckPeriod uint,
	skipUpgradeHeights map[int64]bool,
	blsSigner checkpointingtypes.BlsSigner,
	appOpts servertypes.AppOptions,
	wasmConfig wasmtypes.NodeConfig,
	wasmOpts []wasmkeeper.Option,
	blockedAddress map[string]bool,
) {
	powLimit := btcConfig.PowLimit()
	btcNetParams := btcConfig.NetParams()

	ak.EncCfg = encodingConfig
	appCodec := encodingConfig.Codec

	// set persistent store keys
	keys := storetypes.NewKVStoreKeys(
		authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey,
		minttypes.StoreKey, distrtypes.StoreKey, slashingtypes.StoreKey,
		govtypes.StoreKey, paramstypes.StoreKey, consensusparamtypes.StoreKey, upgradetypes.StoreKey, feegrant.StoreKey,
		evidencetypes.StoreKey, circuittypes.StoreKey, capabilitytypes.StoreKey,
		authzkeeper.StoreKey,
		// Babylon modules
		epochingtypes.StoreKey,
		btclightclienttypes.StoreKey,
		btccheckpointtypes.StoreKey,
		checkpointingtypes.StoreKey,
		monitortypes.StoreKey,
		// IBC-related modules
		ibcexported.StoreKey,
		ibctransfertypes.StoreKey,
		ibcwasmtypes.StoreKey,
		pfmroutertypes.StoreKey,
		icahosttypes.StoreKey,
		icacontrollertypes.StoreKey,
		ratelimittypes.StoreKey,
		// BTC staking related modules
		btcstakingtypes.StoreKey,
		finalitytypes.StoreKey,
		// WASM
		wasmtypes.StoreKey,
		// tokenomics-related modules
		incentivetypes.StoreKey,
		tokenfactorytypes.StoreKey,
	)
	ak.keys = keys

	// set transient store keys
	ak.tkeys = storetypes.NewTransientStoreKeys(paramstypes.TStoreKey, btccheckpointtypes.TStoreKey)

	// set memory store keys
	// NOTE: The testingkey is just mounted for testing purposes. Actual applications should
	// not include this key.
	ak.memKeys = storetypes.NewMemoryStoreKeys(capabilitytypes.MemStoreKey, "testingkey")

	accountKeeper := authkeeper.NewAccountKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[authtypes.StoreKey]),
		authtypes.ProtoBaseAccount,
		maccPerms,
		authcodec.NewBech32Codec(appparams.Bech32PrefixAccAddr),
		appparams.Bech32PrefixAccAddr,
		appparams.AccGov.String(),
	)

	bankKeeper := bankkeeper.NewBaseKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[banktypes.StoreKey]),
		accountKeeper,
		blockedAddress,
		appparams.AccGov.String(),
		logger,
	)

	stakingKeeper := stakingkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[stakingtypes.StoreKey]),
		accountKeeper,
		bankKeeper,
		appparams.AccGov.String(),
		authcodec.NewBech32Codec(appparams.Bech32PrefixValAddr),
		authcodec.NewBech32Codec(appparams.Bech32PrefixConsAddr),
	)

	// NOTE: the epoching module has to be set before the chekpointing module, as the checkpointing module will have access to the epoching module
	epochingKeeper := epochingkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[epochingtypes.StoreKey]),
		bankKeeper,
		stakingKeeper,
		stakingkeeper.NewMsgServerImpl(stakingKeeper),
		appparams.AccGov.String(),
	)

	checkpointingKeeper := checkpointingkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[checkpointingtypes.StoreKey]),
		blsSigner,
		epochingKeeper,
	)

	// register streaming services
	if err := bApp.RegisterStreamingServices(appOpts, keys); err != nil {
		panic(err)
	}

	ak.ParamsKeeper = initParamsKeeper(
		appCodec,
		encodingConfig.Amino,
		keys[paramstypes.StoreKey],
		ak.tkeys[paramstypes.TStoreKey],
	)

	ak.ConsensusParamsKeeper = consensusparamkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[consensusparamtypes.StoreKey]),
		appparams.AccGov.String(),
		runtime.EventService{},
	)
	bApp.SetParamStore(ak.ConsensusParamsKeeper.ParamsStore)

	ak.CapabilityKeeper = capabilitykeeper.NewKeeper(
		appCodec,
		keys[capabilitytypes.StoreKey],
		ak.memKeys[capabilitytypes.MemStoreKey],
	)

	// grant capabilities for the ibc and ibc-transfer modules
	scopedIBCKeeper := ak.CapabilityKeeper.ScopeToModule(ibcexported.ModuleName)
	scopedTransferKeeper := ak.CapabilityKeeper.ScopeToModule(ibctransfertypes.ModuleName)
	scopedWasmKeeper := ak.CapabilityKeeper.ScopeToModule(wasmtypes.ModuleName)
	scopedICAHostKeeper := ak.CapabilityKeeper.ScopeToModule(icahosttypes.SubModuleName)
	scopedICAControllerKeeper := ak.CapabilityKeeper.ScopeToModule(icacontrollertypes.SubModuleName)

	// Applications that wish to enforce statically created ScopedKeepers should call `Seal` after creating
	// their scoped modules in `NewApp` with `ScopeToModule`
	ak.CapabilityKeeper.Seal()

	// add keepers
	ak.AccountKeeper = accountKeeper

	ak.BankKeeper = bankKeeper

	ak.StakingKeeper = stakingKeeper

	ak.CircuitKeeper = circuitkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[circuittypes.StoreKey]),
		appparams.AccGov.String(),
		ak.AccountKeeper.AddressCodec(),
	)
	bApp.SetCircuitBreaker(&ak.CircuitKeeper)

	ak.MintKeeper = mintkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[minttypes.StoreKey]),
		ak.StakingKeeper,
		ak.AccountKeeper,
		ak.BankKeeper,
		authtypes.FeeCollectorName,
		appparams.AccGov.String(),
	)

	ak.DistrKeeper = distrkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[distrtypes.StoreKey]),
		ak.AccountKeeper,
		ak.BankKeeper,
		ak.StakingKeeper,
		authtypes.FeeCollectorName,
		appparams.AccGov.String(),
	)

	// set up incentive keeper
	ak.IncentiveKeeper = incentivekeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[incentivetypes.StoreKey]),
		ak.BankKeeper,
		ak.AccountKeeper,
		&epochingKeeper,
		appparams.AccGov.String(),
		authtypes.FeeCollectorName,
	)

	ak.SlashingKeeper = slashingkeeper.NewKeeper(
		appCodec,
		encodingConfig.Amino,
		runtime.NewKVStoreService(keys[slashingtypes.StoreKey]),
		ak.StakingKeeper,
		appparams.AccGov.String(),
	)

	ak.FeeGrantKeeper = feegrantkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[feegrant.StoreKey]),
		ak.AccountKeeper,
	)
	// register the staking hooks
	// NOTE: stakingKeeper above is passed by reference, so that it will contain these hooks
	ak.StakingKeeper.SetHooks(
		stakingtypes.NewMultiStakingHooks(ak.DistrKeeper.Hooks(), ak.SlashingKeeper.Hooks(), epochingKeeper.Hooks()),
	)

	// set the governance module account as the authority for conducting upgrades
	ak.UpgradeKeeper = upgradekeeper.NewKeeper(
		skipUpgradeHeights,
		runtime.NewKVStoreService(keys[upgradetypes.StoreKey]),
		appCodec,
		homePath,
		bApp,
		appparams.AccGov.String(),
	)

	ak.AuthzKeeper = authzkeeper.NewKeeper(
		runtime.NewKVStoreService(keys[authzkeeper.StoreKey]),
		appCodec,
		bApp.MsgServiceRouter(),
		ak.AccountKeeper,
	)

	ak.IBCKeeper = ibckeeper.NewKeeper(
		appCodec,
		keys[ibcexported.StoreKey],
		ak.GetSubspace(ibcexported.ModuleName),
		ak.StakingKeeper,
		ak.UpgradeKeeper,
		scopedIBCKeeper,
		// From 8.0.0 the IBC keeper requires an authority for the messages
		// `MsgIBCSoftwareUpgrade` and `MsgRecoverClient`
		// https://github.com/cosmos/ibc-go/releases/tag/v8.0.0
		// Gov is the proper authority for those types of messages
		appparams.AccGov.String(),
	)

<<<<<<< HEAD
=======
	ak.EVMTransferKeeper = evmtransferkeeper.NewKeeper(
		appCodec, runtime.NewKVStoreService(keys[evmtypes.ModuleName]),
		ak.GetSubspace(ibctransfertypes.ModuleName),
		ak.IBCKeeper.ChannelKeeper, // ICS4Wrapper
		ak.IBCKeeper.ChannelKeeper,
		bApp.MsgServiceRouter(), ak.AccountKeeper, ak.BankKeeper,
		ak.Erc20Keeper, // Add ERC20 Keeper for ERC20 transfers
		appparams.AccGov.String(),
	)

	ak.EVMKeeper.WithStaticPrecompiles(
		NewAvailableStaticPrecompiles(
			appCodec,
			ak.PreciseBankKeeper,
			ak.Erc20Keeper,
			ak.GovKeeper,
			ak.SlashingKeeper,
			ak.EvidenceKeeper,
		),
	)
	// Create the TokenFactory Keeper
	ak.TokenFactoryKeeper = tokenfactorykeeper.NewKeeper(
		appCodec,
		ak.keys[tokenfactorytypes.StoreKey],
		maccPerms,
		ak.AccountKeeper,
		ak.BankKeeper,
		ak.DistrKeeper,
		tokenFactoryCapabilities,
		tokenfactorykeeper.DefaultIsSudoAdminFunc,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	// Register custom plugins for the wasm module
	wasmOpts = append(
		owasm.RegisterCustomPlugins(
			&ak.TokenFactoryKeeper,
			&epochingKeeper,
			&ak.CheckpointingKeeper,
			&ak.BTCLightClientKeeper,
			&ak.ZoneConciergeKeeper,
		),
		wasmOpts...,
	)

	// Register custom plugins for the tokenfactory module
	wasmOpts = append(
		tokenfactorybindings.RegisterCustomPlugins(
			ak.BankKeeper,
			&ak.TokenFactoryKeeper,
		),
		wasmOpts...,
	)

	// Register gRPC queries for the wasm module
	wasmOpts = append(
		owasm.RegisterGrpcQueries(*bApp.GRPCQueryRouter(), appCodec),
		wasmOpts...,
	)

	// Register message handlers for the wasm module
	wasmOpts = append(
		owasm.RegisterMessageHandler(
			&ak.FinalityKeeper,
		),
		wasmOpts...,
	)

	ak.WasmKeeper = wasmkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[wasmtypes.StoreKey]),
		ak.AccountKeeper,
		ak.BankKeeper,
		ak.StakingKeeper,
		distrkeeper.NewQuerier(ak.DistrKeeper),
		ak.IBCKeeper.ChannelKeeper,
		ak.IBCKeeper.ChannelKeeper,
		ak.TransferKeeper,
		bApp.MsgServiceRouter(),
		bApp.GRPCQueryRouter(),
		homePath,
		wasmConfig,
		wasmtypes.VMConfig{},
		WasmCapabilities(),
		appparams.AccGov.String(),
		wasmOpts...,
	)

>>>>>>> 09820f4 (add custom tokenfactory bindings (#1398))
	// register the proposal types
	// Deprecated: Avoid adding new handlers, instead use the new proposal flow
	// by granting the governance module the right to execute the message.
	// See: https://github.com/cosmos/cosmos-sdk/blob/release/v0.46.x/x/gov/spec/01_concepts.md#proposal-messages
	// TODO: investigate how to migrate to new proposal flow
	govRouter := govv1beta1.NewRouter()
	govRouter.AddRoute(govtypes.RouterKey, govv1beta1.ProposalHandler).
		AddRoute(paramproposal.RouterKey, params.NewParamChangeProposalHandler(ak.ParamsKeeper))

	// TODO: this should be a function parameter
	govConfig := govtypes.DefaultConfig()
	/*
		Example of setting gov params:
		govConfig.MaxMetadataLen = 10000
	*/
	govKeeper := govkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[govtypes.StoreKey]),
		ak.AccountKeeper,
		ak.BankKeeper,
		ak.StakingKeeper,
		ak.DistrKeeper,
		bApp.MsgServiceRouter(),
		govConfig,
		appparams.AccGov.String())

	ak.GovKeeper = *govKeeper.SetHooks(
		govtypes.NewMultiGovHooks(
		// register the governance hooks
		),
	)

	btclightclientKeeper := btclightclientkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[btclightclienttypes.StoreKey]),
		*btcConfig,
		&ak.IncentiveKeeper,
		appparams.AccGov.String(),
	)

	btcCheckpointKeeper := btccheckpointkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[btccheckpointtypes.StoreKey]),
		ak.tkeys[btccheckpointtypes.TStoreKey],
		&btclightclientKeeper,
		&checkpointingKeeper,
		&ak.IncentiveKeeper,
		&powLimit,
		appparams.AccGov.String(),
	)

	ak.RatelimitKeeper = *ratelimitkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[ratelimittypes.StoreKey]),
		ak.GetSubspace(ratelimittypes.ModuleName),
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		ak.BankKeeper,
		ak.IBCKeeper.ChannelKeeper,
		ak.IBCKeeper.ChannelKeeper, // ICS4Wrapper
	)

	// Create Transfer Keepers
	ak.TransferKeeper = ibctransferkeeper.NewKeeper(
		appCodec,
		keys[ibctransfertypes.StoreKey],
		ak.GetSubspace(ibctransfertypes.ModuleName),
		ak.IBCKeeper.ChannelKeeper,
		ak.IBCKeeper.ChannelKeeper,
		ak.IBCKeeper.PortKeeper,
		ak.AccountKeeper,
		ak.BankKeeper,
		scopedTransferKeeper,
		appparams.AccGov.String(),
	)

	ak.PFMRouterKeeper = pfmrouterkeeper.NewKeeper(
		appCodec,
		ak.keys[pfmroutertypes.StoreKey],
		ak.TransferKeeper,
		ak.IBCKeeper.ChannelKeeper,
		ak.BankKeeper,
		ak.RatelimitKeeper,
		appparams.AccGov.String(),
	)

	ak.MonitorKeeper = monitorkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[monitortypes.StoreKey]),
		&btclightclientKeeper,
	)

	// make ZoneConcierge and Monitor to subscribe to the epoching's hooks
	ak.EpochingKeeper = *epochingKeeper.SetHooks(
		epochingtypes.NewMultiEpochingHooks(ak.MonitorKeeper.Hooks()),
	)

	// set up Checkpointing, BTCCheckpoint, and BTCLightclient keepers
	ak.CheckpointingKeeper = *checkpointingKeeper.SetHooks(
		checkpointingtypes.NewMultiCheckpointingHooks(ak.EpochingKeeper.Hooks(), ak.MonitorKeeper.Hooks()),
	)
	ak.BtcCheckpointKeeper = btcCheckpointKeeper

	// set up BTC staking keeper
	ak.BTCStakingKeeper = btcstakingkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[btcstakingtypes.StoreKey]),
		&btclightclientKeeper,
		&btcCheckpointKeeper,
		&ak.IncentiveKeeper,
		btcNetParams,
		appparams.AccGov.String(),
	)

	ak.BTCLightClientKeeper = *btclightclientKeeper.SetHooks(
		btclightclienttypes.NewMultiBTCLightClientHooks(ak.BtcCheckpointKeeper.Hooks(), ak.BTCStakingKeeper.Hooks()),
	)

	// set up finality keeper
	ak.FinalityKeeper = finalitykeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[finalitytypes.StoreKey]),
		ak.BTCStakingKeeper,
		ak.IncentiveKeeper,
		ak.CheckpointingKeeper,
		appparams.AccGov.String(),
	)

	// create evidence keeper with router
	evidenceKeeper := evidencekeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[evidencetypes.StoreKey]),
		ak.StakingKeeper,
		ak.SlashingKeeper,
		ak.AccountKeeper.AddressCodec(),
		runtime.ProvideCometInfoService(),
	)
	// If evidence needs to be handled for the app, set routes in router here and seal
	ak.EvidenceKeeper = *evidenceKeeper

	// Create the TokenFactory Keeper
	ak.TokenFactoryKeeper = tokenfactorykeeper.NewKeeper(
		appCodec,
		ak.keys[tokenfactorytypes.StoreKey],
		maccPerms,
		ak.AccountKeeper,
		ak.BankKeeper,
		ak.DistrKeeper,
		tokenFactoryCapabilities,
		tokenfactorykeeper.DefaultIsSudoAdminFunc,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	wasmOpts = append(owasm.RegisterCustomPlugins(&ak.TokenFactoryKeeper, &epochingKeeper, &ak.CheckpointingKeeper, &ak.BTCLightClientKeeper), wasmOpts...)

	ak.WasmKeeper = wasmkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[wasmtypes.StoreKey]),
		ak.AccountKeeper,
		ak.BankKeeper,
		ak.StakingKeeper,
		distrkeeper.NewQuerier(ak.DistrKeeper),
		ak.IBCKeeper.ChannelKeeper,
		ak.IBCKeeper.ChannelKeeper,
		ak.IBCKeeper.PortKeeper,
		scopedWasmKeeper,
		ak.TransferKeeper,
		bApp.MsgServiceRouter(),
		bApp.GRPCQueryRouter(),
		homePath,
		wasmConfig,
		wasmtypes.VMConfig{},
		WasmCapabilities(),
		appparams.AccGov.String(),
		wasmOpts...,
	)

	ibcWasmConfig :=
		ibcwasmtypes.WasmConfig{
			DataDir:               filepath.Join(homePath, "ibc_08-wasm"),
			SupportedCapabilities: WasmCapabilities(),
			ContractDebugMode:     false,
		}

	ak.IBCWasmKeeper = ibcwasmkeeper.NewKeeperWithConfig(
		appCodec,
		runtime.NewKVStoreService(keys[ibcwasmtypes.StoreKey]),
		ak.IBCKeeper.ClientKeeper,
		appparams.AccGov.String(),
		ibcWasmConfig,
		bApp.GRPCQueryRouter(),
	)

	// Set legacy router for backwards compatibility with gov v1beta1
	ak.GovKeeper.SetLegacyRouter(govRouter)

	icaHostKeeper := icahostkeeper.NewKeeper(
		appCodec, ak.keys[icahosttypes.StoreKey],
		ak.GetSubspace(icahosttypes.SubModuleName),
		ak.IBCKeeper.ChannelKeeper,
		ak.IBCKeeper.ChannelKeeper,
		ak.IBCKeeper.PortKeeper,
		ak.AccountKeeper,
		scopedICAHostKeeper,
		bApp.MsgServiceRouter(),
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	icaHostKeeper.WithQueryRouter(bApp.GRPCQueryRouter())
	ak.ICAHostKeeper = &icaHostKeeper

	icaControllerKeeper := icacontrollerkeeper.NewKeeper(
		appCodec, ak.keys[icacontrollertypes.StoreKey],
		ak.GetSubspace(icacontrollertypes.SubModuleName),
		ak.IBCKeeper.ChannelKeeper,
		ak.IBCKeeper.ChannelKeeper,
		ak.IBCKeeper.PortKeeper,
		scopedICAControllerKeeper,
		bApp.MsgServiceRouter(),
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	ak.ICAControllerKeeper = &icaControllerKeeper

	// Create all supported IBC routes
	var wasmStack porttypes.IBCModule
	wasmStackIBCHandler := wasm.NewIBCHandler(ak.WasmKeeper, ak.IBCKeeper.ChannelKeeper, ak.IBCKeeper.ChannelKeeper)

	// Create Transfer Stack (from bottom to top of stack)
	// - core IBC
	// - ratelimit
	// - PFM (Packet Forwarding Middleware)
	// - callbacks
	// - transfer

	// Create Transfer Stack
	// SendPacket Path:
	// SendPacket -> Transfer -> Callbacks -> PFM -> RateLimit -> IBC core (ICS4Wrapper)
	// RecvPacket Path:
	// RecvPacket -> IBC core -> RateLimit -> PFM -> Callbacks -> Transfer (AddRoute)
	// Receive path should mirror the send path.

	var transferStack porttypes.IBCModule
	transferStack = transfer.NewIBCModule(ak.TransferKeeper)

	cbStack := ibccallbacks.NewIBCMiddleware(transferStack, ak.PFMRouterKeeper, wasmStackIBCHandler, appparams.MaxIBCCallbackGas)
	transferStack = pfmrouter.NewIBCMiddleware(
		cbStack,
		ak.PFMRouterKeeper,
		0, // retries on timeout
		pfmrouterkeeper.DefaultForwardTransferPacketTimeoutTimestamp,
	)
	transferStack = ratelimiter.NewIBCMiddleware(ak.RatelimitKeeper, transferStack)
	ak.TransferKeeper.WithICS4Wrapper(cbStack)

	// Create Interchain Accounts Controller Stack
	// SendPacket Path:
	// SendPacket -> Callbacks -> ICA Controller -> IBC core
	// RecvPacket Path:
	// RecvPacket -> IBC core -> ICA Controller -> Callbacks
	var icaControllerStack porttypes.IBCModule
	icaControllerStack = icacontroller.NewIBCMiddleware(icaControllerStack, *ak.ICAControllerKeeper)
	icaControllerStack = ibccallbacks.NewIBCMiddleware(icaControllerStack, ak.IBCKeeper.ChannelKeeper,
		wasmStackIBCHandler, appparams.MaxIBCCallbackGas)
	icaICS4Wrapper := icaControllerStack.(porttypes.ICS4Wrapper)
	ak.ICAControllerKeeper.WithICS4Wrapper(icaICS4Wrapper)

	// ICA Host stack
	// RecvPacket, message that originates from core IBC and goes down to app, the flow is:
	// channel.RecvPacket -> fee.OnRecvPacket -> icaHost.OnRecvPacket
	icaHostStack := icahost.NewIBCModule(*ak.ICAHostKeeper)

	// Create static IBC router, add ibc-transfer module route,
	// and the other routes (ICA, wasm, zoneconcierge), then set and seal it
	ibcRouter := porttypes.NewRouter().
		AddRoute(ibctransfertypes.ModuleName, transferStack).
		AddRoute(wasmtypes.ModuleName, wasmStack).
		AddRoute(icacontrollertypes.SubModuleName, icaControllerStack).
		AddRoute(icahosttypes.SubModuleName, icaHostStack)

	// Setting Router will finalize all routes by sealing router
	// No more routes can be added
	ak.IBCKeeper.SetRouter(ibcRouter)
}

// initParamsKeeper init params keeper and its subspaces
func initParamsKeeper(appCodec codec.BinaryCodec, legacyAmino *codec.LegacyAmino, key, tkey storetypes.StoreKey) paramskeeper.Keeper {
	paramsKeeper := paramskeeper.NewKeeper(appCodec, legacyAmino, key, tkey)

	// TODO: Only modules which did not migrate yet to new way of hanldling params
	// are the IBC-related modules. Once they are migrated, we can remove this and
	// whole usage of params module
	paramsKeeper.Subspace(ibcexported.ModuleName)
	paramsKeeper.Subspace(ibctransfertypes.ModuleName)
	paramsKeeper.Subspace(tokenfactorytypes.ModuleName)
	paramsKeeper.Subspace(ratelimittypes.ModuleName)

	return paramsKeeper
}
