package app

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	pruningtypes "cosmossdk.io/store/pruning/types"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/ed25519"
	tmjson "github.com/cometbft/cometbft/libs/json"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	cosmosed "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/server/types"
	simsutils "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	appsigner "github.com/babylonlabs-io/babylon/v4/app/signer"
	"github.com/babylonlabs-io/babylon/v4/testutil/signer"
	minttypes "github.com/babylonlabs-io/babylon/v4/x/mint/types"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/crypto/bls12381"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btclighttypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	checkpointingtypes "github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
)

// SetupOptions defines arguments that are passed into `Simapp` constructor.
type SetupOptions struct {
	Logger             log.Logger
	DB                 *dbm.MemDB
	InvCheckPeriod     uint
	SkipUpgradeHeights map[int64]bool
	AppOpts            types.AppOptions
}

func setup(t *testing.T, blsSigner checkpointingtypes.BlsSigner, withGenesis bool, invCheckPeriod uint, btcConf bbn.SupportedBtcNetwork) (*BabylonApp, GenesisState) {
	db := dbm.NewMemDB()
	nodeHome := t.TempDir()

	appOptions := make(simsutils.AppOptionsMap, 0)
	appOptions[flags.FlagHome] = nodeHome // ensure unique folder
	appOptions[server.FlagInvCheckPeriod] = invCheckPeriod
	appOptions["btc-config.network"] = string(btcConf)
	appOptions[server.FlagPruning] = pruningtypes.PruningOptionDefault
	appOptions[server.FlagMempoolMaxTxs] = mempool.DefaultMaxTx
	appOptions[flags.FlagChainID] = "chain-test"
	baseAppOpts := server.DefaultBaseappOptions(appOptions)

	app := NewBabylonApp(
		log.NewNopLogger(),
		db,
		nil,
		true,
		map[int64]bool{},
		invCheckPeriod,
		&blsSigner,
		appOptions,
		EVMChainID,
		EVMAppOptions,
		EmptyWasmOpts,
		baseAppOpts...,
	)
	if withGenesis {
		return app, app.DefaultGenesis()
	}
	return app, GenesisState{}
}

// NewBabylonAppWithCustomOptions initializes a new BabylonApp with custom options.
// Created Babylon application will have one validator with hardcoed amount of tokens.
// This is necessary as from cosmos-sdk 0.46 it is required that there is at least
// one validator in validator set during InitGenesis abci call - https://github.com/cosmos/cosmos-sdk/pull/9697
func NewBabylonAppWithCustomOptions(t *testing.T, isCheckTx bool, blsSigner checkpointingtypes.BlsSigner, options SetupOptions) *BabylonApp {
	t.Helper()
	// create validator set with single validator
	valKeys, err := appsigner.NewValidatorKeys(ed25519.GenPrivKey(), bls12381.GenPrivKey())
	require.NoError(t, err)
	valPubkey, err := cryptocodec.FromCmtPubKeyInterface(valKeys.ValPubkey)
	require.NoError(t, err)
	genesisKey, err := checkpointingtypes.NewGenesisKey(
		sdk.ValAddress(valKeys.ValPubkey.Address()),
		&valKeys.BlsPubkey,
		valKeys.PoP,
		&cosmosed.PubKey{Key: valPubkey.Bytes()},
	)
	require.NoError(t, err)
	genesisValSet := []*checkpointingtypes.GenesisKey{genesisKey}

	acc := authtypes.NewBaseAccount(valPubkey.Address().Bytes(), valPubkey, 0, 0)
	balance := banktypes.Balance{
		Address: acc.GetAddress().String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, math.NewInt(100000000000000))),
	}

	app := NewBabylonApp(
		options.Logger,
		options.DB,
		nil,
		true,
		options.SkipUpgradeHeights,
		options.InvCheckPeriod,
		&blsSigner,
		options.AppOpts,
		EVMChainID,
		EVMAppOptions,
		EmptyWasmOpts,
	)
	genesisState := app.DefaultGenesis()
	genesisState = genesisStateWithValSet(t, app, genesisState, genesisValSet, []authtypes.GenesisAccount{acc}, balance)

	if !isCheckTx {
		// init chain must be called to stop deliverState from being nil
		stateBytes, err := tmjson.MarshalIndent(genesisState, "", " ")
		require.NoError(t, err)

		// Initialize the chain
		consensusParams := simsutils.DefaultConsensusParams
		initialHeight := app.LastBlockHeight() + 1
		consensusParams.Abci = &cmtproto.ABCIParams{VoteExtensionsEnableHeight: initialHeight}
		_, err = app.InitChain(
			&abci.RequestInitChain{
				Validators:      []abci.ValidatorUpdate{},
				ConsensusParams: consensusParams,
				AppStateBytes:   stateBytes,
				InitialHeight:   initialHeight,
			},
		)
		require.NoError(t, err)
	}

	return app
}

