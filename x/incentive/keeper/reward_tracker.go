package keeper

import (
	"context"
	"encoding/binary"
	"errors"

	"cosmossdk.io/store/prefix"
	"github.com/babylonlabs-io/babylon/x/incentive/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	corestoretypes "cosmossdk.io/core/store"
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

	// rewardsRaw, err := k.CalculateDelegationRewards(ctx, val, del, endingPeriod)
	// if err != nil {
	// 	return nil, err
	// }

	if err := k.AddDelegationStaking(ctx, fp, del, amtSat); err != nil {
		return err
	}

	return nil
}

func (k Keeper) BtcDelegationUnbonded(ctx context.Context, fp, del sdk.AccAddress, sat uint64) error {
	amtSat := sdkmath.NewIntFromUint64(sat)

	// withdraw rewards
	//

	if err := k.SubDelegationStaking(ctx, fp, del, amtSat); err != nil {
		return err
	}
	return nil
}

func (k Keeper) CalculateDelegationRewards(ctx context.Context, fp, del sdk.AccAddress, endPeriod uint64) (sdk.Coins, error) {
	delActiveStakedSat, err := k.getDelegationStaking(ctx, fp, del)
	if err != nil {
		return sdk.Coins{}, err
	}

	if delActiveStakedSat.IsZero() {
		return sdk.NewCoins(), nil
	}

}

// calculate the rewards accrued by a delegation between two periods
func (k Keeper) calculateDelegationRewardsBetween(
	ctx context.Context,
	fp, del sdk.AccAddress,
	startingPeriod, endingPeriod uint64,
	delActiveStakedSat sdkmath.Int,
) (sdk.Coins, error) {
	// sanity check
	if startingPeriod > endingPeriod {
		panic("startingPeriod cannot be greater than endingPeriod")
	}

	// sanity check
	if delActiveStakedSat.IsNegative() {
		panic("BTC delegation active stake should not be negative")
	}

	// return staking * (ending - starting)
	starting, err := k.getFinalityProviderHistoricalRewards(ctx, fp, startingPeriod)
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
	rewards := difference.MulInt(delActiveStakedSat)
	return rewards, nil
}

// IncrementFinalityProviderPeriod
func (k Keeper) IncrementFinalityProviderPeriod(ctx context.Context, fp sdk.AccAddress) (endedPeriod uint64, err error) {
	// IncrementValidatorPeriod
	//    gets the current rewards and send to historical the current period (the rewards are stored as "shares" which means the amount of rewards per satoshi)
	//    sets new empty current rewards with new period
	fpCurrentRwd, err := k.GetFinalityProviderCurrentRewards(ctx, fp)
	if err != nil {
		if errors.Is(err, types.ErrFPCurrentRewardsNotFound) {
			// initialize Validator
			err := k.initializeFinalityProvider(ctx, fp)
			if err != nil {
				return 0, err
			}
			return 1, nil
		}
		return 0, err
	}

	fpAmtStaked, err := k.getFinalityProviderStaked(ctx, fp)
	if err != nil {
		return 0, err
	}

	currentRewardsPerSat := sdk.NewCoins()
	if !fpAmtStaked.IsZero() {
		currentRewardsPerSat = fpCurrentRwd.CurrentRewards.QuoInt(fpAmtStaked)
	}

	fpHistoricalRwd, err := k.getFinalityProviderHistoricalRewards(ctx, fp, fpCurrentRwd.Period-1)
	if err != nil {
		return 0, err
	}

	newFpHistoricalRwd := types.NewFinalityProviderHistoricalRewards(fpHistoricalRwd.CumulativeRewardsPerSat.Add(currentRewardsPerSat...))
	if err := k.setFinalityProviderHistoricalRewards(ctx, fp, fpCurrentRwd.Period, newFpHistoricalRwd); err != nil {
		return 0, err
	}

	// initiates a new period with empty rewards
	newCurrentRwd := types.NewFinalityProviderCurrentRewards(sdk.NewCoins(), fpCurrentRwd.Period+1)
	if err := k.setFinalityProviderCurrentRewards(ctx, fp, newCurrentRwd); err != nil {
		return 0, err
	}

	return fpCurrentRwd.Period, nil
}

func (k Keeper) initializeFinalityProvider(ctx context.Context, fp sdk.AccAddress) error {
	// historical rewards starts at the period 0
	err := k.setFinalityProviderHistoricalRewards(ctx, fp, 0, types.NewFinalityProviderHistoricalRewards(sdk.NewCoins()))
	if err != nil {
		return err
	}
	// set current rewards (starting at period 1)
	return k.setFinalityProviderCurrentRewards(ctx, fp, types.NewFinalityProviderCurrentRewards(sdk.NewCoins(), 1))
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

	// validator, err := k.stakingKeeper.Validator(ctx, fp)
	// if err != nil {
	// 	return err
	// }

	// delegation, err := k.stakingKeeper.Delegation(ctx, del, fp)
	// if err != nil {
	// 	return err
	// }

	// sdkCtx := sdk.UnwrapSDKContext(ctx)
	types.NewBTCDelegationRewardsTracker(previousPeriod, 0)
	return nil
	// return k.SetDelegatorStartingInfo(ctx, fp, del, dstrtypes.NewDelegatorStartingInfo(previousPeriod, stake, uint64(sdkCtx.BlockHeight())))
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

// storeFinalityProviderStaked returns the KVStore of the finality provider amount active staked
// prefix: (DelegatorStakedBTCKey)
// key: (FinalityProviderStakedBTCKey)
// value: sdk math Int
func (k Keeper) storeFinalityProviderStaked(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.FinalityProviderStakedBTCKey)
}

