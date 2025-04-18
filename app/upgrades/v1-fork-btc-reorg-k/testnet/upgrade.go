package testnet

import (
	"context"
	"fmt"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/babylonlabs-io/babylon/app/keepers"
	"github.com/babylonlabs-io/babylon/app/upgrades"
	bbn "github.com/babylonlabs-io/babylon/types"
	bclkeeper "github.com/babylonlabs-io/babylon/x/btclightclient/keeper"
	bskeeper "github.com/babylonlabs-io/babylon/x/btcstaking/keeper"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	fkeeper "github.com/babylonlabs-io/babylon/x/finality/keeper"
	ftypes "github.com/babylonlabs-io/babylon/x/finality/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"go.uber.org/zap"
)

const (
	UpgradeName = "v1-btc-reorg-k"
)

func CreateUpgrade() upgrades.Upgrade {
	return upgrades.Upgrade{
		UpgradeName:          UpgradeName,
		CreateUpgradeHandler: CreateUpgradeHandler(),
	}
}

// CreateUpgradeHandler upgrade handler for launch.
func CreateUpgradeHandler() upgrades.UpgradeHandlerCreator {
	return func(mm *module.Manager, cfg module.Configurator, keepers *keepers.AppKeepers) upgradetypes.UpgradeHandler {
		return func(context context.Context, _plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			ctx := sdk.UnwrapSDKContext(context)

			migrations, err := mm.RunMigrations(ctx, cfg, fromVM)
			if err != nil {
				return nil, fmt.Errorf("failed to run migrations: %w", err)
			}

			l := ctx.Logger()

			// this upgrade should be called when there is a BTC reorg higher than K blocks (btccheckpoint.BtcConfirmationDepth)
			btcStkK := keepers.BTCStakingKeeper

			largerBtcReorg := btcStkK.GetLargestBtcReorg(ctx)
			btcBlockHeightRollback := largerBtcReorg.RollbackFrom.Height

			l.Debug("running upgrade due to large BTC reorg", zap.Uint32("btc_block_height_rollback_from", btcBlockHeightRollback))
			// iterate over voting power expiration events to get the latest transactions from
			// rollback height + (maxStaking - btcDel.UnbondingTime) from the latest btc staking parameter

			return migrations, nil
		}
	}
}

