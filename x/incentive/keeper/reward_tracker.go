package keeper

import (
	"context"

	"cosmossdk.io/store/prefix"
	"github.com/babylonlabs-io/babylon/x/incentive/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) BtcDelegationActivated(ctx context.Context, fp, del sdk.AccAddress, sat uint64) error {
	// if btc delegations does exists
	//   BeforeDelegationCreated
	//     IncrementValidatorPeriod

	// if btc delegations exists
	//   BeforeDelegationSharesModified
	//     withdrawDelegationRewards
	//       IncrementValidatorPeriod

	// IncrementValidatorPeriod
	//    gets the current rewards and send to historical the current period (the rewards are stored as "shares" which means the amount of rewards per satoshi)
	//    sets new empty current rewards with new period

	// adds new satoshi activated to
	// val total
	// val, del total

	return nil
}

func (k Keeper) BtcDelegationUnbonded(ctx context.Context, fp, del sdk.AccAddress, sat uint64) error {
	return nil
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

// storeFpCurrentRewards returns the KVStore of the FP current rewards
// prefix: FinalityProviderCurrentRewardsKey
// key: (finality provider cosmos address)
// value: FinalityProviderCurrentRewards
func (k Keeper) storeFpCurrentRewards(ctx context.Context) prefix.Store {
	storeAdaptor := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdaptor, types.FinalityProviderCurrentRewardsKey)
}
