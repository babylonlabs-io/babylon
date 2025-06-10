package keeper_test

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	testutil "github.com/babylonlabs-io/babylon/v3/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/v3/x/finality/types"
)

func FuzzVotingPowerTable(f *testing.F) {
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

		// generate a random batch of finality providers, and commit pub rand list with timestamp
		fps := []*types.FinalityProvider{}
		numFpsWithVotingPower := datagen.RandomInt(r, 10) + 2
		numFps := numFpsWithVotingPower + datagen.RandomInt(r, 10)
		for i := uint64(0); i < numFps; i++ {
			fpSK, _, fp := h.CreateFinalityProvider(r)
			h.CommitPubRandList(r, fpSK, fp, 1, 100, true)
			fps = append(fps, fp)
		}

		// for the first numFpsWithVotingPower finality providers, generate a random number of BTC delegations
		numBTCDels := datagen.RandomInt(r, 10) + 1
		stakingValue := datagen.RandomInt(r, 100000) + 100000
		for i := uint64(0); i < numFpsWithVotingPower; i++ {
			for j := uint64(0); j < numBTCDels; j++ {
				delSK, _, err := datagen.GenRandomBTCKeyPair(r)
				h.NoError(err)
				stakingTxHash, delMsg, del, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegationWithBtcBlockHeight(
					r,
					delSK,
					[]*btcec.PublicKey{fps[i].BtcPk.MustToBTCPK()},
					int64(stakingValue),
					1000,
					0,
					0,
					true,
					false,
					10,
					10,
				)
				h.NoError(err)
				h.CreateCovenantSigs(r, covenantSKs, delMsg, del, 10)
				h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof, 30)
			}
		}

		/*
			assert the first numFpsWithVotingPower finality providers have voting power
		*/
		babylonHeight := datagen.RandomInt(r, 10) + 1
		h.SetCtxHeight(babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: 30}).AnyTimes()
		err := h.BTCStakingKeeper.BeginBlocker(h.Ctx)
		require.NoError(t, err)
		err = h.FinalityKeeper.BeginBlocker(h.Ctx)
		require.NoError(t, err)

		for i := uint64(0); i < numFpsWithVotingPower; i++ {
			power := h.FinalityKeeper.GetVotingPower(h.Ctx, *fps[i].BtcPk, babylonHeight)
			require.Equal(t, numBTCDels*stakingValue, power)
		}
		for i := numFpsWithVotingPower; i < numFps; i++ {
			power := h.FinalityKeeper.GetVotingPower(h.Ctx, *fps[i].BtcPk, babylonHeight)
			require.Zero(t, power)
		}

		// also, get voting power table and assert consistency
		powerTable := h.FinalityKeeper.GetVotingPowerTable(h.Ctx, babylonHeight)
		require.NotNil(t, powerTable)
		for i := uint64(0); i < numFpsWithVotingPower; i++ {
			power := h.FinalityKeeper.GetVotingPower(h.Ctx, *fps[i].BtcPk, babylonHeight)
			require.Equal(t, powerTable[fps[i].BtcPk.MarshalHex()], power)
		}
		// the activation height should be the current Babylon height as well
		activatedHeight, err := h.FinalityKeeper.GetBTCStakingActivatedHeight(h.Ctx)
		require.NoError(t, err)
		require.Equal(t, babylonHeight, activatedHeight)

		/*
			slash a random finality provider and move on
			then assert the slashed finality provider does not have voting power
		*/
		// move to next Babylon height
		h.BTCLightClientKeeper = btclcKeeper
		babylonHeight += 1
		h.SetCtxHeight(babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: 30}).AnyTimes()
		// slash a random finality provider
		slashedIdx := datagen.RandomInt(r, int(numFpsWithVotingPower))
		slashedFp := fps[slashedIdx]
		err = h.BTCStakingKeeper.SlashFinalityProvider(h.Ctx, slashedFp.BtcPk.MustMarshal())
		require.NoError(t, err)
		// index height and record power table
		err = h.BTCStakingKeeper.BeginBlocker(h.Ctx)
		require.NoError(t, err)
		err = h.FinalityKeeper.BeginBlocker(h.Ctx)
		require.NoError(t, err)

		// check if the slashed finality provider's voting power becomes zero
		for i := uint64(0); i < numFpsWithVotingPower; i++ {
			power := h.FinalityKeeper.GetVotingPower(h.Ctx, *fps[i].BtcPk, babylonHeight)
			if i == slashedIdx {
				require.Zero(t, power)
			} else {
				require.Equal(t, numBTCDels*stakingValue, power)
			}
		}
		for i := numFpsWithVotingPower; i < numFps; i++ {
			power := h.FinalityKeeper.GetVotingPower(h.Ctx, *fps[i].BtcPk, babylonHeight)
			require.Zero(t, power)
		}

		// also, get voting power table and assert consistency
		powerTable = h.FinalityKeeper.GetVotingPowerTable(h.Ctx, babylonHeight)
		require.NotNil(t, powerTable)
		for i := uint64(0); i < numFpsWithVotingPower; i++ {
			power := h.FinalityKeeper.GetVotingPower(h.Ctx, *fps[i].BtcPk, babylonHeight)
			if i == slashedIdx {
				require.Zero(t, power)
			}
			require.Equal(t, powerTable[fps[i].BtcPk.MarshalHex()], power)
		}

		/*
			move to 999th BTC block, then assert none of finality providers has voting power (since end height - w < BTC height)
		*/
		// replace the old mocked keeper
		babylonHeight += 1
		h.SetCtxHeight(babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: 999}).AnyTimes()
		err = h.BTCStakingKeeper.BeginBlocker(h.Ctx)
		require.NoError(t, err)

		for _, fp := range fps {
			power := h.FinalityKeeper.GetVotingPower(h.Ctx, *fp.BtcPk, babylonHeight)
			require.Zero(t, power)
		}

		// the activation height should be same as before
		activatedHeight2, err := h.FinalityKeeper.GetBTCStakingActivatedHeight(h.Ctx)
		require.NoError(t, err)
		require.Equal(t, activatedHeight, activatedHeight2)
	})
}
func FuzzRecordVotingPowerDistCache(f *testing.F) {
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

		// generate a random batch of finality providers, and commit
		// pub rand list with timestamp
		numFpsWithVotingPower := datagen.RandomInt(r, 10) + 2
		numFps := numFpsWithVotingPower + datagen.RandomInt(r, 10)
		fpsWithVotingPowerMap := map[string]*types.FinalityProvider{}
		for i := uint64(0); i < numFps; i++ {
			fpSK, _, fp := h.CreateFinalityProvider(r)
			h.CommitPubRandList(r, fpSK, fp, 1, 100, true)
			if i < numFpsWithVotingPower {
				// these finality providers will receive BTC delegations and have voting power
				fpsWithVotingPowerMap[fp.Addr] = fp
			}
		}

		// for the first numFpsWithVotingPower finality providers, generate a random number of BTC
		// delegations and add covenant signatures to activate them
		numBTCDels := datagen.RandomInt(r, 10) + 1
		stakingValue := datagen.RandomInt(r, 100000) + 100000
		for _, fp := range fpsWithVotingPowerMap {
			for j := uint64(0); j < numBTCDels; j++ {
				delSK, _, err := datagen.GenRandomBTCKeyPair(r)
				h.NoError(err)
				stakingTxHash, delMsg, del, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegationWithBtcBlockHeight(
					r,
					delSK,
					[]*btcec.PublicKey{fp.BtcPk.MustToBTCPK()},
					int64(stakingValue),
					1000,
					0,
					0,
					true,
					false,
					10,
					10,
				)
				h.NoError(err)
				h.CreateCovenantSigs(r, covenantSKs, delMsg, del, 10)
				h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof, 30)
			}
		}

		// record voting power distribution cache
		babylonHeight := datagen.RandomInt(r, 10) + 1
		h.Ctx = datagen.WithCtxHeight(h.Ctx, babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: 30}).AnyTimes()
		h.BeginBlocker()

		// assert voting power distribution cache is correct
		dc := h.FinalityKeeper.GetVotingPowerDistCache(h.Ctx, babylonHeight)
		require.NotNil(t, dc)
		require.Equal(t, dc.TotalVotingPower, numFpsWithVotingPower*numBTCDels*stakingValue, dc.String())
		activeFPs := dc.GetActiveFinalityProviderSet()
		for _, fpDistInfo := range activeFPs {
			require.Equal(t, fpDistInfo.TotalBondedSat, numBTCDels*stakingValue)
			fp, ok := fpsWithVotingPowerMap[sdk.AccAddress(fpDistInfo.Addr).String()]
			require.True(t, ok)
			require.Equal(t, fpDistInfo.Commission, fp.Commission)
		}
	})
}

