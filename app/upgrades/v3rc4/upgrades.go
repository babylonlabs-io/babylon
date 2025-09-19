package v3rc4

import (
	"context"

	"cosmossdk.io/collections"
	corestoretypes "cosmossdk.io/core/store"
	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	stkkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btcstkkeeper "github.com/babylonlabs-io/babylon/v4/x/btcstaking/keeper"
	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	costkkeeper "github.com/babylonlabs-io/babylon/v4/x/costaking/keeper"
	costktypes "github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	fkeeper "github.com/babylonlabs-io/babylon/v4/x/finality/keeper"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

// UpgradeName defines the on-chain upgrade name for the Babylon v3rc4 upgrade
const UpgradeName = "v3rc4"

// InitializeCoStakerRwdsTracker initializes the costaker rewards tracker
// It looks for all BTC stakers that are also baby stakers
func InitializeCoStakerRwdsTracker(
	ctx context.Context,
	cdc codec.BinaryCodec,
	costkStoreService corestoretypes.KVStoreService,
	stkKeeper *stkkeeper.Keeper,
	btcStkKeeper btcstkkeeper.Keeper,
	coStkKeeper costkkeeper.Keeper,
	fKeeper fkeeper.Keeper,
) error {
	btcStakers, err := getAllBTCStakers(ctx, btcStkKeeper, fKeeper)
	if err != nil {
		return err
	}

	coStakers, err := buildCoStakersMap(ctx, btcStakers, stkKeeper)
	if err != nil {
		return err
	}

	return saveCoStakersToStore(ctx, cdc, coStkKeeper, costkStoreService, coStakers)
}

type coStaker struct {
	Address        string
	ActiveSatoshis math.Int
	ActiveBaby     math.Int
}

// getAllBTCStakers retrieves all active BTC stakers with pagination
func getAllBTCStakers(ctx context.Context, btcStkKeeper btcstkkeeper.Keeper, fKeeper fkeeper.Keeper) (map[string]math.Int, error) {
	btcStakers := make(map[string]math.Int)
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
			return nil, err
		}

		for _, del := range btcDelRes.BtcDelegations {
			// check if delegating to an active FP
			if !delegatingToActiveFP(del.FpBtcPkList, activeFps) {
				continue
			}
			if staker, found := btcStakers[del.StakerAddr]; found {
				staker = staker.Add(math.NewIntFromUint64(del.TotalSat))
				btcStakers[del.StakerAddr] = staker
				continue
			}

			btcStakers[del.StakerAddr] = math.NewIntFromUint64(del.TotalSat)
		}

		if btcDelRes.Pagination == nil || len(btcDelRes.Pagination.NextKey) == 0 {
			break
		}
		nextKey = btcDelRes.Pagination.NextKey
	}

	return btcStakers, nil
}

// getBabyStakingAmount retrieves total baby staking amount for a staker with pagination
func getBabyStakingAmount(ctx context.Context, stkQuerier stkkeeper.Querier, stakerAddr string) (math.Int, error) {
	totalBaby := math.ZeroInt()
	var nextKey []byte

	for {
		req := &stktypes.QueryDelegatorDelegationsRequest{
			DelegatorAddr: stakerAddr,
			Pagination: &query.PageRequest{
				Key: nextKey,
			},
		}

		res, err := stkQuerier.DelegatorDelegations(ctx, req)
		if err != nil {
			return math.ZeroInt(), err
		}

		for _, del := range res.DelegationResponses {
			totalBaby = totalBaby.Add(del.Balance.Amount)
		}

		if res.Pagination == nil || len(res.Pagination.NextKey) == 0 {
			break
		}
		nextKey = res.Pagination.NextKey
	}

	return totalBaby, nil
}

// buildCoStakersMap builds a map of co-stakers from BTC stakers and their baby staking amounts
func buildCoStakersMap(ctx context.Context, btcStakers map[string]math.Int, stkKeeper *stkkeeper.Keeper) (map[string]coStaker, error) {
	stkQuerier := stkkeeper.NewQuerier(stkKeeper)
	coStakers := make(map[string]coStaker)

	for stkAddr, activeSat := range btcStakers {
		totalBaby, err := getBabyStakingAmount(ctx, stkQuerier, stkAddr)
		if err != nil {
			return nil, err
		}

		if totalBaby.GT(math.ZeroInt()) {
			coStakers[stkAddr] = coStaker{
				Address:        stkAddr,
				ActiveSatoshis: activeSat,
				ActiveBaby:     totalBaby,
			}
		}
	}

	return coStakers, nil
}

// saveCoStakersToStore saves co-stakers to the rewards tracker store
func saveCoStakersToStore(
	ctx context.Context,
	cdc codec.BinaryCodec,
	k costkkeeper.Keeper,
	costkStoreService corestoretypes.KVStoreService,
	coStakers map[string]coStaker,
) error {
	sb := collections.NewSchemaBuilder(costkStoreService)
	rwdTrackers := collections.NewMap(
		sb,
		costktypes.CostakerRewardsTrackerKeyPrefix,
		"costaker_rewards_tracker",
		collections.BytesKey,
		codec.CollValue[costktypes.CostakerRewardsTracker](cdc),
	)
	dp := costktypes.DefaultParams()
	totalScore := math.ZeroInt()
	// we're writing independent key-value
	// pairs to storage, the order shouldn't affect the final state
	for addr, val := range coStakers {
		sdkAddr := sdk.MustAccAddressFromBech32(addr)
		rt := costktypes.NewCostakerRewardsTracker(
			1,
			val.ActiveSatoshis,
			val.ActiveBaby,
			math.ZeroInt(),
		)
		rt.UpdateScore(dp.ScoreRatioBtcByBaby)
		if err := rwdTrackers.Set(ctx, []byte(sdkAddr), rt); err != nil {
			return err
		}
		totalScore = totalScore.Add(rt.TotalScore)
	}

	// make sure current rewards is initialized
	if _, err := k.GetCurrentRewardsInitialized(ctx); err != nil {
		return err
	}

	return k.UpdateCurrentRewardsTotalScore(ctx, totalScore)
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
