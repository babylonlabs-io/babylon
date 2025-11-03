package tmanager

import (
	"encoding/json"
	"fmt"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	staketypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/gogoproto/proto"
	tokenfactorytypes "github.com/strangelove-ventures/tokenfactory/x/tokenfactory/types"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btccheckpointtypes "github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/types"
	btclighttypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	costktypes "github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	finalitytypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
	minttypes "github.com/babylonlabs-io/babylon/v4/x/mint/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	ratelimiter "github.com/cosmos/ibc-apps/modules/rate-limiting/v10/types"
)

const (
	BabylonBtcConfirmationPeriod = 2
	BabylonBtcFinalizationPeriod = 4
	BabylonOpReturnTag           = "01020304"
)

var (
	SelfDelegationAmt     = sdkmath.NewInt(10_000_000000) // 10k ubbn
	InitialSelfDelegation = sdk.NewCoin(appparams.DefaultBondDenom, SelfDelegationAmt)
)

// InitGenesis holds genesis configuration
type InitGenesis struct {
	ChainConfig           *ChainConfig
	GenesisTime           time.Time
	VotingPeriod          time.Duration
	ExpeditedVotingPeriod time.Duration
	InitialTokens         sdk.Coins
}

// StartingBtcStakingParams is the initial btc staking parameters for the chain
type StartingBtcStakingParams struct {
	CovenantCommittee []bbn.BIP340PubKey
	CovenantQuorum    uint32
	MaxStakerQuorum   uint32
	MaxStakerNum      uint32
}

func UpdateGenAccounts(
	appGenState map[string]json.RawMessage,
	newAccs []*authtypes.BaseAccount,
) (accs authtypes.GenesisAccounts, err error) {
	authGenState := authtypes.GetGenesisStateFromAppState(util.Cdc, appGenState)
	accs, err = authtypes.UnpackAccounts(authGenState.Accounts)
	if err != nil {
		return nil, err
	}

	for _, newAcc := range newAccs {
		if accs.Contains(newAcc.GetAddress()) {
			return nil, fmt.Errorf("failed to add same acc twice")
		}
		accs = append(accs, newAcc)
	}

	// sanitize accounts and set the accounts in the genesis state
	accs = authtypes.SanitizeGenesisAccounts(accs)
	genAccs, err := authtypes.PackAccounts(accs)
	if err != nil {
		return nil, err
	}

	authGenState.Accounts = genAccs
	authGenStateBz, err := util.Cdc.MarshalJSON(&authGenState)
	if err != nil {
		return nil, err
	}

	appGenState[authtypes.ModuleName] = authGenStateBz
	return accs, nil
}

func UpdateGenModulesState(
	appGenState map[string]json.RawMessage,
	initGen InitGenesis,
	vals []*ValidatorNode,
	btcHeaders []*btclighttypes.BTCHeaderInfo,
	startingBtcStakingParams *StartingBtcStakingParams,
	bankBalancesToAdd []banktypes.Balance,
	isUpgrade bool,
) error {
	err := UpdateModuleGenesis(appGenState, banktypes.ModuleName, &banktypes.GenesisState{}, UpdateGenesisBank(bankBalancesToAdd))
	if err != nil {
		return err
	}

	err = UpdateModuleGenesis(appGenState, govtypes.ModuleName, &govv1.GenesisState{}, UpdateGenesisGov(initGen.VotingPeriod, initGen.ExpeditedVotingPeriod))
	if err != nil {
		return err
	}

	err = UpdateModuleGenesis(appGenState, minttypes.ModuleName, &minttypes.GenesisState{}, UpdateGenesisMint)
	if err != nil {
		return err
	}

	err = UpdateModuleGenesis(appGenState, costktypes.ModuleName, &costktypes.GenesisState{}, UpdateGenesisCostaking)
	if err != nil {
		return err
	}

	err = UpdateModuleGenesis(appGenState, staketypes.ModuleName, &staketypes.GenesisState{}, UpdateGenesisStake)
	if err != nil {
		return err
	}

	err = UpdateModuleGenesis(appGenState, genutiltypes.ModuleName, &genutiltypes.GenesisState{}, UpdateGenesisGenUtil(vals))
	if err != nil {
		return err
	}

	err = UpdateModuleGenesis(appGenState, btclighttypes.ModuleName, btclighttypes.DefaultGenesis(), UpdateGenesisBtcLightClient(btcHeaders))
	if err != nil {
		return err
	}

	err = UpdateModuleGenesis(appGenState, btccheckpointtypes.ModuleName, btccheckpointtypes.DefaultGenesis(), UpdateGenesisBtccheckpoint)
	if err != nil {
		return err
	}

	err = UpdateModuleGenesis(appGenState, finalitytypes.ModuleName, &finalitytypes.GenesisState{}, UpdateGenesisFinality)
	if err != nil {
		return err
	}

	err = UpdateModuleGenesis(appGenState, ratelimiter.ModuleName, &ratelimiter.GenesisState{}, UpdateGenesisRateLimit)
	if err != nil {
		return fmt.Errorf("failed to update rate limiter genesis state: %w", err)
	}

	err = UpdateModuleGenesis(appGenState, tokenfactorytypes.ModuleName, &tokenfactorytypes.GenesisState{}, UpdateGenesisTokenFactory)
	if err != nil {
		return fmt.Errorf("failed to update tokenfactory genesis state: %w", err)
	}

	// NOTE: in case of the software upgrade test, we don't want to update
	// genesis state since it will introduce version incompatibility of genesis.json
	if !isUpgrade {
		err = UpdateModuleGenesis(appGenState, btcstktypes.ModuleName, &btcstktypes.GenesisState{}, UpdateGenesisBtcStaking(startingBtcStakingParams))
		if err != nil {
			return fmt.Errorf("failed to update btc staking genesis state: %w", err)
		}
	}

	return nil
}

