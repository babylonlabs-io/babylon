package keeper

import (
	"errors"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	bbntypes "github.com/babylonlabs-io/babylon/types"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
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
	currentHeight := ctx.HeaderInfo().Height
	currentTime := ctx.HeaderInfo().Time

	// jail the given finality providers
	fpPks := make([]*bbntypes.BIP340PubKey, 0, len(fpPksHex))
	for _, fpPkHex := range fpPksHex {
		fpPk, err := bbntypes.NewBIP340PubKeyFromHex(fpPkHex)
		if err != nil {
			return fmt.Errorf("invalid finality provider public key %s: %w", fpPkHex, err)
		}
		fpPks = append(fpPks, fpPk)

		voters := k.GetVoters(ctx, uint64(haltingHeight))
		_, voted := voters[fpPkHex]
		if voted {
			// all the given finality providers should not have voted for the halting height
			return fmt.Errorf("the finality provider %s has voted for height %d", fpPkHex, haltingHeight)
		}

		err = k.jailSluggishFinalityProvider(ctx, fpPk)
		if err != nil && !errors.Is(err, bstypes.ErrFpAlreadyJailed) {
			return fmt.Errorf("failed to jail the finality provider %s: %w", fpPkHex, err)
		}

		// update signing info
		signInfo, err := k.FinalityProviderSigningTracker.Get(ctx, fpPk.MustMarshal())
		if err != nil {
			return fmt.Errorf("the signing info of finality provider %s is not created: %w", fpPkHex, err)
		}
		signInfo.JailedUntil = currentTime.Add(params.JailDuration)
		signInfo.MissedBlocksCounter = 0
		if err := k.DeleteMissedBlockBitmap(ctx, fpPk); err != nil {
			return fmt.Errorf("failed to remove the missed block bit map for finality provider %s: %w", fpPkHex, err)
		}
		err = k.FinalityProviderSigningTracker.Set(ctx, fpPk.MustMarshal(), signInfo)
		if err != nil {
			return fmt.Errorf("failed to set the signing info for finality provider %s: %w", fpPkHex, err)
		}

		k.Logger(ctx).Info(
			"finality provider is jailed in the proposal",
			"height", haltingHeight,
			"public_key", fpPkHex,
		)
	}

	// set the all the given finality providers voting power to 0
	for h := uint64(haltingHeight); h <= uint64(currentHeight); h++ {
		distCache := k.GetVotingPowerDistCache(ctx, h)
		activeFps := distCache.GetActiveFinalityProviderSet()
		for _, fpToJail := range fpPks {
			if fp, exists := activeFps[fpToJail.MarshalHex()]; exists {
				fp.IsJailed = true
				k.SetVotingPower(ctx, fpToJail.MustMarshal(), h, 0)
			}
		}

		distCache.ApplyActiveFinalityProviders(params.MaxActiveFinalityProviders)

		// set the voting power distribution cache of the current height
		k.SetVotingPowerDistCache(ctx, h, distCache)
	}

	k.TallyBlocks(ctx)

	return nil
}
