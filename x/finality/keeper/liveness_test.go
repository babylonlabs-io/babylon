package keeper_test

import (
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/core/header"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	babylonApp "github.com/babylonlabs-io/babylon/v4/app"
	"github.com/babylonlabs-io/babylon/v4/app/signingcontext"
	testutil "github.com/babylonlabs-io/babylon/v4/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	btcstakingkeeper "github.com/babylonlabs-io/babylon/v4/x/btcstaking/keeper"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/v4/x/finality/types"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
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
		fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, cKeeper, nil)
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
// HandleLiveness by creating two helpers with the same steps to jailing
// and asserting the jailing events should be with the same order
func FuzzHandleLivenessDeterminism(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := bstypes.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := bstypes.NewMockBtcCheckpointKeeper(ctrl)
		h1 := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)
		h2 := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

		// TODO: add expected values
		fHooks := h1.FinalityHooks.(*ftypes.MockFinalityHooks)
		fHooks.EXPECT().AfterBtcDelegationActivated(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		fHooks.EXPECT().AfterBtcDelegationUnbonded(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		fHooks.EXPECT().AfterBbnFpEntersActiveSet(gomock.Any(), gomock.Any()).AnyTimes()
		fHooks.EXPECT().AfterBbnFpExistsActiveSet(gomock.Any(), gomock.Any()).AnyTimes()
		fHooks = h2.FinalityHooks.(*ftypes.MockFinalityHooks)
		fHooks.EXPECT().AfterBtcDelegationActivated(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		fHooks.EXPECT().AfterBtcDelegationUnbonded(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		fHooks.EXPECT().AfterBbnFpEntersActiveSet(gomock.Any(), gomock.Any()).AnyTimes()
		fHooks.EXPECT().AfterBbnFpExistsActiveSet(gomock.Any(), gomock.Any()).AnyTimes()

		// set all parameters
		covenantSKs, _ := h1.GenAndApplyParams(r)
		params := h1.BTCStakingKeeper.GetParams(h1.Ctx)
		err := h2.BTCStakingKeeper.SetParams(h2.Ctx, params)
		require.NoError(t, err)

		// Generate multiple finality providers
		numFPs := 5 // Can be adjusted or randomized
		fps := make([]*bstypes.FinalityProvider, numFPs)
		for i := 0; i < numFPs; i++ {
			fpSK, fpPK, fp := h1.CreateFinalityProvider(r)
			require.NoError(t, err)
			h2.AddFinalityProvider(fp)
			h1.CommitPubRandList(r, fpSK, fp, 1, 1000, true)
			h2.CommitPubRandList(r, fpSK, fp, 1, 1000, true)
			fps[i] = fp

			// Create delegation for each FP
			stakingValue := int64(2 * 10e8)
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			require.NoError(t, err)
			stakingTxHash, msgCreateBTCDel, actualDel, btcHeaderInfo, inclusionProof, _, err := h1.CreateDelegationWithBtcBlockHeight(
				r,
				delSK,
				[]*btcec.PublicKey{fpPK},
				stakingValue,
				1000,
				0,
				0,
				true,
				false,
				10,
				10,
			)
			require.NoError(t, err)
			stakingTxHash2, msgCreateBTCDel2, actualDel2, btcHeaderInfo2, inclusionProof2, _, err := h2.CreateDelegationWithBtcBlockHeight(
				r,
				delSK,
				[]*btcec.PublicKey{fpPK},
				stakingValue,
				1000,
				0,
				0,
				true,
				false,
				10,
				10,
			)
			require.NoError(t, err)
			// generate and insert new covenant signatures
			h1.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel, 10)
			h2.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel2, actualDel2, 10)
			// activate BTC delegation
			// after that, all BTC delegations will have voting power
			h1.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof, 30)
			h2.AddInclusionProof(stakingTxHash2, btcHeaderInfo2, inclusionProof2, 30)
		}

		fParams := h1.FinalityKeeper.GetParams(h1.Ctx)
		minSignedPerWindow := fParams.MinSignedPerWindowInt()
		maxMissed := fParams.SignedBlocksWindow - minSignedPerWindow

		nextHeight := datagen.RandomInt(r, 10) + 2 + uint64(minSignedPerWindow)

		btcTip := &btclctypes.BTCHeaderInfo{Height: 30}
		// for blocks up to the inactivity boundary, mark the finality provider as having not signed
		sluggishDetectedHeight := nextHeight + uint64(maxMissed)
		for ; nextHeight < sluggishDetectedHeight; nextHeight++ {
			h1.SetCtxHeight(nextHeight)
			h1.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h1.Ctx)).Return(btcTip).AnyTimes()
			h1.BeginBlocker()
			h1.FinalityKeeper.HandleLiveness(h1.Ctx, int64(nextHeight))

			h2.SetCtxHeight(nextHeight)
			h2.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h2.Ctx)).Return(btcTip).AnyTimes()
			h2.BeginBlocker()
			h2.FinalityKeeper.HandleLiveness(h2.Ctx, int64(nextHeight))
		}

		// after next height, the fp will be jailed
		h1.SetCtxHeight(nextHeight)
		h1.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h1.Ctx)).Return(btcTip).AnyTimes()
		h1.BeginBlocker()

		h1.FinalityKeeper.HandleLiveness(h1.Ctx, int64(nextHeight))
		events := h1.BTCStakingKeeper.GetAllPowerDistUpdateEvents(h1.Ctx, btcTip.Height, btcTip.Height)
		require.Equal(t, numFPs, len(events))

		h2.SetCtxHeight(nextHeight)
		h2.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h2.Ctx)).Return(btcTip).AnyTimes()
		h2.BeginBlocker()

		h2.FinalityKeeper.HandleLiveness(h2.Ctx, int64(nextHeight))
		events2 := h2.BTCStakingKeeper.GetAllPowerDistUpdateEvents(h2.Ctx, btcTip.Height, btcTip.Height)
		require.Equal(t, numFPs, len(events))

		// ensure the jailing events are in the same order in two different runs
		for i, e := range events {
			e1, ok := e.Ev.(*bstypes.EventPowerDistUpdate_JailedFp)
			require.True(t, ok)
			e2, ok := events2[i].Ev.(*bstypes.EventPowerDistUpdate_JailedFp)
			require.True(t, ok)
			require.Equal(t, e1.JailedFp.Pk.MarshalHex(), e2.JailedFp.Pk.MarshalHex())
		}
	})
}