//nolint:typecheck
func UpdateModuleGenesis[V proto.Message](appGenState map[string]json.RawMessage, moduleName string, protoVal V, updateGenesis func(V)) error {
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

func UpdateGenesisBank(balancesToAdd []banktypes.Balance) func(bankGenState *banktypes.GenesisState) {
	return func(bankGenState *banktypes.GenesisState) {
		bankGenState.DenomMetadata = append(bankGenState.DenomMetadata, banktypes.Metadata{
			Description: "An example stable token",
			Display:     appparams.DefaultBondDenom,
			Base:        appparams.DefaultBondDenom,
			Symbol:      appparams.DefaultBondDenom,
			Name:        appparams.DefaultBondDenom,
			DenomUnits: []*banktypes.DenomUnit{
				{
					Denom:    appparams.DefaultBondDenom,
					Exponent: 0,
				},
			},
		})
		bankGenState.Balances = append(bankGenState.Balances, balancesToAdd...)
		bankGenState.Balances = banktypes.SanitizeGenesisBalances(bankGenState.Balances)
	}
}

func UpdateGenesisGov(votingPeriod, expeditedVotingPeriod time.Duration) func(govGenState *govv1.GenesisState) {
	return func(govGenState *govv1.GenesisState) {
		govGenState.Params.MinDeposit = sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(100)))
		govGenState.Params.ExpeditedMinDeposit = sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1000)))
		govGenState.Params.VotingPeriod = &votingPeriod
		govGenState.Params.ExpeditedVotingPeriod = &expeditedVotingPeriod
	}
}

func UpdateGenesisMint(mintGenState *minttypes.GenesisState) {
	mintGenState.Minter.BondDenom = appparams.DefaultBondDenom
}

func UpdateGenesisCostaking(gs *costktypes.GenesisState) {
	gs.Params = costktypes.DefaultParams()
}

func UpdateGenesisStake(stakeGenState *staketypes.GenesisState) {
	stakeGenState.Params = staketypes.Params{
		BondDenom:         appparams.DefaultBondDenom,
		MaxValidators:     100,
		MaxEntries:        7,
		HistoricalEntries: 10000,
		UnbondingTime:     staketypes.DefaultUnbondingTime,
		MinCommissionRate: sdkmath.LegacyMustNewDecFromStr("0.03"),
	}
}

func UpdateGenesisBtcLightClient(btcHeaders []*btclighttypes.BTCHeaderInfo) func(blcGenState *btclighttypes.GenesisState) {
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
		work := btclighttypes.CalcWork(&baseBtcHeader)
		blcGenState.BtcHeaders = []*btclighttypes.BTCHeaderInfo{btclighttypes.NewBTCHeaderInfo(&baseBtcHeader, baseBtcHeader.Hash(), 0, &work)}
	}
}

func UpdateGenesisBtccheckpoint(btccheckpointGenState *btccheckpointtypes.GenesisState) {
	btccheckpointGenState.Params = btccheckpointtypes.DefaultParams()
	btccheckpointGenState.Params.BtcConfirmationDepth = BabylonBtcConfirmationPeriod
	btccheckpointGenState.Params.CheckpointFinalizationTimeout = BabylonBtcFinalizationPeriod
	btccheckpointGenState.Params.CheckpointTag = BabylonOpReturnTag
}

func UpdateGenesisFinality(finalityGenState *finalitytypes.GenesisState) {
	finalityGenState.Params = finalitytypes.DefaultParams()
	finalityGenState.Params.FinalityActivationHeight = 0
	finalityGenState.Params.FinalitySigTimeout = 4
	finalityGenState.Params.SignedBlocksWindow = 300
}

func UpdateGenesisBtcStaking(p *StartingBtcStakingParams) func(*btcstktypes.GenesisState) {
	return func(gen *btcstktypes.GenesisState) {
		if p != nil {
			gen.Params[0].MaxStakerNum = p.MaxStakerNum
			gen.Params[0].MaxStakerQuorum = p.MaxStakerQuorum
			if len(p.CovenantCommittee) != 0 && p.CovenantQuorum != 0 {
				gen.Params[0].CovenantPks = p.CovenantCommittee
				gen.Params[0].CovenantQuorum = p.CovenantQuorum
			}
		}
	}
}

func UpdateGenesisGenUtil(vals []*ValidatorNode) func(*genutiltypes.GenesisState) {
	return func(genUtilGenState *genutiltypes.GenesisState) {
		genTxs := make([]json.RawMessage, 0, len(vals))
		for _, val := range vals { // one gentx per val
			createValmsg := val.CreateValidatorMsg(InitialSelfDelegation)
			signedTx := val.SignMsg(createValmsg)

			txRaw, err := util.Cdc.MarshalJSON(signedTx)
			require.NoError(val.T(), err, "genutil genesis setup failed", err)
			genTxs = append(genTxs, txRaw)
		}
		genUtilGenState.GenTxs = genTxs
	}
}

func UpdateGenesisRateLimit(rateLimiterGenState *ratelimiter.GenesisState) {
	path := &ratelimiter.Path{
		Denom:             appparams.DefaultBondDenom,
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
			ChannelValue: sdkmath.NewInt(10_000000), // 10 BABY
		},
	}

	rateLimiterGenState.RateLimits = append(rateLimiterGenState.RateLimits, rateLimit)
}

func UpdateGenesisTokenFactory(tokenfactoryGenState *tokenfactorytypes.GenesisState) {
	tokenfactoryGenState.Params = tokenfactorytypes.Params{
		DenomCreationFee: sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(10000))),
	}
}
