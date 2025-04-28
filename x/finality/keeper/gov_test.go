package keeper_test

import (
	"math/rand"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v2/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v2/testutil/keeper"
	bbntypes "github.com/babylonlabs-io/babylon/v2/types"
	bstypes "github.com/babylonlabs-io/babylon/v2/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/v2/x/finality/keeper"
	"github.com/babylonlabs-io/babylon/v2/x/finality/types"
)

func TestHandleResumeFinalityProposal(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
	iKeeper := types.NewMockIncentiveKeeper(ctrl)
	cKeeper := types.NewMockCheckpointingKeeper(ctrl)
	fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, cKeeper)

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
	fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, nil)

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
		BtcPk:                &slashedFpPk,
		Jailed:               false,
		SlashedBabylonHeight: currentHeight,
	}
	bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), slashedFpPk.MustMarshal()).Return(slashedFp, nil).MaxTimes(1)

	jailedFp := &bstypes.FinalityProvider{
		BtcPk:  &slashedFpPk,
		Jailed: true,
	}
	bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), jailedFpPk.MustMarshal()).Return(jailedFp, nil).MaxTimes(1)

	activeNonVoting := &bstypes.FinalityProvider{
		BtcPk:  &activeFpToBeJailed,
		Jailed: false,
	}
	bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), activeFpToBeJailed.MustMarshal()).Return(activeNonVoting, nil).MaxTimes(1)

	// create a resume finality proposal to jail the last fp only
	// the already jailed and slashed fp should not be called
	bsKeeper.EXPECT().JailFinalityProvider(ctx, activeFpToBeJailed.MustMarshal()).Return(nil).MaxTimes(1)
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
