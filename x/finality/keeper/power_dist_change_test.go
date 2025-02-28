package keeper_test

import (
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	babylonApp "github.com/babylonlabs-io/babylon/app"
	testutil "github.com/babylonlabs-io/babylon/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/types"
	btcctypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	btclckeeper "github.com/babylonlabs-io/babylon/x/btclightclient/keeper"
	btclctypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	btcstakingkeeper "github.com/babylonlabs-io/babylon/x/btcstaking/keeper"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
	finalitykeeper "github.com/babylonlabs-io/babylon/x/finality/keeper"
	ftypes "github.com/babylonlabs-io/babylon/x/finality/types"
)

func FuzzProcessAllPowerDistUpdateEvents_Determinism(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		// set all parameters
		h.GenAndApplyParams(r)

		// generate and insert a number of new finality providers
		fpPKs := []*btcec.PublicKey{}
		for i := 0; i < 5; i++ {
			_, fpPK, _ := h.CreateFinalityProvider(r)
			fpPKs = append(fpPKs, fpPK)
		}

		// empty dist cache
		dc := ftypes.NewVotingPowerDistCache()

		stakingValue := int64(2 * 10e8)

		// generate many new BTC delegations under each finality provider, and their corresponding events
		events := []*types.EventPowerDistUpdate{}
		for _, fpPK := range fpPKs {
			for i := 0; i < 5; i++ {
				delSK, _, err := datagen.GenRandomBTCKeyPair(r)
				h.NoError(err)
				_, _, del, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
					r,
					delSK,
					fpPK,
					stakingValue,
					1000,
					0,
					0,
					false,
					false,
					10,
					30,
				)
				h.NoError(err)
				event := types.NewEventPowerDistUpdateWithBTCDel(&types.EventBTCDelegationStateUpdate{
					StakingTxHash: del.MustGetStakingTxHash().String(),
					NewState:      types.BTCDelegationStatus_ACTIVE,
				})
				events = append(events, event)
			}
		}

		newDc := h.FinalityKeeper.ProcessAllPowerDistUpdateEvents(h.Ctx, dc, events)
		for i := 0; i < 10; i++ {
			newDc2 := h.FinalityKeeper.ProcessAllPowerDistUpdateEvents(h.Ctx, dc, events)
			require.Equal(t, newDc, newDc2)
		}
	})
}

func CreateFpAndBtcDel(
	t *testing.T,
	r *rand.Rand,
) (
	h *testutil.Helper,
	del *types.BTCDelegation,
	covenantSKs []*secp256k1.PrivateKey,
) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mock BTC light client and BTC checkpoint modules
	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
	h = testutil.NewHelper(t, btclcKeeper, btccKeeper)

	// set all parameters
	covenantSKs, _ = h.GenAndApplyParams(r)

	_, fpPK, _ := h.CreateFinalityProvider(r)

	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	h.NoError(err)
	_, _, del, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK,
		fpPK,
		int64(2*10e8),
		1000,
		0,
		0,
		false,
		false,
		10,
		30,
	)
	h.NoError(err)
	return h, del, covenantSKs
}

func FuzzProcessAllPowerDistUpdateEvents_ActiveAndUnbondTogether(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		h, del, _ := CreateFpAndBtcDel(t, r)

		eventActive := types.NewEventPowerDistUpdateWithBTCDel(&types.EventBTCDelegationStateUpdate{
			StakingTxHash: del.MustGetStakingTxHash().String(),
			NewState:      types.BTCDelegationStatus_ACTIVE,
		})
		eventUnbond := types.NewEventPowerDistUpdateWithBTCDel(&types.EventBTCDelegationStateUpdate{
			StakingTxHash: del.MustGetStakingTxHash().String(),
			NewState:      types.BTCDelegationStatus_UNBONDED,
		})
		events := []*types.EventPowerDistUpdate{eventActive, eventUnbond}

		newDc := h.FinalityKeeper.ProcessAllPowerDistUpdateEvents(h.Ctx, ftypes.NewVotingPowerDistCache(), events)
		require.Len(t, newDc.FinalityProviders, 0)
	})
}

func FuzzProcessAllPowerDistUpdateEvents_ActiveAndSlashTogether(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		h, del, _ := CreateFpAndBtcDel(t, r)

		eventActive := types.NewEventPowerDistUpdateWithBTCDel(&types.EventBTCDelegationStateUpdate{
			StakingTxHash: del.MustGetStakingTxHash().String(),
			NewState:      types.BTCDelegationStatus_ACTIVE,
		})
		eventSlash := types.NewEventPowerDistUpdateWithSlashedFP(&del.FpBtcPkList[0])
		events := []*types.EventPowerDistUpdate{eventActive, eventSlash}

		dc := ftypes.NewVotingPowerDistCache()
		newDc := h.FinalityKeeper.ProcessAllPowerDistUpdateEvents(h.Ctx, dc, events)
		require.Len(t, newDc.FinalityProviders, 0)
	})
}