// RollbackBtcStkTxs rollbacks all the BTC staking transactions that were included during the blocks
// which were rollbacked in the BTC reorg.
// Note: the order of the endblock which panic matters, the halt happened at the btcstaking
// so finality and incentive had not run their end blockers in the panic block.
func RollbackBtcStkTxs(
	ctx context.Context,
	btcStkK *bskeeper.Keeper,
	btcLgtK *bclkeeper.Keeper,
	finalK *fkeeper.Keeper,

	stkTxs []chainhash.Hash,
	rollbackUnbondTxs []chainhash.Hash,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	largerBtcReorg := btcStkK.GetLargestBtcReorg(ctx)
	btcBlockHeightRollback := largerBtcReorg.RollbackFrom.Height
	paramsByVs := btcStkK.GetAllParamsByVersion(ctx)
	// the BTC reorg was executed in btclightclient and panic in btcstaking,
	// which means that the tip height is currently the latest BTC block after the whole reorg
	btcTip := btcLgtK.GetTipInfo(ctx)
	bbnHeight := uint64(sdkCtx.HeaderInfo().Height)

	newDc := ftypes.NewVotingPowerDistCache()
	lastVpDstCache := finalK.GetVotingPowerDistCache(ctx, bbnHeight-1)
	satsToUnbondByFpBtcPk := make(map[string]uint64, 0)
	satsToActivateByFpBtcPk := make(map[string]uint64, 0)
	mapStkTxs := map[string]struct{}{}
	mapRollbackUnbondTxs := map[string]struct{}{}

	for _, ubdTx := range rollbackUnbondTxs {
		mapRollbackUnbondTxs[ubdTx.String()] = struct{}{}
	}

	for _, stkTx := range stkTxs {
		mapStkTxs[stkTx.String()] = struct{}{}
		btcDel, err := btcStkK.GetBTCDelegation(ctx, stkTx.String())
		if err != nil {
			return fmt.Errorf("failed to find BTC delegation to staking tx: %s - %w", stkTx.String(), err)
		}

		if !btcDel.HasInclusionProof() {
			return fmt.Errorf("the given BTC delegation does not have inclusion proof: %s, it doesn't need to rollback", stkTx.String())
		}

		p := paramsByVs[btcDel.ParamsVersion]
		if p == nil {
			return fmt.Errorf("failed to get the params version %d for BTC delegation to staking tx: %s", btcDel.ParamsVersion, stkTx.String())
		}

		// At this point each staking transaction is considered that the inclusion proof or any BTC action was made in a BTC block that
		// was rolled back, so there is a need to revert the state that was modified.
		btcStatus := btcDel.GetStatus(btcTip.Height, p.CovenantQuorum)
		switch btcStatus {
		// for pending, expired and verified there is no need to update anything
		case bstypes.BTCDelegationStatus_PENDING:
		case bstypes.BTCDelegationStatus_EXPIRED:
		case bstypes.BTCDelegationStatus_VERIFIED:
			continue
		case bstypes.BTCDelegationStatus_ACTIVE:
			// Decrease the total VP from the voting power table for that FP
			// how do we know which babylon height this btc delegation was
			// activated (Voting Power was included in the VP distribution cache)?
			// since there is no clear way how to get this information, just update the
			// latest voting power table stored in the finality
			for _, fpBTCPK := range btcDel.FpBtcPkList {
				fpBTCPKHex := fpBTCPK.MarshalHex()
				satsToUnbondByFpBtcPk[fpBTCPKHex] += btcDel.TotalSat
			}

			// Remove the inclusion proof of the BTC delegation
			btcDel.EndHeight = 0
			btcDel.StartHeight = 0

			// Unbond in the incentive rewards tracker

			continue
		case bstypes.BTCDelegationStatus_UNBONDED:
			// how to check if the unbonded BTC delegation transaction was rolledback

			ubdTx, err := bbn.NewBTCTxFromBytes(btcDel.BtcUndelegation.UnbondingTx)
			if err != nil {
				return fmt.Errorf("failed to parse unbonding tx %x off staking tx: %s - %w", btcDel.BtcUndelegation.UnbondingTx, stkTx.String(), err)
			}

			ubdTxHash := ubdTx.TxHash().String()
			_, rollback := mapRollbackUnbondTxs[ubdTxHash]
			if !rollback {
				continue
			}

			for _, fpBTCPK := range btcDel.FpBtcPkList {
				fpBTCPKHex := fpBTCPK.MarshalHex()
				satsToActivateByFpBtcPk[fpBTCPKHex] += btcDel.TotalSat
			}

			// if the slash tx was rollbacked there is no need to rollback state, as the BTC can be already slashed
			btcDel.BtcUndelegation.DelegatorUnbondingInfo = nil
			// Add back to the incentive rewards
			// Add back to the voting power table
		}

		// TODO: handle BTC delegation for consumers rollback
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

	// store in state
	finalK.RecordVpAndDistCacheForHeight(ctx, newDc, bbnHeight-1)

	// check out each BTC VP distribution event in which was generated in some BTC staking delegation
	// that had action in the reorg blocks
	for btcHeight := btcBlockHeightRollback; btcHeight <= largerBtcReorg.RollbackFrom.Height; btcHeight++ {
		vpEvents := btcStkK.GetAllPowerDistUpdateEvents(ctx, btcHeight, btcHeight)
		for idx, evt := range vpEvents {
			switch typedEvent := evt.Ev.(type) {
			case *bstypes.EventPowerDistUpdate_BtcDelStateUpdate:
				delEvent := typedEvent.BtcDelStateUpdate
				delStkTxHash := delEvent.StakingTxHash

				_, rollbacked := mapStkTxs[delStkTxHash]
				if !rollbacked {
					continue
				}

				btcStkK.DeletePowerDistEvent(ctx, btcHeight, uint64(idx))
			default: // other events as slashed, jailed, unjail do no matter the BTC pk
				continue
			}
		}
	}

	return nil
}
