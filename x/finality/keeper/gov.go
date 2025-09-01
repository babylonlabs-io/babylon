package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	bbntypes "github.com/babylonlabs-io/babylon/v3/types"
	ftypes "github.com/babylonlabs-io/babylon/v3/x/finality/types"
)

// HandleResumeFinalityProposal handles the resume finality proposal in the following steps:
//  1. check the validity of the proposal
//  2. jail the finality providers from the list and adjust the voting power cache from the
//     halting height to the current height
//  3. tally blocks to ensure finality is resumed
func (k Keeper) HandleResumeFinalityProposal(ctx sdk.Context, fpPksHex []string, haltingHeight uint32) error {
	// a valid proposal should be
	// 1. the halting height along with some parameterized future heights should be indeed non-finalized
	// 2. all the fps from the proposal should have missed the vote for the halting height
	// TODO introduce a parameter to define the finality has been halting for at least some heights

	params := k.GetParams(ctx)
	header := ctx.HeaderInfo()
	currentHeight := header.Height
	currentTime := header.Time

	if uint64(haltingHeight) < params.FinalityActivationHeight {
		return fmt.Errorf("finality halting height %d cannot be lower than finality activation height %d",
			haltingHeight, params.FinalityActivationHeight)
	}

	if int64(haltingHeight) > currentHeight {
		return fmt.Errorf("finality halting height %d is in the future, current height %d", haltingHeight, currentHeight)
	}

	voters := k.GetVoters(ctx, uint64(haltingHeight))

	// jail the given finality providers
	fpPksToJail := make(map[string]struct{}, len(fpPksHex))
	for _, fpPkHex := range fpPksHex {
		fpPk, err := bbntypes.NewBIP340PubKeyFromHex(fpPkHex)
		if err != nil {
			return fmt.Errorf("invalid finality provider public key %s: %w", fpPkHex, err)
		}
		fpPksToJail[fpPkHex] = struct{}{}

		_, voted := voters[fpPkHex]
		if voted {
			// all the given finality providers should not have voted for the halting height
			return fmt.Errorf("the finality provider %s has voted for height %d", fpPkHex, haltingHeight)
		}

		fpBtcPk := fpPk.MustMarshal()
		fp, err := k.BTCStakingKeeper.GetFinalityProvider(ctx, fpBtcPk)
		if err != nil {
			return fmt.Errorf("failed to find the finality provider %s in btcstaking: %w", fpPkHex, err)
		}

		k.Logger(ctx).Debug(
			"fp running proposal resume finality",
			"jailed", fp.IsJailed(),
			"slashed", fp.IsSlashed(),
			"height", haltingHeight,
			"public_key", fpPkHex,
		)

		// if the FP is already jailed or slashed, no need to try to set to jail
		// or to update the signing info
		if fp.IsSlashed() || fp.IsJailed() {
			continue
		}

		k.Logger(ctx).Debug(
			"fp will be jailed",
			"height", haltingHeight,
			"public_key", fpPkHex,
		)

		err = k.jailSluggishFinalityProvider(ctx, fpPk)
		if err != nil {
			return fmt.Errorf("failed to jail the finality provider %s: %w", fpPkHex, err)
		}

		// update signing info
		signInfo, err := k.FinalityProviderSigningTracker.Get(ctx, fpBtcPk)
		if err != nil {
			return fmt.Errorf("the signing info of finality provider %s is not created: %w", fpPkHex, err)
		}
		signInfo.JailedUntil = currentTime.Add(params.JailDuration)
		signInfo.MissedBlocksCounter = 0

		if err := k.DeleteMissedBlockBitmap(ctx, fpPk); err != nil {
			return fmt.Errorf("failed to remove the missed block bit map for finality provider %s: %w", fpPkHex, err)
		}

		err = k.FinalityProviderSigningTracker.Set(ctx, fpBtcPk, signInfo)
		if err != nil {
			return fmt.Errorf("failed to set the signing info for finality provider %s: %w", fpPkHex, err)
		}

		k.Logger(ctx).Info(
			"finality provider was jailed",
			"height", haltingHeight,
			"public_key", fpPkHex,
		)
	}

	// set the all the given finality providers voting power to 0 and updates voting power distribution cache
	for blkHeight := uint64(haltingHeight); blkHeight <= uint64(currentHeight); blkHeight++ {
		dc := k.GetVotingPowerDistCache(ctx, blkHeight)
		newDc := ftypes.NewVotingPowerDistCache()

		for i := range dc.FinalityProviders {
			// create a copy of the finality provider
			fp := *dc.FinalityProviders[i]
			fpBTCPKHex := fp.BtcPk.MarshalHex()

			_, shouldJail := fpPksToJail[fpBTCPKHex]
			if shouldJail {
				// if the fp was already slashed at that height, keep as it was
				// and do not update to jailed.
				if !fp.IsSlashed {
					fp.IsJailed = true
				}

				k.SetVotingPower(ctx, fp.BtcPk.MustMarshal(), blkHeight, 0)
			}

			// add this finality provider to the new cache if it has voting power
			newDc.AddFinalityProviderDistInfo(&fp)
		}

		// sets the total vp in the new dc
		newDc.ApplyActiveFinalityProviders(dc.NumActiveFps)

		// set the voting power distribution cache of the current height
		k.SetVotingPowerDistCache(ctx, blkHeight, newDc)

		// ensure every active finality provider has signing info
		k.HandleFPStateUpdates(ctx, dc, newDc, nil)
	}

	return nil
}
