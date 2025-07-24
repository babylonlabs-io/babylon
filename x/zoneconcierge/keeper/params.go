package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
)

// SetParams sets the x/zoneconcierge module parameters.
func (k Keeper) SetParams(ctx context.Context, p types.Params) error {
	if err := p.Validate(); err != nil {
		return err
	}

	return k.ParamsCollection.Set(ctx, p)
}

// GetParams returns the current x/zoneconcierge module parameters.
func (k Keeper) GetParams(ctx context.Context) (p types.Params) {
	params, err := k.ParamsCollection.Get(ctx)
	if err != nil {
		// Return default params if not found
		return p
	}
	return params
}
