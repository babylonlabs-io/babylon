package v4

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
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btcstkkeeper "github.com/babylonlabs-io/babylon/v4/x/btcstaking/keeper"
	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	costkkeeper "github.com/babylonlabs-io/babylon/v4/x/costaking/keeper"
	costktypes "github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	fkeeper "github.com/babylonlabs-io/babylon/v4/x/finality/keeper"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

const UpgradeName = "v4"

var StoresToAdd = []string{
	costktypes.StoreKey,
}

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades: store.StoreUpgrades{
		Added:   StoresToAdd,
		Deleted: []string{},
	},
}

func CreateUpgradeHandler(mm *module.Manager, configurator module.Configurator, keepers *keepers.AppKeepers) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		currentHeight := uint64(sdkCtx.HeaderInfo().Height)

		// Validate epoch boundary using epoching keeper
		if err := epoching.ValidateEpochBoundary(ctx, keepers.EpochingKeeper); err != nil {
			return nil, fmt.Errorf("epoch boundary validation failed: %w", err)
		}

		// Validate delegation pool module account exists before running migrations
		if err := epoching.ValidateDelegatePoolModuleAccount(ctx, keepers.AccountKeeper); err != nil {
			return nil, fmt.Errorf("spam prevention upgrade validation failed: %w", err)
		}
		// Validate that delegation pool has no locked funds
		if err := epoching.ValidateDelegatePoolEmpty(ctx, keepers.AccountKeeper, keepers.BankKeeper); err != nil {
			// Log warning instead of failing upgrade
			sdkCtx.Logger().Warn("delegate pool had non-zero balance but failed to transfer funds to fee collector - upgrade proceeding", "error", err.Error())
		}

		// Run migrations (includes epoching v1->v2 migration)
		migrations, err := mm.RunMigrations(ctx, configurator, fromVM)
		if err != nil {
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}

		if err := epoching.ValidateMigrationResults(ctx, keepers); err != nil {
			return nil, fmt.Errorf("migration validation failed: %w", err)
		}
		// Log successful upgrade
		epochingPrevVersion := fromVM["epoching"]
		epochingNewVersion := migrations["epoching"]
		sdkCtx.Logger().Info("spam prevention upgrade completed successfully",
			"upgrade", UpgradeName,
			"epoching_migration", fmt.Sprintf("v%d->v%d", epochingPrevVersion, epochingNewVersion),
			"height", currentHeight,
			"epoch_boundary", true,
		)

		// costaking upgrade
		costkStoreKey := keepers.GetKey(costktypes.StoreKey)
		if costkStoreKey == nil {
			return nil, errors.New("invalid costaking types store key")
		}

		coStkStoreService := runtime.NewKVStoreService(costkStoreKey)
		if err := InitializeCoStakerRwdsTracker(ctx, keepers.EncCfg.Codec, coStkStoreService, keepers.StakingKeeper, keepers.BTCStakingKeeper, keepers.CostakingKeeper, keepers.FinalityKeeper); err != nil {
			return nil, err
		}

		return migrations, nil
	}
}

// InitializeCoStakerRwdsTracker initializes the costaker rewards tracker
// It creates trackers for all BTC stakers, BABY stakers, and combined stakers
func InitializeCoStakerRwdsTracker(
	ctx context.Context,
	cdc codec.BinaryCodec,
	costkStoreService corestoretypes.KVStoreService,
	stkKeeper *stkkeeper.Keeper,
	btcStkKeeper btcstkkeeper.Keeper,
	coStkKeeper costkkeeper.Keeper,
	fKeeper fkeeper.Keeper,
) error {
	dp := costktypes.DefaultParams()
	// Save co-staker rwd tracker for all BTC stakers
	if err := saveBTCStakersRwdTracker(ctx, cdc, costkStoreService, btcStkKeeper, fKeeper, dp); err != nil {
		return err
	}

	// Update co-staker rwd tracker with all BABY stakers
	totalScore, err := saveBABYStakersRwdTracker(ctx, cdc, costkStoreService, stkKeeper, dp)
	if err != nil {
		return err
	}

	// make sure current rewards is initialized
	if _, err := coStkKeeper.GetCurrentRewardsInitialized(ctx); err != nil {
		return err
	}

	return coStkKeeper.UpdateCurrentRewardsTotalScore(ctx, totalScore)
}

// saveBABYStakersRwdTracker retrieves all active BABY stakers with pagination and saves them to the costaker rewards tracker
// Returns the total score of the co-staker rewards tracker
func saveBABYStakersRwdTracker(ctx context.Context, cdc codec.BinaryCodec, costkStoreService corestoretypes.KVStoreService, stkKeeper *stkkeeper.Keeper, params costktypes.Params) (math.Int, error) {
	totalScore := math.ZeroInt()
	// Get all BABY stakers
	babyStakers, err := getAllBABYStakers(ctx, stkKeeper)
	if err != nil {
		return totalScore, fmt.Errorf("failed to get all BABY stakers: %w", err)
	}

	// Save BABY stakers to costaker rewards tracker
	for addr, totalBaby := range babyStakers {
		rt, err := upsertCostakerRewardsTracker(ctx, cdc, costkStoreService, addr, math.ZeroInt(), totalBaby, params)
		if err != nil {
			return totalScore, fmt.Errorf("failed to upsert costaker rewards tracker for BABY staker %s: %w", addr, err)
		}

		totalScore = totalScore.Add(rt.TotalScore)
	}

	return totalScore, nil
}

