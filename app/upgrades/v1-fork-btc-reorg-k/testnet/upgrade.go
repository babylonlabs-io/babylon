package testnet

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon/app/keepers"
	"github.com/babylonlabs-io/babylon/app/upgrades"
	bskeeper "github.com/babylonlabs-io/babylon/x/btcstaking/keeper"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	fkeeper "github.com/babylonlabs-io/babylon/x/finality/keeper"
	ftypes "github.com/babylonlabs-io/babylon/x/finality/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/zap"
)

const (
	UpgradeName = "v1-btc-reorg-k"
)

var (
	// MapUnbondStkTxHashRollback this should be the staking transaction
	// hash hex of the unbonding transactions that got rolled back.
	MapUnbondStkTxHashRollback = map[string]struct{}{
		// example staking tx of an unbonding staking transaction
		"a3d84be950961d03a72c04a0128cce000e3476f489e17273cbea71f971b47f61": {},
	}
)

func CreateFork() upgrades.Fork {
	return upgrades.Fork{
		UpgradeName: UpgradeName,
		// TODO: fill with correct block height of fork,
		UpgradeHeight:  12000,
		BeginForkLogic: CreateForkLogic,
	}
}

// CreateForkLogic executes the fork logic to handle BTC reorg large than k.
func CreateForkLogic(context sdk.Context, keepers *keepers.AppKeepers) {
	err := ForkHandler(context, keepers)
	if err != nil {
		panic(fmt.Errorf("failed to run the fork handler: %w", err))
	}
}

// ForkHandler wraps the logic of the fork to return an error
func ForkHandler(context sdk.Context, keepers *keepers.AppKeepers) error {
	ctx := sdk.UnwrapSDKContext(context)
	l := ctx.Logger()

	// this upgrade should be called when there is a BTC reorg higher than K blocks (btccheckpoint.BtcConfirmationDepth)
	btcStkK, btcLgtK, finalK := keepers.BTCStakingKeeper, keepers.BTCLightClientKeeper, keepers.FinalityKeeper

	largerBtcReorg := btcStkK.GetLargestBtcReorg(ctx)
	if largerBtcReorg == nil {
		panic("no btc reorg has happened")
	}
	btcBlockHeightRollbackFrom := largerBtcReorg.RollbackFrom.Height

	l.Debug(
		"running upgrade due to large BTC reorg",
		zap.Uint32("btc_block_height_rollback_from", btcBlockHeightRollbackFrom),
		zap.Uint32("btc_block_height_rollback_to", largerBtcReorg.RollbackTo.Height),
	)

	btcTip := btcLgtK.GetTipInfo(ctx)
	paramsByVs := btcStkK.GetAllParamsByVersion(ctx)

	cacheBtcDelByStkTxHashHex := make(map[string]*bstypes.BTCDelegation, 0)

	// collects and deletes the events that are still not processed but
	// the act that generated the event was rolledbacked:
	// staking tx or unbonding
	mapStkTxHashRollback := HandleDeleteVotingPowerDistributionEvts(
		ctx,
		&btcStkK,
		paramsByVs,
		cacheBtcDelByStkTxHashHex,
		btcTip.Height,
		largerBtcReorg,
		MapUnbondStkTxHashRollback,
	)

	// unsets the values on BTC delegation and reward tracker
	satsToUnbondByFpBtcPk, satsToActivateByFpBtcPk, err := HandleBtcDelegationsAndIncentive(
		ctx,
		&btcStkK,
		&finalK,
		cacheBtcDelByStkTxHashHex,
		btcTip.Height,
		paramsByVs,
		mapStkTxHashRollback,
		MapUnbondStkTxHashRollback,
	)
	if err != nil {
		return fmt.Errorf("failed to handle BTC delegations: %w", err)
	}

	// Updates the voting power table accordingly to the BTC delegations rollback actions.
	HandleVotingPowerDistCache(ctx, &finalK, satsToActivateByFpBtcPk, satsToUnbondByFpBtcPk)

	// deletes the old largest reorg to avoid panic at end blocker again
	return btcStkK.DeleteLargestBtcReorg(ctx)
}

