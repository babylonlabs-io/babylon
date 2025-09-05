package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

// AddFinalityProvider adds the given finality provider to KVStore if it has valid
// commission and it was not inserted before
func (k Keeper) AddFinalityProvider(goCtx context.Context, msg *types.MsgCreateFinalityProvider) error {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := k.GetParams(ctx)

	fpAddr, err := sdk.AccAddressFromBech32(msg.Addr)
	if err != nil {
		return err
	}

	if err := k.CheckDuplicatedFpBbnAddr(ctx, fpAddr); err != nil {
		return err
	}

	// ensure commission rate is
	// - at least the minimum commission rate in parameters, and
	// - at most 1 or less than the MaxRate
	if msg.Commission.Rate.LT(params.MinCommissionRate) {
		return types.ErrCommissionLTMinRate.Wrapf("cannot set finality provider commission to less than minimum rate of %s", params.MinCommissionRate.String())
	}
	commissionInfo := types.NewCommissionInfoWithTime(msg.Commission.MaxRate, msg.Commission.MaxChangeRate, ctx.BlockHeader().Time)
	if err := commissionInfo.Validate(); err != nil {
		return err
	}

	if msg.Commission.Rate.GT(msg.Commission.MaxRate) {
		return types.ErrCommissionGTMaxRate
	}

	// ensure finality provider does not already exist
	if k.HasFinalityProvider(ctx, *msg.BtcPk) {
		return types.ErrFpRegistered
	}

	// default BSN ID is Babylon's chain ID
	bsnID := msg.GetBsnId()
	if bsnID == "" {
		// Babylon chain ID
		bsnID = ctx.ChainID()
	}

	// Consumer finality providers checks
	isConsumer := bsnID != ctx.ChainID()
	if isConsumer {
		// Ensure the consumer is registered
		cr, err := k.BscKeeper.GetConsumerRegister(ctx, bsnID)
		if err != nil || cr == nil {
			return types.ErrFpBSNIdNotRegistered
		}
		// Ensure there's an IBC channel open if it is a Cosmos BSN
		ccm := cr.GetCosmosConsumerMetadata()
		if ccm != nil {
			if ccm.ChannelId == "" || !k.BscKeeper.ConsumerHasIBCChannelOpen(ctx, bsnID, ccm.ChannelId) {
				return types.ErrFpConsumerNoIBCChannelOpen
			}
		}
	}

	// all good, add this finality provider
	fp := types.FinalityProvider{
		Description:    msg.Description,
		Commission:     &msg.Commission.Rate,
		Addr:           msg.Addr,
		BtcPk:          msg.BtcPk,
		Pop:            msg.Pop,
		BsnId:          bsnID,
		CommissionInfo: commissionInfo,
	}

	if err := k.SetFpBbnAddr(goCtx, fp.Address()); err != nil {
		return err
	}
	k.SetFinalityProvider(ctx, &fp)
	k.bsnIndexFinalityProvider(ctx, &fp)

	// Create BTC Staking Consumer Event for the new finality provider
	if isConsumer {
		if err := k.AddBTCStakingConsumerEvent(ctx, fp.BsnId, types.CreateNewFinalityProviderEvent(&fp)); err != nil {
			return err
		}
	}

	// notify subscriber
	return ctx.EventManager().EmitTypedEvent(types.NewEventFinalityProviderCreated(&fp))
}

// SetFinalityProvider adds the given finality provider to KVStore
func (k Keeper) SetFinalityProvider(ctx context.Context, fp *types.FinalityProvider) {
	store := k.finalityProviderStore(ctx)
	fpBytes := k.cdc.MustMarshal(fp)
	store.Set(fp.BtcPk.MustMarshal(), fpBytes)
}

func (k Keeper) SetFpBbnAddr(ctx context.Context, fpAddr sdk.AccAddress) error {
	return k.fpBbnAddr.Set(ctx, fpAddr)
}

// TODO: Examine collections and upgrade handler for this store
// https://github.com/babylonlabs-io/babylon/issues/1308
func (k Keeper) bsnIndexFinalityProvider(ctx context.Context, fp *types.FinalityProvider) {
	indexStore := k.finalityProviderBsnIndexStore(ctx)
	bsnKey := types.BuildBsnIndexKey(fp.BsnId, fp.BtcPk)
	indexStore.Set(bsnKey, []byte{}) // Empty value, just for existence
}

// UpdateFinalityProvider update the given finality provider to KVStore
func (k Keeper) UpdateFinalityProvider(ctx context.Context, fp *types.FinalityProvider) error {
	if !k.HasFinalityProvider(ctx, fp.BtcPk.MustMarshal()) {
		return types.ErrFpNotFound
	}

	k.SetFinalityProvider(ctx, fp)

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

// IterateFinalityProvider iterate over all finality providers
func (k Keeper) IterateFinalityProvider(ctx context.Context, f func(fp types.FinalityProvider) error) error {
	iter := k.finalityProviderStore(ctx).Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var fp types.FinalityProvider
		if err := fp.Unmarshal(iter.Value()); err != nil {
			return err
		}
		if err := f(fp); err != nil {
			return err
		}
	}

	return nil
}

