package initialization

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"path/filepath"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	staketypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	tokenfactorytypes "github.com/strangelove-ventures/tokenfactory/x/tokenfactory/types"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
	e2etypes "github.com/babylonlabs-io/babylon/v4/test/e2e2/types"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btccheckpointtypes "github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/types"
	btclighttypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	finalitytypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
	minttypes "github.com/babylonlabs-io/babylon/v4/x/mint/types"
	ratelimiter "github.com/cosmos/ibc-apps/modules/rate-limiting/v10/types"
)

// NodeConfig is a configuration for the node supplied from the test runner
// to initialization scripts. It should be backwards compatible with earlier
// versions. If this struct is updated, the change must be backported to earlier
// branches that might be used for upgrade testing.
type NodeConfig struct {
	Name               string // name of the config that will also be assigned to Docke container.
	Pruning            string // default, nothing, everything, or custom
	PruningKeepRecent  string // keep all of the last N states (only used with custom pruning)
	PruningInterval    string // delete old states from every Nth block (only used with custom pruning)
	SnapshotInterval   uint64 // statesync snapshot every Nth block (0 to disable)
	SnapshotKeepRecent uint32 // number of recent snapshots to keep and serve (0 to keep all)
	IsValidator        bool   // flag indicating whether a node should be a validator
	BtcNetwork         string // The Bitcoin network used
}

// StartingBtcStakingParams is the initial btc staking parameters for the chain
type StartingBtcStakingParams struct {
	CovenantCommittee []bbn.BIP340PubKey
	CovenantQuorum    uint32
}

const (
	// common
	BabylonDenom        = "ubbn"
	MinGasPrice         = "0.002"
	ValidatorWalletName = "val"
	BabylonOpReturnTag  = "01020304"

	BabylonBtcConfirmationPeriod = 2
	BabylonBtcFinalizationPeriod = 4
	// chainA
	ChainAID        = "bbn-test-a"
	BabylonBalanceA = 3000000000000
	StakeAmountA    = 100000000000
	// chainB
	ChainBID        = "bbn-test-b"
	BabylonBalanceB = 5000000000000
	StakeAmountB    = 400000000000
)

var (
	StakeAmountIntA  = sdkmath.NewInt(StakeAmountA)
	StakeAmountCoinA = sdk.NewCoin(BabylonDenom, StakeAmountIntA)
	StakeAmountIntB  = sdkmath.NewInt(StakeAmountB)
	StakeAmountCoinB = sdk.NewCoin(BabylonDenom, StakeAmountIntB)
	InitBalanceStrA  = fmt.Sprintf("%d%s", BabylonBalanceA, BabylonDenom)
	InitBalanceStrB  = fmt.Sprintf("%d%s", BabylonBalanceB, BabylonDenom)
)

