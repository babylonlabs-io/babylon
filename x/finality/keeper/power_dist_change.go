package keeper

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"cosmossdk.io/collections"
	"cosmossdk.io/store/prefix"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	bbn "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/v3/x/finality/types"
)

/* power distribution update */

// UpdatePowerDist updates the voting power table and distribution cache.
// This is triggered upon each `BeginBlock`
func (k Keeper) UpdatePowerDist(ctx context.Context) {
	height := uint64(sdk.UnwrapSDKContext(ctx).HeaderInfo().Height)
	btcTipHeight := k.BTCStakingKeeper.GetCurrentBTCHeight(ctx)

	// get the power dist cache in the last height
	dc := k.GetVotingPowerDistCache(ctx, height-1)
	if dc == nil {
		// no BTC staker at the prior height
		dc = ftypes.NewVotingPowerDistCache()
	}

	// get all power distribution update events during the previous tip
	// and the current tip
	lastBTCTipHeight := k.BTCStakingKeeper.GetBTCHeightAtBabylonHeight(ctx, height-1)
	events := k.BTCStakingKeeper.GetAllPowerDistUpdateEvents(ctx, lastBTCTipHeight, btcTipHeight)

	// clear all events that have been consumed in this function
	defer func() {
		for i := lastBTCTipHeight; i <= btcTipHeight; i++ {
			k.BTCStakingKeeper.ClearPowerDistUpdateEvents(ctx, i)
		}
	}()

	// reconcile old voting power distribution cache and new events
	// to construct the new distribution
	newDc := k.ProcessAllPowerDistUpdateEvents(ctx, dc, events)

	// record voting power and cache for this height
	k.RecordVotingPowerAndCache(ctx, newDc)
	// emit events for finality providers with state updates
	k.HandleFPStateUpdates(ctx, dc, newDc)
	// record metrics
	k.recordMetrics(newDc)
}

// RecordVotingPowerAndCache assigns voting power to each active finality provider
// with the following consideration:
// 1. the fp must have timestamped pub rand
// 2. the fp must in the top x ranked by the voting power (x is given by maxActiveFps)
func (k Keeper) RecordVotingPowerAndCache(ctx context.Context, newDc *ftypes.VotingPowerDistCache) {
	if newDc == nil {
		panic("the voting power distribution cache cannot be nil")
	}

	babylonTipHeight := uint64(sdk.UnwrapSDKContext(ctx).HeaderInfo().Height)

	// label fps with whether it has timestamped pub rand so that these fps
	// will not be assigned voting power
	for _, fpDistInfo := range newDc.FinalityProviders {
		// TODO calling HasTimestampedPubRand potentially iterates
		// all the pub rand committed by the fpDistInfo, which might slow down
		// the process, need optimization
		fpDistInfo.IsTimestamped = k.HasTimestampedPubRand(ctx, fpDistInfo.BtcPk, babylonTipHeight)
	}

	// apply the finality provider voting power dist info to the new cache
	// after which the cache would have active fps that are top N fps ranked
	// by voting power with timestamped pub rand
	maxActiveFps := k.GetParams(ctx).MaxActiveFinalityProviders
	newDc.ApplyActiveFinalityProviders(maxActiveFps)

	// set voting power table for each active finality providers at this height
	for i := uint32(0); i < newDc.NumActiveFps; i++ {
		fp := newDc.FinalityProviders[i]
		k.SetVotingPower(ctx, fp.BtcPk.MustMarshal(), babylonTipHeight, fp.TotalBondedSat)
	}

	// set the voting power distribution cache of the current height
	k.SetVotingPowerDistCache(ctx, babylonTipHeight, newDc)
}

// HandleFPStateUpdates emits events and triggers hooks for finality providers with state updates
func (k Keeper) HandleFPStateUpdates(ctx context.Context, prevDc, newDc *ftypes.VotingPowerDistCache) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	newlyActiveFPs := newDc.FindNewActiveFinalityProviders(prevDc)
	for _, fp := range newlyActiveFPs {
		if err := k.HandleActivatedFinalityProvider(ctx, fp.BtcPk); err != nil {
			panic(fmt.Errorf("failed to execute after finality provider %s activated", fp.BtcPk.MarshalHex()))
		}

		statusChangeEvent := types.NewFinalityProviderStatusChangeEvent(fp.BtcPk, types.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_ACTIVE)
		if err := sdkCtx.EventManager().EmitTypedEvent(statusChangeEvent); err != nil {
			panic(fmt.Errorf(
				"failed to emit FinalityProviderStatusChangeEvent with status %s: %w",
				types.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_ACTIVE.String(), err))
		}

		k.Logger(sdkCtx).Info("a new finality provider becomes active", "pk", fp.BtcPk.MarshalHex())
	}

	newlyInactiveFPs := newDc.FindNewInactiveFinalityProviders(prevDc)
	for _, fp := range newlyInactiveFPs {
		statusChangeEvent := types.NewFinalityProviderStatusChangeEvent(fp.BtcPk, types.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_INACTIVE)
		if err := sdkCtx.EventManager().EmitTypedEvent(statusChangeEvent); err != nil {
			panic(fmt.Errorf(
				"failed to emit FinalityProviderStatusChangeEvent with status %s: %w",
				types.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_INACTIVE.String(), err))
		}

		k.Logger(sdkCtx).Info("a new finality provider becomes inactive", "pk", fp.BtcPk.MarshalHex())
	}
}