// IsFinalityProviderDeleted returns if the finality provider is soft-deleted or not. The state of soft-deleted
// finality provider currently is only being used to avoid votes of undesired fps.
// Only upgrades, migrations or genesis can soft delete FPs.
func (k Keeper) IsFinalityProviderDeleted(ctx context.Context, fpBtcPk *bbn.BIP340PubKey) bool {
	blocked, err := k.finalityProvidersDeleted.Has(ctx, fpBtcPk.MustMarshal())
	if err != nil {
		return true
	}
	return blocked
}

// SoftDeleteFinalityProvider sets one finality provider as deleted.
// Note: Deleted finality providers can't cast finality votes.
func (k Keeper) SoftDeleteFinalityProvider(ctx context.Context, fpBtcPk *bbn.BIP340PubKey) error {
	return k.finalityProvidersDeleted.Set(ctx, fpBtcPk.MustMarshal())
}

// HasFpRegistered returns if there is already an finality provider registered with the babylon address
func (k Keeper) HasFpRegistered(ctx context.Context, fpBbnAddr sdk.AccAddress) (bool, error) {
	return k.fpBbnAddr.Has(ctx, fpBbnAddr)
}

// IsBabylonGenesisFinalityProvider checks if the finality provider is a Babylon Genesis finality provider
func (k Keeper) BabylonFinalityProviderExists(ctx context.Context, fpBTCPK []byte) bool {
	fp, err := k.GetFinalityProvider(ctx, fpBTCPK)
	if err != nil {
		// if the finality provider is not found, then there is no such Babylon finality provider
		return false
	}
	return fp.SecuresBabylonGenesis(sdk.UnwrapSDKContext(ctx))
}

// SlashFinalityProvider slashes a finality provider with the given PK
// A slashed finality provider will not have voting power
// This function handles both Babylon FPs and consumer FPs
func (k Keeper) SlashFinalityProvider(ctx context.Context, fpBTCPK []byte) error {
	fpBTCPKObj, err := bbn.NewBIP340PubKey(fpBTCPK)
	if err != nil {
		return fmt.Errorf("failed to parse BTC PK: %w", err)
	}

	// First try to get as Babylon finality provider
	fp, err := k.GetFinalityProvider(ctx, *fpBTCPKObj)
	if err != nil {
		return err
	}

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

	// we do not keep power distribution table for consumer FPs, so we only record the event for Babylon FPs
	if fp.SecuresBabylonGenesis(sdk.UnwrapSDKContext(ctx)) {
		// record slashed event. The next `BeginBlock` will consume this
		// event for updating the finality provider set
		powerUpdateEvent := types.NewEventPowerDistUpdateWithSlashedFP(fp.BtcPk)
		k.addPowerDistUpdateEvent(ctx, btcTip.Height, powerUpdateEvent)
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
	k.SetFinalityProvider(ctx, fp)

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

	// ensure finality provider is not slashed yet
	if fp.IsSlashed() {
		return types.ErrFpAlreadySlashed
	}

	// ensure finality provider is already jailed
	if !fp.IsJailed() {
		return types.ErrFpNotJailed
	}

	fp.Jailed = false
	k.SetFinalityProvider(ctx, fp)

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

// finalityProviderBsnIndexStore returns the KVStore of the finality provider BSN index
// prefix: FinalityProviderBsnIndexKey
// key: BSN ID || Bitcoin secp256k1 PK
// value: empty
func (k Keeper) finalityProviderBsnIndexStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.FinalityProviderBsnIndexKey)
}

// UpdateFinalityProviderCommission performs stateful validation checks of a new commission
// rate. If validation fails, an error is returned. If no errors, the commission
// and the CommissionUpdateTime are updated in the provided pointer
func (k Keeper) UpdateFinalityProviderCommission(goCtx context.Context, newCommission *math.LegacyDec, fp *types.FinalityProvider) error {
	if newCommission == nil {
		return nil
	}

	if fp.CommissionInfo == nil {
		return fmt.Errorf("cannot update commission. Finality provider with address %s does not have commission info defined", fp.Addr)
	}

	if newCommission.IsNegative() {
		return stktypes.ErrCommissionNegative
	}

	var (
		ctx       = sdk.UnwrapSDKContext(goCtx)
		blockTime = ctx.BlockHeader().Time
	)
	// check that there were no commission updates in the last 24hs
	if blockTime.Sub(fp.CommissionInfo.UpdateTime).Hours() < 24 {
		return stktypes.ErrCommissionUpdateTime
	}

	// ensure commission rate is at least the minimum commission rate in parameters
	minCommission := k.MinCommissionRate(goCtx)
	if newCommission.LT(minCommission) {
		return types.ErrCommissionLTMinRate.Wrapf(
			"cannot set finality provider commission to less than minimum rate of %s",
			minCommission)
	}

	if newCommission.GT(fp.CommissionInfo.MaxRate) {
		return stktypes.ErrCommissionGTMaxRate.Wrapf(
			"cannot set finality provider commission to more than max rate of %s",
			fp.CommissionInfo.MaxRate.String())
	}

	// check that the change rate does not exceed the max change rate allowed
	// new rate % points change cannot be greater than the max change rate
	if newCommission.Sub(*fp.Commission).Abs().GT(fp.CommissionInfo.MaxChangeRate) {
		return stktypes.ErrCommissionGTMaxChangeRate
	}

	// update commission and commission update time
	fp.Commission = newCommission
	fp.CommissionInfo.UpdateTime = blockTime

	return nil
}