func FuzzVotingPowerTable_ActiveFinalityProviders(f *testing.F) {
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

		// generate a random batch of finality providers, each with a BTC delegation with random power
		fpsWithMeta := []*ftypes.FinalityProviderDistInfo{}
		numFps := datagen.RandomInt(r, 300) + 1
		noTimestampedFps := map[string]bool{}
		for i := uint64(0); i < numFps; i++ {
			// generate finality provider
			fpSK, _, fp := h.CreateFinalityProvider(r)

			// delegate to this finality provider
			stakingValue := datagen.RandomInt(r, 100000) + 100000
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			h.NoError(err)
			stakingTxHash, delMsg, del, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegationWithBtcBlockHeight(
				r,
				delSK,
				[]*btcec.PublicKey{fp.BtcPk.MustToBTCPK()},
				int64(stakingValue),
				1000,
				0,
				0,
				true,
				false,
				10,
				10,
			)
			h.NoError(err)
			h.CreateCovenantSigs(r, covenantSKs, delMsg, del, 10)
			h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof, 30)

			// 30 percent not have timestamped randomness, which causes
			// zero voting power in the table
			fpDistInfo := &ftypes.FinalityProviderDistInfo{BtcPk: fp.BtcPk, TotalBondedSat: stakingValue}
			if r.Intn(10) <= 2 {
				h.CommitPubRandList(r, fpSK, fp, 1, 100, false)
				noTimestampedFps[fp.BtcPk.MarshalHex()] = true
				fpDistInfo.IsTimestamped = false
			} else {
				h.CommitPubRandList(r, fpSK, fp, 1, 100, true)
				fpDistInfo.IsTimestamped = true
			}

			// record voting power
			fpsWithMeta = append(fpsWithMeta, fpDistInfo)
		}

		maxActiveFpsParam := h.FinalityKeeper.GetParams(h.Ctx).MaxActiveFinalityProviders
		// get a map of expected active finality providers
		ftypes.SortFinalityProvidersWithZeroedVotingPower(fpsWithMeta)
		expectedActiveFps := fpsWithMeta[:min(uint32(len(fpsWithMeta)-len(noTimestampedFps)), maxActiveFpsParam)]
		expectedActiveFpsMap := map[string]uint64{}
		for _, fp := range expectedActiveFps {
			expectedActiveFpsMap[fp.BtcPk.MarshalHex()] = fp.TotalBondedSat
		}

		// record voting power table
		babylonHeight := datagen.RandomInt(r, 10) + 1
		h.SetCtxHeight(babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: 30}).AnyTimes()
		h.BeginBlocker()

		// only finality providers in expectedActiveFpsMap have voting power
		for _, fp := range fpsWithMeta {
			power := h.FinalityKeeper.GetVotingPower(h.Ctx, fp.BtcPk.MustMarshal(), babylonHeight)
			if expectedPower, ok := expectedActiveFpsMap[fp.BtcPk.MarshalHex()]; ok {
				require.Equal(t, expectedPower, power)
			} else {
				require.Zero(t, power)
			}
		}

		// also, get voting power table and assert there is
		// min(len(expectedActiveFps), MaxActiveFinalityProviders) active finality providers
		powerTable := h.FinalityKeeper.GetVotingPowerTable(h.Ctx, babylonHeight)
		expectedNumActiveFps := len(expectedActiveFpsMap)
		if expectedNumActiveFps > int(maxActiveFpsParam) {
			expectedNumActiveFps = int(maxActiveFpsParam)
		}
		require.Len(t, powerTable, expectedNumActiveFps)
		// assert consistency of voting power
		for pkHex, expectedPower := range expectedActiveFpsMap {
			require.Equal(t, powerTable[pkHex], expectedPower)
		}
	})
}