// HandleDeleteVotingPowerDistributionEvts iterates over all possible rolledback voting power distribution
// events to verify if they were and delete it if the act that generated then was reorged.
func HandleDeleteVotingPowerDistributionEvts(
	ctx context.Context,
	btcStkK *bskeeper.Keeper,

	paramsByVs map[uint32]*bstypes.Params,
	cacheBtcDelByStkTxHashHex map[string]*bstypes.BTCDelegation,
	btcTipHeight uint32,
	largerBtcReorg *bstypes.LargestBtcReOrg,
	mapUnbondStkTxHashRollback map[string]struct{},
) (mapStkTxHashRollback map[string]struct{}) {
	mapStkTxHashRollback = make(map[string]struct{}, 0)
	eventsToDelete := make([]bstypes.EventIndex, 0)

	higherBtcStakingPeriod := MaxBtcStakingTimeBlocks(paramsByVs)
	btcBlockHeightRollbackFrom := largerBtcReorg.RollbackFrom.Height

	l := sdk.UnwrapSDKContext(ctx).Logger()

	// iterating over all the BTC staking events from the rollback height until latest Tip + the maximum staking period
	for btcHeight := btcBlockHeightRollbackFrom; btcHeight <= btcTipHeight+higherBtcStakingPeriod; btcHeight++ {
		vpEvents := btcStkK.GetAllPowerDistUpdateEvents(ctx, btcHeight, btcHeight)
		for idx, evt := range vpEvents {
			switch typedEvent := evt.Ev.(type) {
			case *bstypes.EventPowerDistUpdate_BtcDelStateUpdate:
				delEvt := typedEvent.BtcDelStateUpdate
				stkTxHash := delEvt.StakingTxHash

				btcDel := loadBtcDel(ctx, btcStkK, cacheBtcDelByStkTxHashHex, stkTxHash)

				if largerBtcReorg.IsBtcHeightRollbacked(btcDel.StartHeight) {
					mapStkTxHashRollback[stkTxHash] = struct{}{}
					eventsToDelete = append(eventsToDelete, bstypes.EventIndex{
						Idx:            uint64(idx),
						BlockHeightBtc: btcHeight,
					})
					l.Debug(
						"staking transaction was rolledbacked",
						zap.String("stk_tx_hash_hex", stkTxHash),
					)
					continue
				}
				// if the btc staking transaction hash was not activated during the rollback
				// verify if the unbond wasn't

				if delEvt.NewState != bstypes.BTCDelegationStatus_UNBONDED {
					// if it is not unbonding, nothing to do
					continue
				}

				_, isUnbondingTxRolledBack := mapUnbondStkTxHashRollback[stkTxHash]
				if isUnbondingTxRolledBack {
					l.Debug(
						"unbonding transaction was rolledbacked",
						zap.String("stk_tx_hash_hex", stkTxHash),
					)
					eventsToDelete = append(eventsToDelete, bstypes.EventIndex{
						Idx:            uint64(idx),
						BlockHeightBtc: btcHeight,
					})
				}

			default: // other events as slashed, jailed, unjail do not matter for the rollback procedure
				continue
			}
		}
	}

	for _, evt := range eventsToDelete {
		btcStkK.DeletePowerDistEvent(ctx, evt.BlockHeightBtc, evt.Idx)
	}

	return mapStkTxHashRollback
}

// HandleBtcDelegationsAndIncentive rollbacks all the BTC staking transactions that were included during the blocks
// which were rollbacked in the BTC reorg.
// Note: the order of the endblock which panic matters, the halt happened at the btcstaking
// so finality and incentive had not run their end blockers in the panic block.
func HandleBtcDelegationsAndIncentive(
	ctx context.Context,
	btcStkK *bskeeper.Keeper,
	finalK *fkeeper.Keeper,

	cacheBtcDelByStkTxHashHex map[string]*bstypes.BTCDelegation,
	btcTipHeight uint32,
	paramsByVs map[uint32]*bstypes.Params,
	mapStkTxHashRollback, mapRollbackUnbondTxs map[string]struct{},
) (satsToUnbondByFpBtcPk, satsToActivateByFpBtcPk map[string]uint64, err error) {
	cacheFpByBtcPkHex := make(map[string]*bstypes.FinalityProvider, 0)
	// collect data to update cache voting power table in finality
	satsToUnbondByFpBtcPk, satsToActivateByFpBtcPk = make(map[string]uint64, 0), make(map[string]uint64, 0)

	for stkTxHash := range mapStkTxHashRollback {
		btcDel := loadBtcDel(ctx, btcStkK, cacheBtcDelByStkTxHashHex, stkTxHash)

		if !btcDel.HasInclusionProof() {
			// it doesn't have inclusion proof, can just be deleted
			btcStkK.DeleteBTCDelegation(ctx, btcDel)
			continue
		}

		p := paramsByVs[btcDel.ParamsVersion]
		if p == nil {
			return nil, nil, fmt.Errorf("failed to get the params version %d for BTC delegation to staking tx: %s", btcDel.ParamsVersion, stkTxHash)
		}

		// verify the current status of the BTC delegation to rollback the state in btcstaking
		btcStatus := btcDel.GetStatus(btcTipHeight, p.CovenantQuorum)
		switch btcStatus {
		// for pending, expired and verified there is no need to update anything
		case bstypes.BTCDelegationStatus_PENDING:
		case bstypes.BTCDelegationStatus_EXPIRED:
		case bstypes.BTCDelegationStatus_VERIFIED:
			continue
		// if the slash tx was rollbacked there is no need to rollback state, as the BTC can be already slashed
		case bstypes.BTCDelegationStatus_ACTIVE:
			// if it is currently active, it should rollback the sats in finality vp table
			// and unbond in the reward tracker
			for _, fpBTCPK := range btcDel.FpBtcPkList {
				fpBTCPKHex := fpBTCPK.MarshalHex()
				satsToUnbondByFpBtcPk[fpBTCPKHex] += btcDel.TotalSat
			}

			// Unbond in the incentive rewards tracker
			finalK.ProcessRewardTracker(ctx, cacheFpByBtcPkHex, btcDel, func(fp, del sdk.AccAddress, sats uint64) {
				finalK.MustProcessBtcDelegationUnbonded(ctx, fp, del, sats)
			})

			btcStkK.DeleteBTCDelegation(ctx, btcDel)
			continue

		case bstypes.BTCDelegationStatus_UNBONDED:
			// verify if the staking transaction hash is in the unbond
			// rolledback list of staking txs
			_, rollback := mapRollbackUnbondTxs[stkTxHash]
			if !rollback {
				continue
			}

			for _, fpBTCPK := range btcDel.FpBtcPkList {
				fpBTCPKHex := fpBTCPK.MarshalHex()
				satsToActivateByFpBtcPk[fpBTCPKHex] += btcDel.TotalSat
			}

			btcDel.BtcUndelegation.DelegatorUnbondingInfo = nil

			// Add back to the incentive rewards tracker
			finalK.ProcessRewardTracker(ctx, cacheFpByBtcPkHex, btcDel, func(fp, del sdk.AccAddress, sats uint64) {
				finalK.MustProcessBtcDelegationActivated(ctx, fp, del, sats)
			})

			btcStkK.SetBTCDelegation(ctx, btcDel)
			continue
		}

		// TODO: handle BTC delegation for consumers rollback
	}

	return satsToUnbondByFpBtcPk, satsToActivateByFpBtcPk, nil
}