// HandleActivatedFinalityProvider updates the signing info start height or create a new signing info
func (k Keeper) HandleActivatedFinalityProvider(ctx context.Context, fpPk *bbn.BIP340PubKey) error {
	signingInfo, err := k.FinalityProviderSigningTracker.Get(ctx, fpPk.MustMarshal())
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err == nil {
		// reset signing info
		signingInfo.StartHeight = sdkCtx.HeaderInfo().Height
		signingInfo.JailedUntil = time.Unix(0, 0).UTC()
	} else if errors.Is(err, collections.ErrNotFound) {
		signingInfo = ftypes.NewFinalityProviderSigningInfo(
			fpPk,
			sdkCtx.BlockHeight(),
			0,
		)
	}

	return k.FinalityProviderSigningTracker.Set(ctx, fpPk.MustMarshal(), signingInfo)
}

func (k Keeper) recordMetrics(dc *ftypes.VotingPowerDistCache) {
	// number of active FPs
	numActiveFPs := int(dc.NumActiveFps)
	types.RecordActiveFinalityProviders(numActiveFPs)
	// number of inactive FPs
	numInactiveFPs := len(dc.FinalityProviders) - numActiveFPs
	types.RecordInactiveFinalityProviders(numInactiveFPs)
	// staked Satoshi
	stakedSats := btcutil.Amount(0)
	for _, fp := range dc.FinalityProviders {
		stakedSats += btcutil.Amount(fp.TotalBondedSat)
	}
	numStakedBTCs := stakedSats.ToBTC()
	types.RecordMetricsKeyStakedBitcoins(float32(numStakedBTCs))
	// TODO: record number of BTC delegations under different status
}

