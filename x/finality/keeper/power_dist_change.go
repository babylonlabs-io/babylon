package keeper

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"
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

	lastBTCTipHeight := k.BTCStakingKeeper.GetBTCHeightAtBabylonHeight(ctx, height-1)
	// clear all events that have been consumed in this function
	defer func() {
		for i := lastBTCTipHeight; i <= btcTipHeight; i++ {
			k.BTCStakingKeeper.ClearPowerDistUpdateEvents(ctx, i)
		}
	}()

	// reconcile old voting power distribution cache and new events
	// to construct the new distribution
	newDc := k.ProcessAllPowerDistUpdateEvents(ctx, dc, lastBTCTipHeight, btcTipHeight)

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
	lastBTCTip uint32,
	curBTCTip uint32,
) *ftypes.VotingPowerDistCache {
	state := ftypes.NewProcessingState()
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	for btcHeight := lastBTCTip; btcHeight <= curBTCTip; btcHeight++ {
		k.processEventsAtHeight(ctx, sdkCtx, btcHeight, state)
	}

	// Process deferred EXPIRED events
	k.processExpiredEvents(ctx, sdkCtx, state)

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

		fpStatus, _ := state.FPStates[fpBTCPKHex]
		switch fpStatus {
		case ftypes.FinalityProviderState_SLASHED:
			// if this finality provider is slashed, continue to avoid
			// assigning delegation to it
			fp.IsSlashed = true
			continue
		case ftypes.FinalityProviderState_JAILED:
			// set IsJailed to be true if the fp is jailed
			// Note that jailed fp can still accept delegations
			// but won't be assigned with voting power
			fp.IsJailed = true
		case ftypes.FinalityProviderState_UNJAILED:
			// set IsJailed to be false if the fp is unjailed
			fp.IsJailed = false
		}

		// process all new BTC delegations under this finality provider
		if fpActiveSats, ok := state.ActivatedSatsByFpBtcPk[fpBTCPKHex]; ok {
			// handle new BTC delegations for this finality provider
			for _, activatedSats := range fpActiveSats {
				fp.AddBondedSats(activatedSats)
			}
			// remove the finality provider entry in fpActiveSats map, so that
			// after the for loop the rest entries in fpActiveSats belongs to new
			// finality providers with new BTC delegations
			delete(state.ActivatedSatsByFpBtcPk, fpBTCPKHex)
		}

		// process all new unbonding BTC delegations under this finality provider
		if fpUnbondedSats, ok := state.UnbondedSatsByFpBtcPk[fpBTCPKHex]; ok {
			// handle unbonded delegations for this finality provider
			for _, unbodedSats := range fpUnbondedSats {
				fp.RemoveBondedSats(unbodedSats)
			}
			// remove the finality provider entry in fpUnbondedSats map, so that
			// after the for loop the rest entries in fpUnbondedSats belongs to new
			// finality providers that might have btc delegations entries
			// that activated and unbonded in the same slice of events
			delete(state.UnbondedSatsByFpBtcPk, fpBTCPKHex)
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
	fpActiveBtcPkHexList := make([]string, 0, len(state.ActivatedSatsByFpBtcPk))
	for fpBTCPKHex := range state.ActivatedSatsByFpBtcPk {
		// if the fp was slashed, should not even be added to the list
		fpStatus, _ := state.FPStates[fpBTCPKHex]
		if fpStatus == ftypes.FinalityProviderState_SLASHED {
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
		newFP, err := k.loadFP(ctx, state.FpByBtcPkHex, fpBTCPKHex)
		if err != nil {
			panic(fmt.Sprintf("unable to load fp %s - %s", state.FpByBtcPkHex, err.Error()))
		}
		if !newFP.SecuresBabylonGenesis(sdkCtx) {
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
		fpStatus, _ := state.FPStates[fpBTCPKHex]
		if fpStatus == ftypes.FinalityProviderState_JAILED {
			fpDistInfo.IsJailed = true
		}
		if fpStatus == ftypes.FinalityProviderState_UNJAILED {
			fpDistInfo.IsJailed = false
		}

		// add each BTC delegation
		fpActiveSats := state.ActivatedSatsByFpBtcPk[fpBTCPKHex]
		for _, activatedSats := range fpActiveSats {
			fpDistInfo.AddBondedSats(activatedSats)
		}

		// edge case where we might be processing an unbonded event
		// from a newly active finality provider in the same slice
		// of events received.
		fpUnbondedSats := state.UnbondedSatsByFpBtcPk[fpBTCPKHex]
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

// processEventsAtHeight processes all power distribution update events at a given BTC height
// and updates the processing state accordingly.
// It iterates through the events, classifying them into BTC delegation updates and finality provider
// state updates. It handles BTC delegation updates immediately, while deferring expired events
// for later processing. Finality provider state updates are processed immediately.
func (k Keeper) processEventsAtHeight(ctx context.Context, sdkCtx sdk.Context, btcHeight uint32, state *ftypes.ProcessingState) {
	iter := k.BTCStakingKeeper.PowerDistUpdateEventBtcHeightStoreIterator(ctx, btcHeight)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var event types.EventPowerDistUpdate
		k.cdc.MustUnmarshal(iter.Value(), &event)

		switch typedEvent := event.Ev.(type) {
		case *types.EventPowerDistUpdate_BtcDelStateUpdate:
			if typedEvent.BtcDelStateUpdate.NewState == types.BTCDelegationStatus_EXPIRED {
				// Defer EXPIRED events for later processing
				state.ExpiredEvents = append(state.ExpiredEvents, typedEvent)
			} else {
				// Process ACTIVE/UNBONDED events immediately
				k.processBtcDelUpdateImmediate(ctx, state, typedEvent)
			}
		default:
			// Process all FP events immediately
			k.processFPEventImmediate(sdkCtx, state, event)
		}
	}
}

// processBtcDelUpdateImmediate processes a BTC delegation update event immediately.
// It handles the BTC delegation state update by checking the new state and
// updating the processing state accordingly.
func (k Keeper) processBtcDelUpdateImmediate(ctx context.Context, state *ftypes.ProcessingState, event *types.EventPowerDistUpdate_BtcDelStateUpdate) {
	delEvent := event.BtcDelStateUpdate
	delStkTxHash := delEvent.StakingTxHash

	btcDel, err := k.BTCStakingKeeper.GetBTCDelegation(ctx, delStkTxHash)
	if err != nil {
		panic(err) // only programming error
	}
	delParams := k.BTCStakingKeeper.GetParamsByVersion(ctx, btcDel.ParamsVersion)

	switch delEvent.NewState {
	case types.BTCDelegationStatus_ACTIVE:
		k.processPowerDistUpdateEventActive(ctx, state, btcDel)
	case types.BTCDelegationStatus_UNBONDED:
		// In case of delegation transtioning from phase-1 it is possible that
		// somebody unbonds before receiving the required covenant signatures.
		hasQuorum, err := k.BTCStakingKeeper.BtcDelHasCovenantQuorums(ctx, btcDel, delParams.CovenantQuorum)
		if err != nil {
			panic(err)
		}
		if hasQuorum {
			// add the unbonded BTC delegation to the map
			k.processPowerDistUpdateEventUnbond(ctx, state, btcDel)
		}
	}
}

func (k Keeper) processExpiredEvents(ctx context.Context, sdkCtx sdk.Context, state *ftypes.ProcessingState) {
	for _, event := range state.ExpiredEvents {
		delEvent := event.BtcDelStateUpdate
		delStkTxHash := delEvent.StakingTxHash

		btcDel, err := k.BTCStakingKeeper.GetBTCDelegation(ctx, delStkTxHash)
		if err != nil {
			panic(err) // only programming error
		}

		delParams := k.BTCStakingKeeper.GetParamsByVersion(ctx, btcDel.ParamsVersion)

		types.EmitExpiredDelegationEvent(sdkCtx, delStkTxHash)
		// We process expired event if:
		// - it hasn't unbonded early
		// - it has all required covenant signatures
		hasQuorum, err := k.BTCStakingKeeper.BtcDelHasCovenantQuorums(ctx, btcDel, delParams.CovenantQuorum)
		if err != nil {
			panic(err)
		}
		if !btcDel.IsUnbondedEarly() && hasQuorum {
			// only adds to the new unbonded list if it hasn't
			// previously unbonded with types.BTCDelegationStatus_UNBONDED
			k.processPowerDistUpdateEventUnbond(ctx, state, btcDel)
		}
	}
}

func (k Keeper) processFPEventImmediate(ctx sdk.Context, state *ftypes.ProcessingState, event types.EventPowerDistUpdate) {
	switch typedEvent := event.Ev.(type) {
	case *types.EventPowerDistUpdate_SlashedFp:
		// record slashed fps
		types.EmitSlashedFPEvent(ctx, typedEvent.SlashedFp.Pk)
		fpBTCPKHex := typedEvent.SlashedFp.Pk.MarshalHex()
		state.FPStates[fpBTCPKHex] = ftypes.FinalityProviderState_SLASHED
		// TODO(rafilx): handle slashed fps prunning
		// It is not possible to slash fp and delete all of his data at the
		// babylon block height that is being processed, because
		// the function RewardBTCStaking is called a few blocks behind.
		// If the data is deleted at the slash event, when slashed fps are
		// receveing rewards from a few blocks behind HandleRewarding
		// verifies the next block height to be rewarded.
	case *types.EventPowerDistUpdate_JailedFp:
		// record jailed fps
		types.EmitJailedFPEvent(ctx, typedEvent.JailedFp.Pk)
		state.FPStates[typedEvent.JailedFp.Pk.MarshalHex()] = ftypes.FinalityProviderState_JAILED
	case *types.EventPowerDistUpdate_UnjailedFp:
		// record unjailed fps
		state.FPStates[typedEvent.UnjailedFp.Pk.MarshalHex()] = ftypes.FinalityProviderState_UNJAILED
	}
}

// processPowerDistUpdateEventUnbond actively updates the unbonded sats
// map and process the incentives reward tracking structures for unbonded btc dels.
func (k Keeper) processPowerDistUpdateEventUnbond(
	ctx context.Context,
	state *ftypes.ProcessingState,
	btcDel *types.BTCDelegation,
) {
	for _, fpBTCPK := range btcDel.FpBtcPkList {
		fpBTCPKHex := fpBTCPK.MarshalHex()
		if !k.BTCStakingKeeper.BabylonFinalityProviderExists(ctx, fpBTCPK) {
			// This is a consumer FP rather than Babylon FP, skip it
			continue
		}
		state.UnbondedSatsByFpBtcPk[fpBTCPKHex] = append(state.UnbondedSatsByFpBtcPk[fpBTCPKHex], btcDel.TotalSat)
	}
	k.processRewardTracker(ctx, state.FpByBtcPkHex, btcDel, func(fp *types.FinalityProvider, del sdk.AccAddress, sats uint64) {
		if fp.SecuresBabylonGenesis(sdk.UnwrapSDKContext(ctx)) {
			k.MustProcessBabylonBtcDelegationUnbonded(ctx, fp.Address(), del, sats)
			return
		}
		// BSNs don't need to add to the event list to be processed at some specific babylon height.
		// Should update the reward tracker structures on the spot and don't care to have the rewards
		// being distributed based on the latest voting power.
		k.MustProcessConsumerBtcDelegationUnbonded(ctx, fp.Address(), del, sats)
	})
}

// processPowerDistUpdateEventActive actively handles the activated sats
// map and process the incentives reward tracking structures for activated btc dels.
func (k Keeper) processPowerDistUpdateEventActive(
	ctx context.Context,
	state *ftypes.ProcessingState,
	btcDel *types.BTCDelegation,
) {
	// newly active BTC delegation
	// add the BTC delegation to each multi-staked finality provider
	for _, fpBTCPK := range btcDel.FpBtcPkList {
		fpBTCPKHex := fpBTCPK.MarshalHex()
		if !k.BTCStakingKeeper.BabylonFinalityProviderExists(ctx, fpBTCPK) {
			// This is a consumer FP rather than Babylon FP, skip it
			continue
		}
		state.ActivatedSatsByFpBtcPk[fpBTCPKHex] = append(state.ActivatedSatsByFpBtcPk[fpBTCPKHex], btcDel.TotalSat)
	}

	// FP could be already slashed when it is being activated, but it is okay
	// since slashed finality providers do not earn rewards
	k.processRewardTracker(ctx, state.FpByBtcPkHex, btcDel, func(fp *types.FinalityProvider, del sdk.AccAddress, sats uint64) {
		if fp.SecuresBabylonGenesis(sdk.UnwrapSDKContext(ctx)) {
			k.MustProcessBabylonBtcDelegationActivated(ctx, fp.Address(), del, sats)
			return
		}
		// BSNs don't need to add to the events, can be processed instantly
		k.MustProcessConsumerBtcDelegationActivated(ctx, fp.Address(), del, sats)
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
//   - The function will be executed over all the Finality providers, including BSNs
//   - The function makes uses of the fpByBtcPkHex cache
func (k Keeper) processRewardTracker(
	ctx context.Context,
	fpByBtcPkHex map[string]*types.FinalityProvider,
	btcDel *types.BTCDelegation,
	f func(fp *types.FinalityProvider, del sdk.AccAddress, sats uint64),
) {
	delAddr := sdk.MustAccAddressFromBech32(btcDel.StakerAddr)
	for _, fpBTCPK := range btcDel.FpBtcPkList {
		fpBtcPk := fpBTCPK.MarshalHex()
		fp, err := k.loadFP(ctx, fpByBtcPkHex, fpBtcPk)
		if err != nil {
			sdkCtx := sdk.UnwrapSDKContext(ctx)
			k.Logger(sdkCtx).Error(
				"failed process the reward tracker for the given fp",
				err,
				"fp_btc_pk", fpBtcPk,
			)
			panic(err)
		}
		f(fp, delAddr, btcDel.TotalSat)
	}
}

// MustProcessBabylonBtcDelegationActivated calls the IncentiveKeeper.AddEventBtcDelegationActivated
// and panics if it errors
func (k Keeper) MustProcessBabylonBtcDelegationActivated(ctx context.Context, fp, del sdk.AccAddress, sats uint64) {
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

// MustProcessConsumerBtcDelegationActivated calls the IncentiveKeeper.BtcDelegationActivated
// and panics if it errors
func (k Keeper) MustProcessConsumerBtcDelegationActivated(ctx context.Context, fp, del sdk.AccAddress, sats uint64) {
	amtSat := sdkmath.NewIntFromUint64(sats)
	err := k.IncentiveKeeper.BtcDelegationActivated(ctx, fp, del, amtSat)
	if err != nil {
		k.Logger(sdk.UnwrapSDKContext(ctx)).Error(
			"failed to activate btc delegation",
			"del", del.String(),
			"fp", fp.String(),
		)
		panic(err)
	}
}

// MustProcessBabylonBtcDelegationUnbonded calls the IncentiveKeeper.AddEventBtcDelegationUnbonded
// and panics if it errors
func (k Keeper) MustProcessBabylonBtcDelegationUnbonded(ctx context.Context, fp, del sdk.AccAddress, sats uint64) {
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

// MustProcessConsumerBtcDelegationUnbonded calls the IncentiveKeeper.BtcDelegationUnbonded
// and panics if it errors
func (k Keeper) MustProcessConsumerBtcDelegationUnbonded(ctx context.Context, fp, del sdk.AccAddress, sats uint64) {
	amtSat := sdkmath.NewIntFromUint64(sats)
	err := k.IncentiveKeeper.BtcDelegationUnbonded(ctx, fp, del, amtSat)
	if err != nil {
		k.Logger(sdk.UnwrapSDKContext(ctx)).Error(
			"failed to unbond btc delegation",
			"del", del.String(),
			"fp", fp.String(),
		)
		panic(err)
	}
}

func (k Keeper) loadFP(
	ctx context.Context,
	cacheFpByBtcPkHex map[string]*types.FinalityProvider,
	fpBTCPKHex string,
) (*types.FinalityProvider, error) {
	fp, found := cacheFpByBtcPkHex[fpBTCPKHex]
	if !found {
		fpBTCPK, err := bbn.NewBIP340PubKeyFromHex(fpBTCPKHex)
		if err != nil {
			return nil, err
		}
		fp, err = k.BTCStakingKeeper.GetFinalityProvider(ctx, *fpBTCPK)
		if err != nil {
			return nil, err
		}
		cacheFpByBtcPkHex[fpBTCPKHex] = fp
	}

	return fp, nil
}
