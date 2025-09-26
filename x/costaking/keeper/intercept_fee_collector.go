package keeper

import (
	"context"
	"strconv"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

// HandleCoinsInFeeCollector intercepts a portion of coins in fee collector, and distributes
// them to costaking module account.
// It is invoked upon every `BeginBlock`.
// The order of begin block to get funds from the fee_collector should be:
// Incentives, costaking and Distribution.
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

	params := k.GetParams(ctx)

	valDirectRwds := ictvtypes.GetCoinsPortion(feesCollectedInt, params.ValidatorsPortion)
	if err := k.allocateValidatorsRewards(ctx, valDirectRwds); err != nil {
		return err
	}

	costakingRewards := ictvtypes.GetCoinsPortion(feesCollectedInt, params.CostakingPortion)
	return k.accumulateCostakingRewards(ctx, costakingRewards)
}

// allocateValidatorsRewards allocates rwds proportionally to validators' voting power,
// but credits them directly to validators' commission, not to delegators.
//
// Implementation detail:
//
//	We reuse distribution’s AllocateTokensToValidator, which pays out according to the
//	validator’s commission rate. To force “all to validator, none to delegators”,
//	we pass a **temporary copy** of the validator with Commission.Rate = 1.0.
//	This copy is NOT written to state. Only the distribution accounting is affected for this allocation.
func (k Keeper) allocateValidatorsRewards(ctx context.Context, rwds sdk.Coins) error {
	if !rwds.IsAllPositive() {
		return nil
	}
	// parse rewards to decCoins
	valsRwds := sdk.NewDecCoinsFromCoins(rwds...)
	goCtx := sdk.UnwrapSDKContext(ctx)
	bondedVotes := goCtx.VoteInfos()
	// determine the total power signing the block
	var totalPwr int64
	for _, voteInfo := range bondedVotes {
		totalPwr += voteInfo.Validator.Power
	}

	// safety check
	if totalPwr == 0 {
		return nil
	}
	totalPwrDec := math.LegacyNewDec(totalPwr)

	// Transfer rewards to the distribution module account
	// 'cause these direct rewards will be allocated to the validators commission
	err := k.bankK.SendCoinsFromModuleToModule(ctx, k.feeCollectorName, distrtypes.ModuleName, rwds)
	if err != nil {
		return err
	}

	for _, vote := range bondedVotes {
		validator, err := k.stkK.ValidatorByConsAddr(ctx, vote.Validator.Address)
		if err != nil {
			return err
		}

		powerFraction := math.LegacyNewDec(vote.Validator.Power).QuoTruncate(totalPwrDec)
		// get validator reward based on voting power
		rwd := valsRwds.MulDecTruncate(powerFraction)

		// set validator commission == 1 to allocate all as rewards for the validator (accumulated in commission)
		// and 0 for the delegators
		updatedVal, ok := validator.(stktypes.Validator)
		// safety check
		if !ok {
			return errorsmod.Wrapf(sdkerrors.ErrInvalidType, "expected %T, got %T", updatedVal, validator)
		}
		updatedVal.Commission.Rate = math.LegacyOneDec()
		if err := k.distrK.AllocateTokensToValidator(ctx, updatedVal, rwd); err != nil {
			return err
		}
	}

	// emit event for direct validator rewards
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeValidatorDirectRewards,
		sdk.NewAttribute(types.AttributeKeyAmount, rwds.String()),
		sdk.NewAttribute(types.AttributeKeyValidatorCount, strconv.Itoa(len(bondedVotes))),
	))

	return nil
}

// accumulateCostakingRewards gets funds from fee collector
func (k Keeper) accumulateCostakingRewards(ctx context.Context, costakingRewards sdk.Coins) error {
	if !costakingRewards.IsAllPositive() {
		return nil
	}

	// transfer the BTC staking reward from fee collector account to costaking module account
	err := k.bankK.SendCoinsFromModuleToModule(ctx, k.feeCollectorName, types.ModuleName, costakingRewards)
	if err != nil {
		return err
	}

	return k.AddRewardsForCostakers(ctx, costakingRewards)
}