// ProcessAllPowerDistUpdateEvents processes all events that affect
// voting power distribution and returns a new distribution cache.
// The following events will affect the voting power distribution:
// - newly active BTC delegations
// - newly unbonded BTC delegations
// - slashed finality providers
// - newly jailed finality providers
// - newly unjailed finality providers
func (k Keeper) ProcessAllPowerDistUpdateEvents(
	ctx context.Context,
	dc *ftypes.VotingPowerDistCache,
	events []*types.EventPowerDistUpdate,
) *ftypes.VotingPowerDistCache {
	// a map where key is finality provider's BTC PK hex and value is a list
	// of BTC delegations satoshis amount that newly become active under this provider
	activatedSatsByFpBtcPk := map[string][]uint64{}
	// a map where key is finality provider's BTC PK hex and value is a list
	// of BTC delegations satoshis that were unbonded or expired without previously
	// being unbonded
	unbondedSatsByFpBtcPk := map[string][]uint64{}
	// a map where key is slashed finality providers' BTC PK
	slashedFPs := map[string]struct{}{}
	// a map where key is jailed finality providers' BTC PK
	jailedFPs := map[string]struct{}{}
	// a map where key is unjailed finality providers' BTC PK
	unjailedFPs := map[string]struct{}{}

	// simple cache to load fp by his btc pk hex
	fpByBtcPkHex := map[string]*types.FinalityProvider{}

	/*
		filter and classify all events into new/expired BTC delegations and jailed/slashed FPs
	*/
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	for _, event := range events {
		switch typedEvent := event.Ev.(type) {
		case *types.EventPowerDistUpdate_BtcDelStateUpdate:
			delEvent := typedEvent.BtcDelStateUpdate
			delStkTxHash := delEvent.StakingTxHash

			btcDel, err := k.BTCStakingKeeper.GetBTCDelegation(ctx, delStkTxHash)
			if err != nil {
				panic(err) // only programming error
			}

			delParams := k.BTCStakingKeeper.GetParamsByVersion(ctx, btcDel.ParamsVersion)

			switch delEvent.NewState {
			case types.BTCDelegationStatus_ACTIVE:
				// newly active BTC delegation
				// add the BTC delegation to each restaked finality provider
				for _, fpBTCPK := range btcDel.FpBtcPkList {
					fpBTCPKHex := fpBTCPK.MarshalHex()
					if !k.BTCStakingKeeper.HasFinalityProvider(ctx, fpBTCPK) {
						// This is a consumer FP rather than Babylon FP, skip it
						continue
					}
					activatedSatsByFpBtcPk[fpBTCPKHex] = append(activatedSatsByFpBtcPk[fpBTCPKHex], btcDel.TotalSat)
				}

				// FP could be already slashed when it is being activated, but it is okay
				// since slashed finality providers do not earn rewards
				k.processRewardTracker(ctx, fpByBtcPkHex, btcDel, func(fp, del sdk.AccAddress, sats uint64) {
					k.MustProcessBtcDelegationActivated(ctx, fp, del, sats)
				})
			case types.BTCDelegationStatus_UNBONDED:
				// In case of delegation transtioning from phase-1 it is possible that
				// somebody unbonds before receiving the required covenant signatures.
				if btcDel.HasCovenantQuorums(delParams.CovenantQuorum) {
					// add the unbonded BTC delegation to the map
					k.processPowerDistUpdateEventUnbond(ctx, fpByBtcPkHex, btcDel, unbondedSatsByFpBtcPk)
				}
			case types.BTCDelegationStatus_EXPIRED:
				types.EmitExpiredDelegationEvent(sdkCtx, delStkTxHash)
				// We process expired event if:
				// - it hasn't unbonded early
				// - it has all required covenant signatures
				if !btcDel.IsUnbondedEarly() && btcDel.HasCovenantQuorums(delParams.CovenantQuorum) {
					// only adds to the new unbonded list if it hasn't
					// previously unbonded with types.BTCDelegationStatus_UNBONDED
					k.processPowerDistUpdateEventUnbond(ctx, fpByBtcPkHex, btcDel, unbondedSatsByFpBtcPk)
				}
			}
		case *types.EventPowerDistUpdate_SlashedFp:
			// record slashed fps
			types.EmitSlashedFPEvent(sdkCtx, typedEvent.SlashedFp.Pk)
			fpBTCPKHex := typedEvent.SlashedFp.Pk.MarshalHex()
			slashedFPs[fpBTCPKHex] = struct{}{}
			// TODO(rafilx): handle slashed fps prunning
			// It is not possible to slash fp and delete all of his data at the
			// babylon block height that is being processed, because
			// the function RewardBTCStaking is called a few blocks behind.
			// If the data is deleted at the slash event, when slashed fps are
			// receveing rewards from a few blocks behind HandleRewarding
			// verifies the next block height to be rewarded.
		case *types.EventPowerDistUpdate_JailedFp:
			// record jailed fps
			types.EmitJailedFPEvent(sdkCtx, typedEvent.JailedFp.Pk)
			jailedFPs[typedEvent.JailedFp.Pk.MarshalHex()] = struct{}{}
		case *types.EventPowerDistUpdate_UnjailedFp:
			// record unjailed fps
			unjailedFPs[typedEvent.UnjailedFp.Pk.MarshalHex()] = struct{}{}
		}
	}

	/*
		At this point, there is voting power update.
		Then, construct a voting power dist cache by reconciling the previous
		cache and all the new events.
	*/
	// TODO: the algorithm needs to iterate over all the finality providers so remains
	// sub-optimal. Ideally we only need to iterate over all events above rather
	// than the entire cache.
	newDc := ftypes.NewVotingPowerDistCache()

	// iterate over all finality providers and apply all events
	for i := range dc.FinalityProviders {
		// create a copy of the finality provider
		fp := *dc.FinalityProviders[i]
		fpBTCPKHex := fp.BtcPk.MarshalHex()

		// if this finality provider is slashed, continue to avoid
		// assigning delegation to it
		_, isSlashed := slashedFPs[fpBTCPKHex]
		if isSlashed {
			fp.IsSlashed = true
			continue
		}

		// set IsJailed to be true if the fp is jailed
		// Note that jailed fp can still accept delegations
		// but won't be assigned with voting power
		if _, ok := jailedFPs[fpBTCPKHex]; ok {
			fp.IsJailed = true
		}

		// set IsJailed to be false if the fp is unjailed
		if _, ok := unjailedFPs[fpBTCPKHex]; ok {
			fp.IsJailed = false
		}

		// process all new BTC delegations under this finality provider
		if fpActiveSats, ok := activatedSatsByFpBtcPk[fpBTCPKHex]; ok {
			// handle new BTC delegations for this finality provider
			for _, activatedSats := range fpActiveSats {
				fp.AddBondedSats(activatedSats)
			}
			// remove the finality provider entry in fpActiveSats map, so that
			// after the for loop the rest entries in fpActiveSats belongs to new
			// finality providers with new BTC delegations
			delete(activatedSatsByFpBtcPk, fpBTCPKHex)
		}

		// process all new unbonding BTC delegations under this finality provider
		if fpUnbondedSats, ok := unbondedSatsByFpBtcPk[fpBTCPKHex]; ok {
			// handle unbonded delegations for this finality provider
			for _, unbodedSats := range fpUnbondedSats {
				fp.RemoveBondedSats(unbodedSats)
			}
			// remove the finality provider entry in fpUnbondedSats map, so that
			// after the for loop the rest entries in fpUnbondedSats belongs to new
			// finality providers that might have btc delegations entries
			// that activated and unbonded in the same slice of events
			delete(unbondedSatsByFpBtcPk, fpBTCPKHex)
		}

		// add this finality provider to the new cache if it has voting power
		if fp.TotalBondedSat > 0 {
			newDc.AddFinalityProviderDistInfo(&fp)
		}
	}

	/*
		process new BTC delegations under new finality providers in activeBTCDels
	*/
	// sort new finality providers in activeBTCDels to ensure determinism
	fpActiveBtcPkHexList := make([]string, 0, len(activatedSatsByFpBtcPk))
	for fpBTCPKHex := range activatedSatsByFpBtcPk {
		// if the fp was slashed, should not even be added to the list
		_, isSlashed := slashedFPs[fpBTCPKHex]
		if isSlashed {
			continue
		}
		fpActiveBtcPkHexList = append(fpActiveBtcPkHexList, fpBTCPKHex)
	}
	sort.SliceStable(fpActiveBtcPkHexList, func(i, j int) bool {
		return fpActiveBtcPkHexList[i] < fpActiveBtcPkHexList[j]
	})

	// for each new finality provider, apply the new BTC delegations to the new dist cache
	for _, fpBTCPKHex := range fpActiveBtcPkHexList {
		// get the finality provider and initialise its dist info
		newFP := k.loadFP(ctx, fpByBtcPkHex, fpBTCPKHex)
		if newFP == nil {
			// This is a consumer FP rather than Babylon FP, skip it
			continue
		}
		// if the fp is slashed it shouldn't be included in the newDc
		if newFP.IsSlashed() {
			// case if a BTC delegation is created without inclusion proof
			// the finality provider gets slashed
			// inclusion proof gets included and generates an EventPowerDistUpdate_BtcDelStateUpdate
			continue
		}
		fpDistInfo := ftypes.NewFinalityProviderDistInfo(newFP)

		// check for jailing cases
		if _, ok := jailedFPs[fpBTCPKHex]; ok {
			fpDistInfo.IsJailed = true
		}
		if _, ok := unjailedFPs[fpBTCPKHex]; ok {
			fpDistInfo.IsJailed = false
		}

		// add each BTC delegation
		fpActiveSats := activatedSatsByFpBtcPk[fpBTCPKHex]
		for _, activatedSats := range fpActiveSats {
			fpDistInfo.AddBondedSats(activatedSats)
		}

		// edge case where we might be processing an unbonded event
		// from a newly active finality provider in the same slice
		// of events received.
		fpUnbondedSats := unbondedSatsByFpBtcPk[fpBTCPKHex]
		for _, unbodedSats := range fpUnbondedSats {
			fpDistInfo.RemoveBondedSats(unbodedSats)
		}

		// add this finality provider to the new cache if it has voting power
		if fpDistInfo.TotalBondedSat > 0 {
			newDc.AddFinalityProviderDistInfo(fpDistInfo)
		}
	}

	return newDc
}

