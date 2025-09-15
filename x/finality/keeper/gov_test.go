package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/core/header"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	bbntypes "github.com/babylonlabs-io/babylon/v4/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/v4/x/finality/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

func TestHandleResumeFinalityProposal(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
	iKeeper := types.NewMockIncentiveKeeper(ctrl)
	cKeeper := types.NewMockCheckpointingKeeper(ctrl)
	fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, cKeeper, types.NewMultiFinalityHooks())

	haltingHeight := uint64(100)
	currentHeight := uint64(110)

	activeFpNum := 3
	activeFpPks := generateNFpPks(t, r, activeFpNum)
	setupActiveFps(t, activeFpPks, haltingHeight, fKeeper, ctx)
	// set voting power table for each height, only the first fp votes
	votedFpPk := activeFpPks[0]
	for h := haltingHeight; h <= currentHeight; h++ {
		fKeeper.SetBlock(ctx, &types.IndexedBlock{
			Height:    h,
			AppHash:   datagen.GenRandomByteArray(r, 32),
			Finalized: false,
		})
		dc := types.NewVotingPowerDistCache()
		for i := 0; i < activeFpNum; i++ {
			fKeeper.SetVotingPower(ctx, activeFpPks[i].MustMarshal(), h, 1)
			dc.AddFinalityProviderDistInfo(&types.FinalityProviderDistInfo{
				BtcPk:          &activeFpPks[i],
				Addr:           datagen.GenRandomAddress(),
				TotalBondedSat: 1,
				IsTimestamped:  true,
			})
		}
		dc.ApplyActiveFinalityProviders(uint32(activeFpNum))
		votedSig, err := bbntypes.NewSchnorrEOTSSig(datagen.GenRandomByteArray(r, 32))
		require.NoError(t, err)
		fKeeper.SetSig(ctx, h, &votedFpPk, votedSig)
		fKeeper.SetVotingPowerDistCache(ctx, h, dc)
	}

	// tally blocks and none of them should be finalised
	iKeeper.EXPECT().RewardBTCStaking(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return().AnyTimes()
	ctx = datagen.WithCtxHeight(ctx, currentHeight)
	fKeeper.TallyBlocks(ctx, uint64(10000))
	for i := haltingHeight; i < currentHeight; i++ {
		ib, err := fKeeper.GetBlock(ctx, i)
		require.NoError(t, err)
		require.False(t, ib.Finalized)
	}

	// create a resume finality proposal to jail the last fp
	bsKeeper.EXPECT().JailFinalityProvider(ctx, gomock.Any()).Return(nil).AnyTimes()
	bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), gomock.Any()).Return(&bstypes.FinalityProvider{
		Addr:   datagen.GenRandomAddress().String(),
		Jailed: false,
	}, nil).AnyTimes()
	err := fKeeper.HandleResumeFinalityProposal(ctx, publicKeysToHex(activeFpPks[1:]), uint32(haltingHeight))
	require.NoError(t, err)

	fKeeper.TallyBlocks(ctx, types.MaxFinalizedRewardedBlocksPerEndBlock)

	for i := haltingHeight; i < currentHeight; i++ {
		ib, err := fKeeper.GetBlock(ctx, i)
		require.NoError(t, err)
		require.True(t, ib.Finalized)
	}
}

