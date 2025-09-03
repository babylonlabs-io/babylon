package v2

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"
	v1types "github.com/babylonlabs-io/babylon/v4/x/epoching/migrations/v1"
	"github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// MigrateStore performs in-place store migrations from v1 to v2.
// Migration adds ExecuteGas and MinAmount parameters to existing Params.

func MigrateStore(
	ctx sdk.Context,
	s storetypes.KVStore,
	cdc codec.BinaryCodec,
) error {
	if err := migrateParams(ctx, s, cdc); err != nil {
		return fmt.Errorf("epoching: migrate params v1->v2: %w", err)
	}
	return nil
}

func migrateParams(ctx sdk.Context, s storetypes.KVStore, cdc codec.BinaryCodec) error {
	// Get the existing params bytes from store
	paramsBz := s.Get(types.ParamsKey)
	if paramsBz == nil {
		// No existing params, set default v2 params
		defaultParams := types.DefaultParams()
		bz, err := cdc.Marshal(&defaultParams)
		if err != nil {
			return fmt.Errorf("marshal default params: %w", err)
		}
		s.Set(types.ParamsKey, bz)
		ctx.Logger().Info("epoching: created default v2 params (no existing params)")
		return nil
	}

	// Unmarshal existing v1 params (EpochInterval only)
	var v1Params v1types.Params
	err := cdc.Unmarshal(paramsBz, &v1Params)
	if err != nil {
		return fmt.Errorf("unmarshal existing v1 params: %w", err)
	}

	// Create v2 params with migrated values + new default fields
	defaultParams := types.DefaultParams()
	v2Params := types.Params{
		EpochInterval: v1Params.EpochInterval,
		ExecuteGas:    defaultParams.ExecuteGas,
		MinAmount:     defaultParams.MinAmount,
	}

	// Marshal updated params
	bz, err := cdc.Marshal(&v2Params)
	if err != nil {
		return fmt.Errorf("marshal migrated params: %w", err)
	}

	// Save migrated params back to store
	s.Set(types.ParamsKey, bz)
	ctx.Logger().Info("epoching: migrated params v1→v2 (added ExecuteGas/MinAmount)")

	return nil
}
