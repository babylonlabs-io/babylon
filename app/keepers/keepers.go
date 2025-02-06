package keepers

import (
	"fmt"
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
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
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
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	ibcwasmkeeper "github.com/cosmos/ibc-go/modules/light-clients/08-wasm/keeper"
	ibcwasmtypes "github.com/cosmos/ibc-go/modules/light-clients/08-wasm/types"
	ibcfee "github.com/cosmos/ibc-go/v8/modules/apps/29-fee"
	ibcfeekeeper "github.com/cosmos/ibc-go/v8/modules/apps/29-fee/keeper"
	ibcfeetypes "github.com/cosmos/ibc-go/v8/modules/apps/29-fee/types"
	"github.com/cosmos/ibc-go/v8/modules/apps/transfer"
	ibctransferkeeper "github.com/cosmos/ibc-go/v8/modules/apps/transfer/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	porttypes "github.com/cosmos/ibc-go/v8/modules/core/05-port/types" // ibc module puts types under `ibchost` rather than `ibctypes`
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"
	ibckeeper "github.com/cosmos/ibc-go/v8/modules/core/keeper"

	appparams "github.com/babylonlabs-io/babylon/app/params"
	bbn "github.com/babylonlabs-io/babylon/types"
	owasm "github.com/babylonlabs-io/babylon/wasmbinding"
	btccheckpointkeeper "github.com/babylonlabs-io/babylon/x/btccheckpoint/keeper"
	btccheckpointtypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	btclightclientkeeper "github.com/babylonlabs-io/babylon/x/btclightclient/keeper"
	btclightclienttypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	btcstakingkeeper "github.com/babylonlabs-io/babylon/x/btcstaking/keeper"
	btcstakingtypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	bsckeeper "github.com/babylonlabs-io/babylon/x/btcstkconsumer/keeper"
	bsctypes "github.com/babylonlabs-io/babylon/x/btcstkconsumer/types"
	checkpointingkeeper "github.com/babylonlabs-io/babylon/x/checkpointing/keeper"
	checkpointingtypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
	epochingkeeper "github.com/babylonlabs-io/babylon/x/epoching/keeper"
	epochingtypes "github.com/babylonlabs-io/babylon/x/epoching/types"
	finalitykeeper "github.com/babylonlabs-io/babylon/x/finality/keeper"
	finalitytypes "github.com/babylonlabs-io/babylon/x/finality/types"
	incentivekeeper "github.com/babylonlabs-io/babylon/x/incentive/keeper"
	incentivetypes "github.com/babylonlabs-io/babylon/x/incentive/types"
	mintkeeper "github.com/babylonlabs-io/babylon/x/mint/keeper"
	minttypes "github.com/babylonlabs-io/babylon/x/mint/types"
	monitorkeeper "github.com/babylonlabs-io/babylon/x/monitor/keeper"
	monitortypes "github.com/babylonlabs-io/babylon/x/monitor/types"
	"github.com/babylonlabs-io/babylon/x/zoneconcierge"
	zckeeper "github.com/babylonlabs-io/babylon/x/zoneconcierge/keeper"
	zctypes "github.com/babylonlabs-io/babylon/x/zoneconcierge/types"
)

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
	CrisisKeeper          *crisiskeeper.Keeper
	UpgradeKeeper         *upgradekeeper.Keeper
	ParamsKeeper          paramskeeper.Keeper
	AuthzKeeper           authzkeeper.Keeper
	EvidenceKeeper        evidencekeeper.Keeper
	FeeGrantKeeper        feegrantkeeper.Keeper
	ConsensusParamsKeeper consensusparamkeeper.Keeper
	CircuitKeeper         circuitkeeper.Keeper

	// Babylon modules
	EpochingKeeper       epochingkeeper.Keeper
	BTCLightClientKeeper btclightclientkeeper.Keeper
	BtcCheckpointKeeper  btccheckpointkeeper.Keeper
	CheckpointingKeeper  checkpointingkeeper.Keeper
	MonitorKeeper        monitorkeeper.Keeper

	// IBC-related modules
	IBCKeeper      *ibckeeper.Keeper        // IBC Keeper must be a pointer in the app, so we can SetRouter on it correctly
	IBCFeeKeeper   ibcfeekeeper.Keeper      // for relayer incentivization - https://github.com/cosmos/ibc/tree/main/spec/app/ics-029-fee-payment
	TransferKeeper ibctransferkeeper.Keeper // for cross-chain fungible token transfers
	IBCWasmKeeper  ibcwasmkeeper.Keeper     // for IBC wasm light clients

	// Integration-related modules
	BTCStkConsumerKeeper bsckeeper.Keeper
	ZoneConciergeKeeper  zckeeper.Keeper

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
	wasmConfig wasmtypes.WasmConfig,
	wasmOpts []wasmkeeper.Option,
	blockedAddress map[string]bool,
) {
	powLimit := btcConfig.PowLimit()
	btcNetParams := btcConfig.NetParams()

	ak.EncCfg = encodingConfig
	appCodec := encodingConfig.Codec

	// set persistent store keys
	keys := storetypes.NewKVStoreKeys(
		authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey, crisistypes.StoreKey,
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
		ibcfeetypes.StoreKey,
		ibcwasmtypes.StoreKey,
		// Integration related modules
		bsctypes.ModuleName,
		zctypes.ModuleName,
		// BTC staking related modules
		btcstakingtypes.StoreKey,
		finalitytypes.StoreKey,
		// WASM
		wasmtypes.StoreKey,
		// tokenomics-related modules
		incentivetypes.StoreKey,
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
	scopedZoneConciergeKeeper := ak.CapabilityKeeper.ScopeToModule(zctypes.ModuleName)
	scopedWasmKeeper := ak.CapabilityKeeper.ScopeToModule(wasmtypes.ModuleName)

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

	ak.CrisisKeeper = crisiskeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[crisistypes.StoreKey]),
		invCheckPeriod,
		ak.BankKeeper,
		authtypes.FeeCollectorName,
		appparams.AccGov.String(),
		ak.AccountKeeper.AddressCodec(),
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

	wasmOpts = append(owasm.RegisterCustomPlugins(&epochingKeeper, &ak.CheckpointingKeeper, &ak.BTCLightClientKeeper, &ak.ZoneConciergeKeeper), wasmOpts...)
	wasmOpts = append(owasm.RegisterGrpcQueries(*bApp.GRPCQueryRouter(), appCodec), wasmOpts...)

	ak.WasmKeeper = wasmkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[wasmtypes.StoreKey]),
		ak.AccountKeeper,
		ak.BankKeeper,
		ak.StakingKeeper,
		distrkeeper.NewQuerier(ak.DistrKeeper),
		ak.IBCFeeKeeper,
		ak.IBCKeeper.ChannelKeeper,
		ak.IBCKeeper.PortKeeper,
		scopedWasmKeeper,
		ak.TransferKeeper,
		bApp.MsgServiceRouter(),
		bApp.GRPCQueryRouter(),
		homePath,
		wasmConfig,
		WasmCapabilities(),
		appparams.AccGov.String(),
		wasmOpts...,
	)

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

	ak.IBCFeeKeeper = ibcfeekeeper.NewKeeper(
		appCodec, keys[ibcfeetypes.StoreKey],
		ak.IBCKeeper.ChannelKeeper, // may be replaced with IBC middleware
		ak.IBCKeeper.ChannelKeeper,
		ak.IBCKeeper.PortKeeper, ak.AccountKeeper, ak.BankKeeper,
	)

	// Create Transfer Keepers
	ak.TransferKeeper = ibctransferkeeper.NewKeeper(
		appCodec,
		keys[ibctransfertypes.StoreKey],
		ak.GetSubspace(ibctransfertypes.ModuleName),
		ak.IBCFeeKeeper,
		ak.IBCKeeper.ChannelKeeper,
		ak.IBCKeeper.PortKeeper,
		ak.AccountKeeper,
		ak.BankKeeper,
		scopedTransferKeeper,
		appparams.AccGov.String(),
	)

	ak.BTCStkConsumerKeeper = bsckeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[bsctypes.StoreKey]),
		ak.AccountKeeper,
		ak.BankKeeper,
		ak.IBCKeeper.ClientKeeper,
		ak.WasmKeeper,
		appparams.AccGov.String(),
	)

	// set up BTC staking keeper
	ak.BTCStakingKeeper = btcstakingkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[btcstakingtypes.StoreKey]),
		&btclightclientKeeper,
		&btcCheckpointKeeper,
		&ak.BTCStkConsumerKeeper,
		&ak.IncentiveKeeper,
		btcNetParams,
		appparams.AccGov.String(),
	)

	// set up finality keeper
	ak.FinalityKeeper = finalitykeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[finalitytypes.StoreKey]),
		ak.BTCStakingKeeper,
		ak.IncentiveKeeper,
		&checkpointingKeeper,
		appparams.AccGov.String(),
	)

	monitorKeeper := monitorkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[monitortypes.StoreKey]),
		&btclightclientKeeper,
	)

	// create querier for KVStore
	storeQuerier, ok := bApp.CommitMultiStore().(storetypes.Queryable)
	if !ok {
		panic(fmt.Errorf("multistore doesn't support queries"))
	}

	zcKeeper := zckeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[zctypes.StoreKey]),
		ak.IBCFeeKeeper,
		ak.IBCKeeper.ClientKeeper,
		ak.IBCKeeper.ConnectionKeeper,
		ak.IBCKeeper.ChannelKeeper,
		ak.IBCKeeper.PortKeeper,
		ak.AccountKeeper,
		ak.BankKeeper,
		&btclightclientKeeper,
		&checkpointingKeeper,
		&btcCheckpointKeeper,
		epochingKeeper,
		storeQuerier,
		&ak.BTCStakingKeeper,
		&ak.BTCStkConsumerKeeper,
		scopedZoneConciergeKeeper,
		appparams.AccGov.String(),
	)

	// make ZoneConcierge and Monitor to subscribe to the epoching's hooks
	epochingKeeper.SetHooks(
		epochingtypes.NewMultiEpochingHooks(zcKeeper.Hooks(), monitorKeeper.Hooks()),
	)
	// set up Checkpointing, BTCCheckpoint, and BTCLightclient keepers
	checkpointingKeeper.SetHooks(
		checkpointingtypes.NewMultiCheckpointingHooks(epochingKeeper.Hooks(), zcKeeper.Hooks(), monitorKeeper.Hooks()),
	)
	btclightclientKeeper.SetHooks(
		btclightclienttypes.NewMultiBTCLightClientHooks(btcCheckpointKeeper.Hooks()),
	)

	// wire the keepers with hooks to the app
	ak.EpochingKeeper = epochingKeeper
	ak.BTCLightClientKeeper = btclightclientKeeper
	ak.CheckpointingKeeper = checkpointingKeeper
	ak.BtcCheckpointKeeper = btcCheckpointKeeper
	ak.MonitorKeeper = monitorKeeper
	ak.ZoneConciergeKeeper = *zcKeeper

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

	// Create all supported IBC routes
	var transferStack porttypes.IBCModule
	transferStack = transfer.NewIBCModule(ak.TransferKeeper)
	transferStack = ibcfee.NewIBCMiddleware(transferStack, ak.IBCFeeKeeper)

	var zoneConciergeStack porttypes.IBCModule
	zoneConciergeStack = zoneconcierge.NewIBCModule(ak.ZoneConciergeKeeper)
	zoneConciergeStack = ibcfee.NewIBCMiddleware(zoneConciergeStack, ak.IBCFeeKeeper)

	var wasmStack porttypes.IBCModule
	wasmStack = wasm.NewIBCHandler(ak.WasmKeeper, ak.IBCKeeper.ChannelKeeper, ak.IBCFeeKeeper)
	wasmStack = ibcfee.NewIBCMiddleware(wasmStack, ak.IBCFeeKeeper)

	// Create static IBC router, add ibc-transfer module route, then set and seal it
	ibcRouter := porttypes.NewRouter().
		AddRoute(ibctransfertypes.ModuleName, transferStack).
		AddRoute(zctypes.ModuleName, zoneConciergeStack).
		AddRoute(wasmtypes.ModuleName, wasmStack)

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

	return paramsKeeper
}