func AddAccount(path, amountStr string, accAddr sdk.AccAddress, forkHeight int) error {
	serverCtx := server.NewDefaultContext()
	config := serverCtx.Config

	config.SetRoot(path)

	coins, err := sdk.ParseCoinsNormalized(amountStr)
	if err != nil {
		return fmt.Errorf("failed to parse coins: %w", err)
	}

	balances := banktypes.Balance{Address: accAddr.String(), Coins: coins.Sort()}
	genAccount := authtypes.NewBaseAccount(accAddr, nil, 0, 0)

	// TODO: Make the SDK make it far cleaner to add an account to GenesisState
	genFile := config.GenesisFile()
	appState, genDoc, err := genutiltypes.GenesisStateFromGenFile(genFile)
	if err != nil {
		return fmt.Errorf("failed to unmarshal genesis state: %w", err)
	}

	genDoc.InitialHeight = int64(forkHeight)

	authGenState := authtypes.GetGenesisStateFromAppState(util.Cdc, appState)

	accs, err := authtypes.UnpackAccounts(authGenState.Accounts)
	if err != nil {
		return fmt.Errorf("failed to get accounts from any: %w", err)
	}

	if accs.Contains(accAddr) {
		return fmt.Errorf("failed to add account to genesis state; account already exists: %s", accAddr)
	}

	// Add the new account to the set of genesis accounts and sanitize the
	// accounts afterwards.
	accs = append(accs, genAccount)
	accs = authtypes.SanitizeGenesisAccounts(accs)

	genAccs, err := authtypes.PackAccounts(accs)
	if err != nil {
		return fmt.Errorf("failed to convert accounts into any's: %w", err)
	}

	authGenState.Accounts = genAccs

	authGenStateBz, err := util.Cdc.MarshalJSON(&authGenState)
	if err != nil {
		return fmt.Errorf("failed to marshal auth genesis state: %w", err)
	}

	appState[authtypes.ModuleName] = authGenStateBz

	bankGenState := banktypes.GetGenesisStateFromAppState(util.Cdc, appState)
	bankGenState.Balances = append(bankGenState.Balances, balances)
	bankGenState.Balances = banktypes.SanitizeGenesisBalances(bankGenState.Balances)

	bankGenStateBz, err := util.Cdc.MarshalJSON(bankGenState)
	if err != nil {
		return fmt.Errorf("failed to marshal bank genesis state: %w", err)
	}

	appState[banktypes.ModuleName] = bankGenStateBz

	appStateJSON, err := json.Marshal(appState)
	if err != nil {
		return fmt.Errorf("failed to marshal application genesis state: %w", err)
	}

	genDoc.AppState = appStateJSON
	return genutil.ExportGenesisFile(genDoc, genFile)
}

func initGenesis(
	chain *internalChain,
	votingPeriod, expeditedVotingPeriod time.Duration,
	forkHeight int,
	btcHeaders []*btclighttypes.BTCHeaderInfo,
	startingBtcStakingParams *StartingBtcStakingParams,
) error {
	// initialize a genesis file
	configDir := chain.nodes[0].configDir()

	for _, val := range chain.nodes {
		addr, err := val.keyInfo.GetAddress()

		if err != nil {
			return err
		}

		if chain.chainMeta.Id == ChainAID {
			// add random coins to test bsn rewards
			r := rand.New(rand.NewSource(time.Now().Unix()))
			initialFundsA := datagen.GenRandomCoins(r).MulInt(sdkmath.NewInt(10))
			initialFundsA = initialFundsA.Add(sdk.NewCoin(BabylonDenom, sdkmath.NewInt(BabylonBalanceA)))
			if err := AddAccount(configDir, initialFundsA.String(), addr, forkHeight); err != nil {
				return err
			}
			continue
		}

		if err := AddAccount(configDir, InitBalanceStrB, addr, forkHeight); err != nil {
			return err
		}
	}

	// copy the genesis file to the remaining validators
	for _, val := range chain.nodes[1:] {
		_, err := util.CopyFile(
			filepath.Join(configDir, "config", "genesis.json"),
			filepath.Join(val.configDir(), "config", "genesis.json"),
		)
		if err != nil {
			return err
		}
	}

	serverCtx := server.NewDefaultContext()
	config := serverCtx.Config

	config.SetRoot(chain.nodes[0].configDir())
	config.Moniker = chain.nodes[0].moniker

	genFilePath := config.GenesisFile()
	appGenState, genDoc, err := genutiltypes.GenesisStateFromGenFile(genFilePath)
	if err != nil {
		return err
	}

	err = e2etypes.UpdateModuleGenesis(appGenState, banktypes.ModuleName, &banktypes.GenesisState{}, e2etypes.UpdateGenesisBank(nil))
	if err != nil {
		return err
	}

	err = e2etypes.UpdateModuleGenesis(appGenState, govtypes.ModuleName, &govv1.GenesisState{}, e2etypes.UpdateGenesisGov(votingPeriod, expeditedVotingPeriod))
	if err != nil {
		return err
	}

	err = e2etypes.UpdateModuleGenesis(appGenState, minttypes.ModuleName, &minttypes.GenesisState{}, e2etypes.UpdateGenesisMint)
	if err != nil {
		return err
	}

	err = e2etypes.UpdateModuleGenesis(appGenState, staketypes.ModuleName, &staketypes.GenesisState{}, e2etypes.UpdateGenesisStake)
	if err != nil {
		return err
	}

	err = e2etypes.UpdateModuleGenesis(appGenState, genutiltypes.ModuleName, &genutiltypes.GenesisState{}, updateGenesisGenUtil(chain))
	if err != nil {
		return err
	}

	err = e2etypes.UpdateModuleGenesis(appGenState, btclighttypes.ModuleName, btclighttypes.DefaultGenesis(), e2etypes.UpdateGenesisBtcLightClient(btcHeaders))
	if err != nil {
		return err
	}

	err = e2etypes.UpdateModuleGenesis(appGenState, btccheckpointtypes.ModuleName, btccheckpointtypes.DefaultGenesis(), e2etypes.UpdateGenesisBtccheckpoint)
	if err != nil {
		return err
	}

	err = e2etypes.UpdateModuleGenesis(appGenState, finalitytypes.ModuleName, &finalitytypes.GenesisState{}, e2etypes.UpdateGenesisFinality)
	if err != nil {
		return err
	}

	err = e2etypes.UpdateModuleGenesis(appGenState, ratelimiter.ModuleName, &ratelimiter.GenesisState{}, e2etypes.UpdateGenesisRateLimit)
	if err != nil {
		return fmt.Errorf("failed to update rate limiter genesis state: %w", err)
	}

	err = e2etypes.UpdateModuleGenesis(appGenState, tokenfactorytypes.ModuleName, &tokenfactorytypes.GenesisState{}, e2etypes.UpdateGenesisTokenFactory)
	if err != nil {
		return fmt.Errorf("failed to update tokenfactory genesis state: %w", err)
	}

	err = e2etypes.UpdateModuleGenesis(appGenState, btcstktypes.ModuleName, &btcstktypes.GenesisState{}, updateGenesisBtcStaking(startingBtcStakingParams))
	if err != nil {
		return fmt.Errorf("failed to update rate limiter genesis state: %w", err)
	}

	bz, err := json.MarshalIndent(appGenState, "", "  ")
	if err != nil {
		return err
	}

	genDoc.AppState = bz

	// write the updated genesis file to each validator
	for _, val := range chain.nodes {
		path := filepath.Join(val.configDir(), "config", "genesis.json")

		// We need to use genutil.ExportGenesisFile to marshal and write the genesis file
		// to use correct json encoding.
		if err = genutil.ExportGenesisFile(genDoc, path); err != nil {
			return fmt.Errorf("failed to export app genesis state: %w", err)
		}
	}
	return nil
}

