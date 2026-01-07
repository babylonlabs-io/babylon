package v4_3

import (
	"context"
	"errors"
	"fmt"

	"cosmossdk.io/collections"
	corestoretypes "cosmossdk.io/core/store"
	"cosmossdk.io/math"
	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/types/query"
	stkkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/babylonlabs-io/babylon/v4/app/keepers"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades/epoching"
	costkkeeper "github.com/babylonlabs-io/babylon/v4/x/costaking/keeper"
	costktypes "github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	epochingkeeper "github.com/babylonlabs-io/babylon/v4/x/epoching/keeper"
)

const UpgradeName = "v4.3"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades: store.StoreUpgrades{
		Added:   []string{},
		Deleted: []string{},
	},
}

func CreateUpgradeHandler(mm *module.Manager, configurator module.Configurator, keepers *keepers.AppKeepers) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		// Run migrations before applying any other state changes.
		migrations, err := mm.RunMigrations(ctx, configurator, fromVM)
		if err != nil {
			return nil, err
		}

		// Validate epoch boundary using epoching keeper
		if err := epoching.ValidateEpochBoundary(ctx, keepers.EpochingKeeper); err != nil {
			return nil, fmt.Errorf("epoch boundary validation failed: %w", err)
		}

		costkStoreKey := keepers.GetKey(costktypes.StoreKey)
		if costkStoreKey == nil {
			return nil, errors.New("invalid costaking types store key")
		}
		coStkStoreService := runtime.NewKVStoreService(costkStoreKey)

		// Reset co-staker rewards tracker for ActiveBaby
		if err := ResetCoStakerRwdsTrackerActiveBaby(
			ctx,
			keepers.EncCfg.Codec,
			coStkStoreService,
			keepers.EpochingKeeper,
			keepers.StakingKeeper,
			keepers.CostakingKeeper,
		); err != nil {
			return nil, err
		}

		return migrations, nil
	}
}

// ResetCoStakerRwdsTrackerActiveBaby resets the ActiveBaby in costaker rewards tracker
// It recalculates ActiveBaby for all BABY stakers based on current delegations to active validators
func ResetCoStakerRwdsTrackerActiveBaby(
	ctx context.Context,
	cdc codec.BinaryCodec,
	costkStoreService corestoretypes.KVStoreService,
	epochingKeeper epochingkeeper.Keeper,
	stkKeeper *stkkeeper.Keeper,
	coStkKeeper costkkeeper.Keeper,
) error {
	sb := collections.NewSchemaBuilder(costkStoreService)
	rwdTrackers := collections.NewMap(
		sb,
		costktypes.CostakerRewardsTrackerKeyPrefix,
		"costaker_rewards_tracker",
		collections.BytesKey,
		codec.CollValue[costktypes.CostakerRewardsTracker](cdc),
	)

	// Zero out ActiveBaby in all existing rewards trackers and track previous values
	accsWithActiveBaby, err := zeroOutCoStakerRwdsActiveBaby(ctx, rwdTrackers)
	if err != nil {
		return err
	}

	params := coStkKeeper.GetParams(ctx)
	endedPeriod, err := coStkKeeper.IncrementRewardsPeriod(ctx)
	if err != nil {
		return err
	}

	// Recalculate ActiveBaby for all BABY stakers based on current delegations to active validators
	if err := updateBABYStakersRwdTracker(ctx, endedPeriod, rwdTrackers, accsWithActiveBaby, epochingKeeper, stkKeeper, coStkKeeper, params); err != nil {
		return err
	}

	totalScore, err := getTotalScore(ctx, rwdTrackers)
	if err != nil {
		return err
	}

	currentRwd, err := coStkKeeper.GetCurrentRewards(ctx)
	if err != nil {
		return err
	}

	currentRwd.TotalScore = totalScore
	if err := currentRwd.Validate(); err != nil {
		return err
	}

	return coStkKeeper.SetCurrentRewards(ctx, *currentRwd)
}

type ActiveBabyTracked struct {
	PreviousActiveBaby math.Int
	CurrentActiveBaby  math.Int
}

// updateBABYStakersRwdTracker retrieves all BABY stakers delegating to active validators and updates their ActiveBaby
func updateBABYStakersRwdTracker(
	ctx context.Context,
	period uint64,
	rwdTrackers collections.Map[[]byte, costktypes.CostakerRewardsTracker],
	accsWithActiveBaby map[string]ActiveBabyTracked,
	epochingKeeper epochingkeeper.Keeper,
	stkKeeper *stkkeeper.Keeper,
	coStkKeeper costkkeeper.Keeper,
	params costktypes.Params,
) error {
	// Get all BABY stakers delegating to active validators in the current epoch
	babyStakers, err := getAllBABYStakers(ctx, epochingKeeper, stkKeeper)
	if err != nil {
		return fmt.Errorf("failed to get all BABY stakers: %w", err)
	}

	// Add current BABY delegations to the tracking map
	for delegatorAddr, babyAmount := range babyStakers {
		data, found := accsWithActiveBaby[delegatorAddr]
		if found {
			data.CurrentActiveBaby = data.CurrentActiveBaby.Add(babyAmount)
			accsWithActiveBaby[delegatorAddr] = data
		} else {
			accsWithActiveBaby[delegatorAddr] = ActiveBabyTracked{
				PreviousActiveBaby: math.ZeroInt(),
				CurrentActiveBaby:  babyAmount,
			}
		}
	}

	// Update the costaker rewards trackers for all accounts
	for accAddrStr, babyTracked := range accsWithActiveBaby {
		if err := updateCostakerActiveBabyRewardsTracker(
			ctx,
			coStkKeeper,
			period,
			rwdTrackers,
			sdk.MustAccAddressFromBech32(accAddrStr),
			params,
			babyTracked,
		); err != nil {
			return err
		}
	}

	return nil
}

