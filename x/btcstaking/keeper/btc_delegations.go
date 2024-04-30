package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/store/prefix"
	asig "github.com/babylonchain/babylon/crypto/schnorr-adaptor-signature"
	bbn "github.com/babylonchain/babylon/types"
	"github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// AddBTCDelegation adds a BTC delegation post verification to the system, including
// - indexing the given BTC delegation in the BTC delegator store,
// - saving it under BTC delegation store, and
// - emit events about this BTC delegation.
func (k Keeper) AddBTCDelegation(ctx sdk.Context, btcDel *types.BTCDelegation) error {
	if err := btcDel.ValidateBasic(); err != nil {
		return err
	}

	// get staking tx hash
	stakingTxHash, err := btcDel.GetStakingTxHash()
	if err != nil {
		return err
	}

	// for each finality provider the delegation restakes to, update its index
	for _, fpBTCPK := range btcDel.FpBtcPkList {
		// get BTC delegation index under this finality provider
		btcDelIndex := k.getBTCDelegatorDelegationIndex(ctx, &fpBTCPK, btcDel.BtcPk)
		if btcDelIndex == nil {
			btcDelIndex = types.NewBTCDelegatorDelegationIndex()
		}
		// index staking tx hash of this BTC delegation
		if err := btcDelIndex.Add(stakingTxHash); err != nil {
			return types.ErrInvalidStakingTx.Wrapf(err.Error())
		}
		// save the index
		k.setBTCDelegatorDelegationIndex(ctx, &fpBTCPK, btcDel.BtcPk, btcDelIndex)
	}

	// save this BTC delegation
	k.setBTCDelegation(ctx, btcDel)

	// notify subscriber
	event := &types.EventBTCDelegationStateUpdate{
		StakingTxHash: stakingTxHash.String(),
		NewState:      types.BTCDelegationStatus_PENDING,
	}
	if err := ctx.EventManager().EmitTypedEvent(event); err != nil {
		panic(fmt.Errorf("failed to emit EventBTCDelegationStateUpdate for the new pending BTC delegation: %w", err))
	}

	// NOTE: we don't need to record events for pending BTC delegations since these
	// do not affect voting power distribution

	// record event that the BTC delegation will become unbonded at endHeight-w
	unbondedEvent := types.NewEventPowerDistUpdateWithBTCDel(&types.EventBTCDelegationStateUpdate{
		StakingTxHash: stakingTxHash.String(),
		NewState:      types.BTCDelegationStatus_UNBONDED,
	})
	wValue := k.btccKeeper.GetParams(ctx).CheckpointFinalizationTimeout
	k.addPowerDistUpdateEvent(ctx, btcDel.EndHeight-wValue, unbondedEvent)

	return nil
}

// addCovenantSigsToBTCDelegation adds signatures from a given covenant member
// to the given BTC delegation
func (k Keeper) addCovenantSigsToBTCDelegation(
	ctx sdk.Context,
	btcDel *types.BTCDelegation,
	covPK *bbn.BIP340PubKey,
	parsedSlashingAdaptorSignatures []asig.AdaptorSignature,
	unbondingTxSig *bbn.BIP340Signature,
	parsedUnbondingSlashingAdaptorSignatures []asig.AdaptorSignature,
	params *types.Params,
) {
	// All is fine add received signatures to the BTC delegation and BtcUndelegation
	btcDel.AddCovenantSigs(
		covPK,
		parsedSlashingAdaptorSignatures,
		unbondingTxSig,
		parsedUnbondingSlashingAdaptorSignatures,
	)

	// set BTC delegation back to KV store
	k.setBTCDelegation(ctx, btcDel)

	// If reaching the covenant quorum after this msg, the BTC delegation becomes
	// active. Then, record and emit this event
	if len(btcDel.CovenantSigs) == int(params.CovenantQuorum) {
		// notify subscriber
		event := &types.EventBTCDelegationStateUpdate{
			StakingTxHash: btcDel.MustGetStakingTxHash().String(),
			NewState:      types.BTCDelegationStatus_ACTIVE,
		}
		if err := ctx.EventManager().EmitTypedEvent(event); err != nil {
			panic(fmt.Errorf("failed to emit EventBTCDelegationStateUpdate for the new active BTC delegation: %w", err))
		}

		// record event that the BTC delegation becomes active at this height
		activeEvent := types.NewEventPowerDistUpdateWithBTCDel(event)
		btcTip := k.btclcKeeper.GetTipInfo(ctx)
		k.addPowerDistUpdateEvent(ctx, btcTip.Height, activeEvent)

		// get consumer ids of only non-Babylon finality providers
		restakedFPConsumerIDs, err := k.restakedFPConsumerIDs(ctx, btcDel.FpBtcPkList)
		if err != nil {
			panic(fmt.Errorf("failed to get consumer ids for the restaked BTC delegation: %w", err))
		}
		consumerEvent, err := types.CreateActiveBTCDelegationEvent(btcDel)
		if err != nil {
			panic(fmt.Errorf("failed to create active BTC delegation event: %w", err))
		}
		for _, consumerID := range restakedFPConsumerIDs {
			if err := k.AddBTCStakingConsumerEvent(ctx, consumerID, consumerEvent); err != nil {
				panic(fmt.Errorf("failed to add active BTC delegation event: %w", err))
			}
		}
	}
}

