package keeper

import (
	"context"
	"errors"

	"github.com/babylonlabs-io/babylon/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	sdkmath "cosmossdk.io/math"
)

var (
	// it is needed to add decimal points when reducing the rewards amount
	// per sat to latter when giving out the rewards to the gauge, reduce
	// the decimal points back, currently 20 decimal points are being added
	// the sdkmath.Int holds a big int which support up to 2^256 integers
	DecimalAccumulatedRewards, _ = sdkmath.NewIntFromString("100000000000000000000")
)

func (k Keeper) BtcDelegationActivated(ctx context.Context, fp, del sdk.AccAddress, sat uint64) error {
	amtSat := sdkmath.NewIntFromUint64(sat)
	return k.btcDelegationModifiedWithPreInitDel(ctx, fp, del, func(ctx context.Context, fp, del sdk.AccAddress) error {
		return k.AddDelegationSat(ctx, fp, del, amtSat)
	})
}

func (k Keeper) BtcDelegationUnbonded(ctx context.Context, fp, del sdk.AccAddress, sat uint64) error {
	amtSat := sdkmath.NewIntFromUint64(sat)
	return k.btcDelegationModifiedWithPreInitDel(ctx, fp, del, func(ctx context.Context, fp, del sdk.AccAddress) error {
		return k.subDelegationSat(ctx, fp, del, amtSat)
	})
}

func (k Keeper) FpSlashed(ctx context.Context, fp sdk.AccAddress) error {
	endedPeriod, err := k.IncrementFinalityProviderPeriod(ctx, fp)
	if err != nil {
		return err
	}

	// remove all the rewards available from the ended periods
	keysBtcDelRwdTracker := make([][]byte, 0)
	if err := k.IterateBTCDelegationRewardsTracker(ctx, fp, func(fp, del sdk.AccAddress) error {
		keysBtcDelRwdTracker = append(keysBtcDelRwdTracker, del.Bytes())
		return k.CalculateBTCDelegationRewardsAndSendToGauge(ctx, fp, del, endedPeriod)
	}); err != nil {
		return err
	}

	// delete all reward tracer that correlates with the slashed finality provider.
	k.deleteKeysFromBTCDelegationRewardsTracker(ctx, fp, keysBtcDelRwdTracker)
	k.deleteAllFromFinalityProviderRwd(ctx, fp)
	return nil
}

func (k Keeper) SendBtcDelegationRewardsToGauge(ctx context.Context, fp, del sdk.AccAddress) error {
	return k.btcDelegationModified(ctx, fp, del)
}

func (k Keeper) sendAllBtcRewardsToGauge(ctx context.Context, del sdk.AccAddress) error {
	return k.iterBtcDelegationsByDelegator(ctx, del, func(del, fp sdk.AccAddress) error {
		return k.SendBtcDelegationRewardsToGauge(ctx, fp, del)
	})
}

func (k Keeper) btcDelegationModified(
	ctx context.Context,
	fp, del sdk.AccAddress,
) error {
	return k.btcDelegationModifiedWithPreInitDel(ctx, fp, del, func(ctx context.Context, fp, del sdk.AccAddress) error { return nil })
}

func (k Keeper) btcDelegationModifiedWithPreInitDel(
	ctx context.Context,
	fp, del sdk.AccAddress,
	preInitializeDelegation func(ctx context.Context, fp, del sdk.AccAddress) error,
) error {
	endedPeriod, err := k.IncrementFinalityProviderPeriod(ctx, fp)
	if err != nil {
		return err
	}

	if err := k.CalculateBTCDelegationRewardsAndSendToGauge(ctx, fp, del, endedPeriod); err != nil {
		return err
	}

	if err := preInitializeDelegation(ctx, fp, del); err != nil {
		return err
	}

	return k.initializeBTCDelegation(ctx, fp, del)
}

