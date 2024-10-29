package keeper

import (
	"errors"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/x/finality/types"
)

// HandleResumeFinalityProposal handles the resume finality proposal in the following steps:
//  1. check the validity of the proposal
//  2. jail the finality providers from the list and adjust the voting power cache from the
//     halting height to the current height
//  3. tally blocks to ensure finality is resumed
func (k Keeper) HandleResumeFinalityProposal(ctx sdk.Context, p *types.ResumeFinalityProposal) error {
	// a valid proposal should be
	// 1. the halting height along with some parameterized future heights should be indeed non-finalized
	// 2. all the fps from the proposal should have missed the vote for the halting height
	// TODO introduce a parameter to define the finality has been halting for at least some heights

	params := k.GetParams(ctx)
	currentHeight := ctx.HeaderInfo().Height
	currentTime := ctx.HeaderInfo().Time

	// jail the given finality providers
	for _, fpPk := range p.FpPks {
		fpHex := fpPk.MarshalHex()
		voters := k.GetVoters(ctx, uint64(p.HaltingHeight))
		_, voted := voters[fpPk.MarshalHex()]
		if voted {
			// all the given finality providers should not have voted for the halting height
			return fmt.Errorf("the finality provider has voted for height %d", p.HaltingHeight)
		}

		err := k.jailSluggishFinalityProvider(ctx, &fpPk)
		if err != nil && !errors.Is(err, bstypes.ErrFpAlreadyJailed) {
			return fmt.Errorf("failed to jail the finality provider: %w", err)
		}

		// update signing info
		signInfo, err := k.FinalityProviderSigningTracker.Get(ctx, fpPk.MustMarshal())
		if err != nil {
			return fmt.Errorf("the signing info is not created: %w", err)
		}
		signInfo.JailedUntil = currentTime.Add(params.JailDuration)
		signInfo.MissedBlocksCounter = 0
		if err := k.DeleteMissedBlockBitmap(ctx, &fpPk); err != nil {
			return fmt.Errorf("failed to remove the missed block bit map: %w", err)
		}
		err = k.FinalityProviderSigningTracker.Set(ctx, fpPk.MustMarshal(), signInfo)
		k.Logger(ctx).Info(
			"finality provider is jailed",
			"height", p.HaltingHeight,
			"public_key", fpHex,
		)
	}

	// set the all the given finality providers voting power to 0
	for h := uint64(p.HaltingHeight); h <= uint64(currentHeight); h++ {
		distCache := k.GetVotingPowerDistCache(ctx, h)
		activeFps := distCache.GetActiveFinalityProviderSet()
		for _, fpToJail := range p.FpPks {
			if fp, exists := activeFps[fpToJail.MarshalHex()]; exists {
				fp.IsJailed = true
				k.SetVotingPower(ctx, fpToJail, h, 0)
			}
		}

		distCache.ApplyActiveFinalityProviders(params.MaxActiveFinalityProviders)

		// set the voting power distribution cache of the current height
		k.SetVotingPowerDistCache(ctx, h, distCache)
	}

	k.TallyBlocks(ctx)

	return nil
}
