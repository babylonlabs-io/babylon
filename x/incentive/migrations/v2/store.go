package v2

import (
	"context"

	bbntypes "github.com/babylonlabs-io/babylon/v4/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

// Keeper the expected keeper interface to perform the migration
type Keeper interface {
	SetFinalityProviderCurrentRewards(ctx context.Context, fp sdk.AccAddress, rwd types.FinalityProviderCurrentRewards) error
	IterateFpCurrentRewards(ctx context.Context, it func(fp sdk.AccAddress, fpCurrRwds types.FinalityProviderCurrentRewards) error) error
}

// MigrateStore performs in-place store migrations.
// Migration updates all finality provider current rewards by the hardcoded decimal rewards
// which adds 20 decimals points to increase precision at the time to calculate how much
// rewards each satoshi staked is entitled to receive.
func MigrateStore(
	ctx sdk.Context,
	k Keeper,
) error {
	return k.IterateFpCurrentRewards(ctx, func(fp sdk.AccAddress, fpCurrRwds types.FinalityProviderCurrentRewards) error {
		if fpCurrRwds.CurrentRewards.IsZero() { // no rewards, there is no need to migrate
			return nil
		}

		currentRewardsWithDecimals, err := bbntypes.CoinsSafeMulInt(fpCurrRwds.CurrentRewards, types.DecimalRewards)
		if err != nil {
			return types.ErrInvalidAmount.Wrapf("unable to migrate to rewards with decimals for fp %s - %s: %v", fp.String(), fpCurrRwds.CurrentRewards.String(), err)
		}

		fpCurrRwds.CurrentRewards = currentRewardsWithDecimals
		err = k.SetFinalityProviderCurrentRewards(ctx, fp, fpCurrRwds)
		if err != nil {
			return types.ErrFPCurrentRewardsInvalid.Wrapf("unable to migrate to rewards with decimals for fp %s - %s: %v", fp.String(), fpCurrRwds.CurrentRewards.String(), err)
		}
		return nil
	})
}
