package keeper_test

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	testutil "github.com/babylonlabs-io/babylon/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	btclctypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

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
		changeAddress, err := datagen.GenRandomBTCAddress(r, h.Net)
		h.NoError(err)

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
				stakingTxHash, delMsg, del, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegation(
					r,
					delSK,
					fp.BtcPk.MustToBTCPK(),
					changeAddress.EncodeAddress(),
					int64(stakingValue),
					1000,
					0,
					0,
					true,
				)
				h.NoError(err)
				h.CreateCovenantSigs(r, covenantSKs, delMsg, del)
				h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof)
			}
		}

		// record voting power distribution cache
		babylonHeight := datagen.RandomInt(r, 10) + 1
		h.Ctx = datagen.WithCtxHeight(h.Ctx, babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: 30}).AnyTimes()
		h.BeginBlocker()

		// assert voting power distribution cache is correct
		dc := h.BTCStakingKeeper.GetVotingPowerDistCache(h.Ctx, babylonHeight)
		require.NotNil(t, dc)
		require.Equal(t, dc.TotalBondedSat, numFpsWithVotingPower*numBTCDels*stakingValue, dc.String())
		activeFPs := dc.GetActiveFinalityProviderSet()
		for _, fpDistInfo := range activeFPs {
			require.Equal(t, fpDistInfo.TotalBondedSat, numBTCDels*stakingValue)
			fp, ok := fpsWithVotingPowerMap[fpDistInfo.Addr]
			require.True(t, ok)
			require.Equal(t, fpDistInfo.Commission, fp.Commission)
			require.Len(t, fpDistInfo.BtcDels, int(numBTCDels))
			for _, delDistInfo := range fpDistInfo.BtcDels {
				require.Equal(t, delDistInfo.TotalSat, stakingValue)
			}
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
		changeAddress, err := datagen.GenRandomBTCAddress(r, h.Net)
		h.NoError(err)

		// generate a random batch of finality providers, each with a BTC delegation with random power
		fpsWithMeta := []*types.FinalityProviderDistInfo{}
		numFps := datagen.RandomInt(r, 300) + 1
		noTimestampedFps := map[string]bool{}
		for i := uint64(0); i < numFps; i++ {
			// generate finality provider
			fpSK, _, fp := h.CreateFinalityProvider(r)

			// delegate to this finality provider
			stakingValue := datagen.RandomInt(r, 100000) + 100000
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			h.NoError(err)
			stakingTxHash, delMsg, del, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegation(
				r,
				delSK,
				fp.BtcPk.MustToBTCPK(),
				changeAddress.EncodeAddress(),
				int64(stakingValue),
				1000,
				0,
				0,
				true,
			)
			h.NoError(err)
			h.CreateCovenantSigs(r, covenantSKs, delMsg, del)
			h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof)

			// 30 percent not have timestamped randomness, which causes
			// zero voting power in the table
			fpDistInfo := &types.FinalityProviderDistInfo{BtcPk: fp.BtcPk, TotalBondedSat: stakingValue}
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
		types.SortFinalityProvidersWithZeroedVotingPower(fpsWithMeta)
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
			power := h.BTCStakingKeeper.GetVotingPower(h.Ctx, fp.BtcPk.MustMarshal(), babylonHeight)
			if expectedPower, ok := expectedActiveFpsMap[fp.BtcPk.MarshalHex()]; ok {
				require.Equal(t, expectedPower, power)
			} else {
				require.Zero(t, power)
			}
		}

		// also, get voting power table and assert there is
		// min(len(expectedActiveFps), MaxActiveFinalityProviders) active finality providers
		powerTable := h.BTCStakingKeeper.GetVotingPowerTable(h.Ctx, babylonHeight)
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
		// change address
		changeAddress, err := datagen.GenRandomBTCAddress(r, h.Net)
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
			stakingTxHash, delMsg, del, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegation(
				r,
				delSK,
				fpPK,
				changeAddress.EncodeAddress(),
				int64(stakingValue),
				1000,
				0,
				0,
				true,
			)
			h.NoError(err)
			h.CreateCovenantSigs(r, covenantSKs, delMsg, del)
			h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof)

			// record voting power
			fpsWithMeta = append(fpsWithMeta, &types.FinalityProviderWithMeta{
				BtcPk:       fp.BtcPk,
				VotingPower: stakingValue,
			})
		}

		// record voting power table
		babylonHeight := datagen.RandomInt(r, 10) + 1
		h.Ctx = datagen.WithCtxHeight(h.Ctx, babylonHeight)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: 30}).AnyTimes()
		h.BeginBlocker()

		// assert that only top `min(MaxActiveFinalityProviders, numFPs)` finality providers have voting power
		sort.SliceStable(fpsWithMeta, func(i, j int) bool {
			return fpsWithMeta[i].VotingPower > fpsWithMeta[j].VotingPower
		})
		for i := 0; i < numActiveFPs; i++ {
			votingPower := h.BTCStakingKeeper.GetVotingPower(h.Ctx, *fpsWithMeta[i].BtcPk, babylonHeight)
			require.Equal(t, fpsWithMeta[i].VotingPower, votingPower)
		}
		for i := numActiveFPs; i < int(numFps); i++ {
			votingPower := h.BTCStakingKeeper.GetVotingPower(h.Ctx, *fpsWithMeta[i].BtcPk, babylonHeight)
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
			stakingTxHash, delMsg, del, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegation(
				r,
				delSK,
				fpBTCPK.MustToBTCPK(),
				changeAddress.EncodeAddress(),
				int64(stakingValue),
				1000,
				0,
				0,
				true,
			)
			h.NoError(err)
			h.CreateCovenantSigs(r, covenantSKs, delMsg, del)
			h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof)

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
			stakingTxHash, delMsg, del, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegation(
				r,
				delSK,
				fpPK,
				changeAddress.EncodeAddress(),
				int64(stakingValue),
				1000,
				0,
				0,
				true,
			)
			h.NoError(err)
			h.CreateCovenantSigs(r, covenantSKs, delMsg, del)
			h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof)

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
			votingPower := h.BTCStakingKeeper.GetVotingPower(h.Ctx, *fpsWithMeta[i].BtcPk, babylonHeight)
			require.Equal(t, fpsWithMeta[i].VotingPower, votingPower)
		}
		for i := numActiveFPs; i < int(numFps); i++ {
			votingPower := h.BTCStakingKeeper.GetVotingPower(h.Ctx, *fpsWithMeta[i].BtcPk, babylonHeight)
			require.Zero(t, votingPower)
		}
	})
}
