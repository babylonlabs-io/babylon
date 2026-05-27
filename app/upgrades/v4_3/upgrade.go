package v4_3

import (
	"context"
	"fmt"

	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"github.com/babylonlabs-io/babylon/v4/app/keepers"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
	btcstkkeeper "github.com/babylonlabs-io/babylon/v4/x/btcstaking/keeper"
	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

const (
	UpgradeName = "v4.3"

	// EventTypePhantomStakeExpansionRemediated is emitted once per parent
	// delegation that is force-unbonded by the GHSA-4rm2-cj74-f62h remediation.
	// External indexers can use it to reconcile prior phantom voting power.
	EventTypePhantomStakeExpansionRemediated = "phantom_stake_expansion_remediated"

	AttrParentStakingTxHash = "parent_staking_tx_hash"
	AttrChildStakingTxHash  = "child_staking_tx_hash"
)

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades:        store.StoreUpgrades{},
}

func CreateUpgradeHandler(mm *module.Manager, cfg module.Configurator, k *keepers.AppKeepers) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		logger := sdkCtx.Logger().With("upgrade", UpgradeName)

		logger.Info("running module migrations")
		vm, err := mm.RunMigrations(ctx, cfg, fromVM)
		if err != nil {
			logger.Error("module migrations failed", "error", err.Error())
			return nil, err
		}

		logger.Info("scanning BTC staking store for poisoned stake-expansion parents (GHSA-4rm2-cj74-f62h)")
		remediated, err := RemediatePoisonedStakeExpansions(sdkCtx, k.BTCStakingKeeper)
		if err != nil {
			logger.Error("stake-expansion remediation failed — upgrade aborting",
				"error", err.Error())
			return nil, err
		}
		logger.Info("upgrade complete", "remediated_parents", len(remediated))
		return vm, nil
	}
}

