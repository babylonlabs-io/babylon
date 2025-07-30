package keeper

import (
	"context"
	"errors"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	bbntypes "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/babylonlabs-io/babylon/v3/x/incentive/types"
)

// AddFinalityProviderRewardsForBtcDelegations gets the current finality provider rewards
// and adds rewards to it, without increasing the finality provider period
// it also does not initiliaze the FP, so it must have been initialized prior
// to adding rewards. In the sense that a FP would first receive active delegations sats
// be properly initialized (creates current and historical reward structures in the store)
// than will start to receive rewards for contributing.
func (k Keeper) AddFinalityProviderRewardsForBtcDelegations(ctx context.Context, fp sdk.AccAddress, rwd sdk.Coins) error {
	fpCurrentRwd, err := k.GetFinalityProviderCurrentRewards(ctx, fp)
	if err != nil {
		return err
	}
	if !fpCurrentRwd.TotalActiveSat.IsPositive() {
		return types.ErrFPCurrentRewardsWithoutVotingPower.Wrapf("fp %s doesn't have positive voting power", fp.String())
	}

	if err := fpCurrentRwd.AddRewards(rwd); err != nil {
		return err
	}
	return k.setFinalityProviderCurrentRewards(ctx, fp, fpCurrentRwd)
}

// BtcDelegationActivated adds new amount of active satoshi to the delegation
// and finality provider. Since it modifies the amount staked, it triggers
// the creation of a new period, which initializes the FP, creates
// historical reward tracker, withdraw the BTC delegation rewards to gauge
// and initializes a new delegation with the just ended period.
func (k Keeper) BtcDelegationActivated(ctx context.Context, fp, del sdk.AccAddress, sat sdkmath.Int) error {
	return k.btcDelegationModifiedWithPreInitDel(ctx, fp, del, func(ctx context.Context, fp, del sdk.AccAddress) error {
		return k.addDelegationSat(ctx, fp, del, sat)
	})
}

// BtcDelegationUnbonded it modifies the total amount of satoshi staked
// for the delegation (fp, del) and for the finality provider by subtracting.
// Since it modifies the active amount it triggers the increment of fp period,
// creationg of new historical reward, withdraw of rewards to gauge
// and initialization of a new delegation.
// It errors out if the unbond amount is higher than the total amount staked.
func (k Keeper) BtcDelegationUnbonded(ctx context.Context, fp, del sdk.AccAddress, sat sdkmath.Int) error {
	return k.btcDelegationModifiedWithPreInitDel(ctx, fp, del, func(ctx context.Context, fp, del sdk.AccAddress) error {
		return k.subDelegationSat(ctx, fp, del, sat)
	})
}

// FpSlashed a slashed finality provider should withdraw all the rewards
// available to it, by iterating over all the delegations for this FP
// and sending to the gauge. After the rewards are removed, it should
// delete every rewards tracker value in the store related to this slashed
// finality provider.
func (k Keeper) FpSlashed(ctx context.Context, fp sdk.AccAddress) error {
	// finalize the period to get a new history with the current rewards available
	endedPeriod, err := k.IncrementFinalityProviderPeriod(ctx, fp)
	if err != nil {
		return err
	}

	// remove all the rewards available from the just ended period
	keysBtcDelRwdTracker := make([][]byte, 0)
	if err := k.IterateBTCDelegationRewardsTracker(ctx, fp, func(fp, del sdk.AccAddress) error {
		keysBtcDelRwdTracker = append(keysBtcDelRwdTracker, del.Bytes())
		return k.CalculateBTCDelegationRewardsAndSendToGauge(ctx, fp, del, endedPeriod)
	}); err != nil {
		return err
	}

	// delete all reward tracker that correlates with the slashed finality provider.
	k.deleteKeysFromBTCDelegationRewardsTracker(ctx, fp, keysBtcDelRwdTracker)
	k.deleteAllFromFinalityProviderRwd(ctx, fp)
	return nil
}

// sendAllBtcRewardsToGauge iterates over all the finality providers associated
// with the delegator and withdraw the rewards available to the gauge.
// This creates new periods for each delegation and finality provider.
func (k Keeper) sendAllBtcRewardsToGauge(ctx context.Context, del sdk.AccAddress) error {
	return k.iterBtcDelegationsByDelegator(ctx, del, func(del, fp sdk.AccAddress) error {
		return k.btcDelegationModified(ctx, fp, del)
	})
}

// btcDelegationModified just calls the BTC delegation modified without
// any action to modify the delegation prior to initialization.
// this could also be called SendBtcDelegationRewardsToGauge.
func (k Keeper) btcDelegationModified(
	ctx context.Context,
	fp, del sdk.AccAddress,
) error {
	return k.btcDelegationModifiedWithPreInitDel(ctx, fp, del, func(ctx context.Context, fp, del sdk.AccAddress) error { return nil })
}

// btcDelegationModifiedWithPreInitDel does the procedure when a BTC delegation has
// some modification in its total amount of active satoshi staked. This function
// increments the finality provider period (that creates a new historical) with
// the ended period, calculates the delegation reward and send to the gauge
// and calls a function prior (preInitializeDelegation) to initialize a new
// BTC delegation, which is useful to apply subtract or add the total
// amount staked by the delegation and FP.
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

