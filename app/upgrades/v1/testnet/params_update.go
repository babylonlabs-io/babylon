package testnet

import (
	"encoding/hex"
	"fmt"
	"time"

	"cosmossdk.io/math"
	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v3/app/keepers"
	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	// Governance params
	VotingPeriod          = 24 * time.Hour
	ExpeditedVotingPeriod = 12 * time.Hour
	// 10 BABY
	MinDeposit = sdk.NewCoin(appparams.DefaultBondDenom, math.NewInt(10000000))
	// 20 BABY
	ExpeditedMinDeposit = sdk.NewCoin(appparams.DefaultBondDenom, math.NewInt(20000000))
	// Consensus params
	BlockGasLimit = int64(250000000)
	// Staking params
	MinCommissionRate, _ = sdkmath.LegacyNewDecFromStr("0.03")
	// Distribution params
	CommunityTax, _ = sdkmath.LegacyNewDecFromStr("0.001")
	// BTC checkpoint params
	BTCCheckpointTag = hex.EncodeToString([]byte("bbt5"))
	// Additional allow address to BTC light client
	ReporterAllowAddress = "bbn1cferwuxd95mdnyh4qnptahmzym0xt9sp9asqnw"
)

// ParamUpgrade make updates to specific params of specific modules
func ParamUpgrade(ctx sdk.Context, k *keepers.AppKeepers) error {
	// update gov params
	govParams, err := k.GovKeeper.Params.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get gov params: %w", err)
	}

	govParams.VotingPeriod = &VotingPeriod
	govParams.ExpeditedVotingPeriod = &ExpeditedVotingPeriod
	govParams.MinDeposit = []sdk.Coin{
		MinDeposit,
	}
	govParams.ExpeditedMinDeposit = []sdk.Coin{
		ExpeditedMinDeposit,
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

	if err := k.ConsensusParamsKeeper.ParamsStore.Set(ctx, consensusParams); err != nil {
		return fmt.Errorf("failed to set consensus params: %w", err)
	}

	// update staking params
	stakingParams, err := k.StakingKeeper.GetParams(ctx)
	if err != nil {
		return fmt.Errorf("failed to get staking params: %w", err)
	}

	stakingParams.MinCommissionRate = MinCommissionRate

	if err := k.StakingKeeper.SetParams(ctx, stakingParams); err != nil {
		return fmt.Errorf("failed to set staking params: %w", err)
	}

	// update distribution params
	distributionParams, err := k.DistrKeeper.Params.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get distribution params: %w", err)
	}

	distributionParams.CommunityTax = CommunityTax

	if err := k.DistrKeeper.Params.Set(ctx, distributionParams); err != nil {
		return fmt.Errorf("failed to set distribution params: %w", err)
	}

	// update btc checkpoint tag
	btcCheckpointParams := k.BtcCheckpointKeeper.GetParams(ctx)

	btcCheckpointParams.CheckpointTag = BTCCheckpointTag

	if err := k.BtcCheckpointKeeper.SetParams(ctx, btcCheckpointParams); err != nil {
		return fmt.Errorf("failed to set btc checkpoint params: %w", err)
	}

	// btc light client allow address
	btcLCParams := k.BTCLightClientKeeper.GetParams(ctx)

	btcLCParams.InsertHeadersAllowList = append(btcLCParams.InsertHeadersAllowList, ReporterAllowAddress)

	if err := k.BTCLightClientKeeper.SetParams(ctx, btcLCParams); err != nil {
		return fmt.Errorf("failed to set btc light client params: %w", err)
	}

	return nil
}
