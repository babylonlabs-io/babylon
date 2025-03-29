package mainnet

import (
	"fmt"
	"time"

	"cosmossdk.io/math"
	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/app/keepers"
	appparams "github.com/babylonlabs-io/babylon/app/params"
	cmttypes "github.com/cometbft/cometbft/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	// Slashing params
	MainnetMinSignedBlocks, _    = sdkmath.LegacyNewDecFromStr("0.6")
	MainnetSlashFractionDowntime = sdkmath.LegacyZeroDec()
	MainnetDowntimeJailDuration  = 300 * time.Second

	// Governance params
	MainnetVotingPeriod = 3 * 24 * time.Hour
	// 50k BABY
	MainnetMinDeposit = sdk.NewCoin(appparams.DefaultBondDenom, math.NewInt(50_000_000000))
	// 200k BABY
	MainnetMaxDepositPeriod      = 14 * 24 * time.Hour
	MainnetExpeditedVotingPeriod = 24 * time.Hour
	MainnetExpeditedMinDeposit   = sdk.NewCoin(appparams.DefaultBondDenom, math.NewInt(200_000_000000))

	// Consensus params
	MainnetBlockGasLimit = int64(300000000)
	// Staking params
	MainnetBabyMinCommissionRate, _ = sdkmath.LegacyNewDecFromStr("0.03")

	// BTC checkpoint params
	MainnetBTCConfirmationDepth = uint32(30)
	MainnetBTCFinalizationDepth = uint32(300)

	// Distribution params
	MainnetCommunityTax          = sdkmath.LegacyZeroDec()
	MainnetWithdrawalAddrEnabled = true
)

// MainnetParamUpgrade make updates to specific params of specific modules
func MainnetParamUpgrade(ctx sdk.Context, k *keepers.AppKeepers) error {
	// update slashing params
	slashingParams, err := k.SlashingKeeper.GetParams(ctx)
	if err != nil {
		return fmt.Errorf("failed to get slash params: %w", err)
	}

	slashingParams.MinSignedPerWindow = MainnetMinSignedBlocks
	slashingParams.DowntimeJailDuration = MainnetDowntimeJailDuration
	slashingParams.SlashFractionDowntime = MainnetSlashFractionDowntime

	if err := slashingParams.Validate(); err != nil {
		return fmt.Errorf("failed to validate slashing params: %w", err)
	}
	if err := k.SlashingKeeper.SetParams(ctx, slashingParams); err != nil {
		return fmt.Errorf("failed to set slashing params: %w", err)
	}

	// update gov params
	govParams, err := k.GovKeeper.Params.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get gov params: %w", err)
	}

	govParams.VotingPeriod = &MainnetVotingPeriod
	govParams.ExpeditedVotingPeriod = &MainnetExpeditedVotingPeriod
	govParams.MaxDepositPeriod = &MainnetMaxDepositPeriod
	govParams.MinDeposit = []sdk.Coin{
		MainnetMinDeposit,
	}
	govParams.ExpeditedMinDeposit = []sdk.Coin{
		MainnetExpeditedMinDeposit,
	}

	if err := govParams.ValidateBasic(); err != nil {
		return fmt.Errorf("failed to validate gov params: %w", err)
	}
	if err := k.GovKeeper.Params.Set(ctx, govParams); err != nil {
		return fmt.Errorf("failed to set gov params: %w", err)
	}

	// update consensus params
	consensusParams, err := k.ConsensusParamsKeeper.ParamsStore.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get consensus params: %w", err)
	}

	consensusParams.Block.MaxGas = MainnetBlockGasLimit

	consparams := cmttypes.ConsensusParamsFromProto(consensusParams)
	if err := consparams.ValidateUpdate(&consensusParams, ctx.HeaderInfo().Height); err != nil {
		return fmt.Errorf("failed to validate consensus params: %w", err)
	}
	if err := k.ConsensusParamsKeeper.ParamsStore.Set(ctx, consensusParams); err != nil {
		return fmt.Errorf("failed to set consensus params: %w", err)
	}

	// update staking params
	stakingParams, err := k.StakingKeeper.GetParams(ctx)
	if err != nil {
		return fmt.Errorf("failed to get staking params: %w", err)
	}

	stakingParams.MinCommissionRate = MainnetBabyMinCommissionRate

	if err := stakingParams.Validate(); err != nil {
		return fmt.Errorf("failed to validate staking params: %w", err)
	}
	if err := k.StakingKeeper.SetParams(ctx, stakingParams); err != nil {
		return fmt.Errorf("failed to set staking params: %w", err)
	}

	// update distribution params
	distributionParams, err := k.DistrKeeper.Params.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get distribution params: %w", err)
	}

	distributionParams.CommunityTax = MainnetCommunityTax
	distributionParams.WithdrawAddrEnabled = MainnetWithdrawalAddrEnabled

	if err := distributionParams.ValidateBasic(); err != nil {
		return fmt.Errorf("failed to validate distribution params: %w", err)
	}
	if err := k.DistrKeeper.Params.Set(ctx, distributionParams); err != nil {
		return fmt.Errorf("failed to set distribution params: %w", err)
	}

	// update btc checkpoint params
	btcCheckpointParams := k.BtcCheckpointKeeper.GetParams(ctx)

	btcCheckpointParams.BtcConfirmationDepth = MainnetBTCConfirmationDepth
	btcCheckpointParams.CheckpointFinalizationTimeout = MainnetBTCFinalizationDepth

	if err := btcCheckpointParams.Validate(); err != nil {
		return fmt.Errorf("failed to validate btc checkpoint params: %w", err)
	}
	if err := k.BtcCheckpointKeeper.SetParams(ctx, btcCheckpointParams); err != nil {
		return fmt.Errorf("failed to set btc checkpoint params: %w", err)
	}

	return nil
}
