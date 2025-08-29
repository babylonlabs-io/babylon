package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

// HandleCoinsInFeeCollector intercepts a portion of coins in fee collector, and distributes
// them to coostaking module account.
// It is invoked upon every `BeginBlock`.
// The order of begin block to get funds from the fee_collector should be:
// Incentives, Coostaking and Distribution.
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

	coostakingRewards := ictvtypes.GetCoinsPortion(feesCollectedInt, params.CoostakingPortion)
	return k.accumulateCoostakingRewards(ctx, coostakingRewards)
}

// allocateValidatorsRewards allocates rewards to validators based on their voting power
// It transfers the rewards to the distribution module account and updates the validator's commission.
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

		powerFraction := math.LegacyNewDec(vote.Validator.Power).QuoTruncate(math.LegacyNewDec(totalPwr))
		// get validator reward based on voting power
		rwd := valsRwds.MulDecTruncate(powerFraction)

		// set validator commission == 1 to allocate all as rewards for the validator (accumulated in commission)
		// and 0 for the delegators
		parsedVal, ok := validator.(stktypes.Validator)
		if !ok {
			return errorsmod.Wrapf(sdkerrors.ErrInvalidType, "expected %T, got %T", parsedVal, validator)
		}
		parsedVal.Commission.Rate = math.LegacyOneDec()
		err = k.distrK.AllocateTokensToValidator(ctx, validator, rwd)
		if err != nil {
			return err
		}
	}
	return nil
}

// accumulateCoostakingRewards gets funds from fee collector
func (k Keeper) accumulateCoostakingRewards(ctx context.Context, coostakingRewards sdk.Coins) error {
	if !coostakingRewards.IsAllPositive() {
		return nil
	}

	// transfer the BTC staking reward from fee collector account to coostaking module account
	err := k.bankK.SendCoinsFromModuleToModule(ctx, k.feeCollectorName, types.ModuleName, coostakingRewards)
	if err != nil {
		return err
	}

	return k.AddCurrentRewards(ctx, coostakingRewards)
}