func (k Keeper) CalculateBTCDelegationRewardsAndSendToGauge(ctx context.Context, fp, del sdk.AccAddress, endPeriod uint64) error {
	rewards, err := k.CalculateBTCDelegationRewards(ctx, fp, del, endPeriod)
	if err != nil {
		if !errors.Is(err, types.ErrBTCDelegationRewardsTrackerNotFound) {
			return err
		}
		rewards = sdk.NewCoins()
	}

	if rewards.IsZero() {
		return nil
	}

	k.accumulateRewardGauge(ctx, types.BTCDelegationType, del, rewards)
	return nil
}

func (k Keeper) CalculateBTCDelegationRewards(ctx context.Context, fp, del sdk.AccAddress, endPeriod uint64) (sdk.Coins, error) {
	btcDelRwdTracker, err := k.GetBTCDelegationRewardsTracker(ctx, fp, del)
	if err != nil {
		return sdk.Coins{}, err
	}

	if btcDelRwdTracker.TotalActiveSat.IsZero() {
		return sdk.NewCoins(), nil
	}

	return k.calculateDelegationRewardsBetween(ctx, fp, del, btcDelRwdTracker, endPeriod)
}

// calculate the rewards accrued by a delegation between two periods
func (k Keeper) calculateDelegationRewardsBetween(
	ctx context.Context,
	fp, del sdk.AccAddress,
	btcDelRwdTracker types.BTCDelegationRewardsTracker,
	endingPeriod uint64,
) (sdk.Coins, error) {
	if btcDelRwdTracker.StartPeriodCumulativeReward > endingPeriod {
		panic("startingPeriod cannot be greater than endingPeriod")
	}

	// return staking * (ending - starting)
	starting, err := k.GetFinalityProviderHistoricalRewards(ctx, fp, btcDelRwdTracker.StartPeriodCumulativeReward)
	if err != nil {
		return sdk.Coins{}, err
	}

	ending, err := k.GetFinalityProviderHistoricalRewards(ctx, fp, endingPeriod)
	if err != nil {
		return sdk.Coins{}, err
	}

	// creates the differenceWithDecimals amount of rewards (ending - starting) periods
	// this differenceWithDecimals is the amount of rewards entitled per satoshi active stake
	differenceWithDecimals := ending.CumulativeRewardsPerSat.Sub(starting.CumulativeRewardsPerSat...)
	if differenceWithDecimals.IsAnyNegative() {
		panic("negative rewards should not be possible")
	}

	rewardsWithDecimals := differenceWithDecimals.MulInt(btcDelRwdTracker.TotalActiveSat)
	// note: necessary to truncate so we don't allow withdrawing more rewardsWithDecimals than owed
	// QuoInt already truncates
	rewards := rewardsWithDecimals.QuoInt(DecimalAccumulatedRewards)
	return rewards, nil
}

// IncrementFinalityProviderPeriod
func (k Keeper) IncrementFinalityProviderPeriod(ctx context.Context, fp sdk.AccAddress) (endedPeriod uint64, err error) {
	// IncrementValidatorPeriod
	//    gets the current rewards and send to historical the current period (the rewards are stored as "shares" which means the amount of rewards per satoshi)
	//    sets new empty current rewards with new period
	fpCurrentRwd, err := k.GetFinalityProviderCurrentRewards(ctx, fp)
	if err != nil {
		if !errors.Is(err, types.ErrFPCurrentRewardsNotFound) {
			return 0, err
		}

		// initialize Validator and return 1 as ended period
		// the ended period is 1 because the just created historical sits at 0
		if _, err := k.initializeFinalityProvider(ctx, fp); err != nil {
			return 0, err
		}
		return 1, nil
	}

	currentRewardsPerSat := sdk.NewCoins()
	if !fpCurrentRwd.TotalActiveSat.IsZero() {
		currentRewardsPerSatWithDecimals := fpCurrentRwd.CurrentRewards.MulInt(DecimalAccumulatedRewards)
		currentRewardsPerSat = currentRewardsPerSatWithDecimals.QuoInt(fpCurrentRwd.TotalActiveSat)
	}

	fpHistoricalRwd, err := k.GetFinalityProviderHistoricalRewards(ctx, fp, fpCurrentRwd.Period-1)
	if err != nil {
		return 0, err
	}

	newFpHistoricalRwd := types.NewFinalityProviderHistoricalRewards(fpHistoricalRwd.CumulativeRewardsPerSat.Add(currentRewardsPerSat...))
	if err := k.setFinalityProviderHistoricalRewards(ctx, fp, fpCurrentRwd.Period, newFpHistoricalRwd); err != nil {
		return 0, err
	}

	// initiates a new period with empty rewards and the same amount of active sat (this value should be updated latter if needed)
	newCurrentRwd := types.NewFinalityProviderCurrentRewards(sdk.NewCoins(), fpCurrentRwd.Period+1, fpCurrentRwd.TotalActiveSat)
	if err := k.setFinalityProviderCurrentRewards(ctx, fp, newCurrentRwd); err != nil {
		return 0, err
	}

	return fpCurrentRwd.Period, nil
}