func FuzzVotingPowerTable_ActiveFinalityProviderRotation(f *testing.F) {
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
		// set random number of max number of finality providers
		// in order to cover cases that number of finality providers is more or
		// less than `MaxActiveFinalityProviders`
		fParams := h.FinalityKeeper.GetParams(h.Ctx)
		fParams.MaxActiveFinalityProviders = uint32(datagen.RandomInt(r, 20) + 10)
		err := h.FinalityKeeper.SetParams(h.Ctx, fParams)
		h.NoError(err)

		numFps := datagen.RandomInt(r, 20) + 10
		numActiveFPs := int(min(numFps, uint64(fParams.MaxActiveFinalityProviders)))

		/*
			Generate a random batch of finality providers, each with a BTC delegation
			with random voting power.
			Then, assert voting power table
		*/
		fpsWithMeta := []*types.FinalityProviderWithMeta{}
		for i := uint64(0); i < numFps; i++ {
			// generate finality provider
			// generate and insert new finality provider
			fpSK, fpPK, fp := h.CreateFinalityProvider(r)
			h.CommitPubRandList(r, fpSK, fp, 1, 100, true)

			// create BTC delegation and add covenant signatures to activate it
			stakingValue := datagen.RandomInt(r, 100000) + 100000
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			h.NoError(err)
			stakingTxHash, delMsg, del, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegationWithBtcBlockHeight(
				r,
				delSK,
				[]*btcec.PublicKey{fpPK},
				int64(stakingValue),
				1000,
				0,
				0,
				true,
				false,
				10,
				10,
			)
			h.NoError(err)
			h.CreateCovenantSigs(r, covenantSKs, delMsg, del, 10)
			h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof, 30)

			// record voting power
			fpsWithMeta = append(fpsWithMeta, &types.FinalityProviderWithMeta{
				BtcPk:       fp.BtcPk,
				VotingPower: stakingValue,
			})
		}

		// record voting power table
		babylonHeight := datagen.RandomInt(r, 10) + 1
		h.Ctx = datagen.WithCtxHeight(h.Ctx, babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: 30})
		h.BeginBlocker()

		// assert that only top `min(MaxActiveFinalityProviders, numFPs)` finality providers have voting power
		sort.SliceStable(fpsWithMeta, func(i, j int) bool {
			return fpsWithMeta[i].VotingPower > fpsWithMeta[j].VotingPower
		})
		for i := 0; i < numActiveFPs; i++ {
			votingPower := h.FinalityKeeper.GetVotingPower(h.Ctx, *fpsWithMeta[i].BtcPk, babylonHeight)
			require.Equal(t, fpsWithMeta[i].VotingPower, votingPower)
		}
		for i := numActiveFPs; i < int(numFps); i++ {
			votingPower := h.FinalityKeeper.GetVotingPower(h.Ctx, *fpsWithMeta[i].BtcPk, babylonHeight)
			require.Zero(t, votingPower)
		}

		/*
			Delegate more tokens to some existing finality providers
			, and create some new finality providers
			Then assert voting power table again
		*/
		// delegate more tokens to some existing finality providers
		for i := uint64(0); i < numFps; i++ {
			if !datagen.OneInN(r, 2) {
				continue
			}

			stakingValue := datagen.RandomInt(r, 100000) + 100000
			fpBTCPK := fpsWithMeta[i].BtcPk
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			h.NoError(err)
			stakingTxHash, delMsg, del, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegationWithBtcBlockHeight(
				r,
				delSK,
				[]*btcec.PublicKey{fpBTCPK.MustToBTCPK()},
				int64(stakingValue),
				1000,
				0,
				0,
				true,
				false,
				10,
				10,
			)
			h.NoError(err)
			h.CreateCovenantSigs(r, covenantSKs, delMsg, del, 10)
			h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof, 30)

			// accumulate voting power for this finality provider
			fpsWithMeta[i].VotingPower += stakingValue

			break
		}
		// create more finality providers
		numNewFps := datagen.RandomInt(r, 20) + 10
		numFps += numNewFps
		numActiveFPs = int(min(numFps, uint64(fParams.MaxActiveFinalityProviders)))
		for i := uint64(0); i < numNewFps; i++ {
			// generate finality provider
			// generate and insert new finality provider
			fpSK, fpPK, fp := h.CreateFinalityProvider(r)
			h.CommitPubRandList(r, fpSK, fp, 1, 100, true)

			// create BTC delegation and add covenant signatures to activate it
			stakingValue := datagen.RandomInt(r, 100000) + 100000
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			h.NoError(err)
			stakingTxHash, delMsg, del, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegationWithBtcBlockHeight(
				r,
				delSK,
				[]*btcec.PublicKey{fpPK},
				int64(stakingValue),
				1000,
				0,
				0,
				true,
				false,
				10,
				10,
			)
			h.NoError(err)
			h.CreateCovenantSigs(r, covenantSKs, delMsg, del, 10)
			h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof, 30)

			// record voting power
			fpsWithMeta = append(fpsWithMeta, &types.FinalityProviderWithMeta{
				BtcPk:       fp.BtcPk,
				VotingPower: stakingValue,
			})
		}

		// record voting power table
		babylonHeight += 1
		h.Ctx = datagen.WithCtxHeight(h.Ctx, babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: 30}).AnyTimes()
		h.BeginBlocker()

		// again, assert that only top `min(MaxActiveFinalityProviders, numFPs)` finality providers have voting power
		sort.SliceStable(fpsWithMeta, func(i, j int) bool {
			return fpsWithMeta[i].VotingPower > fpsWithMeta[j].VotingPower
		})
		for i := 0; i < numActiveFPs; i++ {
			votingPower := h.FinalityKeeper.GetVotingPower(h.Ctx, *fpsWithMeta[i].BtcPk, babylonHeight)
			require.Equal(t, fmt.Sprintf("%d", fpsWithMeta[i].VotingPower), fmt.Sprintf("%d", votingPower))
		}
		for i := numActiveFPs; i < int(numFps); i++ {
			votingPower := h.FinalityKeeper.GetVotingPower(h.Ctx, *fpsWithMeta[i].BtcPk, babylonHeight)
			require.Zero(t, votingPower)
		}
	})
}
