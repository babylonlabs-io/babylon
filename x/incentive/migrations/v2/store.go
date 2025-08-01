package v2

import (
	"context"

	bbntypes "github.com/babylonlabs-io/babylon/v3/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v3/x/incentive/types"
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
		currentRewardsWithDecimals, err := bbntypes.CoinsSafeMulInt(fpCurrRwds.CurrentRewards, types.DecimalRewards)
		if err != nil {
			return err
		}

		fpCurrRwds.CurrentRewards = currentRewardsWithDecimals
		return k.SetFinalityProviderCurrentRewards(ctx, fp, fpCurrRwds)
	})
}
