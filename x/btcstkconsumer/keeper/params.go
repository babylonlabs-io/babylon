package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
)

// GetParams get all parameters as types.Params
func (k Keeper) GetParams(ctx context.Context) (params types.Params) {
	result, err := k.ParamsCollection.Get(ctx)
	if err != nil {
		// Return default params if not found
		return params
	}
	return result
}

// SetParams set the params
func (k Keeper) SetParams(ctx context.Context, params types.Params) error {
	return k.ParamsCollection.Set(ctx, params)
}
