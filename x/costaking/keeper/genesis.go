package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
)

// InitGenesis performs stateful validations and initializes the keeper state from a provided initial genesis state.
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	for _, rwdTrackerEntry := range gs.CostakersRewardsTracker {
		costakerAddr, err := sdk.AccAddressFromBech32(rwdTrackerEntry.CostakerAddress)
		if err != nil {
			return err
		}

		err = k.setCostakerRewardsTracker(ctx, costakerAddr, *rwdTrackerEntry.Tracker)
		if err != nil {
			return err
		}
	}

	for _, histRwd := range gs.HistoricalRewards {
		err := k.setHistoricalRewards(ctx, histRwd.Period, *histRwd.Rewards)
		if err != nil {
			return err
		}
	}

	if gs.CurrentRewards.Rewards != nil {
		if err := k.SetCurrentRewards(ctx, *gs.CurrentRewards.Rewards); err != nil {
			return err
		}
	}

	return k.SetParams(ctx, gs.Params)
}
