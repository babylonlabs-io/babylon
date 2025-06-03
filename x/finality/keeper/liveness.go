package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v3/types"
	finalitytypes "github.com/babylonlabs-io/babylon/v3/x/finality/types"
)

// HandleLiveness handles liveness of each active finality provider for a given height
// including jailing sluggish finality providers and applying punishment (TBD)
func (k Keeper) HandleLiveness(ctx context.Context, height int64) {
	// get all the active finality providers for the height
	vpTableOrdered := k.GetVotingPowerTableOrdered(ctx, uint64(height))
	// get all the voters for the height
	voterBTCPKs := k.GetVoters(ctx, uint64(height))

	// Iterate over all the finality providers which *should* have signed this block
	// store whether or not they have actually signed it, identify sluggish
	// ones, and apply punishment (TBD)
	// Iterate over all the finality providers in sorted order by the voting power
	for _, fpWithVp := range vpTableOrdered {
		fpPkHex := fpWithVp.FpPk.MarshalHex()
		_, ok := voterBTCPKs[fpPkHex]
		missed := !ok

		err := k.HandleFinalityProviderLiveness(ctx, fpWithVp.FpPk, missed, height)
		if err != nil {
			panic(fmt.Errorf("failed to handle liveness of finality provider %s: %w", fpPkHex, err))
		}
	}
}

// HandleFinalityProviderLiveness updates the voting history of the given finality provider and
// jail sluggish the finality provider if the number of missed block is reached to the threshold in a
// sliding window
func (k Keeper) HandleFinalityProviderLiveness(ctx context.Context, fpPk *types.BIP340PubKey, missed bool, height int64) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	fp, err := k.BTCStakingKeeper.GetFinalityProvider(ctx, fpPk.MustMarshal())
	if err != nil {
		return err
	}

	// don't update missed blocks when finality provider is already slashed or jailed
	if fp.IsSlashed() || fp.IsJailed() {
		k.Logger(sdkCtx).Debug(
			"skip handling liveness",
			"height", height,
			"public_key", fpPk.MarshalHex(),
			"is_slashed", fp.IsSlashed(),
			"is_jailed", fp.IsJailed(),
		)
		return nil
	}

	updated, signInfo, err := k.UpdateSigningInfo(ctx, fpPk, missed, height)
	if err != nil {
		return err
	}

	signedBlocksWindow := params.SignedBlocksWindow
	minSignedPerWindow := params.MinSignedPerWindowInt()

	if missed {
		k.Logger(sdkCtx).Debug(
			"absent finality provider",
			"height", height,
			"public_key", fpPk.MarshalHex(),
			"missed", signInfo.MissedBlocksCounter,
			"threshold", minSignedPerWindow,
		)
	}

	minHeight := signInfo.StartHeight + signedBlocksWindow
	maxMissed := signedBlocksWindow - minSignedPerWindow

	// if the number of missed block reaches the threshold within the sliding window
	// jail the finality provider
	if height > minHeight && signInfo.MissedBlocksCounter > maxMissed {
		updated = true

		if err := k.jailSluggishFinalityProvider(ctx, fpPk); err != nil {
			return fmt.Errorf("failed to jail sluggish finality provider %s: %w", fpPk.MarshalHex(), err)
		}

		signInfo.JailedUntil = sdkCtx.HeaderInfo().Time.Add(params.JailDuration)
		// we need to reset the counter & bitmap so that the finality provider won't be
		// immediately jailed after unjailing.
		signInfo.MissedBlocksCounter = 0
		if err := k.DeleteMissedBlockBitmap(ctx, fpPk); err != nil {
			return fmt.Errorf("failed to remove the missed block bit map: %w", err)
		}

		k.Logger(sdkCtx).Info(
			"finality provider is jailed",
			"height", height,
			"public_key", fpPk.MarshalHex(),
		)
	}

	// Set the updated signing info
	if updated {
		return k.FinalityProviderSigningTracker.Set(ctx, fpPk.MustMarshal(), *signInfo)
	}

	return nil
}

func (k Keeper) UpdateSigningInfo(
	ctx context.Context,
	fpPk *types.BIP340PubKey,
	missed bool,
	height int64,
) (bool, *finalitytypes.FinalityProviderSigningInfo, error) {
	params := k.GetParams(ctx)
	// fetch signing info
	signInfo, err := k.FinalityProviderSigningTracker.Get(ctx, fpPk.MustMarshal())
	if err != nil {
		return false, nil, fmt.Errorf("the signing info is not created")
	}

	signedBlocksWindow := params.SignedBlocksWindow

	// Compute the relative index, so we count the blocks the finality provider *should*
	// have signed. We will also use the 0-value default signing info if not present.
	// The index is in the range [0, SignedBlocksWindow)
	// and is used to see if a finality provider signed a block at the given height, which
	// is represented by a bit in the bitmap.
	// The finality provider start height should get mapped to index 0, so we computed index as:
	// (height - startHeight) % signedBlocksWindow
	//
	// NOTE: There is subtle different behavior between genesis finality provider and non-genesis
	// finality providers.
	// A genesis finality provider will start at index 0, whereas a non-genesis finality provider's
	// startHeight will be the block they become active for, but the first block they vote on will be
	// one later. (And thus their first vote is at index 1)
	//
	// NOTE: due to the introduction of the parameter `FinalitySigTimeout`, if there are `x`
	// consecutive blocks for which a fp is non-active in the middle of the fp being active
	// where `x` < FinalitySigTimeout, it is possible that `signInfo.StartHeight > height`
	// in this case, it should return directly because it indicates that the fp does not
	// need to vote for the height we are examining. This ensures the index calculated
	// below will not be negative
	if signInfo.StartHeight > height {
		return false, &signInfo, nil
	}
	index := (height - signInfo.StartHeight) % signedBlocksWindow

	// determine if the finality provider signed the previous block
	previous, err := k.GetMissedBlockBitmapValue(ctx, fpPk, index)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get the finality provider's bitmap value: %w", err)
	}

	modifiedSignInfo := false
	switch {
	case !previous && missed:
		// Bitmap value has changed from not missed to missed, so we flip the bit
		// and increment the counter.
		if err := k.SetMissedBlockBitmapValue(ctx, fpPk, index, true); err != nil {
			return false, nil, err
		}

		signInfo.IncrementMissedBlocksCounter()
		modifiedSignInfo = true

	case previous && !missed:
		// Bitmap value has changed from missed to not missed, so we flip the bit
		// and decrement the counter.
		if err := k.SetMissedBlockBitmapValue(ctx, fpPk, index, false); err != nil {
			return false, nil, err
		}

		signInfo.DecrementMissedBlocksCounter()
		modifiedSignInfo = true

	default:
		// bitmap value at this index has not changed, no need to update counter
	}

	return modifiedSignInfo, &signInfo, nil
}

func (k Keeper) jailSluggishFinalityProvider(ctx context.Context, fpBtcPk *types.BIP340PubKey) error {
	err := k.BTCStakingKeeper.JailFinalityProvider(ctx, fpBtcPk.MustMarshal())
	if err != nil {
		return err
	}

	err = sdk.UnwrapSDKContext(ctx).EventManager().EmitTypedEvent(
		finalitytypes.NewEventJailedFinalityProvider(fpBtcPk),
	)
	if err != nil {
		return fmt.Errorf("failed to emit sluggish finality provider detected event: %w", err)
	}

	finalitytypes.IncrementJailedFinalityProviderCounter()

	return nil
}
