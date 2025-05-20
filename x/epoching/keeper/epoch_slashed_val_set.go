package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	"github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// setSlashedVotingPower sets the total amount of voting power that has been slashed in the epoch
func (k Keeper) setSlashedVotingPower(ctx context.Context, epochNumber uint64, power int64) {
	store := k.slashedVotingPowerStore(ctx)

	// key: epochNumber
	epochNumberBytes := sdk.Uint64ToBigEndian(epochNumber)
	// value: power
	powerBytes, err := sdkmath.NewInt(power).Marshal()
	if err != nil {
		panic(errorsmod.Wrap(types.ErrMarshal, err.Error()))
	}

	store.Set(epochNumberBytes, powerBytes)
}

// InitSlashedVotingPower sets the slashed voting power of the current epoch to 0
// This is called upon a new epoch
func (k Keeper) InitSlashedVotingPower(ctx context.Context) {
	epochNumber := k.GetEpoch(ctx).EpochNumber
	k.setSlashedVotingPower(ctx, epochNumber, 0)
}

// InitGenSlashedVotingPower sets the slashed voting power of the current epoch to 0
// or sets the provided slashed validator sets.
// This is called upon initialising the genesis state
func (k Keeper) InitGenSlashedVotingPower(ctx context.Context, genSlashedValSet []*types.EpochValidatorSet) error {
	if len(genSlashedValSet) > 0 {
		for _, ev := range genSlashedValSet {
			if err := k.setEpochSlashedValSet(ctx, ev); err != nil {
				return err
			}
		}
		return nil
	}
	k.InitSlashedVotingPower(ctx)
	return nil
}

// GetSlashedVotingPower fetches the amount of slashed voting power of a given epoch
func (k Keeper) GetSlashedVotingPower(ctx context.Context, epochNumber uint64) int64 {
	store := k.slashedVotingPowerStore(ctx)

	// key: epochNumber
	epochNumberBytes := sdk.Uint64ToBigEndian(epochNumber)
	bz := store.Get(epochNumberBytes)
	if bz == nil {
		panic(types.ErrUnknownSlashedVotingPower)
	}
	// get value
	var slashedVotingPower sdkmath.Int
	if err := slashedVotingPower.Unmarshal(bz); err != nil {
		panic(errorsmod.Wrap(types.ErrUnmarshal, err.Error()))
	}

	return slashedVotingPower.Int64()
}

// AddSlashedValidator adds a slashed validator to the set of the current epoch
// This is called upon hook `BeforeValidatorSlashed` exposed by the staking module
func (k Keeper) AddSlashedValidator(ctx context.Context, valAddr sdk.ValAddress) error {
	epochNumber := k.GetEpoch(ctx).EpochNumber
	store := k.slashedValSetStore(ctx, epochNumber)
	thisVotingPower, err := k.GetValidatorVotingPower(ctx, epochNumber, valAddr)
	if err != nil {
		panic(errorsmod.Wrap(types.ErrMarshal, err.Error()))
	}
	thisVotingPowerBytes, err := sdkmath.NewInt(thisVotingPower).Marshal()
	if err != nil {
		panic(errorsmod.Wrap(types.ErrMarshal, err.Error()))
	}

	// insert into "set of slashed addresses" as KV pair, where
	// - key: valAddr
	// - value: thisVotingPower
	store.Set(valAddr, thisVotingPowerBytes)

	// add voting power
	slashedVotingPower := k.GetSlashedVotingPower(ctx, epochNumber)
	if err != nil {
		// we don't panic here since it's possible that the most powerful validator outside the validator set enrols to the validator after this validator is slashed.
		return err
	}
	k.setSlashedVotingPower(ctx, epochNumber, slashedVotingPower+thisVotingPower)
	return nil
}

// GetSlashedValidators returns the set of slashed validators of a given epoch
func (k Keeper) GetSlashedValidators(ctx context.Context, epochNumber uint64) types.ValidatorSet {
	valSet := types.ValidatorSet{}
	store := k.slashedValSetStore(ctx, epochNumber)
	// add each valAddr, which is the key
	iterator := store.Iterator(nil, nil)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		addr := sdk.ValAddress(iterator.Key())
		powerBytes := iterator.Value()
		if powerBytes == nil {
			panic(types.ErrUnknownValidator)
		}
		var power sdkmath.Int
		if err := power.Unmarshal(powerBytes); err != nil {
			panic(errorsmod.Wrap(types.ErrUnmarshal, err.Error()))
		}
		val := types.Validator{Addr: addr, Power: power.Int64()}
		valSet = append(valSet, val)
	}

	return valSet
}

// ClearSlashedValidators removes all slashed validators in the set
// TODO: This is called upon the epoch is checkpointed
func (k Keeper) ClearSlashedValidators(ctx context.Context, epochNumber uint64) {
	// prefix : SlashedValidatorSetKey || epochNumber
	store := k.slashedValSetStore(ctx, epochNumber)

	// remove all entries with this prefix
	iterator := store.Iterator(nil, nil)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		key := iterator.Key()
		store.Delete(key)
	}

	// forget the slashed voting power of this epoch
	epochNumberBytes := sdk.Uint64ToBigEndian(epochNumber)
	k.slashedVotingPowerStore(ctx).Delete(epochNumberBytes)
}

// slashedValSetStore returns the KVStore of the slashed validator set for a given epoch
// prefix : SlashedValidatorSetKey || epochNumber
func (k Keeper) slashedValSetStore(ctx context.Context, epochNumber uint64) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	slashedValStore := prefix.NewStore(storeAdapter, types.SlashedValidatorSetKey)
	epochNumberBytes := sdk.Uint64ToBigEndian(epochNumber)
	return prefix.NewStore(slashedValStore, epochNumberBytes)
}

// slashedVotingPower returns the KVStore of the slashed voting power
// prefix: SlashedVotingPowerKey
func (k Keeper) slashedVotingPowerStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.SlashedVotingPowerKey)
}

// setEpochSlashedValSet sets the epoch's slashed validator set.
// It is used in InitGenesis logic only.
func (k Keeper) setEpochSlashedValSet(ctx context.Context, ev *types.EpochValidatorSet) error {
	if ev == nil {
		return nil
	}

	var (
		totalSlashedPower = int64(0) // initialize epoch slashed voting power
		valSetStore       = k.slashedValSetStore(ctx, ev.EpochNumber)
	)

	// set slashed validators
	for _, v := range ev.Validators {
		powerBytes, err := sdkmath.NewInt(v.Power).Marshal()
		if err != nil {
			return errorsmod.Wrap(types.ErrMarshal, err.Error())
		}
		valSetStore.Set(v.Addr, powerBytes)
		totalSlashedPower += v.Power
	}

	epochNumberBytes := sdk.Uint64ToBigEndian(ev.EpochNumber)
	// value: power
	powerBytes, err := sdkmath.NewInt(totalSlashedPower).Marshal()
	if err != nil {
		return errorsmod.Wrap(types.ErrMarshal, err.Error())
	}
	// set epoch's slashed voting power
	k.slashedVotingPowerStore(ctx).Set(epochNumberBytes, powerBytes)
	return nil
}
