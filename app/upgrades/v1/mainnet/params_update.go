package mainnet

import (
	"fmt"
	"time"

	"cosmossdk.io/math"
	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/app/keepers"
	appparams "github.com/babylonlabs-io/babylon/app/params"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	// Slashing params
	MainnetMinSignedBlocks, _    = sdkmath.LegacyNewDecFromStr("0.6")
	MainnetSlashFractionDowntime = sdkmath.LegacyZeroDec()
	MainnetDowntimeJailDuration  = 300 * time.Second

	// Governance params
	MainnetVotingPeriod = 3 * 24 * time.Hour
	// 50k BBN
	MainnetMinDeposit = sdk.NewCoin(appparams.DefaultBondDenom, math.NewInt(50_000_000000))
	// 200k BBN
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

	if err := k.GovKeeper.Params.Set(ctx, govParams); err != nil {
		return fmt.Errorf("failed to set gov params: %w", err)
	}

	// update consensus params
	consensusParams, err := k.ConsensusParamsKeeper.ParamsStore.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get consensus params: %w", err)
	}

	consensusParams.Block.MaxGas = MainnetBlockGasLimit

	if err := k.ConsensusParamsKeeper.ParamsStore.Set(ctx, consensusParams); err != nil {
		return fmt.Errorf("failed to set consensus params: %w", err)
	}

	// update staking params
	stakingParams, err := k.StakingKeeper.GetParams(ctx)
	if err != nil {
		return fmt.Errorf("failed to get staking params: %w", err)
	}

	stakingParams.MinCommissionRate = MainnetBabyMinCommissionRate

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

	if err := k.DistrKeeper.Params.Set(ctx, distributionParams); err != nil {
		return fmt.Errorf("failed to set distribution params: %w", err)
	}

	// update btc checkpoint params
	btcCheckpointParams := k.BtcCheckpointKeeper.GetParams(ctx)

	btcCheckpointParams.BtcConfirmationDepth = MainnetBTCConfirmationDepth
	btcCheckpointParams.CheckpointFinalizationTimeout = MainnetBTCFinalizationDepth

	if err := k.BtcCheckpointKeeper.SetParams(ctx, btcCheckpointParams); err != nil {
		return fmt.Errorf("failed to set btc checkpoint params: %w", err)
	}

	return nil
}
