package keeper

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"cosmossdk.io/collections"
	"cosmossdk.io/store/prefix"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/x/finality/types"
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
	k.recordVotingPowerAndCache(ctx, newDc)
	// emit events for finality providers with state updates
	k.handleFPStateUpdates(ctx, dc, newDc)
	// record metrics
	k.recordMetrics(newDc)
}

// recordVotingPowerAndCache assigns voting power to each active finality provider
// with the following consideration:
// 1. the fp must have timestamped pub rand
// 2. the fp must in the top x ranked by the voting power (x is given by maxActiveFps)
func (k Keeper) recordVotingPowerAndCache(ctx context.Context, newDc *ftypes.VotingPowerDistCache) {
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

// handleFPStateUpdates emits events and triggers hooks for finality providers with state updates
func (k Keeper) handleFPStateUpdates(ctx context.Context, prevDc, newDc *ftypes.VotingPowerDistCache) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	newlyActiveFPs := newDc.FindNewActiveFinalityProviders(prevDc)
	for _, fp := range newlyActiveFPs {
		if err := k.handleActivatedFinalityProvider(ctx, fp.BtcPk); err != nil {
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

// handleActivatedFinalityProvider updates the signing info start height or create a new signing info
func (k Keeper) handleActivatedFinalityProvider(ctx context.Context, fpPk *bbn.BIP340PubKey) error {
	signingInfo, err := k.FinalityProviderSigningTracker.Get(ctx, fpPk.MustMarshal())
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err == nil {
		signingInfo.StartHeight = sdkCtx.BlockHeight()
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
	// of BTC delegations that newly become active under this provider
	activeBTCDels := map[string][]*types.BTCDelegation{}
	// a map where key is unbonded BTC delegation's staking tx hash
	unbondedBTCDels := map[string]struct{}{}
	// a map where key is slashed finality providers' BTC PK
	slashedFPs := map[string]struct{}{}
	// a map where key is jailed finality providers' BTC PK
	jailedFPs := map[string]struct{}{}
	// a map where key is unjailed finality providers' BTC PK
	unjailedFPs := map[string]struct{}{}
	newActiveBtcDels := make([]*btcDelWithStkTxHash, 0)
	newUnbondedBtcDels := make([]*btcDelWithStkTxHash, 0)

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

			btcDelWithStkTxHash := &btcDelWithStkTxHash{
				StakingTxHash: delStkTxHash,
				BTCDelegation: btcDel,
			}

			switch delEvent.NewState {
			case types.BTCDelegationStatus_ACTIVE:
				// newly active BTC delegation
				// add the BTC delegation to each restaked finality provider
				for _, fpBTCPK := range btcDel.FpBtcPkList {
					fpBTCPKHex := fpBTCPK.MarshalHex()
					activeBTCDels[fpBTCPKHex] = append(activeBTCDels[fpBTCPKHex], btcDel)
				}
				newActiveBtcDels = append(newActiveBtcDels, btcDelWithStkTxHash)
			case types.BTCDelegationStatus_UNBONDED:
				// emit expired event if it is not early unbonding
				if !btcDel.IsUnbondedEarly() {
					types.EmitExpiredDelegationEvent(sdkCtx, delStkTxHash)
				}

				// add the unbonded BTC delegation to the map
				unbondedBTCDels[delStkTxHash] = struct{}{}
				newUnbondedBtcDels = append(newUnbondedBtcDels, btcDelWithStkTxHash)
			}
		case *types.EventPowerDistUpdate_SlashedFp:
			// record slashed fps
			types.EmitSlashedFPEvent(sdkCtx, typedEvent.SlashedFp.Pk)
			slashedFPs[typedEvent.SlashedFp.Pk.MarshalHex()] = struct{}{}
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
	// TODO: the algorithm needs to iterate over all BTC delegations so remains
	// sub-optimal. Ideally we only need to iterate over all events above rather
	// than the entire cache. This is made difficulty since BTC delegations are
	// not keyed in the cache. Need to find a way to optimise this.
	newDc := ftypes.NewVotingPowerDistCache()

	// iterate over all finality providers and apply all events
	for i := range dc.FinalityProviders {
		// create a copy of the finality provider
		fp := *dc.FinalityProviders[i]
		fp.TotalBondedSat = 0
		fp.BtcDels = []*ftypes.BTCDelDistInfo{}

		fpBTCPKHex := fp.BtcPk.MarshalHex()

		// if this finality provider is slashed, continue to avoid
		// assigning delegation to it
		_, isSlashed := slashedFPs[fpBTCPKHex]
		if isSlashed {
			if err := k.IncentiveKeeper.FpSlashed(ctx, fp.GetAddress()); err != nil {
				panic(err)
			}
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

		// add all BTC delegations that are not unbonded to the new finality provider
		for j := range dc.FinalityProviders[i].BtcDels {
			btcDel := *dc.FinalityProviders[i].BtcDels[j]

			_, isUnbondedBtcDelegation := unbondedBTCDels[btcDel.StakingTxHash]
			if isUnbondedBtcDelegation {
				continue
			}
			// if it is not unbonded add to the del dist info
			fp.AddBTCDelDistInfo(&btcDel)
		}

		// process all new BTC delegations under this finality provider
		if fpActiveBTCDels, ok := activeBTCDels[fpBTCPKHex]; ok {
			// handle new BTC delegations for this finality provider
			for _, d := range fpActiveBTCDels {
				fp.AddBTCDel(d)
			}
			// remove the finality provider entry in activeBTCDels map, so that
			// after the for loop the rest entries in activeBTCDels belongs to new
			// finality providers with new BTC delegations
			delete(activeBTCDels, fpBTCPKHex)
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
	fpActiveBtcPkHexList := make([]string, 0, len(activeBTCDels))
	for fpBTCPKHex := range activeBTCDels {
		fpActiveBtcPkHexList = append(fpActiveBtcPkHexList, fpBTCPKHex)
	}
	sort.SliceStable(fpActiveBtcPkHexList, func(i, j int) bool {
		return fpActiveBtcPkHexList[i] < fpActiveBtcPkHexList[j]
	})

	// simple cache to load fp by his btc pk hex
	cacheFpByBtcPkHex := map[string]*types.FinalityProvider{}

	// for each new finality provider, apply the new BTC delegations to the new dist cache
	for _, fpBTCPKHex := range fpActiveBtcPkHexList {
		// get the finality provider and initialise its dist info
		newFP := k.loadFP(ctx, cacheFpByBtcPkHex, fpBTCPKHex)
		fpDistInfo := ftypes.NewFinalityProviderDistInfo(newFP)

		// add each BTC delegation
		fpActiveBTCDels := activeBTCDels[fpBTCPKHex]
		for _, d := range fpActiveBTCDels {
			fpDistInfo.AddBTCDel(d)
		}

		// add this finality provider to the new cache if it has voting power
		if fpDistInfo.TotalBondedSat > 0 {
			newDc.AddFinalityProviderDistInfo(fpDistInfo)
		}
	}

	k.processBtcDelegations(ctx, cacheFpByBtcPkHex, newActiveBtcDels, func(fp, del sdk.AccAddress, sats uint64) {
		err := k.IncentiveKeeper.BtcDelegationActivated(ctx, fp, del, sats)
		if err != nil {
			panic(err)
		}
	})

	k.processBtcDelegations(ctx, cacheFpByBtcPkHex, newUnbondedBtcDels, func(fp, del sdk.AccAddress, sats uint64) {
		err := k.IncentiveKeeper.BtcDelegationUnbonded(ctx, fp, del, sats)
		if err != nil {
			panic(err)
		}
	})

	return newDc
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

// processBtcDelegations sorts the btc delegations and
// executed the function by passing the fp, delegator address
// and the total amount of satoshi in that delegation.
func (k Keeper) processBtcDelegations(
	ctx context.Context,
	cacheFpByBtcPkHex map[string]*types.FinalityProvider,
	btcDels []*btcDelWithStkTxHash,
	f func(fp, del sdk.AccAddress, sats uint64),
) {
	sort.SliceStable(btcDels, func(i, j int) bool {
		return btcDels[i].StakingTxHash < btcDels[j].StakingTxHash
	})

	for _, btcDel := range btcDels {
		delAddr := sdk.MustAccAddressFromBech32(btcDel.StakerAddr)
		for _, fpBTCPK := range btcDel.FpBtcPkList {
			fpBTCPKHex := fpBTCPK.MarshalHex()
			fp := k.loadFP(ctx, cacheFpByBtcPkHex, fpBTCPKHex)
			fpAddr := sdk.MustAccAddressFromBech32(fp.Addr)

			f(fpAddr, delAddr, btcDel.TotalSat)
		}
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
			panic(err) // only programming error
		}
		cacheFpByBtcPkHex[fpBTCPKHex] = fp
	}

	return fp
}

type btcDelWithStkTxHash struct {
	StakingTxHash string
	*types.BTCDelegation
}