func TestHandleResumeFinalityProposalWithJailedAndSlashedFp(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
	iKeeper := types.NewMockIncentiveKeeper(ctrl)
	// cKeeper := types.NewMockCheckpointingKeeper(ctrl)
	// TODO: add expected finality mocks data
	fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, nil, types.NewMultiFinalityHooks())

	haltingHeight := uint64(100)
	currentHeight := uint64(110)

	// 4 FPs, after the missed blocks
	// one will be slashed one jailed and two active
	// each one with same VP, only one of the active is correctly voting
	numFps := 4
	activeFpPks := generateNFpPks(t, r, numFps)
	setupActiveFps(t, activeFpPks, haltingHeight, fKeeper, ctx)
	// set voting power table for each height, only the first fp votes
	activeVotingFpPk, slashedFpPk, jailedFpPk, activeFpToBeJailed := activeFpPks[0], activeFpPks[1], activeFpPks[2], activeFpPks[3]
	for h := haltingHeight; h <= currentHeight; h++ {
		fKeeper.SetBlock(ctx, &types.IndexedBlock{
			Height:    h,
			AppHash:   datagen.GenRandomByteArray(r, 32),
			Finalized: false,
		})
		dc := types.NewVotingPowerDistCache()
		for i := 0; i < numFps; i++ {
			fKeeper.SetVotingPower(ctx, activeFpPks[i].MustMarshal(), h, 1)
			dc.AddFinalityProviderDistInfo(&types.FinalityProviderDistInfo{
				BtcPk:          &activeFpPks[i],
				TotalBondedSat: 1,
				IsTimestamped:  true,
			})
		}
		dc.ApplyActiveFinalityProviders(uint32(numFps))
		votedSig, err := bbntypes.NewSchnorrEOTSSig(datagen.GenRandomByteArray(r, 32))
		require.NoError(t, err)
		fKeeper.SetSig(ctx, h, &activeVotingFpPk, votedSig)
		fKeeper.SetVotingPowerDistCache(ctx, h, dc)
	}

	// tally blocks and none of them should be finalised
	ctx = datagen.WithCtxHeight(ctx, currentHeight)
	fKeeper.TallyBlocks(ctx, uint64(10000))
	for i := haltingHeight; i < currentHeight; i++ {
		ib, err := fKeeper.GetBlock(ctx, i)
		require.NoError(t, err)
		require.False(t, ib.Finalized)
	}

	slashedFp := &bstypes.FinalityProvider{
		Addr:                 datagen.GenRandomAddress().String(),
		BtcPk:                &slashedFpPk,
		Jailed:               false,
		SlashedBabylonHeight: currentHeight,
	}
	bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), slashedFpPk.MustMarshal()).Return(slashedFp, nil).AnyTimes()

	jailedFp := &bstypes.FinalityProvider{
		Addr:   datagen.GenRandomAddress().String(),
		BtcPk:  &jailedFpPk,
		Jailed: true,
	}
	bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), jailedFpPk.MustMarshal()).Return(jailedFp, nil).AnyTimes()

	activeNonVoting := &bstypes.FinalityProvider{
		Addr:   datagen.GenRandomAddress().String(),
		BtcPk:  &activeFpToBeJailed,
		Jailed: false,
	}
	bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), activeFpToBeJailed.MustMarshal()).Return(activeNonVoting, nil).AnyTimes()

	// create a resume finality proposal to jail the last fp only
	// the already jailed and slashed fp should not be called
	bsKeeper.EXPECT().JailFinalityProvider(ctx, activeFpToBeJailed.MustMarshal()).Return(nil).Times(1)
	err := fKeeper.HandleResumeFinalityProposal(ctx, publicKeysToHex(activeFpPks[1:]), uint32(haltingHeight))
	require.NoError(t, err)

	fKeeper.TallyBlocks(ctx, types.MaxFinalizedRewardedBlocksPerEndBlock)

	for i := haltingHeight; i <= currentHeight; i++ {
		ib, err := fKeeper.GetBlock(ctx, i)
		require.NoError(t, err)
		require.True(t, ib.Finalized)
	}

	// check that the active non voting fp had the signing info updated
	signingInfo, err := fKeeper.FinalityProviderSigningTracker.Get(ctx, activeFpToBeJailed)
	require.NoError(t, err)
	require.Zero(t, signingInfo.MissedBlocksCounter)

	params := fKeeper.GetParams(ctx)
	currentTime := ctx.HeaderInfo().Time
	require.Equal(t, signingInfo.JailedUntil.String(), currentTime.Add(params.JailDuration).String())

	// verifies the voting power distribution cache has set the FPs as jailed
	// and the correct fps have zero voting power.
	for h := haltingHeight; h <= currentHeight; h++ {
		distCache := fKeeper.GetVotingPowerDistCache(ctx, h)

		// checks the FPs have bonded sats, but zero voting power
		for _, fpDstCache := range distCache.FinalityProviders {
			vp := fKeeper.GetVotingPower(ctx, *fpDstCache.BtcPk, h)
			require.Equal(t, fpDstCache.TotalBondedSat, uint64(1))

			switch fpDstCache.BtcPk.MarshalHex() {
			case activeVotingFpPk.MarshalHex():
				require.Equal(t, vp, uint64(1))
				require.False(t, fpDstCache.IsJailed)
				require.False(t, fpDstCache.IsSlashed)
			case slashedFpPk.MarshalHex():
				require.Zero(t, vp)
				// slashed could have been slashed for a few blocks before and
				// it should keep as slashed, don't update to jailed
				require.True(t, fpDstCache.IsSlashed || fpDstCache.IsJailed)
			case jailedFpPk.MarshalHex():
				require.Zero(t, vp)
				require.True(t, fpDstCache.IsJailed)
			case activeFpToBeJailed.MarshalHex():
				require.Zero(t, vp)
				require.True(t, fpDstCache.IsJailed)
			default:
				t.Error("unexpected fp")
			}
		}
	}
}