func FuzzProcessAllPowerDistUpdateEvents_PreApprovalWithSlahedFP(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		h, delNoPreApproval, covenantSKs := CreateFpAndBtcDel(t, r)

		// activates one delegation to the finality provider without preapproval
		eventActive := types.NewEventPowerDistUpdateWithBTCDel(&types.EventBTCDelegationStateUpdate{
			StakingTxHash: delNoPreApproval.MustGetStakingTxHash().String(),
			NewState:      types.BTCDelegationStatus_ACTIVE,
		})

		newDc := h.FinalityKeeper.ProcessAllPowerDistUpdateEvents(h.Ctx, ftypes.NewVotingPowerDistCache(), []*types.EventPowerDistUpdate{eventActive})
		// updates as if that fp is timestamping
		for _, fp := range newDc.FinalityProviders {
			fp.IsTimestamped = true
		}
		// FP is active and has voting power.
		newDc.ApplyActiveFinalityProviders(100)
		require.Len(t, newDc.FinalityProviders, 1)
		require.Equal(t, newDc.TotalVotingPower, delNoPreApproval.TotalSat)

		// simulating a new BTC delegation with preapproval coming
		delSKPreApproval, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)

		stakingTxHash, msgCreateBTCDelPreApproval, delPreApproval, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSKPreApproval,
			delNoPreApproval.FpBtcPkList[0].MustToBTCPK(),
			int64(2*10e8),
			1000,
			0,
			0,
			true,
			false,
			10,
			10,
		)
		h.NoError(err)

		// should not modify the amount of voting power
		newDc.ApplyActiveFinalityProviders(100)
		require.Len(t, newDc.FinalityProviders, 1)
		require.Equal(t, newDc.TotalVotingPower, delPreApproval.TotalSat)

		// slash the fp
		slashEvent := types.NewEventPowerDistUpdateWithSlashedFP(&delPreApproval.FpBtcPkList[0])
		newDc = h.FinalityKeeper.ProcessAllPowerDistUpdateEvents(h.Ctx, newDc, []*types.EventPowerDistUpdate{slashEvent})

		// fp should have be erased from the list
		newDc.ApplyActiveFinalityProviders(100)
		require.Len(t, newDc.FinalityProviders, 0)
		require.Zero(t, newDc.TotalVotingPower)

		// activates the preapproval delegation
		btcTip := btclctypes.BTCHeaderInfo{Height: 30}

		h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDelPreApproval, delPreApproval, btcTip.Height)
		h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof, btcTip.Height)

		activatedDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		h.Equal(activatedDel.TotalSat, uint64(msgCreateBTCDelPreApproval.StakingValue))

		// simulates the del tx getting activated
		eventActive = types.NewEventPowerDistUpdateWithBTCDel(&types.EventBTCDelegationStateUpdate{
			StakingTxHash: delPreApproval.MustGetStakingTxHash().String(),
			NewState:      types.BTCDelegationStatus_ACTIVE,
		})
		// it will get included in the new vp dist, but will not have voting power after ApplyActiveFinalityProviders
		newDc = h.FinalityKeeper.ProcessAllPowerDistUpdateEvents(h.Ctx, newDc, []*types.EventPowerDistUpdate{eventActive})
		require.Len(t, newDc.FinalityProviders, 1)

		for _, fp := range newDc.FinalityProviders {
			fp.IsTimestamped = true
			fp.IsSlashed = true
		}
		newDc.ApplyActiveFinalityProviders(100)
		require.Equal(t, newDc.TotalVotingPower, uint64(0))
		require.Equal(t, newDc.NumActiveFps, uint32(0))
	})
}

func FuzzProcessAllPowerDistUpdateEvents_ActiveAndJailTogether(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		h, del, _ := CreateFpAndBtcDel(t, r)

		eventActive := types.NewEventPowerDistUpdateWithBTCDel(&types.EventBTCDelegationStateUpdate{
			StakingTxHash: del.MustGetStakingTxHash().String(),
			NewState:      types.BTCDelegationStatus_ACTIVE,
		})
		eventJailed := types.NewEventPowerDistUpdateWithJailedFP(&del.FpBtcPkList[0])
		events := []*types.EventPowerDistUpdate{eventActive, eventJailed}

		newDc := h.FinalityKeeper.ProcessAllPowerDistUpdateEvents(h.Ctx, ftypes.NewVotingPowerDistCache(), events)
		for _, fp := range newDc.FinalityProviders {
			fp.IsTimestamped = true
		}
		newDc.ApplyActiveFinalityProviders(100)
		require.Len(t, newDc.FinalityProviders, 1)
		require.Zero(t, newDc.TotalVotingPower)
	})
}

func FuzzProcessAllPowerDistUpdateEvents_SlashActiveFp(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))
		h, del, _ := CreateFpAndBtcDel(t, r)

		eventActive := types.NewEventPowerDistUpdateWithBTCDel(&types.EventBTCDelegationStateUpdate{
			StakingTxHash: del.MustGetStakingTxHash().String(),
			NewState:      types.BTCDelegationStatus_ACTIVE,
		})
		events := []*types.EventPowerDistUpdate{eventActive}

		newDc := h.FinalityKeeper.ProcessAllPowerDistUpdateEvents(h.Ctx, ftypes.NewVotingPowerDistCache(), events)
		for _, fp := range newDc.FinalityProviders {
			fp.IsTimestamped = true
		}
		newDc.ApplyActiveFinalityProviders(100)
		require.Equal(t, newDc.TotalVotingPower, del.TotalSat)

		// afer the fp has some active voting power slash it
		eventSlash := types.NewEventPowerDistUpdateWithSlashedFP(&del.FpBtcPkList[0])
		events = []*types.EventPowerDistUpdate{eventSlash}

		newDc = h.FinalityKeeper.ProcessAllPowerDistUpdateEvents(h.Ctx, newDc, events)
		newDc.ApplyActiveFinalityProviders(100)
		require.Len(t, newDc.FinalityProviders, 0)
		require.Equal(t, newDc.TotalVotingPower, uint64(0))
	})
}

