package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

// SetFinalityProvider adds the given finality provider to KVStore
func (k Keeper) SetFinalityProvider(ctx context.Context, fp *types.FinalityProvider) {
	store := k.finalityProviderStore(ctx)
	fpBytes := k.cdc.MustMarshal(fp)
	store.Set(fp.BtcPk.MustMarshal(), fpBytes)
}

// HasFinalityProvider checks if the finality provider exists
func (k Keeper) HasFinalityProvider(ctx context.Context, fpBTCPK []byte) bool {
	store := k.finalityProviderStore(ctx)
	return store.Has(fpBTCPK)
}

// GetFinalityProvider gets the finality provider with the given finality provider Bitcoin PK
func (k Keeper) GetFinalityProvider(ctx context.Context, fpBTCPK []byte) (*types.FinalityProvider, error) {
	store := k.finalityProviderStore(ctx)
	if !k.HasFinalityProvider(ctx, fpBTCPK) {
		return nil, types.ErrFpNotFound
	}
	fpBytes := store.Get(fpBTCPK)
	var fp types.FinalityProvider
	k.cdc.MustUnmarshal(fpBytes, &fp)
	return &fp, nil
}

// SlashFinalityProvider slashes a finality provider with the given PK
// A slashed finality provider will not have voting power
func (k Keeper) SlashFinalityProvider(ctx context.Context, fpBTCPK []byte) error {
	// ensure finality provider exists
	fp, err := k.GetFinalityProvider(ctx, fpBTCPK)
	if err != nil {
		return err
	}

	// ensure finality provider is not slashed yet
	if fp.IsSlashed() {
		return types.ErrFpAlreadySlashed
	}

	// set finality provider to be slashed
	fp.SlashedBabylonHeight = uint64(sdk.UnwrapSDKContext(ctx).HeaderInfo().Height)
	btcTip := k.btclcKeeper.GetTipInfo(ctx)
	if btcTip == nil {
		return fmt.Errorf("failed to get current BTC tip")
	}
	fp.SlashedBtcHeight = btcTip.Height
	k.SetFinalityProvider(ctx, fp)

	// record slashed event. The next `BeginBlock` will consume this
	// event for updating the finality provider set
	powerUpdateEvent := types.NewEventPowerDistUpdateWithSlashedFP(fp.BtcPk)
	k.addPowerDistUpdateEvent(ctx, btcTip.Height, powerUpdateEvent)

	return nil
}

// PropagateFPSlashingToConsumers propagates the slashing of a finality provider (FP) to all relevant consumer chains.
// It processes all delegations associated with the given FP and creates slashing events for each affected consumer chain.
//
// The function performs the following steps:
//  1. Retrieves all BTC delegations associated with the given finality provider.
//  2. Collects slashed events for each consumer chain using collectSlashedConsumerEvents:
//     a. For each delegation, creates a SlashedBTCDelegation event.
//     b. Identifies the consumer chains associated with the FPs in the delegation.
//     c. Ensures that each consumer chain receives only one event per delegation, even if multiple FPs in the delegation belong to the same consumer.
//  3. Sends the collected events to their respective consumer chains.
//
// Parameters:
// - ctx: The context for the operation.
// - fpBTCPK: The Bitcoin public key of the finality provider being slashed.
//
// Returns:
// - An error if any operation fails, nil otherwise.
func (k Keeper) PropagateFPSlashingToConsumers(ctx context.Context, fpBTCPK *bbn.BIP340PubKey) error {
	// Get all delegations for this finality provider
	delegations, err := k.getFPBTCDelegations(ctx, fpBTCPK)
	if err != nil {
		return err
	}

	// Collect slashed events for each consumer
	consumerEvents, err := k.collectSlashedConsumerEvents(ctx, delegations)
	if err != nil {
		return err
	}

	// Send collected events to each involved consumer chain
	for consumerID, events := range consumerEvents {
		if err := k.AddBTCStakingConsumerEvents(ctx, consumerID, events); err != nil {
			return err
		}
	}

	return nil
}

// collectSlashedConsumerEvents processes delegations and collects slashing events for each consumer chain.
// It ensures that each consumer receives only one event per delegation, even if multiple finality providers
// in the delegation belong to the same consumer.
//
// Parameters:
// - ctx: The context for the operation.
// - delegations: A slice of BTCDelegation objects to process.
//
// Returns:
// - A map where the key is the consumer ID and the value is a slice of BTCStakingConsumerEvents.
// - An error if any operation fails, nil otherwise.
func (k Keeper) collectSlashedConsumerEvents(ctx context.Context, delegations []*types.BTCDelegation) (map[string][]*types.BTCStakingConsumerEvent, error) {
	// Create a map to store FP to consumer ID mappings
	fpToConsumerMap := make(map[string]string)

	// Map to collect events for each consumer
	consumerEvents := make(map[string][]*types.BTCStakingConsumerEvent)

	for _, delegation := range delegations {
		consumerEvent := types.CreateSlashedBTCDelegationEvent(delegation)

		// Track consumers seen for this delegation
		seenConsumers := make(map[string]bool)

		for _, delegationFPBTCPK := range delegation.FpBtcPkList {
			fpBTCPKHex := delegationFPBTCPK.MarshalHex()
			consumerID, exists := fpToConsumerMap[fpBTCPKHex]
			if !exists {
				// If not in map, check if it's a Babylon FP or get its consumer
				if _, err := k.GetFinalityProvider(ctx, delegationFPBTCPK); err == nil {
					continue // It's a Babylon FP, skip
				} else if consumerID, err = k.bscKeeper.GetConsumerOfFinalityProvider(ctx, &delegationFPBTCPK); err == nil {
					// Found consumer, add to map
					fpToConsumerMap[fpBTCPKHex] = consumerID
				} else {
					return nil, types.ErrFpNotFound.Wrapf("finality provider pk %s is not found", fpBTCPKHex)
				}
			}

			// Add event to the consumer's event list only if not seen for this delegation
			if !seenConsumers[consumerID] {
				consumerEvents[consumerID] = append(consumerEvents[consumerID], consumerEvent)
				seenConsumers[consumerID] = true
			}
		}
	}

	return consumerEvents, nil
}

// RevertSluggishFinalityProvider sets the Sluggish flag of the given finality provider
// to false
func (k Keeper) RevertSluggishFinalityProvider(ctx context.Context, fpBTCPK []byte) error {
	// ensure finality provider exists
	fp, err := k.GetFinalityProvider(ctx, fpBTCPK)
	if err != nil {
		return err
	}

	// ignore the finality provider is already slashed
	// or detected as sluggish
	if fp.IsSlashed() || fp.IsSluggish() {
		return nil
	}

	fp.Sluggish = false
	k.SetFinalityProvider(ctx, fp)

	return nil
}

// finalityProviderStore returns the KVStore of the finality provider set
// prefix: FinalityProviderKey
// key: Bitcoin secp256k1 PK
// value: FinalityProvider object
func (k Keeper) finalityProviderStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.FinalityProviderKey)
}
