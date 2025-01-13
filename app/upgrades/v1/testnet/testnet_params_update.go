package testnet

import (
	"encoding/hex"
	"fmt"
	"time"

	"cosmossdk.io/math"
	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/app/keepers"
	appparams "github.com/babylonlabs-io/babylon/app/params"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	// Governance params
	TestnetVotingPeriod          = 24 * time.Hour
	TestnetExpeditedVotingPeriod = 12 * time.Hour
	// 10 BBN
	TestnetMinDeposit = sdk.NewCoin(appparams.DefaultBondDenom, math.NewInt(10000000))
	// 20 BBN
	TestnetExpeditedMinDeposit = sdk.NewCoin(appparams.DefaultBondDenom, math.NewInt(20000000))
	// Consensus params
	TestnetBlockGasLimit = int64(250000000)
	// Staking params
	TestnetMinCommissionRate, _ = sdkmath.LegacyNewDecFromStr("0.03")
	// Distribution params
	TestnetCommunityTax, _ = sdkmath.LegacyNewDecFromStr("0.001")
	// BTC checkpoint params
	TestnetBTCCheckpointTag = hex.EncodeToString([]byte("bbt5"))
	// Additional allow address to BTC light client
	TestnetReporterAllowAddress = "bbn1cferwuxd95mdnyh4qnptahmzym0xt9sp9asqnw"
)

// TestnetParamUpgrade make updates to specific params of specific modules
func TestnetParamUpgrade(ctx sdk.Context, k *keepers.AppKeepers) error {
	// update gov params
	govParams, err := k.GovKeeper.Params.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get gov params: %w", err)
	}

	govParams.VotingPeriod = &TestnetVotingPeriod
	govParams.ExpeditedVotingPeriod = &TestnetExpeditedVotingPeriod
	govParams.MinDeposit = []sdk.Coin{
		TestnetMinDeposit,
	}
	govParams.ExpeditedMinDeposit = []sdk.Coin{
		TestnetExpeditedMinDeposit,
	}

	if err := k.GovKeeper.Params.Set(ctx, govParams); err != nil {
		return fmt.Errorf("failed to set gov params: %w", err)
	}

	// update consensus params
	consensusParams, err := k.ConsensusParamsKeeper.ParamsStore.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get consensus params: %w", err)
	}

	consensusParams.Block.MaxGas = TestnetBlockGasLimit

	if err := k.ConsensusParamsKeeper.ParamsStore.Set(ctx, consensusParams); err != nil {
		return fmt.Errorf("failed to set consensus params: %w", err)
	}

	// update staking params
	stakingParams, err := k.StakingKeeper.GetParams(ctx)
	if err != nil {
		return fmt.Errorf("failed to get staking params: %w", err)
	}

	stakingParams.MinCommissionRate = TestnetMinCommissionRate

	if err := k.StakingKeeper.SetParams(ctx, stakingParams); err != nil {
		return fmt.Errorf("failed to set staking params: %w", err)
	}

	// update distribution params
	distributionParams, err := k.DistrKeeper.Params.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get distribution params: %w", err)
	}

	distributionParams.CommunityTax = TestnetCommunityTax

	if err := k.DistrKeeper.Params.Set(ctx, distributionParams); err != nil {
		return fmt.Errorf("failed to set distribution params: %w", err)
	}

	// update btc checkpoint tag
	btcCheckpointParams := k.BtcCheckpointKeeper.GetParams(ctx)

	btcCheckpointParams.CheckpointTag = TestnetBTCCheckpointTag

	if err := k.BtcCheckpointKeeper.SetParams(ctx, btcCheckpointParams); err != nil {
		return fmt.Errorf("failed to set btc checkpoint params: %w", err)
	}

	// btc light client allow address
	btcLCParams := k.BTCLightClientKeeper.GetParams(ctx)

	btcLCParams.InsertHeadersAllowList = append(btcLCParams.InsertHeadersAllowList, TestnetReporterAllowAddress)

	if err := k.BTCLightClientKeeper.SetParams(ctx, btcLCParams); err != nil {
		return fmt.Errorf("failed to set btc light client params: %w", err)
	}

	return nil
}