func updateGenesisGenUtil(c *internalChain) func(*genutiltypes.GenesisState) {
	return func(genUtilGenState *genutiltypes.GenesisState) {
		// generate genesis txs
		genTxs := make([]json.RawMessage, 0, len(c.nodes))
		for _, node := range c.nodes {
			if !node.isValidator {
				continue
			}

			stakeAmountCoin := StakeAmountCoinA
			if c.chainMeta.Id != ChainAID {
				stakeAmountCoin = StakeAmountCoinB
			}

			createValmsg, err := node.buildCreateValidatorMsg(stakeAmountCoin, node.consensusKey)
			if err != nil {
				panic("genutil genesis setup failed: " + err.Error())
			}

			signedTx, err := node.signMsg(createValmsg)
			if err != nil {
				panic("genutil genesis setup failed: " + err.Error())
			}

			txRaw, err := util.Cdc.MarshalJSON(signedTx)
			if err != nil {
				panic("genutil genesis setup failed: " + err.Error())
			}
			genTxs = append(genTxs, txRaw)
		}
		genUtilGenState.GenTxs = genTxs
	}
}

func updateGenesisBtcStaking(p *StartingBtcStakingParams) func(*btcstktypes.GenesisState) {
	return func(gen *btcstktypes.GenesisState) {
		gen.Params[0].MaxFinalityProviders = 5

		if p != nil {
			gen.Params[0].CovenantPks = p.CovenantCommittee
			gen.Params[0].CovenantQuorum = p.CovenantQuorum
		}
	}
}
