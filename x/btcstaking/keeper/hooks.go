package keeper

import (
	"context"
	"fmt"

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
	fp, err := h.k.GetFinalityProvider(ctx, fpPk.MustMarshal())
	if err != nil {
		return err
	}

	if fp.IsSluggish() {
		return fmt.Errorf("the finality provider %s is already detected as sluggish", fpPk.MarshalHex())
	}

	fp.Sluggish = true

	h.k.setFinalityProvider(ctx, fp)

	return nil
}