// storeDelegationFpStaked returns the KVStore of the delegator amount staked
// prefix: (DelegatorStakedBTCKey)
// key: (FpAddr, DelAddr)
// value: sdk math Int
func (k Keeper) storeDelegationFpStaked(ctx context.Context, fp sdk.AccAddress) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	st := prefix.NewStore(storeAdapter, types.DelegationStakedBTCKey)
	return prefix.NewStore(st, fp.Bytes())
}

func (k Keeper) addFinalityProviderStaked(ctx context.Context, fp sdk.AccAddress, amt sdkmath.Int) error {
	st := k.storeFinalityProviderStaked(ctx)
	key := fp.Bytes()

	return OperationWithInt(st, key, func(currentFpStaked sdkmath.Int) sdkmath.Int {
		return currentFpStaked.Add(amt)
	})
}

func (k Keeper) getFinalityProviderStaked(ctx context.Context, fp sdk.AccAddress) (amt sdkmath.Int, err error) {
	st := k.storeFinalityProviderStaked(ctx)
	key := fp.Bytes()

	return PrefixStoreGetInt(st, key)
}

func (k Keeper) subFinalityProviderStaked(ctx context.Context, fp sdk.AccAddress, amt sdkmath.Int) error {
	st := k.storeFinalityProviderStaked(ctx)
	key := fp.Bytes()

	return OperationWithInt(st, key, func(currentFpStaked sdkmath.Int) sdkmath.Int {
		return currentFpStaked.Sub(amt)
	})
}

func (k Keeper) getDelegationStaking(ctx context.Context, fp, del sdk.AccAddress) (sdkmath.Int, error) {
	st := k.storeDelegationFpStaked(ctx, fp)
	key := del.Bytes()

	return PrefixStoreGetInt(st, key)
}

func (k Keeper) AddDelegationStaking(ctx context.Context, fp, del sdk.AccAddress, amt sdkmath.Int) error {
	st := k.storeDelegationFpStaked(ctx, fp)
	key := del.Bytes()

	err := OperationWithInt(st, key, func(currenDelegationStaked sdkmath.Int) sdkmath.Int {
		return currenDelegationStaked.Add(amt)
	})
	if err != nil {
		return err
	}

	return k.addFinalityProviderStaked(ctx, fp, amt)
}

func (k Keeper) SubDelegationStaking(ctx context.Context, fp, del sdk.AccAddress, amt sdkmath.Int) error {
	st := k.storeDelegationFpStaked(ctx, fp)
	key := fp.Bytes()

	err := OperationWithInt(st, key, func(currenDelegationStaked sdkmath.Int) sdkmath.Int {
		return currenDelegationStaked.Sub(amt)
	})
	if err != nil {
		return err
	}

	return k.subFinalityProviderStaked(ctx, fp, amt)
}

func OperationWithInt(st prefix.Store, key []byte, update func(vIntFromStore sdkmath.Int) (updatedValue sdkmath.Int)) (err error) {
	currentValue, err := PrefixStoreGetInt(st, key)
	if err != nil {
		return err
	}

	currentValue = update(currentValue)
	bz, err := currentValue.Marshal()
	if err != nil {
		return err
	}

	st.Set(key, bz)
	return nil
}

func PrefixStoreGetInt(st prefix.Store, key []byte) (vInt sdkmath.Int, err error) {
	if !st.Has(key) {
		return sdkmath.NewInt(0), nil
	}

	bz := st.Get(key)
	vInt, err = ParseInt(bz)
	if err != nil {
		return sdkmath.Int{}, err
	}

	return vInt, nil
}

// StoreSetInt stores an sdkmath.Int from the KVStore.
func StoreSetInt(kv corestoretypes.KVStore, key []byte, vInt sdkmath.Int) (err error) {
	bz, err := vInt.Marshal()
	if err != nil {
		return err
	}
	return kv.Set(key, bz)
}

// StoreGetInt retrieves an sdkmath.Int from the KVStore. It returns zero int if not found.
func StoreGetInt(kv corestoretypes.KVStore, key []byte) (vInt sdkmath.Int, err error) {
	exists, err := kv.Has(key)
	if err != nil {
		return sdkmath.Int{}, err
	}

	if !exists {
		return sdkmath.NewInt(0), nil
	}

	bz, err := kv.Get(key)
	if err != nil {
		return sdkmath.Int{}, err
	}

	vInt, err = ParseInt(bz)
	if err != nil {
		return sdkmath.Int{}, err
	}
	return vInt, nil
}

// ParseInt parses an sdkmath.Int from bytes.
func ParseInt(bz []byte) (sdkmath.Int, error) {
	var val sdkmath.Int
	if err := val.Unmarshal(bz); err != nil {
		return val, err
	}
	return val, nil
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
