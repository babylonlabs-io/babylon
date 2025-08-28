package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

// HandleCoinsInFeeCollector intercepts a portion of coins in fee collector, and distributes
// them to BTC staking gauge of the current height.
// It is invoked upon every `BeginBlock`.
// adapted from https://github.com/cosmos/cosmos-sdk/blob/release/v0.47.x/x/distribution/keeper/allocation.go#L15-L26
func (k Keeper) HandleCoinsInFeeCollector(ctx context.Context) {
	params := k.GetParams(ctx)

	// find the fee collector account
	feeCollector := k.accountKeeper.GetModuleAccount(ctx, k.feeCollectorName)
	// get all balances in the fee collector account,
	// where the balance includes minted tokens in the previous block
	feesCollectedInt := k.bankKeeper.GetAllBalances(ctx, feeCollector.GetAddress())

	// don't intercept if there is no fee in fee collector account
	if !feesCollectedInt.IsAllPositive() {
		return
	}

	// record FP direct rewards for the current height
	fpsRewards := types.GetCoinsPortion(feesCollectedInt, params.FpPortion)

	// record BTC staking gauge for the current height
	btcStakingPortion := params.BTCStakingPortion()
	btcStakingReward := types.GetCoinsPortion(feesCollectedInt, btcStakingPortion)
	// Transfer corresponding amount (fp direct rewards + btc_staking rewards)
	// from fee collector account to incentive module account
	// TODO: maybe we should not transfer reward to BTC staking gauge before BTC staking is activated
	// this is tricky to implement since finality module will depend on incentive and incentive cannot
	// depend on finality module due to cyclic dependency
	k.accumulateRewards(ctx, btcStakingReward, fpsRewards)
}
