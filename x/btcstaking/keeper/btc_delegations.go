package keeper

import (
	"context"
	"fmt"
	"slices"

	"cosmossdk.io/store/prefix"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	asig "github.com/babylonlabs-io/babylon/v3/crypto/schnorr-adaptor-signature"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
)

// AddBTCDelegation adds a BTC delegation post verification to the system, including
// - indexing the given BTC delegation in the BTC delegator store,
// - saving it under BTC delegation store, and
// - emit events about this BTC delegation.
func (k Keeper) AddBTCDelegation(
	ctx sdk.Context,
	btcDel *types.BTCDelegation,
) error {
	// get staking tx hash
	stakingTxHash, err := btcDel.GetStakingTxHash()
	if err != nil {
		return err
	}

	// for each finality provider the delegation restakes to, update its index
	for _, fpBTCPK := range btcDel.FpBtcPkList {
		fpBTCPK := fpBTCPK // remove when update to go1.22
		// get BTC delegation index under this finality provider
		btcDelIndex := k.getBTCDelegatorDelegationIndex(ctx, &fpBTCPK, btcDel.BtcPk)
		if btcDelIndex == nil {
			btcDelIndex = types.NewBTCDelegatorDelegationIndex()
		}
		// index staking tx hash of this BTC delegation
		if err := btcDelIndex.Add(stakingTxHash); err != nil {
			return types.ErrInvalidStakingTx.Wrapf("error adding staking tx hash to BTC delegator index: %s", err.Error())
		}
		// save the index
		k.setBTCDelegatorDelegationIndex(ctx, &fpBTCPK, btcDel.BtcPk, btcDelIndex)
	}

	// save this BTC delegation
	k.setBTCDelegation(ctx, btcDel)

	if err := ctx.EventManager().EmitTypedEvents(types.NewBtcDelCreationEvent(
		btcDel,
	)); err != nil {
		panic(fmt.Errorf("failed to emit events for the new pending BTC delegation: %w", err))
	}

	// NOTE: we don't need to record events for pending BTC delegations since these
	// do not affect voting power distribution
	// NOTE: we only insert unbonded event if the delegation already has inclusion proof
	if btcDel.HasInclusionProof() {
		if err := ctx.EventManager().EmitTypedEvent(types.NewInclusionProofEvent(
			stakingTxHash.String(),
			btcDel.StartHeight,
			btcDel.EndHeight,
			types.BTCDelegationStatus_PENDING,
		)); err != nil {
			panic(fmt.Errorf("failed to emit EventBTCDelegationInclusionProofReceived for the new pending BTC delegation: %w", err))
		}

		// record event that the BTC delegation will become expired (unbonded) at EndHeight-w
		// This event will be generated to subscribers as block event, when the
		// btc light client block height will reach btcDel.EndHeight-wValue
		expiredEvent := types.NewEventPowerDistUpdateWithBTCDel(&types.EventBTCDelegationStateUpdate{
			StakingTxHash: stakingTxHash.String(),
			NewState:      types.BTCDelegationStatus_EXPIRED,
		})

		// NOTE: we should have verified that EndHeight > btcTip.Height + unbonding_time
		k.addPowerDistUpdateEvent(ctx, btcDel.EndHeight-btcDel.UnbondingTime, expiredEvent)
	}

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
	btcTipHeight uint32,
) {
	hadQuorum := btcDel.HasCovenantQuorums(params.CovenantQuorum)

	// All is fine add received signatures to the BTC delegation and BtcUndelegation
	btcDel.AddCovenantSigs(
		covPK,
		parsedSlashingAdaptorSignatures,
		unbondingTxSig,
		parsedUnbondingSlashingAdaptorSignatures,
	)

	// set BTC delegation back to KV store
	k.setBTCDelegation(ctx, btcDel)

	if err := ctx.EventManager().EmitTypedEvent(types.NewCovenantSignatureReceivedEvent(
		btcDel,
		covPK,
		unbondingTxSig,
	)); err != nil {
		panic(fmt.Errorf("failed to emit EventCovenantSignatureRecevied for the new active BTC delegation: %w", err))
	}

	// If reaching the covenant quorum after this msg, the BTC delegation becomes
	// active. Then, record and emit this event
	// We only emit power distribution events, and external quorum events if it
	// is the first time the quorum is reached
	if !hadQuorum && btcDel.HasCovenantQuorums(params.CovenantQuorum) {
		if btcDel.HasInclusionProof() {
			quorumReachedEvent := types.NewCovenantQuorumReachedEvent(
				btcDel,
				types.BTCDelegationStatus_ACTIVE,
			)
			if err := ctx.EventManager().EmitTypedEvent(quorumReachedEvent); err != nil {
				panic(fmt.Errorf("failed to emit emit for the new verified BTC delegation: %w", err))
			}

			// record event that the BTC delegation becomes active at this height
			activeEvent := types.NewEventPowerDistUpdateWithBTCDel(
				&types.EventBTCDelegationStateUpdate{
					StakingTxHash: btcDel.MustGetStakingTxHash().String(),
					NewState:      types.BTCDelegationStatus_ACTIVE,
				},
			)
			k.addPowerDistUpdateEvent(ctx, btcTipHeight, activeEvent)

			// notify consumer chains about the active BTC delegation
			k.notifyConsumersOnActiveBTCDel(ctx, btcDel)
		} else {
			quorumReachedEvent := types.NewCovenantQuorumReachedEvent(
				btcDel,
				types.BTCDelegationStatus_VERIFIED,
			)

			if err := ctx.EventManager().EmitTypedEvent(quorumReachedEvent); err != nil {
				panic(fmt.Errorf("failed to emit emit for the new verified BTC delegation: %w", err))
			}
		}
	}
}