// CalculateBTCDelegationRewardsAndSendToGauge calculates the rewards of the delegator based on the
// StartPeriodCumulativeReward and the received endPeriod and sends to the delegator gauge.
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

	k.accumulateRewardGauge(ctx, types.BTC_STAKER, del, rewards)
	return nil
}

// CalculateBTCDelegationRewards calculates the rewards entitled for this delegation
// from the starting period cumulative reward and the ending period received as parameter
// It returns the amount of rewards without decimals (it removes the DecimalAccumulatedRewards).
func (k Keeper) CalculateBTCDelegationRewards(ctx context.Context, fp, del sdk.AccAddress, endPeriod uint64) (sdk.Coins, error) {
	btcDelRwdTracker, err := k.GetBTCDelegationRewardsTracker(ctx, fp, del)
	if err != nil {
		return sdk.Coins{}, err
	}

	if btcDelRwdTracker.TotalActiveSat.IsZero() {
		return sdk.NewCoins(), nil
	}

	return k.calculateDelegationRewardsBetween(ctx, fp, btcDelRwdTracker, endPeriod)
}

// calculateDelegationRewardsBetween calculate the rewards accured of a delegation between
// two period, the endingPeriod received in param and the StartPeriodCumulativeReward of
// the BTCDelegationRewardsTracker. It gets the CumulativeRewardsPerSat of the ending
// period and subtracts the CumulativeRewardsPerSat of the starting period
// that give the total amount of rewards that one satoshi is entitle to receive
// in rewards between those two period. To get the amount this delegation should
// receive, it multiplies by the total amount of active satoshi this delegation has.
// One note, before give out the rewards it quotes by the DecimalAccumulatedRewards
// to get it ready to be sent out to the delegator reward gauge.
func (k Keeper) calculateDelegationRewardsBetween(
	ctx context.Context,
	fp sdk.AccAddress,
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

	rewardsWithDecimals, valid := differenceWithDecimals.SafeMulInt(btcDelRwdTracker.TotalActiveSat)
	if !valid {
		return sdk.Coins{}, types.ErrInvalidAmount.Wrap("math overflow")
	}
	// note: necessary to truncate so we don't allow withdrawing more rewardsWithDecimals than owed
	// QuoInt already truncates
	rewards := rewardsWithDecimals.QuoInt(types.DecimalAccumulatedRewards)
	return rewards, nil
}

// IncrementFinalityProviderPeriod gets or initializes the finality provider,
// increases the period from the current FP rewards and empty the rewards.
// It also creates a new historical with the ended period and sets the rewards
// of the newly historical period as the amount from the previous historical
// plus the amount of rewards that each satoshi staked is entitled to receive.
// The rewards in the historical are stored with multiplied decimals
// (DecimalAccumulatedRewards) to increase precision, and need to be
// reduced when the rewards are calculated in calculateDelegationRewardsBetween
// prior to send out to the delegator gauge.
func (k Keeper) IncrementFinalityProviderPeriod(ctx context.Context, fp sdk.AccAddress) (endedPeriod uint64, err error) {
	// IncrementValidatorPeriod
	//    gets the current rewards and send to historical the current period (the rewards are stored as "shares" which means the amount of rewards per satoshi)
	//    sets new empty current rewards with new period
	fpCurrentRwd, err := k.GetFinalityProviderCurrentRewards(ctx, fp)
	if err != nil {
		if !errors.Is(err, types.ErrFPCurrentRewardsNotFound) {
			return 0, err
		}

		// initialize Validator and return 1 as ended period due
		// to the created historical FP rewards starts at period 0
		if _, err := k.initializeFinalityProvider(ctx, fp); err != nil {
			return 0, err
		}
		return 1, nil
	}

	currentRewardsPerSat := sdk.NewCoins()
	if !fpCurrentRwd.TotalActiveSat.IsZero() {
		currentRewardsPerSatWithDecimals, err := bbntypes.CoinsSafeMulInt(fpCurrentRwd.CurrentRewards, types.DecimalAccumulatedRewards)
		if err != nil {
			return 0, err
		}

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

	// initiates a new period with empty rewards and the same amount of active sat
	newCurrentRwd := types.NewFinalityProviderCurrentRewards(sdk.NewCoins(), fpCurrentRwd.Period+1, fpCurrentRwd.TotalActiveSat)
	if err := k.setFinalityProviderCurrentRewards(ctx, fp, newCurrentRwd); err != nil {
		return 0, err
	}

	return fpCurrentRwd.Period, nil
}

// initializeFinalityProvider initializes a new finality provider current rewards at period 1, empty rewards and zero sats
// and also creates a new historical rewards at period 0 and zero rewards as well.
// It does not verifies if it exists prior to overwrite, who calls it needs to verify.
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

// initializeBTCDelegation creates a new BTCDelegationRewardsTracker from the
// previous acumulative rewards period of the finality provider. This function
// should be called right after a BTC delegator withdraw his rewards (in our
// case send the rewards to the reward gauge). Reminder that at every new
// modification to the amount of satoshi staked from this btc delegator to
// this finality provider (activivation or unbonding) of BTC delegations, it
// should withdraw all rewards (send to gauge) and initialize a new BTCDelegationRewardsTracker.
// TODO: add reference count to keep track of possible prunning state of val rewards
func (k Keeper) initializeBTCDelegation(ctx context.Context, fp, del sdk.AccAddress) error {
	// period has already been incremented prior to call this function
	// it is needed to store the period ended by this delegation action
	// as a starting point of the delegation rewards calculation
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