func TestMissedBlockCounterGoesNegativeWithBitmapResetNew(t *testing.T) {
	// Setup app and context
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	app := babylonApp.Setup(t, false)
	ctx := app.BaseApp.NewContext(false)

	// Set initial block height
	initHeader := ctx.HeaderInfo()
	initHeader.Height = int64(100)
	ctx = ctx.WithHeaderInfo(initHeader)

	btcStkK, finalityK := app.BTCStakingKeeper, app.FinalityKeeper
	msgSrvrBtcStk := btcstakingkeeper.NewMsgServerImpl(btcStkK)

	signingContext := signingcontext.FpPopContextV0(ctx.ChainID(), app.BTCStakingKeeper.ModuleAddress())

	// Create finality provider
	fpBtcSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	fpMsg, err := datagen.GenRandomCreateFinalityProviderMsgWithBTCBabylonSKs(r, fpBtcSK, signingContext, datagen.GenRandomAddress())
	require.NoError(t, err)
	_, err = msgSrvrBtcStk.CreateFinalityProvider(ctx, fpMsg)
	require.NoError(t, err)
	fpPk := fpMsg.BtcPk

	// 1. Create signing info for our FP
	signingInfo := types.NewFinalityProviderSigningInfo(
		fpPk,
		ctx.HeaderInfo().Height,
		0, // Initially 0 missed blocks
	)
	err = finalityK.FinalityProviderSigningTracker.Set(ctx, fpPk.MustMarshal(), signingInfo)
	require.NoError(t, err)

	// 2. Simulate FP missing a block using UpdateSigningInfo method
	missedHeight := ctx.HeaderInfo().Height + 3
	modified, signInfoPtr, err := finalityK.UpdateSigningInfo(ctx, fpPk, true, missedHeight)
	require.NoError(t, err)
	require.True(t, modified, "SigningInfo should be modified")

	// 3. Save the updated signing info
	err = finalityK.FinalityProviderSigningTracker.Set(ctx, fpPk.MustMarshal(), *signInfoPtr)
	require.NoError(t, err)

	// 4. Verify FP has 1 missed block
	signingInfo, err = finalityK.FinalityProviderSigningTracker.Get(ctx, fpPk.MustMarshal())
	require.NoError(t, err)
	require.Equal(t, int64(1), signingInfo.MissedBlocksCounter, "Missed blocks counter should be 1")

	// Calculate the index of the missed block
	index := (missedHeight - signingInfo.StartHeight) % finalityK.GetParams(ctx).SignedBlocksWindow
	hasMissed, err := finalityK.GetMissedBlockBitmapValue(ctx, fpPk, index)
	require.NoError(t, err)
	require.True(t, hasMissed, "Block should be marked as missed")

	// 5. Simulate FP becoming inactive and then active again using actual HandleActivatedFinalityProvider
	err = finalityK.HandleActivatedFinalityProvider(ctx, fpPk)
	require.NoError(t, err)

	// 6. Verify counter is not reset
	signingInfo, err = finalityK.FinalityProviderSigningTracker.Get(ctx, fpPk.MustMarshal())
	require.NoError(t, err)
	require.Equal(t, int64(1), signingInfo.MissedBlocksCounter, "Counter should be reset to 0")

	// 7. Now use updateSigningInfo to simulate FP signing the same index as previously missed block
	modified, signInfoPtr, err = finalityK.UpdateSigningInfo(ctx, fpPk, false, missedHeight+finalityK.GetParams(ctx).SignedBlocksWindow)
	require.NoError(t, err)
	require.True(t, modified, "SigningInfo should be modified")

	// 8. Verify counter is not reduced to zero
	t.Logf("Final MissedBlocksCounter value: %d", signInfoPtr.MissedBlocksCounter)
	require.Equal(t, int64(0), signInfoPtr.MissedBlocksCounter,
		"Missed blocks counter is not reduced to 0 after a successful vote")
}
