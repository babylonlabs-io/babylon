package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"
	sdk "github.com/cosmos/cosmos-sdk/types"

	bbntypes "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/babylon/x/finality/types"
)

// AfterFinalityProviderActivated updates the signing info start height or create a new signing info
func (k Keeper) AfterFinalityProviderActivated(ctx context.Context, fpPk *bbntypes.BIP340PubKey) error {
	signingInfo, err := k.FinalityProviderSigningTracker.Get(ctx, fpPk.MustMarshal())
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err == nil {
		signingInfo.StartHeight = sdkCtx.BlockHeight()
	} else if errors.Is(err, collections.ErrNotFound) {
		signingInfo = types.NewFinalityProviderSigningInfo(
			fpPk,
			sdkCtx.BlockHeight(),
			0,
		)
	}

	return k.FinalityProviderSigningTracker.Set(ctx, fpPk.MustMarshal(), signingInfo)
}