func genesisStateWithValSet(t *testing.T,
	app *BabylonApp, genesisState GenesisState,
	valSet []*checkpointingtypes.GenesisKey, genAccs []authtypes.GenesisAccount,
	balances ...banktypes.Balance,
) GenesisState {
	// set genesis accounts
	authGenesis := authtypes.NewGenesisState(authtypes.DefaultParams(), genAccs)
	genesisState[authtypes.ModuleName] = app.AppCodec().MustMarshalJSON(authGenesis)

	validators := make([]stakingtypes.Validator, 0, len(valSet))
	delegations := make([]stakingtypes.Delegation, 0, len(valSet))

	bondAmt := sdk.DefaultPowerReduction.MulRaw(1000)

	for _, valGenKey := range valSet {
		pkAny, err := codectypes.NewAnyWithValue(valGenKey.ValPubkey)
		require.NoError(t, err)
		validator := stakingtypes.Validator{
			OperatorAddress:   valGenKey.ValidatorAddress,
			ConsensusPubkey:   pkAny,
			Jailed:            false,
			Status:            stakingtypes.Bonded,
			Tokens:            bondAmt,
			DelegatorShares:   math.LegacyOneDec(),
			Description:       stakingtypes.Description{},
			UnbondingHeight:   int64(0),
			UnbondingTime:     time.Unix(0, 0).UTC(),
			Commission:        stakingtypes.NewCommission(math.LegacyZeroDec(), math.LegacyNewDec(100), math.LegacyNewDec(2)),
			MinSelfDelegation: math.ZeroInt(),
		}

		validators = append(validators, validator)
		delegations = append(delegations, stakingtypes.NewDelegation(genAccs[0].GetAddress().String(), valGenKey.ValidatorAddress, math.LegacyOneDec()))
		// blsKeys = append(blsKeys, checkpointingtypes.NewGenesisKey(sdk.ValAddress(val.Address), genesisBLSPubkey))
	}
	// total bond amount = bond amount * number of validators
	require.Equal(t, len(validators), len(delegations))
	totalBondAmt := bondAmt.MulRaw(int64(len(validators)))

	// set validators and delegations
	stakingGenesis := stakingtypes.NewGenesisState(stakingtypes.DefaultParams(), validators, delegations)
	stakingGenesis.Params.BondDenom = appparams.DefaultBondDenom
	genesisState[stakingtypes.ModuleName] = app.AppCodec().MustMarshalJSON(stakingGenesis)

	checkpointingGenesis := &checkpointingtypes.GenesisState{
		GenesisKeys: valSet,
	}
	genesisState[checkpointingtypes.ModuleName] = app.AppCodec().MustMarshalJSON(checkpointingGenesis)

	totalSupply := sdk.NewCoins()
	for _, b := range balances {
		// add genesis acc tokens to total supply
		totalSupply = totalSupply.Add(b.Coins...)
	}
	for range delegations {
		// add delegated tokens to total supply
		totalSupply = totalSupply.Add(sdk.NewCoin(appparams.DefaultBondDenom, bondAmt))
	}

	// add bonded amount to bonded pool module account
	balances = append(balances, banktypes.Balance{
		Address: authtypes.NewModuleAddress(stakingtypes.BondedPoolName).String(),
		Coins:   sdk.Coins{sdk.NewCoin(appparams.DefaultBondDenom, totalBondAmt)},
	})

	// update total supply
	bankGenesis := banktypes.NewGenesisState(
		banktypes.DefaultGenesisState().Params,
		balances,
		totalSupply,
		[]banktypes.Metadata{},
		[]banktypes.SendEnabled{},
	)
	genesisState[banktypes.ModuleName] = app.AppCodec().MustMarshalJSON(bankGenesis)

	return genesisState
}

// Setup initializes a new BabylonApp. A Nop logger is set in BabylonApp.
// Created Babylon application will have one validator with hardoced amount of tokens.
// This is necessary as from cosmos-sdk 0.46 it is required that there is at least
// one validator in validator set during InitGenesis abci call - https://github.com/cosmos/cosmos-sdk/pull/9697
func Setup(t *testing.T, isCheckTx bool) *BabylonApp {
	t.Helper()
	return SetupWithBitcoinConf(t, isCheckTx, bbn.BtcSimnet)
}

// SetupWithBitcoinConf initializes a new BabylonApp with a specific btc network config.
// A Nop logger is set in BabylonApp. Created Babylon application will have one validator
// with hardoced amount of tokens. This is necessary as from cosmos-sdk 0.46 it is required
// that there is at least one validator in validator set during InitGenesis abci call
// https://github.com/cosmos/cosmos-sdk/pull/9697
func SetupWithBitcoinConf(t *testing.T, isCheckTx bool, btcConf bbn.SupportedBtcNetwork) *BabylonApp {
	t.Helper()

	tbs, err := signer.SetupTestBlsSigner()
	require.NoError(t, err)
	blsSigner := checkpointingtypes.BlsSigner(tbs)

	cmtPrivKey := ed25519.GenPrivKey()

	// generate genesis account
	acc := authtypes.NewBaseAccount(
		cmtPrivKey.PubKey().Address().Bytes(),
		&cosmosed.PubKey{Key: cmtPrivKey.PubKey().Bytes()},
		0,
		0,
	)
	balance := banktypes.Balance{
		Address: acc.GetAddress().String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, math.NewInt(100000000000000))),
	}
	// create validator set with single validator
	genesisKey, err := signer.GenesisKeyFromPrivSigner(cmtPrivKey, tbs.PrivKey, sdk.ValAddress(acc.GetAddress()))
	require.NoError(t, err)
	genesisValSet := []*checkpointingtypes.GenesisKey{genesisKey}

	app := SetupWithGenesisValSet(t, btcConf, genesisValSet, blsSigner, []authtypes.GenesisAccount{acc}, balance)

	return app
}