// getAllBABYStakers retrieves all BABY stakers with pagination
func getAllBABYStakers(ctx context.Context, stkKeeper *stkkeeper.Keeper) (map[string]math.Int, error) {
	stkQuerier := stkkeeper.NewQuerier(stkKeeper)
	babyStakers := make(map[string]math.Int)

	// First get all validators
	var nextKey []byte
	for {
		req := &stktypes.QueryValidatorsRequest{
			Pagination: &query.PageRequest{
				Key: nextKey,
			},
		}

		res, err := stkQuerier.Validators(ctx, req)
		if err != nil {
			return nil, err
		}

		// For each validator, get all delegations
		for _, validator := range res.Validators {
			if err := getValidatorDelegations(ctx, stkQuerier, validator.OperatorAddress, babyStakers); err != nil {
				return nil, err
			}
		}

		if res.Pagination == nil || len(res.Pagination.NextKey) == 0 {
			break
		}
		nextKey = res.Pagination.NextKey
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

// saveBTCStakersRwdTracker retrieves all active BTC stakers with pagination and saves them to the costaker rewards tracker
func saveBTCStakersRwdTracker(ctx context.Context,
	cdc codec.BinaryCodec,
	costkStoreService corestoretypes.KVStoreService,
	btcStkKeeper btcstkkeeper.Keeper,
	fKeeper fkeeper.Keeper,
	params costktypes.Params,
) error {
	// To count as btc staker for the co-staking rewards
	// need to be delegating to a FP within the current active set
	// This runs on preblocker (before BeginBlock), so the active set to consider should be from previous height
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.HeaderInfo().Height)
	vp := fKeeper.GetVotingPowerDistCache(ctx, height-1)
	if vp == nil {
		vp = ftypes.NewVotingPowerDistCache()
	}
	activeFps := vp.GetActiveFinalityProviderSet()

	var nextKey []byte

	for {
		req := &btcstktypes.QueryBTCDelegationsRequest{
			Status: btcstktypes.BTCDelegationStatus_ACTIVE,
			Pagination: &query.PageRequest{
				Key: nextKey,
			},
		}

		btcDelRes, err := btcStkKeeper.BTCDelegations(ctx, req)
		if err != nil {
			return err
		}

		for _, del := range btcDelRes.BtcDelegations {
			// check if delegating to an active FP
			if !delegatingToActiveFP(del.FpBtcPkList, activeFps) {
				continue
			}
			_, err := upsertCostakerRewardsTracker(ctx, cdc, costkStoreService, del.StakerAddr, math.NewIntFromUint64(del.TotalSat), math.ZeroInt(), params)
			if err != nil {
				return err
			}
		}

		if btcDelRes.Pagination == nil || len(btcDelRes.Pagination.NextKey) == 0 {
			break
		}
		nextKey = btcDelRes.Pagination.NextKey
	}

	return nil
}

// upsertCostakerRewardsTracker creates or updates a costaker rewards tracker
func upsertCostakerRewardsTracker(
	ctx context.Context,
	cdc codec.BinaryCodec,
	costkStoreService corestoretypes.KVStoreService,
	stakerAddr string,
	btcAmount math.Int,
	babyAmount math.Int,
	params costktypes.Params,
) (*costktypes.CostakerRewardsTracker, error) {
	sb := collections.NewSchemaBuilder(costkStoreService)
	rwdTrackers := collections.NewMap(
		sb,
		costktypes.CostakerRewardsTrackerKeyPrefix,
		"costaker_rewards_tracker",
		collections.BytesKey,
		codec.CollValue[costktypes.CostakerRewardsTracker](cdc),
	)

	sdkAddr := sdk.MustAccAddressFromBech32(stakerAddr)
	addrKey := []byte(sdkAddr)

	// Try to get existing tracker
	existing, err := rwdTrackers.Get(ctx, addrKey)
	if err != nil && !errors.Is(err, collections.ErrNotFound) {
		return nil, err
	}

	var rt costktypes.CostakerRewardsTracker
	if errors.Is(err, collections.ErrNotFound) {
		// Create new tracker
		rt = costktypes.NewCostakerRewardsTracker(
			1,
			btcAmount,
			babyAmount,
			math.ZeroInt(),
		)
	} else {
		// Update existing tracker
		rt = existing
		rt.ActiveSatoshis = rt.ActiveSatoshis.Add(btcAmount)
		rt.ActiveBaby = rt.ActiveBaby.Add(babyAmount)
	}

	// Update score
	rt.UpdateScore(params.ScoreRatioBtcByBaby)

	// Save tracker
	if err := rwdTrackers.Set(ctx, addrKey, rt); err != nil {
		return nil, err
	}

	return &rt, nil
}

func delegatingToActiveFP(fpBtcPks []bbn.BIP340PubKey, activeFps map[string]*ftypes.FinalityProviderDistInfo) bool {
	// check if delegating to an active FP
	isActiveDel := false
	for _, fpBtcPk := range fpBtcPks {
		if _, ok := activeFps[fpBtcPk.MarshalHex()]; ok {
			isActiveDel = true
			break
		}
	}

	return isActiveDel
}