// RemediatePoisonedStakeExpansions scans the BTC staking store for
// stake-expansion children that were unbonded early before their inclusion
// proof was added (the GHSA-4rm2-cj74-f62h attack pattern) and force-unbonds
// the corresponding parent delegations. The child's own staking transaction
// is the canonical proof that the parent's UTXO was spent on Bitcoin, so it
// is recorded as the parent's `DelegatorUnbondingInfo.SpendStakeTx`.
//
// The remediation is intentionally idempotent: parents that are already
// unbonded (legitimately or by a previous run) are skipped. Returns the
// staking-tx hashes (as hex strings) of remediated parents in iteration
// order.
func RemediatePoisonedStakeExpansions(
	ctx sdk.Context,
	bk btcstkkeeper.Keeper,
) ([]string, error) {
	logger := ctx.Logger().With("upgrade", UpgradeName, "module", "stake_expansion_remediation")

	type pair struct {
		parent     *btcstktypes.BTCDelegation
		spendTxBz  []byte
		childHash  string
		parentHash string
	}
	// Collect first, mutate after — avoids invalidating the store iterator.
	var toRemediate []pair
	// Dedupe by parent hash. On Bitcoin, only ONE spender of the parent's
	// UTXO can ever confirm; on babylon we may have registered multiple
	// stake expansions of the same ACTIVE parent before any of them
	// confirmed. We only need to mark the parent UNBONDED once, so dedupe
	// here and record the first encountered child as SpendStakeTx.
	seenParent := make(map[string]struct{})

	var (
		scanned                    int
		poisonedChildren           int
		skippedParentMissing       int
		skippedParentAlreadyUnbond int
		skippedParentNotActive     int
		skippedDuplicateParent     int
	)

	if err := bk.IterateBTCDelegations(ctx, func(child *btcstktypes.BTCDelegation) error {
		scanned++
		if !child.IsStakeExpansion() {
			return nil
		}
		if child.HasInclusionProof() {
			return nil
		}
		if child.BtcUndelegation == nil || child.BtcUndelegation.DelegatorUnbondingInfo == nil {
			return nil
		}

		// At this point the child is the GHSA-4rm2-cj74-f62h pattern:
		// stake-expansion, no inclusion proof, but DelegatorUnbondingInfo set.
		poisonedChildren++
		childHashStr := child.MustGetStakingTxHash().String()

		prevHash, err := child.StakeExpansionTxHash()
		if err != nil {
			logger.Error("invalid stake-expansion previous tx hash on poisoned child — aborting upgrade",
				"child_staking_tx_hash", childHashStr,
				"error", err.Error())

			return fmt.Errorf("invalid stake-expansion previous tx hash on child %s: %w",
				childHashStr, err)
		}
		parentHashStr := prevHash.String()

		parent, err := bk.GetBTCDelegation(ctx, parentHashStr)
		if err != nil {
			// Parent not in store — nothing to remediate. Could happen if the
			// parent was already pruned by a future feature; safe to skip but
			// unusual enough to warrant a Warn so operators notice.
			skippedParentMissing++
			logger.Warn("found poisoned child but parent delegation is not in store, skipping",
				"child_staking_tx_hash", childHashStr,
				"parent_staking_tx_hash", parentHashStr,
				"error", err.Error())

			return nil //nolint:nilerr
		}
		if parent.BtcUndelegation != nil && parent.BtcUndelegation.DelegatorUnbondingInfo != nil {
			// Already unbonded — either remediated previously or unbonded
			// through a legitimate path that succeeded.
			skippedParentAlreadyUnbond++
			logger.Debug("found poisoned child but parent already unbonded, skipping",
				"child_staking_tx_hash", childHashStr,
				"parent_staking_tx_hash", parentHashStr)

			return nil
		}

		// Skip parents that are no longer ACTIVE for any reason (e.g.,
		// their staking timelock expired and status became EXPIRED). The
		// phantom-voting-power harm we are remediating only applies while
		// the parent is contributing voting power; force-unbonding an
		// already-zero-power parent would enqueue a redundant
		// EventBTCDelegationStateUpdate{UNBONDED} on top of the prior
		// expiry transition.
		//
		// IsBtcDelegationActive is the canonical "is this delegation
		// ACTIVE?" check: it resolves the delegation's covenant quorum
		// from its OWN ParamsVersion (not the chain-current params, which
		// may have been bumped since the delegation was created), and for
		// chained stake expansions also resolves the prev-stake's quorum
		// from the prev-stake's own ParamsVersion. It returns a non-nil
		// error for both real failures (delegation missing, params version
		// missing) and the not-ACTIVE case; either way we skip the parent.
		_, parentActive, _ := bk.IsBtcDelegationActive(ctx, parentHashStr)
		if !parentActive {
			skippedParentNotActive++
			logger.Debug("found poisoned child but parent is not ACTIVE, skipping",
				"child_staking_tx_hash", childHashStr,
				"parent_staking_tx_hash", parentHashStr)

			return nil
		}

		if _, dup := seenParent[parentHashStr]; dup {
			// Multiple registered stake expansions for the same ACTIVE parent
			// are possible on babylon (registration only checks the parent is
			// ACTIVE), but only one can ever confirm on Bitcoin. The recorded
			// SpendStakeTx is best-effort — the first child encountered in
			// iteration order. Voting power deduction works regardless of
			// which one we pick.
			skippedDuplicateParent++
			logger.Warn("multiple poisoned children point to the same parent — only one stake expansion can spend the parent's UTXO on Bitcoin, skipping duplicate",
				"child_staking_tx_hash", childHashStr,
				"parent_staking_tx_hash", parentHashStr)

			return nil
		}
		seenParent[parentHashStr] = struct{}{}

		logger.Info("queued poisoned parent for remediation",
			"parent_staking_tx_hash", parentHashStr,
			"child_staking_tx_hash", childHashStr)

		toRemediate = append(toRemediate, pair{
			parent:     parent,
			spendTxBz:  child.StakingTx,
			childHash:  childHashStr,
			parentHash: parentHashStr,
		})

		return nil
	}); err != nil {
		return nil, err
	}

	logger.Info("scan complete",
		"delegations_scanned", scanned,
		"poisoned_children_found", poisonedChildren,
		"to_remediate", len(toRemediate),
		"skipped_parent_missing", skippedParentMissing,
		"skipped_parent_already_unbonded", skippedParentAlreadyUnbond,
		"skipped_parent_not_active", skippedParentNotActive,
		"skipped_duplicate_parent", skippedDuplicateParent,
	)

	if len(toRemediate) == 0 {
		logger.Info("no poisoned stake-expansion parents to remediate")

		return nil, nil
	}

	remediated := make([]string, 0, len(toRemediate))
	for _, p := range toRemediate {
		bk.BtcUndelegate(ctx, p.parent, &btcstktypes.DelegatorUnbondingInfo{
			SpendStakeTx: p.spendTxBz,
		})
		ctx.EventManager().EmitEvent(sdk.NewEvent(
			EventTypePhantomStakeExpansionRemediated,
			sdk.NewAttribute(AttrParentStakingTxHash, p.parentHash),
			sdk.NewAttribute(AttrChildStakingTxHash, p.childHash),
		))
		logger.Info("force-unbonded poisoned parent",
			"parent_staking_tx_hash", p.parentHash,
			"child_staking_tx_hash", p.childHash)
		remediated = append(remediated, p.parentHash)
	}

	logger.Info("remediation complete", "remediated_parents", len(remediated))

	return remediated, nil
}
