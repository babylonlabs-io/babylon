package initialization

import (
	"encoding/json"
	"fmt"
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
	"github.com/cosmos/gogoproto/proto"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btccheckpointtypes "github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/types"
	blctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	btclighttypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	costktypes "github.com/babylonlabs-io/babylon/v4/x/costaking/types"
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

	InitBalanceStrA = fmt.Sprintf("%d%s", BabylonBalanceA, BabylonDenom)
	InitBalanceStrB = fmt.Sprintf("%d%s", BabylonBalanceB, BabylonDenom)
)

func addAccount(path, moniker, amountStr string, accAddr sdk.AccAddress, forkHeight int) error {
	serverCtx := server.NewDefaultContext()
	config := serverCtx.Config

	config.SetRoot(path)
	config.Moniker = moniker

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

//nolint:typecheck
func updateModuleGenesis[V proto.Message](appGenState map[string]json.RawMessage, moduleName string, protoVal V, updateGenesis func(V)) error {
	if err := util.Cdc.UnmarshalJSON(appGenState[moduleName], protoVal); err != nil {
		return err
	}
	updateGenesis(protoVal)
	newGenState := protoVal

	bz, err := util.Cdc.MarshalJSON(newGenState)
	if err != nil {
		return err
	}
	appGenState[moduleName] = bz
	return nil
}

func initGenesis(
	chain *internalChain,
	votingPeriod, expeditedVotingPeriod time.Duration,
	forkHeight int,
	btcHeaders []*btclighttypes.BTCHeaderInfo,
) error {
	// initialize a genesis file
	configDir := chain.nodes[0].configDir()

	for _, val := range chain.nodes {
		addr, err := val.keyInfo.GetAddress()

		if err != nil {
			return err
		}

		if chain.chainMeta.Id == ChainAID {
			if err := addAccount(configDir, "", InitBalanceStrA, addr, forkHeight); err != nil {
				return err
			}
		} else if chain.chainMeta.Id == ChainBID {
			if err := addAccount(configDir, "", InitBalanceStrB, addr, forkHeight); err != nil {
				return err
			}
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

	err = updateModuleGenesis(appGenState, banktypes.ModuleName, &banktypes.GenesisState{}, updateBankGenesis)
	if err != nil {
		return err
	}

	err = updateModuleGenesis(appGenState, govtypes.ModuleName, &govv1.GenesisState{}, updateGovGenesis(votingPeriod, expeditedVotingPeriod))
	if err != nil {
		return err
	}

	err = updateModuleGenesis(appGenState, minttypes.ModuleName, &minttypes.GenesisState{}, updateMintGenesis)
	if err != nil {
		return err
	}

	err = e2ev2.UpdateModuleGenesis(appGenState, costktypes.ModuleName, &costktypes.GenesisState{}, e2ev2.UpdateGenesisCostaking)
	if err != nil {
		return err
	}

	err = e2ev2.UpdateModuleGenesis(appGenState, staketypes.ModuleName, &staketypes.GenesisState{}, e2ev2.UpdateGenesisStake)
	if err != nil {
		return err
	}

	err = updateModuleGenesis(appGenState, genutiltypes.ModuleName, &genutiltypes.GenesisState{}, updateGenUtilGenesis(chain))
	if err != nil {
		return err
	}

	err = updateModuleGenesis(appGenState, blctypes.ModuleName, blctypes.DefaultGenesis(), updateBtcLightClientGenesis(btcHeaders))
	if err != nil {
		return err
	}

	err = updateModuleGenesis(appGenState, btccheckpointtypes.ModuleName, btccheckpointtypes.DefaultGenesis(), updateBtccheckpointGenesis)
	if err != nil {
		return err
	}

	err = updateModuleGenesis(appGenState, finalitytypes.ModuleName, &finalitytypes.GenesisState{}, updateFinalityGenesis)
	if err != nil {
		return err
	}

	err = updateModuleGenesis(appGenState, ratelimiter.ModuleName, &ratelimiter.GenesisState{}, applyRateLimitsToChainConfig)
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

func updateBankGenesis(bankGenState *banktypes.GenesisState) {
	bankGenState.DenomMetadata = append(bankGenState.DenomMetadata, banktypes.Metadata{
		Description: "An example stable token",
		Display:     BabylonDenom,
		Base:        BabylonDenom,
		Symbol:      BabylonDenom,
		Name:        BabylonDenom,
		DenomUnits: []*banktypes.DenomUnit{
			{
				Denom:    BabylonDenom,
				Exponent: 0,
			},
		},
	})
}

func updateGovGenesis(votingPeriod, expeditedVotingPeriod time.Duration) func(govGenState *govv1.GenesisState) {
	return func(govGenState *govv1.GenesisState) {
		govGenState.Params.MinDeposit = sdk.NewCoins(sdk.NewCoin(BabylonDenom, sdkmath.NewInt(100)))
		govGenState.Params.ExpeditedMinDeposit = sdk.NewCoins(sdk.NewCoin(BabylonDenom, sdkmath.NewInt(1000)))
		govGenState.Params.VotingPeriod = &votingPeriod
		govGenState.Params.ExpeditedVotingPeriod = &expeditedVotingPeriod
	}
}

func updateMintGenesis(mintGenState *minttypes.GenesisState) {
	mintGenState.Minter.BondDenom = BabylonDenom
}

func updateStakeGenesis(stakeGenState *staketypes.GenesisState) {
	stakeGenState.Params = staketypes.Params{
		BondDenom:         BabylonDenom,
		MaxValidators:     100,
		MaxEntries:        7,
		HistoricalEntries: 10000,
		UnbondingTime:     staketypes.DefaultUnbondingTime,
		MinCommissionRate: sdkmath.LegacyZeroDec(),
	}
}

func updateBtcLightClientGenesis(btcHeaders []*btclighttypes.BTCHeaderInfo) func(blcGenState *blctypes.GenesisState) {
	return func(blcGenState *btclighttypes.GenesisState) {
		if len(btcHeaders) > 0 {
			blcGenState.BtcHeaders = btcHeaders
			return
		}

		btcSimnetGenesisHex := "0100000000000000000000000000000000000000000000000000000000000000000000003ba3edfd7a7b12b27ac72c3e67768f617fc81bc3888a51323a9fb8aa4b1e5e4a45068653ffff7f2002000000"
		baseBtcHeader, err := bbn.NewBTCHeaderBytesFromHex(btcSimnetGenesisHex)
		if err != nil {
			panic(err)
		}
		work := blctypes.CalcWork(&baseBtcHeader)
		blcGenState.BtcHeaders = []*blctypes.BTCHeaderInfo{blctypes.NewBTCHeaderInfo(&baseBtcHeader, baseBtcHeader.Hash(), 0, &work)}
	}
}

func updateBtccheckpointGenesis(btccheckpointGenState *btccheckpointtypes.GenesisState) {
	btccheckpointGenState.Params = btccheckpointtypes.DefaultParams()
	btccheckpointGenState.Params.BtcConfirmationDepth = BabylonBtcConfirmationPeriod
	btccheckpointGenState.Params.CheckpointFinalizationTimeout = BabylonBtcFinalizationPeriod
	btccheckpointGenState.Params.CheckpointTag = BabylonOpReturnTag
}

func updateFinalityGenesis(finalityGenState *finalitytypes.GenesisState) {
	finalityGenState.Params = finalitytypes.DefaultParams()
	finalityGenState.Params.FinalityActivationHeight = 0
	finalityGenState.Params.FinalitySigTimeout = 4
	finalityGenState.Params.SignedBlocksWindow = 300
}

func updateGenUtilGenesis(c *internalChain) func(*genutiltypes.GenesisState) {
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

func applyRateLimitsToChainConfig(rateLimiterGenState *ratelimiter.GenesisState) {
	path := &ratelimiter.Path{
		Denom:             "ubbn",
		ChannelOrClientId: "channel-0",
	}

	quota := &ratelimiter.Quota{
		MaxPercentSend: sdkmath.NewInt(90),
		MaxPercentRecv: sdkmath.NewInt(90),
		DurationHours:  24,
	}

	rateLimit := ratelimiter.RateLimit{
		Path:  path,
		Quota: quota,
		Flow: &ratelimiter.Flow{
			Inflow:       sdkmath.NewInt(0),
			Outflow:      sdkmath.NewInt(0),
			ChannelValue: sdkmath.NewInt(1_000_000),
		},
	}

	rateLimiterGenState.RateLimits = append(rateLimiterGenState.RateLimits, rateLimit)
}