// HandleVotingPowerDistCache decreases the total VP from the
// voting power table for the given FPs. There is no clear way
// to know which babylon height the BTC delegation was activated
// (The babylon height in which the voting Power was included in
// the VP distribution cache). For this reason, just update the
// latest voting power table stored in the finality to keep
// the records correctly from now on.
// Note: We can't use the current block height, as it will be updated
// in this end blocker.
func HandleVotingPowerDistCache(
	ctx context.Context,
	finalK *fkeeper.Keeper,
	satsToActivateByFpBtcPk, satsToUnbondByFpBtcPk map[string]uint64,
) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// the BTC reorg was executed in btclightclient and panic in btcstaking,
	// which means that the tip height is currently the latest BTC block after the whole reorg
	bbnHeight := uint64(sdkCtx.HeaderInfo().Height)
	vpDstCacheHeight := bbnHeight - 1

	newDc := ftypes.NewVotingPowerDistCache()
	lastVpDstCache := finalK.GetVotingPowerDistCache(ctx, vpDstCacheHeight)
	if lastVpDstCache == nil {
		// no need to update if there wasn't a voting power cache for that babylon height
		return
	}

	// Updates the new voting power distribution cache
	for i := range lastVpDstCache.FinalityProviders {
		// create a copy of the finality provider
		fp := *lastVpDstCache.FinalityProviders[i]
		fpBTCPKHex := fp.BtcPk.MarshalHex()

		satsToUnbond, okUnbond := satsToUnbondByFpBtcPk[fpBTCPKHex]
		if okUnbond {
			fp.RemoveBondedSats(satsToUnbond)
		}

		satsToActivate, okActivate := satsToActivateByFpBtcPk[fpBTCPKHex]
		if okActivate {
			fp.AddBondedSats(satsToActivate)
		}

		// add this finality provider to the new cache if it has voting power
		if fp.TotalBondedSat > 0 {
			newDc.AddFinalityProviderDistInfo(&fp)
		}
	}

	// Update the voting power table in the state, accordingly
	finalK.RecordVpAndDistCacheForHeight(ctx, newDc, vpDstCacheHeight)
}

// MaxBtcStakingTimeBlocks iterates over all the parameters to get the higher staking time
// in blocks
func MaxBtcStakingTimeBlocks(paramsByVs map[uint32]*bstypes.Params) uint32 {
	higherBtcStakingPeriod := uint32(0)

	for _, p := range paramsByVs {
		if p.MaxStakingTimeBlocks <= higherBtcStakingPeriod {
			continue
		}
		higherBtcStakingPeriod = p.MaxStakingTimeBlocks
	}

	return higherBtcStakingPeriod
}

// loadBtcDel caches the btc delegation based on the staking tx hash hex
// NOTE: the btc delegation staking transactions hash hex must exists.
func loadBtcDel(
	ctx context.Context,
	btcStkK *bskeeper.Keeper,
	cacheBtcDelByStkTxHashHex map[string]*bstypes.BTCDelegation,
	stkTxHashHex string,
) *bstypes.BTCDelegation {
	del, found := cacheBtcDelByStkTxHashHex[stkTxHashHex]
	if !found {
		btcDel, err := btcStkK.GetBTCDelegation(ctx, stkTxHashHex)
		if err != nil {
			panic(fmt.Errorf("failed to find BTC delegation to staking tx: %s - %w", stkTxHashHex, err).Error())
		}
		cacheBtcDelByStkTxHashHex[stkTxHashHex] = btcDel
		return btcDel
	}

	return del
}