func FuzzSlashFinalityProviderEvent(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)

		// generate and insert new finality provider
		fpSK, fpPK, fp := h.CreateFinalityProvider(r)
		h.CommitPubRandList(r, fpSK, fp, 1, 100, true)

		/*
			insert new BTC delegation and give it covenant quorum
			ensure that it has voting power
		*/
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTxHash, msgCreateBTCDel, actualDel, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			fpPK,
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
		// give it a quorum number of covenant signatures
		h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel, 10)
		// activate the BTC delegation
		h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof, 30)

		// execute BeginBlock
		btcTip := &btclctypes.BTCHeaderInfo{Height: 30}
		babylonHeight := datagen.RandomInt(r, 10) + 1
		h.SetCtxHeight(babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(btcTip).AnyTimes()
		h.BeginBlocker()
		// ensure the finality provider has voting power at this height
		require.Equal(t, uint64(stakingValue), h.FinalityKeeper.GetVotingPower(h.Ctx, *fp.BtcPk, babylonHeight))

		/*
			Slash the finality provider and execute BeginBlock
			Then, ensure the finality provider does not have voting power anymore
		*/
		err = h.BTCStakingKeeper.SlashFinalityProvider(h.Ctx, fp.BtcPk.MustMarshal())
		h.NoError(err)

		err = h.BTCStakingKeeper.SlashFinalityProvider(h.Ctx, fp.BtcPk.MustMarshal())
		require.ErrorIs(t, err, types.ErrFpAlreadySlashed)

		err = h.BTCStakingKeeper.JailFinalityProvider(h.Ctx, fp.BtcPk.MustMarshal())
		require.ErrorIs(t, err, types.ErrFpAlreadySlashed)

		// at this point, there should be only 1 event that the finality provider is slashed
		btcTipHeight := btclcKeeper.GetTipInfo(h.Ctx).Height
		events := h.BTCStakingKeeper.GetAllPowerDistUpdateEvents(h.Ctx, btcTipHeight, btcTipHeight)
		require.Len(t, events, 1)
		slashedFPEvent := events[0].GetSlashedFp()
		require.NotNil(t, slashedFPEvent)
		require.Equal(t, fp.BtcPk.MustMarshal(), slashedFPEvent.Pk.MustMarshal())

		// execute BeginBlock
		babylonHeight += 1
		h.SetCtxHeight(babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(btcTip).AnyTimes()
		h.BeginBlocker()
		// ensure the finality provider does not have voting power anymore
		require.Zero(t, h.FinalityKeeper.GetVotingPower(h.Ctx, *fp.BtcPk, babylonHeight))
	})
}

func FuzzJailFinalityProviderEvents(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)

		// generate and insert new finality provider
		fpSK, fpPK, fp := h.CreateFinalityProvider(r)
		h.CommitPubRandList(r, fpSK, fp, 1, 100, true)

		/*
			insert new BTC delegation and give it covenant quorum
			ensure that it has voting power
		*/
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTxHash, msgCreateBTCDel, actualDel, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			fpPK,
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
		// give it a quorum number of covenant signatures
		h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel, 10)
		// activate the BTC delegation
		h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof, 30)

		// execute BeginBlock
		btcTip := &btclctypes.BTCHeaderInfo{Height: 30}
		babylonHeight := datagen.RandomInt(r, 10) + 1
		h.SetCtxHeight(babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(btcTip)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(btcTip)
		h.BeginBlocker()
		// ensure the finality provider is not jailed and has voting power at this height

		fpBeforeJailing, err := h.BTCStakingKeeper.GetFinalityProvider(h.Ctx, fp.BtcPk.MustMarshal())
		h.NoError(err)
		require.False(t, fpBeforeJailing.IsJailed())
		require.Equal(t, uint64(stakingValue), h.FinalityKeeper.GetVotingPower(h.Ctx, *fp.BtcPk, babylonHeight))

		/*
			Jail the finality provider and execute BeginBlock
			Then, ensure the finality provider does not have voting power anymore
		*/
		err = h.BTCStakingKeeper.JailFinalityProvider(h.Ctx, fp.BtcPk.MustMarshal())
		h.NoError(err)

		err = h.BTCStakingKeeper.JailFinalityProvider(h.Ctx, fp.BtcPk.MustMarshal())
		require.ErrorIs(t, err, types.ErrFpAlreadyJailed)

		// ensure the jailed label is set
		fpAfterJailing, err := h.BTCStakingKeeper.GetFinalityProvider(h.Ctx, fp.BtcPk.MustMarshal())
		h.NoError(err)
		require.True(t, fpAfterJailing.IsJailed())

		// at this point, there should be only 1 event that the finality provider is jailed
		btcTipHeight := uint32(30)
		events := h.BTCStakingKeeper.GetAllPowerDistUpdateEvents(h.Ctx, btcTipHeight, btcTipHeight)
		require.Len(t, events, 1)
		jailedFPEvent := events[0].GetJailedFp()
		require.NotNil(t, jailedFPEvent)
		require.Equal(t, fp.BtcPk.MustMarshal(), jailedFPEvent.Pk.MustMarshal())

		// execute BeginBlock
		babylonHeight += 1
		h.SetCtxHeight(babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(btcTip)
		h.BeginBlocker()
		// ensure the finality provider does not have voting power anymore
		require.Zero(t, h.FinalityKeeper.GetVotingPower(h.Ctx, *fp.BtcPk, babylonHeight))

		/*
				insert another active BTC delegation and check whether the jailed
			    fp has voting power
		*/
		stakingValue = int64(2 * 10e8)
		h.NoError(err)
		delSK2, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTxHash2, msgCreateBTCDel, actualDel, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK2,
			fpPK,
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
		// give it a quorum number of covenant signatures
		h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel, 10)
		// activate the BTC delegation
		h.AddInclusionProof(stakingTxHash2, btcHeaderInfo, inclusionProof, 30)

		// execute BeginBlock
		btcTip = &btclctypes.BTCHeaderInfo{Height: 30}
		babylonHeight += 1
		h.SetCtxHeight(babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(btcTip).AnyTimes()
		h.BeginBlocker()
		// ensure the finality provider is not jailed and has voting power at this height

		fpAfterJailing, err = h.BTCStakingKeeper.GetFinalityProvider(h.Ctx, fp.BtcPk.MustMarshal())
		h.NoError(err)
		require.True(t, fpAfterJailing.IsJailed())
		require.Equal(t, uint64(0), h.FinalityKeeper.GetVotingPower(h.Ctx, *fp.BtcPk, babylonHeight))
	})
}

