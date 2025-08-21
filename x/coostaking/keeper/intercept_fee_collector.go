package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

// HandleCoinsInFeeCollector intercepts a portion of coins in fee collector, and distributes
// them to coostaking module account.
// It is invoked upon every `BeginBlock`.
// adapted from https://github.com/babylonlabs-io/babylon/blob/main/x/incentive/abci.go#L14
func (k Keeper) HandleCoinsInFeeCollector(ctx context.Context) error {
	// find the fee collector account
	feeCollector := k.accK.GetModuleAccount(ctx, k.feeCollectorName)
	// get all balances in the fee collector account,
	// where the balance includes minted tokens in the previous block
	feesCollectedInt := k.bankK.GetAllBalances(ctx, feeCollector.GetAddress())

	// don't intercept if there is no fee in fee collector account
	if !feesCollectedInt.IsAllPositive() {
		return nil
	}

	coostakingPortion := k.GetParams(ctx).CoostakingPortion
	coostakingRewards := ictvtypes.GetCoinsPortion(feesCollectedInt, coostakingPortion)
	return k.AccumulateCoostakingRewards(ctx, coostakingRewards)
}

// AccumulateCoostakingRewards gets funds from fee collector
func (k Keeper) AccumulateCoostakingRewards(ctx context.Context, coostakingRewards sdk.Coins) error {
	// transfer the BTC staking reward from fee collector account to coostaking module account
	err := k.bankK.SendCoinsFromModuleToModule(ctx, k.feeCollectorName, types.ModuleName, coostakingRewards)
	if err != nil {
		return err
	}

	err = k.AddCurrentRewards(ctx, coostakingRewards)
	if err != nil {
		return err
	}
	return nil
}
