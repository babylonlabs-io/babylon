package mainnet

import (
	"fmt"
	"time"

	"cosmossdk.io/math"
	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v3/app/keepers"
	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	"github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	// Slashing params
	MinSignedBlocks, _    = sdkmath.LegacyNewDecFromStr("0.6")
	SlashFractionDowntime = sdkmath.LegacyZeroDec()
	DowntimeJailDuration  = 300 * time.Second

	// Governance params
	VotingPeriod = 3 * 24 * time.Hour
	// 50k BABY
	MinDeposit = sdk.NewCoin(appparams.DefaultBondDenom, math.NewInt(50_000_000000))
	// 200k BABY
	MaxDepositPeriod          = 14 * 24 * time.Hour
	ExpeditedVotingPeriod     = 24 * time.Hour
	ExpeditedMinDeposit       = sdk.NewCoin(appparams.DefaultBondDenom, math.NewInt(200_000_000000))
	MinInitialDepositRatio, _ = sdkmath.LegacyNewDecFromStr("0.1")

	// Consensus params
	BlockGasLimit = int64(300000000)
	// Staking params
	BabyMinCommissionRate, _ = sdkmath.LegacyNewDecFromStr("0.03")

	// BTC checkpoint params
	BTCConfirmationDepth   = uint32(30)
	BTCFinalizationTimeout = uint32(300)

	// Distribution params
	CommunityTax          = sdkmath.LegacyZeroDec()
	WithdrawalAddrEnabled = true
)

// ParamUpgrade make updates to specific params of specific modules
func ParamUpgrade(ctx sdk.Context, k *keepers.AppKeepers) error {
	// update slashing params
	slashingParams, err := k.SlashingKeeper.GetParams(ctx)
	if err != nil {
		return fmt.Errorf("failed to get slash params: %w", err)
	}

	slashingParams.MinSignedPerWindow = MinSignedBlocks
	slashingParams.DowntimeJailDuration = DowntimeJailDuration
	slashingParams.SlashFractionDowntime = SlashFractionDowntime

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

	govParams.VotingPeriod = &VotingPeriod
	govParams.ExpeditedVotingPeriod = &ExpeditedVotingPeriod
	govParams.MaxDepositPeriod = &MaxDepositPeriod
	govParams.MinDeposit = []sdk.Coin{
		MinDeposit,
	}
	govParams.ExpeditedMinDeposit = []sdk.Coin{
		ExpeditedMinDeposit,
	}
	govParams.MinInitialDepositRatio = MinInitialDepositRatio.String()

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

	consensusParams.Block.MaxGas = BlockGasLimit
	if consensusParams.Version == nil {
		consensusParams.Version = &types.VersionParams{
			App: 1,
		}
	}
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

	stakingParams.MinCommissionRate = BabyMinCommissionRate

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

	distributionParams.CommunityTax = CommunityTax
	distributionParams.WithdrawAddrEnabled = WithdrawalAddrEnabled

	if err := distributionParams.ValidateBasic(); err != nil {
		return fmt.Errorf("failed to validate distribution params: %w", err)
	}
	if err := k.DistrKeeper.Params.Set(ctx, distributionParams); err != nil {
		return fmt.Errorf("failed to set distribution params: %w", err)
	}

	// update btc checkpoint params
	btcCheckpointParams := k.BtcCheckpointKeeper.GetParams(ctx)

	btcCheckpointParams.BtcConfirmationDepth = BTCConfirmationDepth
	btcCheckpointParams.CheckpointFinalizationTimeout = BTCFinalizationTimeout

	if err := btcCheckpointParams.Validate(); err != nil {
		return fmt.Errorf("failed to validate btc checkpoint params: %w", err)
	}
	if err := k.BtcCheckpointKeeper.SetParams(ctx, btcCheckpointParams); err != nil {
		return fmt.Errorf("failed to set btc checkpoint params: %w", err)
	}

	return nil
}