func FuzzUnjailFinalityProviderEvents(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)

		// generate and insert new finality provider
		fpSK, fpPK, fp := h.CreateFinalityProvider(r)
		h.CommitPubRandList(r, fpSK, fp, 1, 100, true)

		/*
			insert new BTC delegation and give it covenant quorum
			ensure that it has voting power
		*/
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTxHash, msgCreateBTCDel, actualDel, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			fpPK,
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
		// give it a quorum number of covenant signatures
		h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel, 10)
		// activate the BTC delegation
		h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof, 30)

		// execute BeginBlock
		btcTip := &btclctypes.BTCHeaderInfo{Height: 30}
		babylonHeight := datagen.RandomInt(r, 10) + 1
		h.SetCtxHeight(babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(btcTip).AnyTimes()
		h.BeginBlocker()

		// ensure the finality provider is not jailed and has voting power
		fpBeforeJailing, err := h.BTCStakingKeeper.GetFinalityProvider(h.Ctx, fp.BtcPk.MustMarshal())
		h.NoError(err)
		require.False(t, fpBeforeJailing.IsJailed())
		require.Equal(t, uint64(stakingValue), h.FinalityKeeper.GetVotingPower(h.Ctx, *fp.BtcPk, babylonHeight))

		// try unjail fp that is not jailed, should expect error
		err = h.BTCStakingKeeper.UnjailFinalityProvider(h.Ctx, fp.BtcPk.MustMarshal())
		require.ErrorIs(t, err, types.ErrFpNotJailed)

		/*
			Jail the finality provider and execute BeginBlock
			Then, ensure the finality provider does not have voting power anymore
		*/
		err = h.BTCStakingKeeper.JailFinalityProvider(h.Ctx, fp.BtcPk.MustMarshal())
		h.NoError(err)

		// ensure the jailed label is set
		fpAfterJailing, err := h.BTCStakingKeeper.GetFinalityProvider(h.Ctx, fp.BtcPk.MustMarshal())
		h.NoError(err)
		require.True(t, fpAfterJailing.IsJailed())

		// execute BeginBlock
		babylonHeight += 1
		h.SetCtxHeight(babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(btcTip).AnyTimes()
		h.BeginBlocker()
		// ensure the finality provider does not have voting power anymore
		require.Zero(t, h.FinalityKeeper.GetVotingPower(h.Ctx, *fp.BtcPk, babylonHeight))

		/*
			Unjail the finality provider and execute BeginBlock
			Ensure that the finality provider regains voting power
		*/
		err = h.BTCStakingKeeper.UnjailFinalityProvider(h.Ctx, fp.BtcPk.MustMarshal())
		h.NoError(err)

		// ensure the jailed label is reverted
		fpAfterUnjailing, err := h.BTCStakingKeeper.GetFinalityProvider(h.Ctx, fp.BtcPk.MustMarshal())
		h.NoError(err)
		require.False(t, fpAfterUnjailing.IsJailed())

		// execute BeginBlock
		babylonHeight += 1
		h.SetCtxHeight(babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(btcTip).AnyTimes()
		h.BeginBlocker()
		// ensure the finality provider does not have voting power anymore
		require.Equal(t, uint64(stakingValue), h.FinalityKeeper.GetVotingPower(h.Ctx, *fp.BtcPk, babylonHeight))
	})
}

func FuzzBTCDelegationEvents_NoPreApproval(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)

		// generate and insert new finality provider
		fpSK, fpPK, fp := h.CreateFinalityProvider(r)

		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)

		stakingParams := h.BTCStakingKeeper.GetParamsWithVersion(h.Ctx).Params
		stakingTxHash, msgCreateBTCDel, actualDel, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			fpPK,
			stakingValue,
			1000,
			0,
			0,
			false,
			false,
			10,
			30,
		)
		h.NoError(err)

		/*
			at this point, there should be 1 event that BTC delegation
			will become expired at end height - w
		*/
		// there exists no event at the current BTC tip
		btcTip := &btclctypes.BTCHeaderInfo{Height: 30}
		events := h.BTCStakingKeeper.GetAllPowerDistUpdateEvents(h.Ctx, btcTip.Height, btcTip.Height)
		require.Len(t, events, 0)

		// the BTC delegation will be expired at end height - unbonding_time
		unbondedHeight := actualDel.EndHeight - stakingParams.UnbondingTimeBlocks
		events = h.BTCStakingKeeper.GetAllPowerDistUpdateEvents(h.Ctx, unbondedHeight, unbondedHeight)
		require.Len(t, events, 1)
		btcDelStateUpdate := events[0].GetBtcDelStateUpdate()
		require.NotNil(t, btcDelStateUpdate)
		require.Equal(t, stakingTxHash, btcDelStateUpdate.StakingTxHash)
		require.Equal(t, types.BTCDelegationStatus_EXPIRED, btcDelStateUpdate.NewState)

		// ensure this finality provider does not have voting power at the current height
		babylonHeight := datagen.RandomInt(r, 10) + 1
		h.SetCtxHeight(babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(btcTip).Times(2)
		h.BeginBlocker()
		require.Zero(t, h.FinalityKeeper.GetVotingPower(h.Ctx, *fp.BtcPk, babylonHeight))

		/*
			Generate a quorum number of covenant signatures
			Then, there should be an event that the BTC delegation becomes
			active at the current height
		*/
		h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel, btcTip.Height)

		events = h.BTCStakingKeeper.GetAllPowerDistUpdateEvents(h.Ctx, btcTip.Height, btcTip.Height)
		require.Len(t, events, 1)
		btcDelStateUpdate = events[0].GetBtcDelStateUpdate()
		require.NotNil(t, btcDelStateUpdate)
		require.Equal(t, stakingTxHash, btcDelStateUpdate.StakingTxHash)
		require.Equal(t, types.BTCDelegationStatus_ACTIVE, btcDelStateUpdate.NewState)

		// ensure this finality provider does not have voting power at the current height
		// due to no timestamped randomness
		babylonHeight += 1
		h.SetCtxHeight(babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(btcTip)
		h.BeginBlocker()
		require.Zero(t, h.FinalityKeeper.GetVotingPower(h.Ctx, *fp.BtcPk, babylonHeight))

		// ensure this finality provider has voting power at the current height after having timestamped pub rand
		babylonHeight += 1
		h.SetCtxHeight(babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(btcTip)
		h.CommitPubRandList(r, fpSK, fp, 1, 100, true)
		h.BeginBlocker()
		require.Equal(t, uint64(stakingValue), h.FinalityKeeper.GetVotingPower(h.Ctx, *fp.BtcPk, babylonHeight))

		// ensure event queue is cleared at BTC tip height
		events = h.BTCStakingKeeper.GetAllPowerDistUpdateEvents(h.Ctx, btcTip.Height, btcTip.Height)
		require.Len(t, events, 0)

		/*
			BTC height reaches end height - w, such that the BTC delegation becomes expired
			ensure the finality provider does not have voting power anymore
		*/
		babylonHeight += 1
		h.SetCtxHeight(babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: unbondedHeight}).AnyTimes()
		h.BeginBlocker()
		require.Zero(t, h.FinalityKeeper.GetVotingPower(h.Ctx, *fp.BtcPk, babylonHeight))

		// ensure the unbonded event is processed and cleared
		events = h.BTCStakingKeeper.GetAllPowerDistUpdateEvents(h.Ctx, unbondedHeight, unbondedHeight)
		require.Len(t, events, 0)
	})
}