// getAllBABYStakers retrieves all BABY stakers by iterating over the current epoch's active validators
func getAllBABYStakers(ctx context.Context, epochingKeeper epochingkeeper.Keeper, stkKeeper *stkkeeper.Keeper) (map[string]math.Int, error) {
	stkQuerier := stkkeeper.NewQuerier(stkKeeper)
	babyStakers := make(map[string]math.Int)

	// Get the current epoch's validator set from epoching keeper
	valSet := epochingKeeper.GetCurrentValidatorSet(ctx)

	// Iterate over validators in the current epoch
	for _, val := range valSet {
		valAddr := sdk.ValAddress(val.Addr)

		// Get all delegations for this active validator
		if err := getValidatorDelegations(ctx, stkQuerier, valAddr.String(), babyStakers); err != nil {
			return nil, fmt.Errorf("failed to get delegations for validator %s: %w", valAddr.String(), err)
		}
	}

	return babyStakers, nil
}

// getValidatorDelegations gets all delegations for a specific validator
func getValidatorDelegations(ctx context.Context, stkQuerier stkkeeper.Querier, validatorAddr string, babyStakers map[string]math.Int) error {
	var nextKey []byte

	for {
		req := &stktypes.QueryValidatorDelegationsRequest{
			ValidatorAddr: validatorAddr,
			Pagination: &query.PageRequest{
				Key: nextKey,
			},
		}

		res, err := stkQuerier.ValidatorDelegations(ctx, req)
		if err != nil {
			return err
		}

		for _, delegation := range res.DelegationResponses {
			delegatorAddr := delegation.Delegation.DelegatorAddress
			amount := delegation.Balance.Amount

			if existing, found := babyStakers[delegatorAddr]; found {
				babyStakers[delegatorAddr] = existing.Add(amount)
			} else {
				babyStakers[delegatorAddr] = amount
			}
		}

		if res.Pagination == nil || len(res.Pagination.NextKey) == 0 {
			break
		}
		nextKey = res.Pagination.NextKey
	}

	return nil
}

// updateCostakerActiveBabyRewardsTracker creates or updates a costaker rewards tracker with ActiveBaby
func updateCostakerActiveBabyRewardsTracker(
	ctx context.Context,
	coStkKeeper costkkeeper.Keeper,
	endedPeriod uint64,
	rwdTrackers collections.Map[[]byte, costktypes.CostakerRewardsTracker],
	stakerAddr sdk.AccAddress,
	params costktypes.Params,
	babyTracked ActiveBabyTracked,
) error {
	addrKey := []byte(stakerAddr)

	// Try to get existing tracker
	rt, err := rwdTrackers.Get(ctx, addrKey)
	if err != nil && !errors.Is(err, collections.ErrNotFound) {
		return err
	}

	if errors.Is(err, collections.ErrNotFound) {
		// This should not happen as we're updating existing trackers only
		return nil
	}

	// Update existing tracker (set the ActiveBaby because it was zeroed out before)
	// Update the StartPeriodCumulativeReward only if the ActiveBaby is changing
	rt.ActiveBaby = rt.ActiveBaby.Add(babyTracked.CurrentActiveBaby)
	diff := babyTracked.CurrentActiveBaby.Sub(babyTracked.PreviousActiveBaby)
	if !diff.IsZero() { // if there is any diff, the period needs to be increased
		rt.StartPeriodCumulativeReward = endedPeriod
		if err := coStkKeeper.CalculateCostakerRewardsAndSendToGauge(ctx, stakerAddr, endedPeriod); err != nil {
			return err
		}
	}

	// Update score
	rt.UpdateScore(params.ScoreRatioBtcByBaby)

	// Save tracker
	if err := rwdTrackers.Set(ctx, addrKey, rt); err != nil {
		return err
	}

	return nil
}

// zeroOutCoStakerRwdsActiveBaby zeros out ActiveBaby in all costaker rewards trackers
func zeroOutCoStakerRwdsActiveBaby(
	ctx context.Context,
	rwdTrackers collections.Map[[]byte, costktypes.CostakerRewardsTracker],
) (map[string]ActiveBabyTracked, error) {
	accsWithActiveBaby := make(map[string]ActiveBabyTracked)
	iter, err := rwdTrackers.Iterate(ctx, nil)
	if err != nil {
		return accsWithActiveBaby, err
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		costakerAddr, err := iter.Key()
		if err != nil {
			return accsWithActiveBaby, err
		}

		tracker, err := iter.Value()
		if err != nil {
			return accsWithActiveBaby, err
		}
		if tracker.ActiveBaby.IsZero() {
			continue
		}

		sdkAddr := sdk.AccAddress(costakerAddr)
		accsWithActiveBaby[sdkAddr.String()] = ActiveBabyTracked{
			PreviousActiveBaby: tracker.ActiveBaby,
			CurrentActiveBaby:  math.ZeroInt(),
		}

		// Zero out ActiveBaby
		tracker.ActiveBaby = math.ZeroInt()
		if err := rwdTrackers.Set(ctx, costakerAddr, tracker); err != nil {
			return accsWithActiveBaby, err
		}
	}

	return accsWithActiveBaby, nil
}

func getTotalScore(
	ctx context.Context,
	rwdTrackers collections.Map[[]byte, costktypes.CostakerRewardsTracker],
) (math.Int, error) {
	totalScore := math.ZeroInt()
	iter, err := rwdTrackers.Iterate(ctx, nil)
	if err != nil {
		return totalScore, err
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		tracker, err := iter.Value()
		if err != nil {
			return totalScore, err
		}

		totalScore = totalScore.Add(tracker.TotalScore)
	}

	return totalScore, nil
}
