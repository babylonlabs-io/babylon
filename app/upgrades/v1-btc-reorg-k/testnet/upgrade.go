package testnet

import (
	"context"
	"fmt"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/babylonlabs-io/babylon/app/keepers"
	"github.com/babylonlabs-io/babylon/app/upgrades"
	bclkeeper "github.com/babylonlabs-io/babylon/x/btclightclient/keeper"
	bskeeper "github.com/babylonlabs-io/babylon/x/btcstaking/keeper"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	fkeeper "github.com/babylonlabs-io/babylon/x/finality/keeper"
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
func RollbackBtcStkTxs(
	ctx context.Context,
	btcStkK *bskeeper.Keeper,
	btcLgtK *bclkeeper.Keeper,
	finalK *fkeeper.Keeper,

	stkTxs ...chainhash.Hash,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	paramsByVs := btcStkK.GetAllParamsByVersion(ctx)
	btcTip := btcLgtK.GetTipInfo(ctx)

	bbnHeight := uint64(sdkCtx.HeaderInfo().Height)
	lastVpDstCache := finalK.GetVotingPowerDistCache(ctx, bbnHeight-1)
	satsToUnbondByFpBtcPk := make(map[string]uint64, 0)

	for _, stkTx := range stkTxs {
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

		// At this point each staking transaction is considered that the inclusion proof was made in a BTC block that was rolled back
		// or the
		btcStatus := btcDel.GetStatus(btcTip.Height, p.CovenantQuorum)
		switch btcStatus {
		case bstypes.BTCDelegationStatus_PENDING:
		case bstypes.BTCDelegationStatus_EXPIRED:
		case bstypes.BTCDelegationStatus_VERIFIED:
			continue
		case bstypes.BTCDelegationStatus_ACTIVE:
			// Decrease the total VP from the voting power table for that FP
			// how do we know which babylon height this btc delegation was activated?
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
			continue

		}

		for i := range lastVpDstCache.FinalityProviders {
			// create a copy of the finality provider
			fp := *lastVpDstCache.FinalityProviders[i]
			fpBTCPKHex := fp.BtcPk.MarshalHex()

			satsToUnbond, ok := satsToUnbondByFpBtcPk[fpBTCPKHex]
			if !ok {
				continue
			}
			fp.RemoveBondedSats(satsToUnbond)
		}
		// TODO: handle BTC delegation for consumers rollback
	}

	return nil
}
