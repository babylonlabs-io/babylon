package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	bbntypes "github.com/babylonlabs-io/babylon/types"
)

func (k Keeper) JailFinalityProvidersFromHeight(ctx sdk.Context, fps []bbntypes.BIP340PubKey, height uint32) error {
	return nil
}
