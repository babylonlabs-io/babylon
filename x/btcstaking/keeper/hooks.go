package keeper

import (
	"context"

	bbntypes "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/babylon/x/finality/types"
)

var _ types.FinalityHooks = Hooks{}

// Hooks wrapper struct for BTC staking keeper
type Hooks struct {
	k Keeper
}

// Return the finality hooks
func (k Keeper) Hooks() Hooks {
	return Hooks{k}
}

// AfterSluggishFinalityProviderDetected updates the status of the given finality provider to `sluggish`
func (h Hooks) AfterSluggishFinalityProviderDetected(ctx context.Context, fpPk *bbntypes.BIP340PubKey) error {
	return h.k.JailFinalityProvider(ctx, fpPk.MustMarshal())
}
