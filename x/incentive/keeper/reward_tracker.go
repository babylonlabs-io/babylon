package keeper

import (
	"context"
	"encoding/binary"
	"errors"

	"cosmossdk.io/store/prefix"
	"github.com/babylonlabs-io/babylon/x/incentive/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	sdkmath "cosmossdk.io/math"
)

func (k Keeper) FpSlashed(ctx context.Context, fp sdk.AccAddress) error {
	// withdrawDelegationRewards
	// Delete all the delegations reward tracker associated with this FP
	// Delete the FP reward tracker
	return nil
}

func (k Keeper) BtcDelegationActivated(ctx context.Context, fp, del sdk.AccAddress, sat uint64) error {
	amtSat := sdkmath.NewIntFromUint64(sat)
	// if btc delegations does not exists
	//   BeforeDelegationCreated
	//     IncrementValidatorPeriod
	//   		initializeDelegation
	//      AddDelegationStaking

	// if btc delegations exists
	//   BeforeDelegationSharesModified
	//     withdrawDelegationRewards
	//       IncrementValidatorPeriod

	// IncrementValidatorPeriod
	//    gets the current rewards and send to historical the current period (the rewards are stored as "shares" which means the amount of rewards per satoshi)
	//    sets new empty current rewards with new period

	endedPeriod, err := k.IncrementFinalityProviderPeriod(ctx, fp)
	if err != nil {
		return err
	}

	rewards, err := k.CalculateDelegationRewards(ctx, fp, del, endedPeriod)
	if err != nil {
		if !errors.Is(err, types.ErrBTCDelegationRewardsTrackerNotFound) {
			return err
		}
		rewards = sdk.NewCoins()
	}

	if !rewards.IsZero() {
		k.accumulateRewardGauge(ctx, types.BTCDelegationType, del, rewards)
	}

	if err := k.AddDelegationSat(ctx, fp, del, amtSat); err != nil {
		return err
	}

	return k.initializeBTCDelegation(ctx, fp, del)
}

func (k Keeper) BtcDelegationUnbonded(ctx context.Context, fp, del sdk.AccAddress, sat uint64) error {
	amtSat := sdkmath.NewIntFromUint64(sat)

	// withdraw rewards
	//

	if err := k.SubDelegationSat(ctx, fp, del, amtSat); err != nil {
		return err
	}
	return nil
}

func (k Keeper) CalculateDelegationRewards(ctx context.Context, fp, del sdk.AccAddress, endPeriod uint64) (sdk.Coins, error) {
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
	// sanity check
	if btcDelRwdTracker.StartPeriodCumulativeReward > endingPeriod {
		panic("startingPeriod cannot be greater than endingPeriod")
	}

	// sanity check
	// if btcDelRwdTracker..IsNegative() {
	// 	panic("BTC delegation active stake should not be negative")
	// }

	// return staking * (ending - starting)
	starting, err := k.getFinalityProviderHistoricalRewards(ctx, fp, btcDelRwdTracker.StartPeriodCumulativeReward)
	if err != nil {
		return sdk.Coins{}, err
	}

	ending, err := k.getFinalityProviderHistoricalRewards(ctx, fp, endingPeriod)
	if err != nil {
		return sdk.Coins{}, err
	}

	// creates the difference amount of rewards (ending - starting) periods
	// this difference is the amount of rewards entitled per satoshi active stake
	difference := ending.CumulativeRewardsPerSat.Sub(starting.CumulativeRewardsPerSat...)
	if difference.IsAnyNegative() {
		panic("negative rewards should not be possible")
	}

	// note: necessary to truncate so we don't allow withdrawing more rewards than owed
	rewards := difference.MulInt(btcDelRwdTracker.TotalActiveSat)
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
		if _, err := k.initializeFinalityProvider(ctx, fp); err != nil {
			return 0, err
		}
		return 1, nil
	}

	currentRewardsPerSat := sdk.NewCoins()
	if !fpCurrentRwd.TotalActiveSat.IsZero() {
		currentRewardsPerSat = fpCurrentRwd.CurrentRewards.QuoInt(fpCurrentRwd.TotalActiveSat)
	}

	fpHistoricalRwd, err := k.getFinalityProviderHistoricalRewards(ctx, fp, fpCurrentRwd.Period-1)
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

