package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
)

// SetParams sets the x/coostaking module parameters.
func (k Keeper) SetParams(ctx context.Context, p types.Params) error {
	if err := p.Validate(); err != nil {
		return err
	}
	return k.params.Set(ctx, p)
}

// GetParams returns the current x/coostaking module parameters.
func (k Keeper) GetParams(ctx context.Context) (p types.Params) {
	p, err := k.params.Get(ctx)
	if err != nil {
		panic(err)
	}
	return p
}
