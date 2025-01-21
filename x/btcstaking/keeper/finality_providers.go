package keeper

import (
	"context"
	"fmt"

	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

// AddFinalityProvider adds the given finality provider to KVStore if it has valid
// commission and it was not inserted before
func (k Keeper) AddFinalityProvider(goCtx context.Context, msg *types.MsgCreateFinalityProvider) error {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := k.GetParams(ctx)
	// ensure commission rate is
	// - at least the minimum commission rate in parameters, and
	// - at most 1
	if msg.Commission.LT(params.MinCommissionRate) {
		return types.ErrCommissionLTMinRate.Wrapf("cannot set finality provider commission to less than minimum rate of %s", params.MinCommissionRate.String())
	}
	if msg.Commission.GT(sdkmath.LegacyOneDec()) {
		return types.ErrCommissionGTMaxRate
	}

	// ensure finality provider does not already exist
	if k.HasFinalityProvider(ctx, *msg.BtcPk) {
		return types.ErrFpRegistered
	}

	// default consumer ID is Babylon's chain ID
	consumerID := msg.GetConsumerId()
	if consumerID == "" {
		// Babylon chain ID
		consumerID = ctx.ChainID()
	}

	// all good, add this finality provider
	fp := types.FinalityProvider{
		Description: msg.Description,
		Commission:  msg.Commission,
		Addr:        msg.Addr,
		BtcPk:       msg.BtcPk,
		Pop:         msg.Pop,
		ConsumerId:  consumerID,
	}

	if consumerID == ctx.ChainID() {
		k.setFinalityProvider(ctx, &fp)
	} else {
		if err := k.SetConsumerFinalityProvider(ctx, &fp, consumerID); err != nil {
			return err
		}
	}

	// notify subscriber
	return ctx.EventManager().EmitTypedEvent(types.NewEventFinalityProviderCreated(&fp))
}

// setFinalityProvider adds the given finality provider to KVStore
func (k Keeper) setFinalityProvider(ctx context.Context, fp *types.FinalityProvider) {
	store := k.finalityProviderStore(ctx)
	fpBytes := k.cdc.MustMarshal(fp)
	store.Set(fp.BtcPk.MustMarshal(), fpBytes)
}

// UpdateFinalityProvider update the given finality provider to KVStore
func (k Keeper) UpdateFinalityProvider(ctx context.Context, fp *types.FinalityProvider) error {
	if !k.HasFinalityProvider(ctx, fp.BtcPk.MustMarshal()) {
		return types.ErrFpNotFound
	}

	k.setFinalityProvider(ctx, fp)

	return nil
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
	k.setFinalityProvider(ctx, fp)

	// record slashed event. The next `BeginBlock` will consume this
	// event for updating the finality provider set
	powerUpdateEvent := types.NewEventPowerDistUpdateWithSlashedFP(fp.BtcPk)
	k.addPowerDistUpdateEvent(ctx, btcTip.Height, powerUpdateEvent)

	return nil
}

// SlashConsumerFinalityProvider slashes a consumer finality provider with the given PK
func (k Keeper) SlashConsumerFinalityProvider(ctx context.Context, consumerID string, fpBTCPK *bbn.BIP340PubKey) error {
	// Get consumer finality provider
	fp, err := k.BscKeeper.GetConsumerFinalityProvider(ctx, consumerID, fpBTCPK)
	if err != nil {
		return err
	}

	// Return error if already slashed
	if fp.IsSlashed() {
		return types.ErrFpAlreadySlashed
	}

	// Set slashed height
	fp.SlashedBabylonHeight = uint64(sdk.UnwrapSDKContext(ctx).HeaderInfo().Height)
	btcTip := k.btclcKeeper.GetTipInfo(ctx)
	if btcTip == nil {
		return fmt.Errorf("failed to get current BTC tip")
	}
	fp.SlashedBtcHeight = btcTip.Height
	k.BscKeeper.SetConsumerFinalityProvider(ctx, fp)

	// Process all delegations for this consumer finality provider and record slashed events
	err = k.HandleFPBTCDelegations(ctx, fpBTCPK, func(btcDel *types.BTCDelegation) error {
		stakingTxHash := btcDel.MustGetStakingTxHash().String()
		eventSlashedBTCDelegation := types.NewEventPowerDistUpdateWithSlashedBTCDelegation(stakingTxHash)
		k.addPowerDistUpdateEvent(ctx, btcTip.Height, eventSlashedBTCDelegation)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to handle BTC delegations: %w", err)
	}

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
	// Map to collect events for each consumer
	consumerEvents := make(map[string][]*types.BTCStakingConsumerEvent)
	// Create a map to store FP to consumer ID mappings
	fpToConsumerMap := make(map[string]string)

	// Process all delegations for this finality provider and collect slashing events
	// for each consumer chain. Ensures that each consumer receives only one event per
	// delegation, even if multiple finality providers in the delegation belong to the same consumer.
	err := k.HandleFPBTCDelegations(ctx, fpBTCPK, func(delegation *types.BTCDelegation) error {
		consumerEvent := types.CreateSlashedBTCDelegationEvent(delegation)

		// Track consumers seen for this delegation
		seenConsumers := make(map[string]struct{})

		for _, delegationFPBTCPK := range delegation.FpBtcPkList {
			fpBTCPKHex := delegationFPBTCPK.MarshalHex()
			consumerID, exists := fpToConsumerMap[fpBTCPKHex]
			if !exists {
				// If not in map, check if it's a Babylon FP or get its consumer
				// TODO: avoid querying GetFinalityProvider again by passing the result
				// https://github.com/babylonlabs-io/babylon/blob/873f1232365573a97032037af4ac99b5e3fcada8/x/btcstaking/keeper/btc_delegators.go#L79 to this function
				if _, err := k.GetFinalityProvider(ctx, delegationFPBTCPK); err == nil {
					continue // It's a Babylon FP, skip
				} else if consumerID, err = k.BscKeeper.GetConsumerOfFinalityProvider(ctx, &delegationFPBTCPK); err == nil {
					// Found consumer, add to map
					fpToConsumerMap[fpBTCPKHex] = consumerID
				} else {
					return types.ErrFpNotFound.Wrapf("finality provider pk %s is not found", fpBTCPKHex)
				}
			}

			// Only add event once per consumer per delegation
			if _, ok := seenConsumers[consumerID]; !ok {
				consumerEvents[consumerID] = append(consumerEvents[consumerID], consumerEvent)
				seenConsumers[consumerID] = struct{}{}
			}
		}
		return nil
	})
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

// JailFinalityProvider jails a finality provider with the given PK
// A jailed finality provider will not have voting power until it is
// unjailed (assuming it still ranks top N and has timestamped pub rand)
func (k Keeper) JailFinalityProvider(ctx context.Context, fpBTCPK []byte) error {
	// ensure finality provider exists
	fp, err := k.GetFinalityProvider(ctx, fpBTCPK)
	if err != nil {
		return err
	}

	// ensure finality provider is not slashed yet
	if fp.IsSlashed() {
		return types.ErrFpAlreadySlashed
	}

	// ensure finality provider is not jailed yet
	if fp.IsJailed() {
		return types.ErrFpAlreadyJailed
	}

	// set finality provider to be jailed
	fp.Jailed = true
	k.setFinalityProvider(ctx, fp)

	btcTip := k.btclcKeeper.GetTipInfo(ctx)
	if btcTip == nil {
		return fmt.Errorf("failed to get current BTC tip")
	}

	// record jailed event. The next `BeginBlock` will consume this
	// event for updating the finality provider set
	powerUpdateEvent := types.NewEventPowerDistUpdateWithJailedFP(fp.BtcPk)
	k.addPowerDistUpdateEvent(ctx, btcTip.Height, powerUpdateEvent)

	return nil
}

// UnjailFinalityProvider reverts the Jailed flag of a finality provider
func (k Keeper) UnjailFinalityProvider(ctx context.Context, fpBTCPK []byte) error {
	// ensure finality provider exists
	fp, err := k.GetFinalityProvider(ctx, fpBTCPK)
	if err != nil {
		return err
	}

	// ensure finality provider is already jailed
	if !fp.IsJailed() {
		return types.ErrFpNotJailed
	}

	fp.Jailed = false
	k.setFinalityProvider(ctx, fp)

	btcTip := k.btclcKeeper.GetTipInfo(ctx)
	if btcTip == nil {
		return fmt.Errorf("failed to get current BTC tip")
	}

	// record unjailed event. The next `BeginBlock` will consume this
	// event for updating the finality provider set
	powerUpdateEvent := types.NewEventPowerDistUpdateWithUnjailedFP(fp.BtcPk)
	k.addPowerDistUpdateEvent(ctx, btcTip.Height, powerUpdateEvent)

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
