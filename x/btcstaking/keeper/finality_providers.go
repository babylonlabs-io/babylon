package keeper

import (
	"context"
	"fmt"

	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

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

	// all good, add this finality provider
	fp := types.FinalityProvider{
		Description: msg.Description,
		Commission:  msg.Commission,
		Addr:        msg.Addr,
		BtcPk:       msg.BtcPk,
		Pop:         msg.Pop,
	}
	k.setFinalityProvider(ctx, &fp)

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