func (k Keeper) processPowerDistUpdateEventUnbond(
	ctx context.Context,
	cacheFpByBtcPkHex map[string]*types.FinalityProvider,
	btcDel *types.BTCDelegation,
	unbondedSatsByFpBtcPk map[string][]uint64,
) {
	for _, fpBTCPK := range btcDel.FpBtcPkList {
		fpBTCPKHex := fpBTCPK.MarshalHex()
		if !k.BTCStakingKeeper.HasFinalityProvider(ctx, fpBTCPK) {
			// This is a consumer FP rather than Babylon FP, skip it
			continue
		}
		unbondedSatsByFpBtcPk[fpBTCPKHex] = append(unbondedSatsByFpBtcPk[fpBTCPKHex], btcDel.TotalSat)
	}
	k.processRewardTracker(ctx, cacheFpByBtcPkHex, btcDel, func(fp, del sdk.AccAddress, sats uint64) {
		k.MustProcessBtcDelegationUnbonded(ctx, fp, del, sats)
	})
}

func (k Keeper) SetVotingPowerDistCache(ctx context.Context, height uint64, dc *ftypes.VotingPowerDistCache) {
	store := k.votingPowerDistCacheStore(ctx)
	store.Set(sdk.Uint64ToBigEndian(height), k.cdc.MustMarshal(dc))
}

