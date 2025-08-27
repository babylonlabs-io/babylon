package btcstaking

import (
	"context"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"github.com/babylonlabs-io/babylon/v4/app/keepers"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

type (
	FpWithSats struct {
		btcstktypes.FinalityProvider
		TotalSatsStaked uint64
	}
	// BtcStkKeeper the expected keeper interface to perform the upgrade
	BtcStkKeeper interface {
		IterateFinalityProvider(ctx context.Context, f func(fp btcstktypes.FinalityProvider) error) error
		SetFpBbnAddr(ctx context.Context, fpAddr sdk.AccAddress) error
		SoftDeleteFinalityProvider(ctx context.Context, fpBtcPk *bbn.BIP340PubKey) error
		FpTotalSatsStaked(ctx context.Context, fpBTCPK *bbn.BIP340PubKey) (uint64, error)
	}
)

func CreateUpgradeHandlerFpSoftDeleteDupAddr(mm *module.Manager, configurator module.Configurator, keepers *keepers.AppKeepers) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		migrations, err := mm.RunMigrations(ctx, configurator, fromVM)
		if err != nil {
			return nil, err
		}

		err = FpSoftDeleteDupAddr(ctx, keepers.BTCStakingKeeper)
		if err != nil {
			return nil, err
		}

		return migrations, nil
	}
}

// FpSoftDeleteDupAddr Iterates over all the finality providers and sets their fp bbn address
// To block registration with the same address. It also soft deletes (unables the btc pk to
// cast votes) of those which are already duplicated fp address. The deletion decision factor is:
// 1. Highest voted block height
// 2. Total amount of sats staked
func FpSoftDeleteDupAddr(ctx context.Context, btcStkK BtcStkKeeper) error {
	fpsByAddr := make(map[string][]*FpWithSats)

	err := btcStkK.IterateFinalityProvider(ctx, func(fp btcstktypes.FinalityProvider) error {
		fpsByAddr[fp.Addr] = append(fpsByAddr[fp.Addr], &FpWithSats{
			FinalityProvider: fp,
			TotalSatsStaked:  0,
		})

		// it doesn't matter if it sets more than once
		return btcStkK.SetFpBbnAddr(ctx, fp.Address())
	})
	if err != nil {
		return err
	}

	// order the list for determinism
	fpsDupAddr := make([]string, 0)
	for addr, groupByAddr := range fpsByAddr {
		if len(groupByAddr) < 2 {
			// no duplicated address
			continue
		}
		fpsDupAddr = append(fpsDupAddr, addr)
	}

	// determinism
	sort.Slice(fpsDupAddr, func(i, j int) bool {
		return fpsDupAddr[i] > fpsDupAddr[j]
	})
	fpsToDelete := make([]*bbn.BIP340PubKey, 0)

	for _, fpAddr := range fpsDupAddr {
		dupGroup := fpsByAddr[fpAddr]
		// order the group by their highest voted height
		sort.SliceStable(dupGroup, func(i, j int) bool {
			return dupGroup[i].HighestVotedHeight > dupGroup[j].HighestVotedHeight
		})

		highestVotedBlockOfGroup := dupGroup[0].HighestVotedHeight
		// if there is any doubt which fp should not be deleted (same highest voted height)
		// check which one has the highest total amount of sats staked
		if highestVotedBlockOfGroup == dupGroup[1].HighestVotedHeight {
			for _, fp := range dupGroup {
				if highestVotedBlockOfGroup > fp.HighestVotedHeight {
					// no need to check total vp if it is lower than the max voted height
					continue
				}

				// only load the amount of sats staked of the ones that matter
				totalSats, err := btcStkK.FpTotalSatsStaked(ctx, fp.BtcPk)
				if err != nil {
					return err
				}
				fp.TotalSatsStaked = totalSats
			}
		}

		// order the group by their highest voted height,
		// if tied, order by the highest total sats staked
		sort.SliceStable(dupGroup, func(i, j int) bool {
			if dupGroup[i].HighestVotedHeight == dupGroup[j].HighestVotedHeight {
				return dupGroup[i].TotalSatsStaked > dupGroup[j].TotalSatsStaked
			}

			return dupGroup[i].HighestVotedHeight > dupGroup[j].HighestVotedHeight
		})

		// soft delete the ones after the first value in slice
		for _, fp := range dupGroup[1:] {
			fpsToDelete = append(fpsToDelete, fp.BtcPk)
		}
	}

	for _, deleteFpBtcPk := range fpsToDelete {
		err := btcStkK.SoftDeleteFinalityProvider(ctx, deleteFpBtcPk)
		if err != nil {
			return err
		}
	}

	return nil
}