func (k Keeper) GetFinalityProviderCurrentRewards(ctx context.Context, fp sdk.AccAddress) (types.FinalityProviderCurrentRewards, error) {
	key := fp.Bytes()
	bz := k.storeFpCurrentRewards(ctx).Get(key)
	if bz == nil {
		return types.FinalityProviderCurrentRewards{}, types.ErrFPCurrentRewardsNotFound
	}

	var value types.FinalityProviderCurrentRewards
	if err := k.cdc.Unmarshal(bz, &value); err != nil {
		return types.FinalityProviderCurrentRewards{}, err
	}
	return value, nil
}

func (k Keeper) GetBTCDelegationRewardsTracker(ctx context.Context, fp, del sdk.AccAddress) (types.BTCDelegationRewardsTracker, error) {
	key := del.Bytes()
	bz := k.storeBTCDelegationRewardsTracker(ctx, fp).Get(key)
	if bz == nil {
		return types.BTCDelegationRewardsTracker{}, types.ErrBTCDelegationRewardsTrackerNotFound
	}

	var value types.BTCDelegationRewardsTracker
	if err := k.cdc.Unmarshal(bz, &value); err != nil {
		return types.BTCDelegationRewardsTracker{}, err
	}
	return value, nil
}

func (k Keeper) setBTCDelegationRewardsTracker(ctx context.Context, fp, del sdk.AccAddress, rwd types.BTCDelegationRewardsTracker) error {
	key := del.Bytes()
	bz, err := rwd.Marshal()
	if err != nil {
		return err
	}

	k.storeBTCDelegationRewardsTracker(ctx, fp).Set(key, bz)
	return nil
}

func (k Keeper) setFinalityProviderCurrentRewards(ctx context.Context, fp sdk.AccAddress, rwd types.FinalityProviderCurrentRewards) error {
	key := fp.Bytes()
	bz, err := rwd.Marshal()
	if err != nil {
		return err
	}

	k.storeFpCurrentRewards(ctx).Set(key, bz)
	return nil
}

func (k Keeper) getFinalityProviderHistoricalRewards(ctx context.Context, fp sdk.AccAddress, period uint64) (types.FinalityProviderHistoricalRewards, error) {
	key := make([]byte, 8)
	binary.LittleEndian.PutUint64(key, period)

	bz := k.storeFpHistoricalRewards(ctx, fp).Get(key)
	if bz == nil {
		return types.FinalityProviderHistoricalRewards{}, types.ErrFPCurrentRewardsNotFound
	}

	var value types.FinalityProviderHistoricalRewards
	if err := k.cdc.Unmarshal(bz, &value); err != nil {
		return types.FinalityProviderHistoricalRewards{}, err
	}
	return value, nil
}

func (k Keeper) setFinalityProviderHistoricalRewards(ctx context.Context, fp sdk.AccAddress, period uint64, rwd types.FinalityProviderHistoricalRewards) error {
	key := make([]byte, 8)
	binary.LittleEndian.PutUint64(key, period)

	bz, err := rwd.Marshal()
	if err != nil {
		return err
	}

	k.storeFpHistoricalRewards(ctx, fp).Set(key, bz)
	return nil
}

// storeBTCDelegationRewardsTracker returns the KVStore of the FP current rewards
// prefix: BTCDelegationRewardsTrackerKey
// key: (FpAddr, DelAddr)
// value: BTCDelegationRewardsTracker
func (k Keeper) storeBTCDelegationRewardsTracker(ctx context.Context, fp sdk.AccAddress) prefix.Store {
	storeAdaptor := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	st := prefix.NewStore(storeAdaptor, types.BTCDelegationRewardsTrackerKey)
	return prefix.NewStore(st, fp.Bytes())
}

