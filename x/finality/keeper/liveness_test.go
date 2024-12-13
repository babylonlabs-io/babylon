package keeper_test

import (
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/core/header"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	testutil "github.com/babylonlabs-io/babylon/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	btclctypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/x/finality/types"
)

func FuzzHandleLiveness(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
		bsKeeper.EXPECT().JailFinalityProvider(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		iKeeper := types.NewMockIncentiveKeeper(ctrl)
		cKeeper := types.NewMockCheckpointingKeeper(ctrl)
		fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, cKeeper)
		blockTime := time.Now()
		ctx = ctx.WithHeaderInfo(header.Info{Time: blockTime})

		params := fKeeper.GetParams(ctx)
		fpPk, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(t, err)
		bsKeeper.EXPECT().GetFinalityProvider(gomock.Any(), fpPk.MustMarshal()).Return(&bstypes.FinalityProvider{Jailed: false}, nil).AnyTimes()
		signingInfo := types.NewFinalityProviderSigningInfo(
			fpPk,
			1,
			0,
		)
		err = fKeeper.FinalityProviderSigningTracker.Set(ctx, fpPk.MustMarshal(), signingInfo)
		require.NoError(t, err)

		// activate BTC staking protocol at a random height
		activatedHeight := int64(datagen.RandomInt(r, 10) + 1)

		// for signed blocks, mark the finality provider as having signed
		height := activatedHeight
		for ; height < activatedHeight+params.SignedBlocksWindow; height++ {
			err := fKeeper.HandleFinalityProviderLiveness(ctx, fpPk, false, height)
			require.NoError(t, err)
		}
		signingInfo, err = fKeeper.FinalityProviderSigningTracker.Get(ctx, fpPk.MustMarshal())
		require.NoError(t, err)
		require.Equal(t, int64(0), signingInfo.MissedBlocksCounter)

		minSignedPerWindow := params.MinSignedPerWindowInt()
		maxMissed := params.SignedBlocksWindow - minSignedPerWindow
		// for blocks up to the inactivity boundary, mark the finality provider as having not signed
		sluggishDetectedHeight := height + maxMissed
		for ; height < sluggishDetectedHeight; height++ {
			err := fKeeper.HandleFinalityProviderLiveness(ctx, fpPk, true, height)
			require.NoError(t, err)
			signingInfo, err = fKeeper.FinalityProviderSigningTracker.Get(ctx, fpPk.MustMarshal())
			require.NoError(t, err)
			require.GreaterOrEqual(t, maxMissed, signingInfo.MissedBlocksCounter)
		}

		// after next height, the fp will be jailed
		err = fKeeper.HandleFinalityProviderLiveness(ctx, fpPk, true, height)
		require.NoError(t, err)
		signingInfo, err = fKeeper.FinalityProviderSigningTracker.Get(ctx, fpPk.MustMarshal())
		require.NoError(t, err)
		require.Equal(t, blockTime.Add(params.JailDuration).Unix(), signingInfo.JailedUntil.Unix())
		require.Equal(t, int64(0), signingInfo.MissedBlocksCounter)
	})
}

// FuzzHandleLivenessDeterminism tests the property of determinism of
// HandleLiveness
func FuzzHandleLivenessDeterminism(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := bstypes.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := bstypes.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)
		changeAddress, err := datagen.GenRandomBTCAddress(r, h.Net)
		require.NoError(t, err)

		// Generate multiple finality providers
		numFPs := 5 // Can be adjusted or randomized
		fps := make([]*bstypes.FinalityProvider, numFPs)
		for i := 0; i < numFPs; i++ {
			fpSK, fpPK, fp := h.CreateFinalityProvider(r)
			h.CommitPubRandList(r, fpSK, fp, 1, 1000, true)
			fps[i] = fp

			// Create delegation for each FP
			stakingValue := int64(2 * 10e8)
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			h.NoError(err)
			stakingTxHash, msgCreateBTCDel, actualDel, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegationWithBtcBlockHeight(
				r,
				delSK,
				fpPK,
				changeAddress.EncodeAddress(),
				stakingValue,
				1000,
				0,
				0,
				true,
				false,
				10,
				10,
			)
			h.NoError(err)
			// generate and insert new covenant signatures
			h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel, 10)
			// activate BTC delegation
			// after that, all BTC delegations will have voting power
			h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof, 30)
		}

		params := h.FinalityKeeper.GetParams(h.Ctx)
		minSignedPerWindow := params.MinSignedPerWindowInt()
		maxMissed := params.SignedBlocksWindow - minSignedPerWindow

		nextHeight := datagen.RandomInt(r, 10) + 2 + uint64(minSignedPerWindow)

		btcTip := &btclctypes.BTCHeaderInfo{Height: 30}
		// for blocks up to the inactivity boundary, mark the finality provider as having not signed
		sluggishDetectedHeight := nextHeight + uint64(maxMissed)
		for ; nextHeight < sluggishDetectedHeight; nextHeight++ {
			h.SetCtxHeight(nextHeight)
			h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(btcTip).AnyTimes()
			h.BeginBlocker()
			h.FinalityKeeper.HandleLiveness(h.Ctx, int64(nextHeight))
		}

		// after next height, the fp will be jailed
		h.SetCtxHeight(nextHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(btcTip).AnyTimes()
		h.BeginBlocker()

		h.FinalityKeeper.HandleLiveness(h.Ctx, int64(nextHeight))
		events := h.BTCStakingKeeper.GetAllPowerDistUpdateEvents(h.Ctx, btcTip.Height, btcTip.Height)
		require.Equal(t, numFPs, len(events))

		err = h.StateStore.RollbackToVersion(int64(nextHeight) - 1)
		require.NoError(t, err)
		h.FinalityKeeper.HandleLiveness(h.Ctx, int64(nextHeight))
		events2 := h.BTCStakingKeeper.GetAllPowerDistUpdateEvents(h.Ctx, btcTip.Height, btcTip.Height)
		require.Equal(t, numFPs, len(events))

		for i, e := range events {
			e1, ok := e.Ev.(*bstypes.EventPowerDistUpdate_JailedFp)
			require.True(t, ok)
			e2, ok := events2[i].Ev.(*bstypes.EventPowerDistUpdate_JailedFp)
			require.True(t, ok)
			require.Equal(t, e1.JailedFp.Pk.MarshalHex(), e2.JailedFp.Pk.MarshalHex())
		}
	})
}