func FuzzBTCDelegationEvents_WithPreApproval(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)

		// generate and insert new finality provider
		fpSK, fpPK, fp := h.CreateFinalityProvider(r)

		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTxHash, msgCreateBTCDel, actualDel, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			fpPK,
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

		btcTip := btclctypes.BTCHeaderInfo{Height: 30} // TODO: parameterise

		// ensure this finality provider does not have voting power at the current height
		babylonHeight := datagen.RandomInt(r, 10) + 1
		h.SetCtxHeight(babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btcTip)
		h.BeginBlocker()
		require.Zero(t, h.FinalityKeeper.GetVotingPower(h.Ctx, *fp.BtcPk, babylonHeight))

		/*
			Generate a quorum number of covenant signatures
		*/
		h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel, btcTip.Height)
		// no event will be emitted to the event bus upon an verified BTC delegation
		// since it does not affect voting power distribution
		events := h.BTCStakingKeeper.GetAllPowerDistUpdateEvents(h.Ctx, btcTip.Height, btcTip.Height)
		require.Len(t, events, 0)

		// ensure this finality provider does not have voting power at the current height
		babylonHeight += 1
		h.SetCtxHeight(babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btcTip)
		h.BeginBlocker()
		require.Zero(t, h.FinalityKeeper.GetVotingPower(h.Ctx, *fp.BtcPk, babylonHeight))

		/*
			submit the inclusion proof to activate the BTC delegation
			at this point, there should be
			- 1 event that BTC delegation becomes active at the current height
			- 1 event that BTC delegation will become expired at end height - w
		*/
		h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof, btcTip.Height)
		activatedDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		// there exists 1 event that the BTC delegation becomes active
		events = h.BTCStakingKeeper.GetAllPowerDistUpdateEvents(h.Ctx, btcTip.Height, btcTip.Height)
		require.Len(t, events, 1)
		btcDelStateUpdate := events[0].GetBtcDelStateUpdate()
		require.NotNil(t, btcDelStateUpdate)
		require.Equal(t, stakingTxHash, btcDelStateUpdate.StakingTxHash)
		require.Equal(t, types.BTCDelegationStatus_ACTIVE, btcDelStateUpdate.NewState)

		// the BTC delegation will be unbonded at end height - unbonding_time
		unbondedHeight := activatedDel.EndHeight - h.BTCStakingKeeper.GetParams(h.Ctx).UnbondingTimeBlocks
		events = h.BTCStakingKeeper.GetAllPowerDistUpdateEvents(h.Ctx, unbondedHeight, unbondedHeight)
		require.Len(t, events, 1)
		btcDelStateUpdate = events[0].GetBtcDelStateUpdate()
		require.NotNil(t, btcDelStateUpdate)
		require.Equal(t, stakingTxHash, btcDelStateUpdate.StakingTxHash)
		require.Equal(t, types.BTCDelegationStatus_EXPIRED, btcDelStateUpdate.NewState)

		// ensure this finality provider does not have voting power at the current height
		// due to no timestamped randomness
		babylonHeight += 1
		h.SetCtxHeight(babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btcTip)
		h.BeginBlocker()
		require.Zero(t, h.FinalityKeeper.GetVotingPower(h.Ctx, *fp.BtcPk, babylonHeight))

		// ensure this finality provider has voting power at the current height after having timestamped pub rand
		babylonHeight += 1
		h.SetCtxHeight(babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btcTip)
		h.CommitPubRandList(r, fpSK, fp, 1, 100, true)
		h.BeginBlocker()
		require.Equal(t, uint64(stakingValue), h.FinalityKeeper.GetVotingPower(h.Ctx, *fp.BtcPk, babylonHeight))

		// ensure event queue is cleared at BTC tip height
		events = h.BTCStakingKeeper.GetAllPowerDistUpdateEvents(h.Ctx, btcTip.Height, btcTip.Height)
		require.Len(t, events, 0)

		/*
			BTC height reaches end height - w, such that the BTC delegation becomes expired
			ensure the finality provider does not have voting power anymore
		*/
		babylonHeight += 1
		h.SetCtxHeight(babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: unbondedHeight}).AnyTimes()
		h.BeginBlocker()
		require.Zero(t, h.FinalityKeeper.GetVotingPower(h.Ctx, *fp.BtcPk, babylonHeight))

		// ensure the unbonded event is processed and cleared
		events = h.BTCStakingKeeper.GetAllPowerDistUpdateEvents(h.Ctx, unbondedHeight, unbondedHeight)
		require.Len(t, events, 0)
	})
}