func (k Keeper) notifyConsumersOnActiveBTCDel(ctx context.Context, btcDel *types.BTCDelegation) {
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

// btcUndelegate adds the signature of the unbonding tx signed by the staker
// to the given BTC delegation
func (k Keeper) btcUndelegate(
	ctx sdk.Context,
	btcDel *types.BTCDelegation,
	u *types.DelegatorUnbondingInfo,
	stakeSpendingTx []byte,
	proof *types.InclusionProof,
) {
	btcDel.BtcUndelegation.DelegatorUnbondingInfo = u
	k.setBTCDelegation(ctx, btcDel)

	if !btcDel.HasInclusionProof() {
		return
	}

	// notify subscriber about this unbonded BTC delegation
	event := &types.EventBTCDelegationStateUpdate{
		StakingTxHash: btcDel.MustGetStakingTxHash().String(),
		NewState:      types.BTCDelegationStatus_UNBONDED,
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
	consumerEvent, err := types.CreateUnbondedBTCDelegationEvent(btcDel, stakeSpendingTx, proof)
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
// one of them is a Babylon finality provider. It also checks whether the BTC stake is
// restaked to FPs of consumer chains. It enforces:
// 1. Total number of FPs <= min of all consumers' max_multi_staked_fps
// 2. At most 1 FP per consumer/BSN
func (k Keeper) validateRestakedFPs(ctx context.Context, fpBTCPKs []bbn.BIP340PubKey) (bool, error) {
	restakedToBabylon := false
	restakedToConsumers := false

	// Track FPs per consumer to enforce at-most-1-FP-per-consumer
	consumerFPs := make(map[string]struct{})
	// Track min max_multi_staked_fps across all consumers
	minMaxMultiStakedFPs := ^uint32(0) // Initialize with MaxUint32

	for i := range fpBTCPKs {
		fpBTCPK := fpBTCPKs[i]

		// find the fp and determine whether it's Babylon fp or consumer chain fp
		if fp, err := k.GetFinalityProvider(ctx, fpBTCPK); err == nil {
			// ensure the finality provider is not slashed
			if fp.IsSlashed() {
				return false, types.ErrFpAlreadySlashed
			}
			restakedToBabylon = true
			continue
		} else if consumerID, err := k.BscKeeper.GetConsumerOfFinalityProvider(ctx, &fpBTCPK); err == nil {
			// Check if we already have an FP from this consumer
			if _, exists := consumerFPs[consumerID]; exists {
				return false, types.ErrTooManyFPsFromSameConsumer.Wrapf("consumer %s already has an FP", consumerID)
			}
			consumerFPs[consumerID] = struct{}{}

			// Get consumer's max_multi_staked_fps
			maxMultiStakedFps, err := k.BscKeeper.GetConsumerRegistryMaxMultiStakedFps(ctx, consumerID)
			if err != nil {
				return false, err
			}
			if maxMultiStakedFps < minMaxMultiStakedFPs {
				minMaxMultiStakedFPs = maxMultiStakedFps
			}

			fp, err := k.BscKeeper.GetConsumerFinalityProvider(ctx, consumerID, &fpBTCPK)
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

	// Check if total number of FPs exceeds min max_multi_staked_fps
	if minMaxMultiStakedFPs > 0 && uint32(len(fpBTCPKs)) > minMaxMultiStakedFPs {
		return false, types.ErrTooManyFPs.Wrapf("total FPs %d exceeds min max_multi_staked_fps %d", len(fpBTCPKs), minMaxMultiStakedFPs)
	}

	return restakedToConsumers, nil
}

// restakedFPConsumerIDs returns the unique consumer IDs of non-Babylon finality providers
// The returned list is sorted in order to make sure the function is deterministic
func (k Keeper) restakedFPConsumerIDs(ctx context.Context, fpBTCPKs []bbn.BIP340PubKey) ([]string, error) {
	consumerIDMap := make(map[string]struct{})

	for i := range fpBTCPKs {
		fpBTCPK := fpBTCPKs[i]
		if _, err := k.GetFinalityProvider(ctx, fpBTCPK); err == nil {
			continue
		} else if consumerID, err := k.BscKeeper.GetConsumerOfFinalityProvider(ctx, &fpBTCPK); err == nil {
			consumerIDMap[consumerID] = struct{}{}
		} else {
			return nil, types.ErrFpNotFound.Wrapf("finality provider pk %s is not found", fpBTCPK.MarshalHex())
		}
	}

	uniqueConsumerIDs := make([]string, 0, len(consumerIDMap))
	for consumerID := range consumerIDMap {
		uniqueConsumerIDs = append(uniqueConsumerIDs, consumerID)
	}

	// Sort consumer IDs for deterministic ordering
	slices.Sort(uniqueConsumerIDs)

	return uniqueConsumerIDs, nil
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

// IsBtcDelegationActive returns true and no error if the BTC delegation is active.
// If it is not active it returns false and the reason as an error
func (k Keeper) IsBtcDelegationActive(ctx context.Context, stakingTxHash string) (bool, error) {
	btcDel, bsParams, err := k.getBTCDelWithParams(ctx, stakingTxHash)
	if err != nil {
		return false, err
	}

	btcTip := k.btclcKeeper.GetTipInfo(ctx)
	status := btcDel.GetStatus(btcTip.Height, bsParams.CovenantQuorum)
	if status != types.BTCDelegationStatus_ACTIVE {
		return false, fmt.Errorf("BTC delegation %s is not active, current status is %s", stakingTxHash, status.String())
	}

	return true, nil
}

func (k Keeper) getBTCDelWithParams(
	ctx context.Context,
	stakingTxHash string,
) (*types.BTCDelegation, *types.Params, error) {
	btcDel, err := k.GetBTCDelegation(ctx, stakingTxHash)
	if err != nil {
		return nil, nil, err
	}

	bsParams := k.GetParamsByVersion(ctx, btcDel.ParamsVersion)
	if bsParams == nil {
		panic("params version in BTC delegation is not found")
	}

	return btcDel, bsParams, nil
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
