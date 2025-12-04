package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
)

var _ types.QueryServer = Keeper{}

// Params returns the parameters of the module
func (k Keeper) Params(ctx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(sdkCtx)

	return &types.QueryParamsResponse{
		Params: params,
	}, nil
}

// CostakerRewardsTracker returns the costaker rewards tracker for a given address
func (k Keeper) CostakerRewardsTracker(ctx context.Context, req *types.QueryCostakerRewardsTrackerRequest) (*types.QueryCostakerRewardsTrackerResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.CostakerAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "costaker address cannot be empty")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	costakerAddr, err := sdk.AccAddressFromBech32(req.CostakerAddress)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid costaker address")
	}

	tracker, err := k.GetCostakerRewards(sdkCtx, costakerAddr)
	if err != nil {
		return nil, status.Error(codes.NotFound, "costaker rewards tracker not found")
	}

	return &types.QueryCostakerRewardsTrackerResponse{
		StartPeriodCumulativeReward: tracker.StartPeriodCumulativeReward,
		ActiveSatoshis:              tracker.ActiveSatoshis,
		ActiveBaby:                  tracker.ActiveBaby,
		TotalScore:                  tracker.TotalScore,
	}, nil
}

// HistoricalRewards returns the historical rewards for a given period
func (k Keeper) HistoricalRewards(ctx context.Context, req *types.QueryHistoricalRewardsRequest) (*types.QueryHistoricalRewardsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	rewards, err := k.GetHistoricalRewards(sdkCtx, req.Period)
	if err != nil {
		return nil, status.Error(codes.NotFound, "historical rewards not found for the given period")
	}

	return &types.QueryHistoricalRewardsResponse{
		CumulativeRewardsPerScore: rewards.CumulativeRewardsPerScore,
	}, nil
}

// CurrentRewards returns the current rewards for the costaking pool
func (k Keeper) CurrentRewards(ctx context.Context, req *types.QueryCurrentRewardsRequest) (*types.QueryCurrentRewardsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	currentRewards, err := k.GetCurrentRewards(sdkCtx)
	if err != nil {
		return nil, status.Error(codes.NotFound, "current rewards not found")
	}

	return &types.QueryCurrentRewardsResponse{
		Rewards:    currentRewards.Rewards,
		Period:     currentRewards.Period,
		TotalScore: currentRewards.TotalScore,
	}, nil
}

// QueryValidateCostakers returns all costakers and validates their active satoshis and active baby amounts
func (k Keeper) QueryValidateCostakers(ctx context.Context, req *types.QueryValidateCostakersRequest) (*types.QueryValidateCostakersResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	costakers := make(map[string]types.CostakerDelegation)
	totalActiveSatoshis := math.ZeroInt()
	totalActiveBaby := math.ZeroInt()
	totalScore := math.ZeroInt()

	costkActiveBaby := make(map[string]uint64)
	k.stkK.IterateLastValidatorPowers(ctx, func(operator sdk.ValAddress, power int64) bool {
		val, err := k.stkK.GetValidator(ctx, operator)
		if err != nil {
			return false
		}

		valDels, err := k.stkK.GetValidatorDelegations(ctx, operator)
		if err != nil {
			return false
		}

		for _, del := range valDels {
			tokens := val.TokensFromShares(del.Shares)
			costkActiveBaby[del.DelegatorAddress] += tokens.TruncateInt().Uint64()
		}

		return false
	})

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	currBlockHeight := uint64(sdkCtx.BlockHeader().Height)
	vpDstCache := k.finalityK.GetVotingPowerDistCache(ctx, currBlockHeight)
	if vpDstCache == nil {
		return nil, fmt.Errorf("vp dst cache nil for height %d", currBlockHeight)
	}

	btcTip := k.btcStkK.BtcTip(ctx)
	bsParamsByVersion := make(map[uint32]*btcstktypes.Params)
	costkActiveSats := make(map[string]uint64)

	activeFps := vpDstCache.GetActiveFinalityProviderSet()
	for _, activeFp := range activeFps {
		k.btcStkK.HandleFPBTCDelegations(ctx, activeFp.BtcPk, func(b *btcstktypes.BTCDelegation) error {
			bsParams, found := bsParamsByVersion[b.ParamsVersion]
			if !found {
				bsParams = k.btcStkK.GetParamsByVersion(ctx, b.ParamsVersion)
				bsParamsByVersion[b.ParamsVersion] = bsParams
			}

			status := b.GetStatus(btcTip, bsParams.CovenantQuorum, 0)
			if status != btcstktypes.BTCDelegationStatus_ACTIVE {
				return nil
			}

			costkActiveSats[b.StakerAddr] += b.TotalSat
			return nil
		})
	}

	iter, err := k.costakerRewardsTracker.Iterate(ctx, nil)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		key, err := iter.Key()
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		tracker, err := iter.Value()
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		addr := sdk.AccAddress(key)
		costaker := types.CostakerDelegation{
			CostakerAddress: addr.String(),
			ActiveSatoshis:  tracker.ActiveSatoshis,
			ActiveBaby:      tracker.ActiveBaby,
			TotalScore:      tracker.TotalScore,
		}
		costakers[addr.String()] = costaker

		totalActiveSatoshis = totalActiveSatoshis.Add(tracker.ActiveSatoshis)
		totalActiveBaby = totalActiveBaby.Add(tracker.ActiveBaby)
		totalScore = totalScore.Add(tracker.TotalScore)
	}

	for costkAddr, costk := range costakers {
		if costk.ActiveBaby.IsPositive() {
			amt, found := costkActiveBaby[costkAddr]
			if !found {
				fmt.Printf("\n costk %s has active baby amt %s in x/costaking but no baby del found", costkAddr, costk.ActiveBaby.String())
			} else {
				if amt != costk.ActiveBaby.Uint64() {
					fmt.Printf("\n costk %s has active baby amt %s in x/costaking but diff amount from baby dels %d", costkAddr, costk.ActiveBaby.String(), amt)
				}
			}

			delete(costkActiveBaby, costkAddr)
		}

		if costk.ActiveSatoshis.IsPositive() {
			amt, found := costkActiveSats[costkAddr]
			if !found {
				fmt.Printf("\n costk %s has active sats amt %s in x/costaking but no btc del found", costkAddr, costk.ActiveSatoshis.String())
			} else {
				if amt != costk.ActiveSatoshis.Uint64() {
					fmt.Printf("\n costk %s has active sats amt %s in x/costaking but diff amount from btc dels %d", costkAddr, costk.ActiveSatoshis.String(), amt)
				}
			}

			delete(costkActiveSats, costkAddr)
		}
	}

	for costkAddr, amt := range costkActiveBaby {
		fmt.Printf("\n Baby delegation %s - %d not in costaking", costkAddr, amt)
	}

	for costkAddr, amt := range costkActiveSats {
		fmt.Printf("\n Btc delegation %s - %d not in costaking", costkAddr, amt)
	}

	return &types.QueryValidateCostakersResponse{
		TotalActiveSatoshis: totalActiveSatoshis,
		TotalActiveBaby:     totalActiveBaby,
		TotalScore:          totalScore,
	}, nil
}