func TestDoNotGenerateDuplicateEventsAfterHavingCovenantQuorum(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mock BTC light client and BTC checkpoint modules
	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
	h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

	// set all parameters
	covenantSKs, _ := h.GenAndApplyParams(r)

	// generate and insert new finality provider
	_, fpPK, fp := h.CreateFinalityProvider(r)

	// generate and insert new BTC delegation
	stakingValue := int64(2 * 10e8)
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	h.NoError(err)
	stakingParams := h.BTCStakingKeeper.GetParamsWithVersion(h.Ctx).Params
	expectedStakingTxHash, msgCreateBTCDel, actualDel, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK,
		fpPK,
		stakingValue,
		1000,
		0,
		0,
		false,
		false,
		10,
		30,
	)
	h.NoError(err)
	/*
		at this point, there should be 1 event that BTC delegation
		will become expired at end height - min_unbonding_time
	*/
	// there exists no event at the current BTC tip
	btcTip := &btclctypes.BTCHeaderInfo{Height: 30}
	events := h.BTCStakingKeeper.GetAllPowerDistUpdateEvents(h.Ctx, btcTip.Height, btcTip.Height)
	require.Len(t, events, 0)

	// the BTC delegation will be expired (unbonded) at end height - unbonding_time
	unbondedHeight := actualDel.EndHeight - stakingParams.UnbondingTimeBlocks
	events = h.BTCStakingKeeper.GetAllPowerDistUpdateEvents(h.Ctx, unbondedHeight, unbondedHeight)
	require.Len(t, events, 1)
	btcDelStateUpdate := events[0].GetBtcDelStateUpdate()
	require.NotNil(t, btcDelStateUpdate)
	require.Equal(t, expectedStakingTxHash, btcDelStateUpdate.StakingTxHash)
	require.Equal(t, types.BTCDelegationStatus_EXPIRED, btcDelStateUpdate.NewState)

	// ensure this finality provider does not have voting power at the current height
	babylonHeight := datagen.RandomInt(r, 10) + 1
	h.SetCtxHeight(babylonHeight)
	h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(btcTip).AnyTimes()
	h.BeginBlocker()
	require.Zero(t, h.FinalityKeeper.GetVotingPower(h.Ctx, *fp.BtcPk, babylonHeight))

	msgs := h.GenerateCovenantSignaturesMessages(r, covenantSKs, msgCreateBTCDel, actualDel)

	// Generate and report covenant signatures from all covenant members.
	for _, m := range msgs {
		mCopy := m
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(btcTip).AnyTimes()
		_, err = h.MsgServer.AddCovenantSigs(h.Ctx, mCopy)
		h.NoError(err)
	}

	// event though all covenant signatures are reported, only one event should be generated
	events = h.BTCStakingKeeper.GetAllPowerDistUpdateEvents(h.Ctx, btcTip.Height, btcTip.Height)
	// we should only have one event that the BTC delegation becomes active
	require.Len(t, events, 1)
	btcDelStateUpdate = events[0].GetBtcDelStateUpdate()
	require.NotNil(t, btcDelStateUpdate)
	require.Equal(t, expectedStakingTxHash, btcDelStateUpdate.StakingTxHash)
	require.Equal(t, types.BTCDelegationStatus_ACTIVE, btcDelStateUpdate.NewState)
}

// TestHandleLivenessPanic is to test whether the handle liveness will panic
// in the case where an fp becomes active -> non-active -> active quickly
func TestHandleLivenessPanic(t *testing.T) {
	// Initial setup
	r := rand.New(rand.NewSource(12312312312))
	app := babylonApp.Setup(t, false)
	ctx := app.BaseApp.NewContext(false)

	defaultStakingKeeper := app.StakingKeeper
	btcStakingKeeper := app.BTCStakingKeeper
	btcStakingMsgServer := btcstakingkeeper.NewMsgServerImpl(btcStakingKeeper)
	btcLcKeeper := app.BTCLightClientKeeper
	btcLcMsgServer := btclckeeper.NewMsgServerImpl(btcLcKeeper)

	btcCcKeeper := app.BtcCheckpointKeeper
	epochingKeeper := app.EpochingKeeper
	checkpointingKeeper := app.CheckpointingKeeper

	finalityKeeper := app.FinalityKeeper
	finalityMsgServer := finalitykeeper.NewMsgServerImpl(finalityKeeper)
	finalityParams := ftypes.DefaultParams()
	finalityParams.MaxActiveFinalityProviders = 5
	_ = finalityKeeper.SetParams(ctx, finalityParams)

	epochingKeeper.InitEpoch(ctx)
	initHeader := ctx.HeaderInfo()
	initHeader.Height = int64(1)
	ctx = ctx.WithHeaderInfo(initHeader)

	// Generate Covenant related keys
	covenantSKs, covenantPKs, err := datagen.GenRandomBTCKeyPairs(r, 1)
	require.NoError(t, err)
	slashingAddress, err := datagen.GenRandomBTCAddress(r, &chaincfg.SimNetParams)
	require.NoError(t, err)
	slashingPkScript, err := txscript.PayToAddrScript(slashingAddress)
	require.NoError(t, err)

	CcParams := btcCcKeeper.GetParams(ctx)
	CcParams.BtcConfirmationDepth = 1 // for simulation
	err = btcCcKeeper.SetParams(ctx, CcParams)
	require.NoError(t, err)

	// 0. BTCStakingKeeper parameter setting
	err = btcStakingKeeper.SetParams(ctx, types.Params{
		CovenantPks:               bbn.NewBIP340PKsFromBTCPKs(covenantPKs),
		CovenantQuorum:            1,
		MinStakingValueSat:        10000,
		MaxStakingValueSat:        int64(4000 * 10e8),
		MinStakingTimeBlocks:      400,
		MaxStakingTimeBlocks:      10000,
		SlashingPkScript:          slashingPkScript,
		MinSlashingTxFeeSat:       100,
		MinCommissionRate:         sdkmath.LegacyMustNewDecFromStr("0.01"),
		SlashingRate:              sdkmath.LegacyNewDecWithPrec(int64(datagen.RandomInt(r, 41)+10), 2),
		UnbondingTimeBlocks:       100,
		UnbondingFeeSat:           1000,
		AllowListExpirationHeight: 0,
		BtcActivationHeight:       1,
	})
	require.NoError(t, err)

	valset, err := defaultStakingKeeper.GetAllValidators(ctx)
	require.NoError(t, err)
	t.Logf("[+] initial validator set length : %d\n", len(valset))

	header := ctx.HeaderInfo()
	maximumSimulateBlocks := 5

	// Epoch and checkpoint setting
	t.Logf("Current Epoch Number : %d\n", epochingKeeper.GetEpoch(ctx).GetEpochNumber())
	checkpointingKeeper.SetLastFinalizedEpoch(ctx, 1)

	// Among externally created FPs, save the FP where i==5
	var targetFp *types.FinalityProvider
	var targetFpSK *btcec.PrivateKey

	fpNum := 6
	for i := 0; i < fpNum; i++ {
		// Create FP externally and pass it when called
		fpSK, _, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		fp, err := datagen.GenRandomFinalityProviderWithBTCSK(r, fpSK)
		require.NoError(t, err)
		// Save when i is 5
		if i == 1 {
			targetFp = fp
			targetFpSK = fpSK
		}
		createDelegationWithFinalityProvider(
			t, ctx, r, i,
			fp, fpSK, // Pass already created FP info
			btcStakingMsgServer, btcLcMsgServer, finalityMsgServer,
			btcStakingKeeper, btcLcKeeper,
			covenantSKs, covenantPKs, false,
		)
	}

	// Block simulation
	for i := 0; i < maximumSimulateBlocks; i++ {
		ctx = ctx.WithHeaderInfo(header)
		ctx = ctx.WithBlockHeight(header.Height)

		t.Logf("-------- BeginBlock : %d ---------\n", header.Height)
		_, err := app.BeginBlocker(ctx)
		require.NoError(t, err)

		dc := finalityKeeper.GetVotingPowerDistCache(ctx, uint64(header.Height))
		activeFps := dc.GetActiveFinalityProviderSet()
		var fpsList []*ftypes.FinalityProviderDistInfo
		for _, v := range activeFps {
			fpsList = append(fpsList, v)
		}
		ftypes.SortFinalityProvidersWithZeroedVotingPower(fpsList)

		t.Logf("block height : %d, activeFps length : %d\n", ctx.HeaderInfo().Height, len(fpsList))
		for fpIndex, fp := range fpsList {
			t.Logf("fpIndex : %d, active fp address : %v, voting power : %d\n",
				fpIndex, fp.BtcPk.MarshalHex(), fp.TotalBondedSat)
		}

		// Example: At block height 3, create additional delegation using FP (targetFp) created at i==5
		if i == 2 {
			// targetFp and targetFpSK must be non-nil
			// Create FP externally and pass it when called
			newfpSK, _, err := datagen.GenRandomBTCKeyPair(r)
			require.NoError(t, err)
			newfp, err := datagen.GenRandomFinalityProviderWithBTCSK(r, newfpSK)
			require.NoError(t, err)

			createDelegationWithFinalityProvider(
				t, ctx, r, fpNum,
				newfp, newfpSK, // Use i==5 FP info
				btcStakingMsgServer, btcLcMsgServer, finalityMsgServer,
				btcStakingKeeper, btcLcKeeper,
				covenantSKs, covenantPKs, false,
			)
		}

		if i == 3 {
			// targetFp and targetFpSK must be non-nil
			require.NotNil(t, targetFp)
			require.NotNil(t, targetFpSK)
			createDelegationWithFinalityProvider(
				t, ctx, r, 5,
				targetFp, targetFpSK, // Use i==5 FP info
				btcStakingMsgServer, btcLcMsgServer, finalityMsgServer,
				btcStakingKeeper, btcLcKeeper,
				covenantSKs, covenantPKs, true,
			)
		}

		_, err = app.EndBlocker(ctx)
		t.Logf("-------- EndBlock height : %d---------\n", header.Height)
		require.NoError(t, err)
		header.Height++
	}
}