func (k Keeper) GetVotingPowerDistCache(ctx context.Context, height uint64) *ftypes.VotingPowerDistCache {
	store := k.votingPowerDistCacheStore(ctx)
	rdcBytes := store.Get(sdk.Uint64ToBigEndian(height))
	if len(rdcBytes) == 0 {
		return nil
	}
	var dc ftypes.VotingPowerDistCache
	k.cdc.MustUnmarshal(rdcBytes, &dc)
	return &dc
}

func (k Keeper) RemoveVotingPowerDistCache(ctx context.Context, height uint64) {
	store := k.votingPowerDistCacheStore(ctx)
	store.Delete(sdk.Uint64ToBigEndian(height))
}

// votingPowerDistCacheStore returns the KVStore of the voting power distribution cache
// prefix: VotingPowerDistCacheKey
// key: Babylon block height
// value: VotingPowerDistCache
func (k Keeper) votingPowerDistCacheStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, ftypes.VotingPowerDistCacheKey)
}

// processRewardTracker loads Babylon FPs from the given BTC delegation
// and executes the given function over each Babylon FP, delegator address
// and satoshi amounts.
// NOTE:
//   - The function will only be executed over Babylon FPs but not consumer FPs
//   - The function makes uses of the fpByBtcPkHex cache, and the cache only
//     contains Babylon FPs but not consumer FPs
func (k Keeper) processRewardTracker(
	ctx context.Context,
	fpByBtcPkHex map[string]*types.FinalityProvider,
	btcDel *types.BTCDelegation,
	f func(fp, del sdk.AccAddress, sats uint64),
) {
	delAddr := sdk.MustAccAddressFromBech32(btcDel.StakerAddr)
	for _, fpBTCPK := range btcDel.FpBtcPkList {
		fp := k.loadFP(ctx, fpByBtcPkHex, fpBTCPK.MarshalHex())
		if fp == nil {
			// This is a consumer FP rather than Babylon FP, skip it
			continue
		}
		f(fp.Address(), delAddr, btcDel.TotalSat)
	}
}

// MustProcessBtcDelegationActivated calls the IncentiveKeeper.AddEventBtcDelegationActivated
// and panics if it errors
func (k Keeper) MustProcessBtcDelegationActivated(ctx context.Context, fp, del sdk.AccAddress, sats uint64) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.HeaderInfo().Height)
	err := k.IncentiveKeeper.AddEventBtcDelegationActivated(ctx, height, fp, del, sats)
	if err != nil {
		k.Logger(sdkCtx).Error(
			"failed to add event of activated BTC delegation",
			"blockHeight", height,
		)
		panic(err)
	}
}

// MustProcessBtcDelegationUnbonded calls the IncentiveKeeper.AddEventBtcDelegationUnbonded
// and panics if it errors
func (k Keeper) MustProcessBtcDelegationUnbonded(ctx context.Context, fp, del sdk.AccAddress, sats uint64) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.HeaderInfo().Height)
	err := k.IncentiveKeeper.AddEventBtcDelegationUnbonded(ctx, height, fp, del, sats)
	if err != nil {
		k.Logger(sdkCtx).Error(
			"failed to add event of unbonded BTC delegation",
			"blockHeight", height,
		)
		panic(err)
	}
}

func (k Keeper) loadFP(
	ctx context.Context,
	cacheFpByBtcPkHex map[string]*types.FinalityProvider,
	fpBTCPKHex string,
) *types.FinalityProvider {
	fp, found := cacheFpByBtcPkHex[fpBTCPKHex]
	if !found {
		fpBTCPK, err := bbn.NewBIP340PubKeyFromHex(fpBTCPKHex)
		if err != nil {
			panic(err) // only programming error
		}
		fp, err = k.BTCStakingKeeper.GetFinalityProvider(ctx, *fpBTCPK)
		if err != nil {
			// This is a consumer FP, return nil and the caller shall
			// skip it
			return nil
		}
		cacheFpByBtcPkHex[fpBTCPKHex] = fp
	}

	return fp
}