func (k Keeper) initializeFinalityProvider(ctx context.Context, fp sdk.AccAddress) (types.FinalityProviderCurrentRewards, error) {
	// historical rewards starts at the period 0
	err := k.setFinalityProviderHistoricalRewards(ctx, fp, 0, types.NewFinalityProviderHistoricalRewards(sdk.NewCoins()))
	if err != nil {
		return types.FinalityProviderCurrentRewards{}, err
	}

	// set current rewards (starting at period 1)
	newFp := types.NewFinalityProviderCurrentRewards(sdk.NewCoins(), 1, sdkmath.ZeroInt())
	return newFp, k.setFinalityProviderCurrentRewards(ctx, fp, newFp)
}

// initializeBTCDelegation creates a new BTCDelegationRewardsTracker from the previous acumulative rewards
// period of the finality provider and it should be called right after a BTC delegator withdraw his rewards
// (in our case send the rewards to the reward gauge). Reminder that at every new modification to the amount
// of satoshi staked from this btc delegator to this finality provider (activivation or unbonding) of BTC
// delegations, it should withdraw all rewards (send to gauge) and initialize a new BTCDelegationRewardsTracker.
// TODO: add reference count to keep track of possible prunning state of val rewards
func (k Keeper) initializeBTCDelegation(ctx context.Context, fp, del sdk.AccAddress) error {
	// period has already been incremented - we want to store the period ended by this delegation action
	valCurrentRewards, err := k.GetFinalityProviderCurrentRewards(ctx, fp)
	if err != nil {
		return err
	}
	previousPeriod := valCurrentRewards.Period - 1

	btcDelRwdTracker, err := k.GetBTCDelegationRewardsTracker(ctx, fp, del)
	if err != nil {
		return err
	}

	rwd := types.NewBTCDelegationRewardsTracker(previousPeriod, btcDelRwdTracker.TotalActiveSat)
	return k.setBTCDelegationRewardsTracker(ctx, fp, del, rwd)
}

func (k Keeper) AddFinalityProviderRewardsForBtcDelegations(ctx context.Context, fp sdk.AccAddress, rwd sdk.Coins) error {
	fpCurrentRwd, err := k.GetFinalityProviderCurrentRewards(ctx, fp)
	if err != nil {
		return err
	}

	fpCurrentRwd.AddRewards(rwd)
	return k.setFinalityProviderCurrentRewards(ctx, fp, fpCurrentRwd)
}

func (k Keeper) AddDelegationSat(ctx context.Context, fp, del sdk.AccAddress, amt sdkmath.Int) error {
	btcDelRwdTracker, err := k.GetBTCDelegationRewardsTracker(ctx, fp, del)
	if err != nil {
		if !errors.Is(err, types.ErrBTCDelegationRewardsTrackerNotFound) {
			return err
		}

		// first delegation to this pair (fp, del), can start as 0 previous period as it
		// will be updated soon as initilize btc delegation
		btcDelRwdTracker = types.NewBTCDelegationRewardsTracker(0, sdkmath.ZeroInt())
	}

	btcDelRwdTracker.AddTotalActiveSat(amt)
	if err := k.setBTCDelegationRewardsTracker(ctx, fp, del, btcDelRwdTracker); err != nil {
		return err
	}

	return k.addFinalityProviderStaked(ctx, fp, amt)
}