func createDelegationWithFinalityProvider(
	t *testing.T,
	ctx sdk.Context,
	r *rand.Rand,
	fpIndex int,
	fpInfo *types.FinalityProvider, // Must be non-nil
	fpSK *btcec.PrivateKey, // Must be non-nil
	btcStakingMsgServer types.MsgServer,
	btcLcMsgServer btclctypes.MsgServer,
	finalityMsgServer ftypes.MsgServer, // Use finality related MsgServer type
	btcStakingKeeper btcstakingkeeper.Keeper, // keeper (passed by value)
	btcLcKeeper btclckeeper.Keeper,
	covenantSKs []*btcec.PrivateKey,
	covenantPKs []*btcec.PublicKey,
	createFinalityProviderSkip bool,
) {
	require.NotNil(t, fpInfo, "fpInfo must be provided")
	require.NotNil(t, fpSK, "fpSK must be provided")
	finalityFP := fpInfo
	finalityPriv := fpSK

	// 1. Create and Commit FinalityProvider (call separate function)
	if createFinalityProviderSkip == false {
		createAndCommitFinalityProvider(t, ctx, r, finalityFP, finalityPriv, btcStakingMsgServer, finalityMsgServer)
	}

	// 2. Prepare delegation creation
	bsParams := btcStakingKeeper.GetParams(ctx)
	covPKs, err := bbn.NewBTCPKsFromBIP340PKs(bsParams.CovenantPks)
	require.NoError(t, err)
	stakingValue := int64((fpIndex + 1) * 10e8)
	unbondingTime := bsParams.UnbondingTimeBlocks

	// Generate delegator keys and create Staking info
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	stakingTime := 1000

	testStakingInfo := datagen.GenBTCStakingSlashingInfo(
		r, t, &chaincfg.SimNetParams,
		delSK, []*btcec.PublicKey{finalityFP.BtcPk.MustToBTCPK()},
		covPKs, bsParams.CovenantQuorum,
		uint16(stakingTime), stakingValue,
		bsParams.SlashingPkScript, bsParams.SlashingRate,
		uint16(unbondingTime),
	)

	stakingMsgTx := testStakingInfo.StakingTx
	serializedStakingTx, err := bbn.SerializeBTCTx(stakingMsgTx)
	require.NoError(t, err)

	// Delegator account and PoP creation
	acc := datagen.GenRandomAccount()
	stakerAddr := sdk.MustAccAddressFromBech32(acc.Address)
	pop, err := datagen.NewPoPBTC(stakerAddr, delSK)
	require.NoError(t, err)

	// Tx inclusion proof for Tx
	prevBlockHeader := btcLcKeeper.GetTipInfo(ctx).Header.ToBlockHeader()
	btcHeaderWithProof := datagen.CreateBlockWithTransaction(r, prevBlockHeader, stakingMsgTx)
	btcHeader := btcHeaderWithProof.HeaderBytes

	dummy1Tx := datagen.CreateDummyTx()
	dummy1HeaderWithProof := datagen.CreateBlockWithTransaction(r, btcHeader.ToBlockHeader(), dummy1Tx)
	dummy1Header := dummy1HeaderWithProof.HeaderBytes

	txInclusionProof := types.NewInclusionProof(
		&btcctypes.TransactionKey{Index: 1, Hash: btcHeader.Hash()},
		btcHeaderWithProof.SpvProof.MerkleNodes,
	)
	headers := []bbn.BTCHeaderBytes{btcHeader, dummy1Header}
	insertHeaderMsg := &btclctypes.MsgInsertHeaders{
		Signer:  stakerAddr.String(),
		Headers: headers,
	}
	_, err = btcLcMsgServer.InsertHeaders(ctx, insertHeaderMsg)
	require.NoError(t, err)

	// Delegator signature creation
	slashingPathInfo, err := testStakingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)
	delegatorSig, err := testStakingInfo.SlashingTx.Sign(
		stakingMsgTx, 0, slashingPathInfo.GetPkScriptPath(), delSK,
	)
	require.NoError(t, err)

	// 3. Unbonding related info creation
	stkTxHash := testStakingInfo.StakingTx.TxHash()
	unbondingValue := stakingValue - datagen.UnbondingTxFee
	testUnbondingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r, t, &chaincfg.SimNetParams,
		delSK, []*btcec.PublicKey{finalityFP.BtcPk.MustToBTCPK()},
		covenantPKs, bsParams.CovenantQuorum,
		wire.NewOutPoint(&stkTxHash, datagen.StakingOutIdx),
		uint16(unbondingTime), unbondingValue,
		bsParams.SlashingPkScript, bsParams.SlashingRate,
		uint16(unbondingTime),
	)
	unbondingTx, err := bbn.SerializeBTCTx(testUnbondingInfo.UnbondingTx)
	require.NoError(t, err)
	delUnbondingSlashingSig, err := testUnbondingInfo.GenDelSlashingTxSig(delSK)
	require.NoError(t, err)

	// 4. Delegation creation message sending
	msgCreateBTCDel := &types.MsgCreateBTCDelegation{
		StakerAddr:                    stakerAddr.String(),
		FpBtcPkList:                   []bbn.BIP340PubKey{*finalityFP.BtcPk},
		BtcPk:                         bbn.NewBIP340PubKeyFromBTCPK(delSK.PubKey()),
		Pop:                           pop,
		StakingTime:                   uint32(stakingTime),
		StakingValue:                  stakingValue,
		StakingTx:                     serializedStakingTx,
		StakingTxInclusionProof:       txInclusionProof,
		SlashingTx:                    testStakingInfo.SlashingTx,
		DelegatorSlashingSig:          delegatorSig,
		UnbondingTx:                   unbondingTx,
		UnbondingTime:                 unbondingTime,
		UnbondingValue:                unbondingValue,
		UnbondingSlashingTx:           testUnbondingInfo.SlashingTx,
		DelegatorUnbondingSlashingSig: delUnbondingSlashingSig,
	}
	_, err = btcStakingMsgServer.CreateBTCDelegation(ctx, msgCreateBTCDel)
	require.NoError(t, err)

	// 5. Covenant Signature addition
	stakingTxHash := testStakingInfo.StakingTx.TxHash()
	vPKs, err := bbn.NewBTCPKsFromBIP340PKs(msgCreateBTCDel.FpBtcPkList)
	require.NoError(t, err)

	covenantSlashingTxSigs, err := datagen.GenCovenantAdaptorSigs(
		covenantSKs, vPKs,
		testStakingInfo.StakingTx, slashingPathInfo.GetPkScriptPath(),
		msgCreateBTCDel.SlashingTx,
	)
	require.NoError(t, err)

	unbondingSlashingPathInfo, err := testUnbondingInfo.UnbondingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)
	covenantUnbondingSlashingTxSigs, err := datagen.GenCovenantAdaptorSigs(
		covenantSKs, vPKs,
		testUnbondingInfo.UnbondingTx, unbondingSlashingPathInfo.GetPkScriptPath(),
		testUnbondingInfo.SlashingTx,
	)
	require.NoError(t, err)

	unbondingPathInfo, err := testStakingInfo.StakingInfo.UnbondingPathSpendInfo()
	require.NoError(t, err)
	covUnbondingSigs, err := datagen.GenCovenantUnbondingSigs(
		covenantSKs, testStakingInfo.StakingTx,
		0, unbondingPathInfo.GetPkScriptPath(),
		testUnbondingInfo.UnbondingTx,
	)
	require.NoError(t, err)

	msgAddCovenantSig := &types.MsgAddCovenantSigs{
		Signer:                  msgCreateBTCDel.StakerAddr,
		Pk:                      covenantSlashingTxSigs[0].CovPk,
		StakingTxHash:           stakingTxHash.String(),
		SlashingTxSigs:          covenantSlashingTxSigs[0].AdaptorSigs,
		UnbondingTxSig:          bbn.NewBIP340SignatureFromBTCSig(covUnbondingSigs[0]),
		SlashingUnbondingTxSigs: covenantUnbondingSlashingTxSigs[0].AdaptorSigs,
	}
	_, err = btcStakingMsgServer.AddCovenantSigs(ctx, msgAddCovenantSig)
	require.NoError(t, err)
}

func createAndCommitFinalityProvider(
	t *testing.T,
	ctx sdk.Context,
	r *rand.Rand,
	fp *types.FinalityProvider,
	fpSK *btcec.PrivateKey,
	btcStakingMsgServer types.MsgServer,
	finalityMsgServer ftypes.MsgServer,
) {
	fpMsg := &types.MsgCreateFinalityProvider{
		Addr:        fp.Addr,
		Description: fp.Description,
		Commission:  types.NewCommissionRates(*fp.Commission, fp.CommissionInfo.MaxRate, fp.CommissionInfo.MaxChangeRate),
		BtcPk:       fp.BtcPk,
		Pop:         fp.Pop,
	}
	_, err := btcStakingMsgServer.CreateFinalityProvider(ctx, fpMsg)
	require.NoError(t, err)

	_, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(r, fpSK, 1, 300)
	require.NoError(t, err)
	_, err = finalityMsgServer.CommitPubRandList(ctx, msgCommitPubRandList)
	require.NoError(t, err)
}