// SetupWithGenesisValSet initializes a new BabylonApp with a validator set and genesis accounts
// that also act as delegators. For simplicity, each validator is bonded with a delegation
// of one consensus engine unit (10^6) in the default token of the babylon app from first genesis
// account. A Nop logger is set in BabylonApp.
// Note that the privSigner should be the 0th item of valSet
func SetupWithGenesisValSet(t *testing.T, btcConf bbn.SupportedBtcNetwork, valSet []*checkpointingtypes.GenesisKey, blsSigner checkpointingtypes.BlsSigner, genAccs []authtypes.GenesisAccount, balances ...banktypes.Balance) *BabylonApp {
	t.Helper()
	app, genesisState := setup(t, blsSigner, true, 5, btcConf)
	genesisState = genesisStateWithValSet(t, app, genesisState, valSet, genAccs, balances...)

	stateBytes, err := json.MarshalIndent(genesisState, "", " ")
	require.NoError(t, err)

	// init chain will set the validator set and initialize the genesis accounts
	consensusParams := simsutils.DefaultConsensusParams
	consensusParams.Block.MaxGas = 100 * simsutils.DefaultGenTxGas
	// it is required that the VoteExtensionsEnableHeight > 0 to enable vote extension
	initialHeight := app.LastBlockHeight() + 1
	consensusParams.Abci = &cmtproto.ABCIParams{VoteExtensionsEnableHeight: initialHeight}
	_, err = app.InitChain(&abci.RequestInitChain{
		ChainId:         app.ChainID(),
		Time:            time.Now().UTC(),
		Validators:      []abci.ValidatorUpdate{},
		ConsensusParams: consensusParams,
		InitialHeight:   initialHeight,
		AppStateBytes:   stateBytes,
	})
	require.NoError(t, err)

	_, err = app.FinalizeBlock(&abci.RequestFinalizeBlock{
		Height: initialHeight,
		Hash:   app.LastCommitID().Hash,
	})
	require.NoError(t, err)

	return app
}

// createRandomAccounts is a strategy used by addTestAddrs() in order to generated addresses in random order.
func createRandomAccounts(accNum int) []sdk.AccAddress {
	testAddrs := make([]sdk.AccAddress, accNum)
	for i := 0; i < accNum; i++ {
		pk := ed25519.GenPrivKey().PubKey()
		testAddrs[i] = sdk.AccAddress(pk.Address())
	}

	return testAddrs
}

// AddTestAddrs constructs and returns accNum amount of accounts with an
// initial balance of accAmt in random order
func AddTestAddrs(app *BabylonApp, ctx sdk.Context, accNum int, accAmt math.Int) ([]sdk.AccAddress, error) {
	testAddrs := createRandomAccounts(accNum)

	bondDenom, err := app.StakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, err
	}
	initCoins := sdk.NewCoins(sdk.NewCoin(bondDenom, accAmt))

	for _, addr := range testAddrs {
		initAccountWithCoins(app, ctx, addr, initCoins)
	}

	return testAddrs, nil
}

func initAccountWithCoins(app *BabylonApp, ctx sdk.Context, addr sdk.AccAddress, coins sdk.Coins) {
	err := app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coins)
	if err != nil {
		panic(err)
	}

	err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, addr, coins)
	if err != nil {
		panic(err)
	}
}

// SignetBtcHeaderGenesis returns the BTC Header block zero from signet.
func SignetBtcHeaderGenesis(cdc codec.Codec) (*btclighttypes.BTCHeaderInfo, error) {
	var btcHeaderGenesis btclighttypes.BTCHeaderInfo
	// signet btc header 0
	btcHeaderGenesisStr := `{
	 	"header": "0100000000000000000000000000000000000000000000000000000000000000000000003ba3edfd7a7b12b27ac72c3e67768f617fc81bc3888a51323a9fb8aa4b1e5e4a008f4d5fae77031e8ad22203",
	 	"hash": "00000008819873e925422c1ff0f99f7cc9bbb232af63a077a480a3633bee1ef6",
		"work": "77414720"
	}`
	buff := bytes.NewBufferString(btcHeaderGenesisStr)

	err := cdc.UnmarshalJSON(buff.Bytes(), &btcHeaderGenesis)
	if err != nil {
		return nil, err
	}

	return &btcHeaderGenesis, nil
}