// btcUndelegate adds the signature of the unbonding tx signed by the staker
// to the given BTC delegation
func (k Keeper) btcUndelegate(
	ctx sdk.Context,
	btcDel *types.BTCDelegation,
	unbondingTxSig *bbn.BIP340Signature,
) {
	btcDel.BtcUndelegation.DelegatorUnbondingSig = unbondingTxSig

	// set BTC delegation back to KV store
	k.setBTCDelegation(ctx, btcDel)

	// notify subscriber about this unbonded BTC delegation
	event := &types.EventBTCDelegationStateUpdate{
		StakingTxHash: btcDel.MustGetStakingTxHash().String(),
		NewState:      types.BTCDelegationStatus_UNBONDED,
	}

	if err := ctx.EventManager().EmitTypedEvent(event); err != nil {
		panic(fmt.Errorf("failed to emit EventBTCDelegationStateUpdate for the new unbonded BTC delegation: %w", err))
	}

	// record event that the BTC delegation becomes unbonded at this height
	unbondedEvent := types.NewEventPowerDistUpdateWithBTCDel(event)
	btcTip := k.btclcKeeper.GetTipInfo(ctx)
	k.addPowerDistUpdateEvent(ctx, btcTip.Height, unbondedEvent)

	// get consumer ids of only non-Babylon finality providers
	restakedFPConsumerIDs, err := k.restakedFPConsumerIDs(ctx, btcDel.FpBtcPkList)
	if err != nil {
		panic(fmt.Errorf("failed to get consumer ids for the restaked BTC delegation: %w", err))
	}
	// create consumer event for unbonded BTC delegation and add it to the consumer's event store
	consumerEvent, err := types.CreateUnbondedBTCDelegationEvent(btcDel)
	if err != nil {
		panic(fmt.Errorf("failed to create unbonded BTC delegation event: %w", err))
	}
	for _, consumerID := range restakedFPConsumerIDs {
		if err = k.AddBTCStakingConsumerEvent(ctx, consumerID, consumerEvent); err != nil {
			panic(fmt.Errorf("failed to add active BTC delegation event: %w", err))
		}
	}
}

func (k Keeper) setBTCDelegation(ctx context.Context, btcDel *types.BTCDelegation) {
	store := k.btcDelegationStore(ctx)
	stakingTxHash := btcDel.MustGetStakingTxHash()
	btcDelBytes := k.cdc.MustMarshal(btcDel)
	store.Set(stakingTxHash[:], btcDelBytes)
}

// validateRestakedFPs ensures all finality providers are known to Babylon and at least
// 1 one of them is a Babylon finality provider. It also checks whether the BTC stake is
// restaked to FPs of consumer chains
func (k Keeper) validateRestakedFPs(ctx context.Context, fpBTCPKs []bbn.BIP340PubKey) (bool, error) {
	restakedToBabylon := false
	restakedToConsumers := false

	for _, fpBTCPK := range fpBTCPKs {
		// find the fp and determine whether it's Babylon fp or consumer chain fp
		if fp, err := k.GetFinalityProvider(ctx, fpBTCPK); err == nil {
			// ensure the finality provider is not slashed
			if fp.IsSlashed() {
				return false, types.ErrFpAlreadySlashed
			}
			restakedToBabylon = true
			continue
		} else if consumerID, err := k.bscKeeper.GetConsumerOfFinalityProvider(ctx, &fpBTCPK); err == nil {
			fp, err := k.bscKeeper.GetConsumerFinalityProvider(ctx, consumerID, &fpBTCPK)
			if err != nil {
				return false, err
			}
			// ensure the finality provider is not slashed
			if fp.IsSlashed() {
				return false, types.ErrFpAlreadySlashed
			}
			restakedToConsumers = true
			continue
		} else {
			return false, types.ErrFpNotFound.Wrapf("finality provider pk %s is not found", fpBTCPK.MarshalHex())
		}
	}
	if !restakedToBabylon {
		// a BTC delegation has to stake to at least 1 Babylon finality provider
		return false, types.ErrNoBabylonFPRestaked
	}
	return restakedToConsumers, nil
}

// restakedFPConsumerIDs returns the consumer IDs of non-Babylon finality providers
func (k Keeper) restakedFPConsumerIDs(ctx context.Context, fpBTCPKs []bbn.BIP340PubKey) ([]string, error) {
	var consumerIDs []string

	for _, fpBTCPK := range fpBTCPKs {
		if _, err := k.GetFinalityProvider(ctx, fpBTCPK); err == nil {
			continue
		} else if consumerID, err := k.bscKeeper.GetConsumerOfFinalityProvider(ctx, &fpBTCPK); err == nil {
			consumerIDs = append(consumerIDs, consumerID)
		} else {
			return nil, types.ErrFpNotFound.Wrapf("finality provider pk %s is not found", fpBTCPK.MarshalHex())
		}
	}

	return consumerIDs, nil
}

// GetBTCDelegation gets the BTC delegation with a given staking tx hash
func (k Keeper) GetBTCDelegation(ctx context.Context, stakingTxHashStr string) (*types.BTCDelegation, error) {
	// decode staking tx hash string
	stakingTxHash, err := chainhash.NewHashFromStr(stakingTxHashStr)
	if err != nil {
		return nil, err
	}
	btcDel := k.getBTCDelegation(ctx, *stakingTxHash)
	if btcDel == nil {
		return nil, types.ErrBTCDelegationNotFound
	}

	return btcDel, nil
}

func (k Keeper) getBTCDelegation(ctx context.Context, stakingTxHash chainhash.Hash) *types.BTCDelegation {
	store := k.btcDelegationStore(ctx)
	btcDelBytes := store.Get(stakingTxHash[:])
	if len(btcDelBytes) == 0 {
		return nil
	}
	var btcDel types.BTCDelegation
	k.cdc.MustUnmarshal(btcDelBytes, &btcDel)
	return &btcDel
}

// btcDelegationStore returns the KVStore of the BTC delegations
// prefix: BTCDelegationKey
// key: BTC delegation's staking tx hash
// value: BTCDelegation
func (k Keeper) btcDelegationStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.BTCDelegationKey)
}