// storeFpCurrentRewards returns the KVStore of the FP current rewards
// prefix: FinalityProviderCurrentRewardsKey
// key: (finality provider cosmos address)
// value: FinalityProviderCurrentRewards
func (k Keeper) storeFpCurrentRewards(ctx context.Context) prefix.Store {
	storeAdaptor := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdaptor, types.FinalityProviderCurrentRewardsKey)
}

// storeFpHistoricalRewards returns the KVStore of the FP historical rewards
// prefix: FinalityProviderHistoricalRewardsKey
// key: (finality provider cosmos address, period)
// value: FinalityProviderCurrentRewards
func (k Keeper) storeFpHistoricalRewards(ctx context.Context, fp sdk.AccAddress) prefix.Store {
	storeAdaptor := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	st := prefix.NewStore(storeAdaptor, types.FinalityProviderHistoricalRewardsKey)
	return prefix.NewStore(st, fp.Bytes())
}

func (k Keeper) addFinalityProviderStaked(ctx context.Context, fp sdk.AccAddress, amt sdkmath.Int) error {
	fpCurrentRwd, err := k.GetFinalityProviderCurrentRewards(ctx, fp)
	if err != nil {
		if !errors.Is(err, types.ErrFPCurrentRewardsNotFound) {
			return err
		}

		// this is needed as the amount of sats for the FP is inside the FpCurrentRewards
		fpCurrentRwd, err = k.initializeFinalityProvider(ctx, fp)
		if err != nil {
			return err
		}
	}

	fpCurrentRwd.AddTotalActiveSat(amt)
	return k.setFinalityProviderCurrentRewards(ctx, fp, fpCurrentRwd)
}

func (k Keeper) subFinalityProviderStaked(ctx context.Context, fp sdk.AccAddress, amt sdkmath.Int) error {
	fpCurrentRwd, err := k.GetFinalityProviderCurrentRewards(ctx, fp)
	if err != nil {
		return err
	}

	fpCurrentRwd.SubTotalActiveSat(amt)
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

// SubDelegationSat there is no need to check if the fp or delegation exists, because they should exist
// otherwise it is probably a programming error calling to subtract the amount of active sat without
// having any sat added in the first place.
func (k Keeper) SubDelegationSat(ctx context.Context, fp, del sdk.AccAddress, amt sdkmath.Int) error {
	btcDelRwdTracker, err := k.GetBTCDelegationRewardsTracker(ctx, fp, del)
	if err != nil {
		return err
	}

	btcDelRwdTracker.SubTotalActiveSat(amt)
	if err := k.setBTCDelegationRewardsTracker(ctx, fp, del, btcDelRwdTracker); err != nil {
		return err
	}

	return k.subFinalityProviderStaked(ctx, fp, amt)
}

// IterateBTCDelegators iterates over all the delegators that have some active BTC delegator
// staked and the total satoshi staked for that delegator address until an error is returned
// or the iterator finishes. Stops if error is returned.
// Should keep track of the total satoshi staked per delegator to avoid iterating over the
// delegator delegations
// func (k Keeper) IterateBTCDelegators(ctx context.Context, i func(delegator sdk.AccAddress, totalSatoshiStaked sdkmath.Int) error) error {
// 	st := k.storeDelStaked(ctx)

// 	iter := st.Iterator(nil, nil)
// 	defer iter.Close()

// 	for ; iter.Valid(); iter.Next() {
// 		sdkAddrBz := iter.Key()
// 		delAddr := sdk.AccAddress(sdkAddrBz)

// 		delBtcStaked, err := ParseInt(iter.Value())
// 		if err != nil {
// 			return err
// 		}

// 		err = i(delAddr, delBtcStaked)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	return nil
// }