func TestHandleResumeFinalityWithBadHaltingHeight(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	k, ctx := keepertest.FinalityKeeper(t, nil, nil, nil, nil)
	ctx = ctx.WithHeaderInfo(header.Info{
		Height: 35,
	})
	actErr := k.HandleResumeFinalityProposal(ctx, []string{}, 50)
	require.EqualError(t, actErr, fmt.Sprintf("finality halting height %d is in the future, current height %d", 50, 35))

	p := k.GetParams(ctx)
	p.FinalityActivationHeight = 20
	err := k.SetParams(ctx, p)
	require.NoError(t, err)

	actErr = k.HandleResumeFinalityProposal(ctx, []string{}, 16)
	require.EqualError(t, actErr, fmt.Sprintf("finality halting height %d cannot be lower than finality activation height %d", 16, 20))
}

func TestHandleResumeFinalityProposalMissingSigningInfo(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
	iKeeper := types.NewMockIncentiveKeeper(ctrl)
	cKeeper := types.NewMockCheckpointingKeeper(ctrl)
	fHooks := types.NewMockFinalityHooks(ctrl)
	fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, cKeeper, fHooks)

	// TODO: add expected values
	fHooks.EXPECT().AfterBtcDelegationActivated(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	fHooks.EXPECT().AfterBtcDelegationUnbonded(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	fHooks.EXPECT().AfterBbnFpEntersActiveSet(gomock.Any(), gomock.Any()).AnyTimes()
	fHooks.EXPECT().AfterBbnFpRemovedFromActiveSet(gomock.Any(), gomock.Any()).AnyTimes()

	// Setup heights
	haltingHeight := uint64(100)
	currentHeight := uint64(110)
	ctx = datagen.WithCtxHeight(ctx, currentHeight)

	// Setup 3 active FPs with proper signing info
	activeFpNum := 3
	activeFpPks := generateNFpPks(t, r, activeFpNum)
	setupActiveFps(t, activeFpPks, haltingHeight, fKeeper, ctx)

	// Setup 1 inactive FP WITHOUT signing info
	inactiveFpPk, err := datagen.GenRandomBIP340PubKey(r)
	require.NoError(t, err)

	// Setup blocks and distribution cache
	for h := haltingHeight; h <= currentHeight; h++ {
		// Create non-finalized block
		fKeeper.SetBlock(ctx, &types.IndexedBlock{
			Height:    h,
			AppHash:   datagen.GenRandomByteArray(r, 32),
			Finalized: false,
		})

		// Set up distribution cache
		dc := types.NewVotingPowerDistCache()

		// Add active FPs with high power
		for i := 0; i < activeFpNum; i++ {
			fKeeper.SetVotingPower(ctx, activeFpPks[i].MustMarshal(), h, 100)
			dc.AddFinalityProviderDistInfo(&types.FinalityProviderDistInfo{
				BtcPk:          &activeFpPks[i],
				TotalBondedSat: 100,
				IsTimestamped:  true,
			})
		}

		// Create a copy of inactiveFpPk to use in the struct
		inactivePkCopy := *inactiveFpPk

		// Add inactive FP with lower power (only to cache, not as active voter)
		dc.AddFinalityProviderDistInfo(&types.FinalityProviderDistInfo{
			BtcPk:          &inactivePkCopy,
			TotalBondedSat: 50,
			IsTimestamped:  true,
		})

		// Only top 3 FPs are active
		dc.ApplyActiveFinalityProviders(3)

		// Only first FP votes
		votedSig, err := bbntypes.NewSchnorrEOTSSig(datagen.GenRandomByteArray(r, 32))
		require.NoError(t, err)
		fKeeper.SetSig(ctx, h, &activeFpPks[0], votedSig)

		// Set the cache
		fKeeper.SetVotingPowerDistCache(ctx, h, dc)
	}

	// Set up expectations for the FP objects
	for i := 0; i < activeFpNum; i++ {
		fpPk := activeFpPks[i]
		bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), fpPk.MustMarshal()).DoAndReturn(
			func(_ interface{}, _ []byte) (*bstypes.FinalityProvider, error) {
				fp := &bstypes.FinalityProvider{
					Jailed: false,
					Addr:   datagen.GenRandomAddress().String(),
				}
				fp.BtcPk = new(bbntypes.BIP340PubKey)
				*fp.BtcPk = fpPk
				return fp, nil
			},
		).AnyTimes()
	}

	// Setup the inactive FP expectation
	inactiveFpAddr := datagen.GenRandomAddress()
	bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), inactiveFpPk.MustMarshal()).DoAndReturn(
		func(_ interface{}, _ []byte) (*bstypes.FinalityProvider, error) {
			fp := &bstypes.FinalityProvider{
				Jailed: false,
				Addr:   inactiveFpAddr.String(),
			}
			fp.BtcPk = new(bbntypes.BIP340PubKey)
			*fp.BtcPk = *inactiveFpPk
			return fp, nil
		},
	).AnyTimes()

	// Setup expectations for jailing
	bsKeeper.EXPECT().JailFinalityProvider(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	iKeeper.EXPECT().RewardBTCStaking(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return().AnyTimes()

	// Verify that initially the inactive FP is NOT part of the active set
	initialDc := fKeeper.GetVotingPowerDistCache(ctx, currentHeight)
	initialActiveFPs := initialDc.GetActiveFinalityProviderSet()
	require.NotContains(t, initialActiveFPs, inactiveFpPk.MarshalHex(),
		"Inactive FP should not be active initially")

	// Now jail two active FPs, which should make room for the inactive FP to become active
	err = fKeeper.HandleResumeFinalityProposal(
		ctx,
		[]string{activeFpPks[1].MarshalHex(), activeFpPks[2].MarshalHex()},
		uint32(haltingHeight),
	)
	require.NoError(t, err)

	// Verify the inactive FP is now active after redistribution
	dc := fKeeper.GetVotingPowerDistCache(ctx, currentHeight)
	activeFPs := dc.GetActiveFinalityProviderSet()
	require.Contains(t, activeFPs, inactiveFpPk.MarshalHex(), "Inactive FP should now be active")

	// Create next height
	nextHeight := currentHeight + 1
	ctx = datagen.WithCtxHeight(ctx, nextHeight)

	// Set up the voting power table to include the inactive FP at next height
	// This ensures it will be processed by HandleLiveness
	fKeeper.SetBlock(ctx, &types.IndexedBlock{
		Height:    nextHeight,
		AppHash:   datagen.GenRandomByteArray(r, 32),
		Finalized: false,
	})

	// Create a DC for the next height
	nextDc := types.NewVotingPowerDistCache()

	// Add inactive FP as active FP
	inactivePkCopy := *inactiveFpPk
	nextDc.AddFinalityProviderDistInfo(&types.FinalityProviderDistInfo{
		BtcPk:          &inactivePkCopy,
		TotalBondedSat: 50,
		IsTimestamped:  true,
	})

	// Add remaining active FP
	nextDc.AddFinalityProviderDistInfo(&types.FinalityProviderDistInfo{
		BtcPk:          &activeFpPks[0],
		TotalBondedSat: 100,
		IsTimestamped:  true,
	})

	// Set both FPs as active
	nextDc.ApplyActiveFinalityProviders(2)
	fKeeper.SetVotingPowerDistCache(ctx, nextHeight, nextDc)
	fKeeper.SetVotingPower(ctx, inactiveFpPk.MustMarshal(), nextHeight, 50)
	fKeeper.SetVotingPower(ctx, activeFpPks[0].MustMarshal(), nextHeight, 100)

	nextActiveSet := nextDc.GetActiveFinalityProviderSet()
	require.Contains(t, nextActiveSet, inactiveFpPk.MarshalHex(), "FP should be active in next height cache")

	// Verify we have signing info for the new active FP
	_, err = fKeeper.FinalityProviderSigningTracker.Get(ctx, inactiveFpPk.MustMarshal())
	require.NoError(t, err, "Should have signing info for new active FP")

	require.NotPanics(t, func() {
		fKeeper.HandleLiveness(ctx, int64(nextHeight))
	}, "Unexpected panic due to missing signing info for newly active FP")
}

func generateNFpPks(t *testing.T, r *rand.Rand, n int) []bbntypes.BIP340PubKey {
	fpPks := make([]bbntypes.BIP340PubKey, 0, n)
	for i := 0; i < n; i++ {
		fpPk, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(t, err)
		fpPks = append(fpPks, *fpPk)
	}

	return fpPks
}

func publicKeysToHex(pks []bbntypes.BIP340PubKey) []string {
	hexPks := make([]string, len(pks))
	for i, pk := range pks {
		hexPks[i] = pk.MarshalHex()
	}
	return hexPks
}

func setupActiveFps(t *testing.T, fpPks []bbntypes.BIP340PubKey, height uint64, fKeeper *keeper.Keeper, ctx sdk.Context) {
	for _, fpPk := range fpPks {
		signingInfo := types.NewFinalityProviderSigningInfo(
			&fpPk,
			int64(height),
			0,
		)
		err := fKeeper.FinalityProviderSigningTracker.Set(ctx, fpPk, signingInfo)
		require.NoError(t, err)
	}
}
