package cmd

import (
	"encoding/hex"
	"strings"
	"time"

	"cosmossdk.io/math"

	tmrand "github.com/cometbft/cometbft/libs/rand"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	babylonApp "github.com/babylonlabs-io/babylon/app"
	btcctypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	btcltypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	btcstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

const (
	flagMaxActiveValidators        = "max-active-validators"
	flagBtcConfirmationDepth       = "btc-confirmation-depth"
	flagEpochInterval              = "epoch-interval"
	flagBtcFinalizationTimeout     = "btc-finalization-timeout"
	flagCheckpointTag              = "checkpoint-tag"
	flagBaseBtcHeaderHex           = "btc-base-header"
	flagBaseBtcHeaderHeight        = "btc-base-header-height"
	flagAllowedReporterAddresses   = "allowed-reporter-addresses"
	flagInflationRateChange        = "inflation-rate-change"
	flagInflationMax               = "inflation-max"
	flagInflationMin               = "inflation-min"
	flagGoalBonded                 = "goal-bonded"
	flagBlocksPerYear              = "blocks-per-year"
	flagGenesisTime                = "genesis-time"
	flagBlockGasLimit              = "block-gas-limit"
	flagVoteExtensionEnableHeight  = "vote-extension-enable-height"
	flagCovenantPks                = "covenant-pks"
	flagCovenantQuorum             = "covenant-quorum"
	flagMinStakingAmtSat           = "min-staking-amount-sat"
	flagMaxStakingAmtSat           = "max-staking-amount-sat"
	flagMinStakingTimeBlocks       = "min-staking-time-blocks"
	flagMaxStakingTimeBlocks       = "max-staking-time-blocks"
	flagMaxActiveFinalityProviders = "max-active-finality-providers"
	flagMinUnbondingTime           = "min-unbonding-time"
	flagUnbondingFeeSat            = "unbonding-fee-sat"
	flagSlashingPkScript           = "slashing-pk-script"
	flagMinSlashingFee             = "min-slashing-fee-sat"
	flagSlashingRate               = "slashing-rate"
	flagMinCommissionRate          = "min-commission-rate"
)

type GenesisCLIArgs struct {
	ChainID                      string
	MaxActiveValidators          uint32
	BtcConfirmationDepth         uint64
	BtcFinalizationTimeout       uint64
	CheckpointTag                string
	EpochInterval                uint64
	BaseBtcHeaderHex             string
	BaseBtcHeaderHeight          uint64
	AllowedReporterAddresses     []string
	InflationRateChange          float64
	InflationMax                 float64
	InflationMin                 float64
	GoalBonded                   float64
	BlocksPerYear                uint64
	GenesisTime                  time.Time
	BlockGasLimit                int64
	VoteExtensionEnableHeight    int64
	CovenantPKs                  []string
	CovenantQuorum               uint32
	MinStakingAmtSat             int64
	MaxStakingAmtSat             int64
	MinStakingTimeBlocks         uint16
	MaxStakingTimeBlocks         uint16
	SlashingPkScript             string
	MinSlashingTransactionFeeSat int64
	SlashingRate                 math.LegacyDec
	MaxActiveFinalityProviders   uint32
	MinUnbondingTime             uint16
	UnbondingFeeSat              int64
	MinCommissionRate            math.LegacyDec
}

func addGenesisFlags(cmd *cobra.Command) {
	cmd.Flags().String(flags.FlagChainID, "", "genesis file chain-id, if left blank will be randomly created")
	// staking flags
	cmd.Flags().Uint32(flagMaxActiveValidators, 10, "Maximum number of validators.")
	// btccheckpoint flags
	cmd.Flags().Uint64(flagBtcConfirmationDepth, 6, "Confirmation depth for Bitcoin headers.")
	cmd.Flags().Uint64(flagBtcFinalizationTimeout, 20, "Finalization timeout for Bitcoin headers.")
	cmd.Flags().String(flagCheckpointTag, btcctypes.DefaultCheckpointTag, "Hex encoded tag for babylon checkpoint on btc")
	// epoch args
	cmd.Flags().Uint64(flagEpochInterval, 400, "Number of blocks between epochs. Must be more than 0.")
	// btclightclient args
	// Genesis header for the simnet
	cmd.Flags().String(flagBaseBtcHeaderHex, "0100000000000000000000000000000000000000000000000000000000000000000000003ba3edfd7a7b12b27ac72c3e67768f617fc81bc3888a51323a9fb8aa4b1e5e4a45068653ffff7f2002000000", "Hex of the base Bitcoin header.")
	cmd.Flags().String(flagAllowedReporterAddresses, strings.Join(btcltypes.DefaultParams().InsertHeadersAllowList, ","), "addresses of reporters allowed to submit Bitcoin headers to babylon")
	cmd.Flags().Uint64(flagBaseBtcHeaderHeight, 0, "Height of the base Bitcoin header.")
	// btcstaking args
	cmd.Flags().String(flagCovenantPks, strings.Join(btcstypes.DefaultParams().CovenantPksHex(), ","), "Bitcoin staking covenant public keys, comma separated")
	cmd.Flags().Uint32(flagCovenantQuorum, btcstypes.DefaultParams().CovenantQuorum, "Bitcoin staking covenant quorum")
	cmd.Flags().Int64(flagMinStakingAmtSat, 500000, "Minimum staking amount in satoshis")
	cmd.Flags().Int64(flagMaxStakingAmtSat, 100000000000, "Maximum staking amount in satoshis")
	cmd.Flags().Uint16(flagMinStakingTimeBlocks, 100, "Minimum staking time in blocks")
	cmd.Flags().Uint16(flagMaxStakingTimeBlocks, 10000, "Maximum staking time in blocks")
	cmd.Flags().String(flagSlashingPkScript, hex.EncodeToString(btcstypes.DefaultParams().SlashingPkScript), "Bitcoin staking slashing pk script. Hex encoded.")
	cmd.Flags().Int64(flagMinSlashingFee, 1000, "Bitcoin staking minimum slashing fee")
	cmd.Flags().String(flagMinCommissionRate, "0", "Bitcoin staking validator minimum commission rate")
	cmd.Flags().String(flagSlashingRate, "0.1", "Bitcoin staking slashing rate")
	cmd.Flags().Uint32(flagMaxActiveFinalityProviders, 100, "Bitcoin staking maximum active finality providers")
	cmd.Flags().Uint16(flagMinUnbondingTime, 0, "Min timelock on unbonding transaction in btc blocks")
	cmd.Flags().Int64(flagUnbondingFeeSat, 1000, "Required fee for unbonding transaction in satoshis")
	// inflation args
	cmd.Flags().Float64(flagInflationRateChange, 0.13, "Inflation rate change")
	cmd.Flags().Float64(flagInflationMax, 0.2, "Maximum inflation")
	cmd.Flags().Float64(flagInflationMin, 0.07, "Minimum inflation")
	cmd.Flags().Float64(flagGoalBonded, 0.67, "Bonded tokens goal")
	cmd.Flags().Uint64(flagBlocksPerYear, 6311520, "Blocks per year")
	// genesis args
	cmd.Flags().Int64(flagGenesisTime, time.Now().Unix(), "Genesis time")
	// blocks args
	cmd.Flags().Int64(flagBlockGasLimit, babylonApp.DefaultGasLimit, "Block gas limit")
	cmd.Flags().Int64(flagVoteExtensionEnableHeight, babylonApp.DefaultVoteExtensionsEnableHeight, "Vote extension enable height")
}

func parseGenesisFlags(cmd *cobra.Command) *GenesisCLIArgs {
	chainID, _ := cmd.Flags().GetString(flags.FlagChainID)
	maxActiveValidators, _ := cmd.Flags().GetUint32(flagMaxActiveValidators)
	btcConfirmationDepth, _ := cmd.Flags().GetUint64(flagBtcConfirmationDepth)
	btcFinalizationTimeout, _ := cmd.Flags().GetUint64(flagBtcFinalizationTimeout)
	checkpointTag, _ := cmd.Flags().GetString(flagCheckpointTag)
	epochInterval, _ := cmd.Flags().GetUint64(flagEpochInterval)
	baseBtcHeaderHex, _ := cmd.Flags().GetString(flagBaseBtcHeaderHex)
	baseBtcHeaderHeight, _ := cmd.Flags().GetUint64(flagBaseBtcHeaderHeight)
	reporterAddresses, _ := cmd.Flags().GetString(flagAllowedReporterAddresses)
	covenantPks, _ := cmd.Flags().GetString(flagCovenantPks)
	covenantQuorum, _ := cmd.Flags().GetUint32(flagCovenantQuorum)
	minStakingAmtSat, _ := cmd.Flags().GetInt64(flagMinStakingAmtSat)
	maxStakingAmtSat, _ := cmd.Flags().GetInt64(flagMaxStakingAmtSat)
	minStakingTimeBlocks, _ := cmd.Flags().GetUint16(flagMinStakingTimeBlocks)
	maxStakingTimeBlocks, _ := cmd.Flags().GetUint16(flagMaxStakingTimeBlocks)
	slashingPkScript, _ := cmd.Flags().GetString(flagSlashingPkScript)
	minSlashingFee, _ := cmd.Flags().GetInt64(flagMinSlashingFee)
	minCommissionRate, _ := cmd.Flags().GetString(flagMinCommissionRate)
	slashingRate, _ := cmd.Flags().GetString(flagSlashingRate)
	maxActiveFinalityProviders, _ := cmd.Flags().GetUint32(flagMaxActiveFinalityProviders)
	minUnbondingTime, _ := cmd.Flags().GetUint16(flagMinUnbondingTime)
	unbondingFeeSat, _ := cmd.Flags().GetInt64(flagUnbondingFeeSat)
	genesisTimeUnix, _ := cmd.Flags().GetInt64(flagGenesisTime)
	inflationRateChange, _ := cmd.Flags().GetFloat64(flagInflationRateChange)
	inflationMax, _ := cmd.Flags().GetFloat64(flagInflationMax)
	inflationMin, _ := cmd.Flags().GetFloat64(flagInflationMin)
	goalBonded, _ := cmd.Flags().GetFloat64(flagGoalBonded)
	blocksPerYear, _ := cmd.Flags().GetUint64(flagBlocksPerYear)
	blockGasLimit, _ := cmd.Flags().GetInt64(flagBlockGasLimit)
	voteExtensionEnableHeight, _ := cmd.Flags().GetInt64(flagVoteExtensionEnableHeight)

	if chainID == "" {
		chainID = "chain-" + tmrand.NewRand().Str(6)
	}

	var allowedReporterAddresses []string = make([]string, 0)
	if reporterAddresses != "" {
		allowedReporterAddresses = strings.Split(reporterAddresses, ",")
	}

	genesisTime := time.Unix(genesisTimeUnix, 0)

	return &GenesisCLIArgs{
		ChainID:                      chainID,
		MaxActiveValidators:          maxActiveValidators,
		BtcConfirmationDepth:         btcConfirmationDepth,
		BtcFinalizationTimeout:       btcFinalizationTimeout,
		CheckpointTag:                checkpointTag,
		EpochInterval:                epochInterval,
		BaseBtcHeaderHeight:          baseBtcHeaderHeight,
		BaseBtcHeaderHex:             baseBtcHeaderHex,
		AllowedReporterAddresses:     allowedReporterAddresses,
		CovenantPKs:                  strings.Split(covenantPks, ","),
		CovenantQuorum:               covenantQuorum,
		MinStakingAmtSat:             minStakingAmtSat,
		MaxStakingAmtSat:             maxStakingAmtSat,
		MinStakingTimeBlocks:         minStakingTimeBlocks,
		MaxStakingTimeBlocks:         maxStakingTimeBlocks,
		SlashingPkScript:             slashingPkScript,
		MinSlashingTransactionFeeSat: minSlashingFee,
		MinCommissionRate:            math.LegacyMustNewDecFromStr(minCommissionRate),
		SlashingRate:                 math.LegacyMustNewDecFromStr(slashingRate),
		MaxActiveFinalityProviders:   maxActiveFinalityProviders,
		MinUnbondingTime:             minUnbondingTime,
		UnbondingFeeSat:              unbondingFeeSat,
		GenesisTime:                  genesisTime,
		InflationRateChange:          inflationRateChange,
		InflationMax:                 inflationMax,
		InflationMin:                 inflationMin,
		GoalBonded:                   goalBonded,
		BlocksPerYear:                blocksPerYear,
		BlockGasLimit:                blockGasLimit,
		VoteExtensionEnableHeight:    voteExtensionEnableHeight,
	}
}
