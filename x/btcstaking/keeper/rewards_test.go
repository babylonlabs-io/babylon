package keeper_test

import (
	"math/rand"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	testutil "github.com/babylonlabs-io/babylon/v4/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/testutil/mocks"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	"github.com/btcsuite/btcd/btcec/v2"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

const (
	invalidBsnConsumerID = "non-existent-consumer"
)

func FuzzDistributeFpCommissionAndBtcDelRewards(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 500)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		btccKForFinality := ftypes.NewMockCheckpointingKeeper(ctrl)

		ictvK := testutil.NewMockIctvKeeperK(ctrl)
		chK := mocks.NewMockZoneConciergeChannelKeeper(ctrl)

		db := dbm.NewMemDB()
		stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())
		heightAfterMultiStakingAllowListExpiration := int64(10)
		h := testutil.NewHelperWithStoreAndIncentive(t, db, stateStore, btclcKeeper, btccKeeper, btccKForFinality, ictvK, chK, nil, nil).WithBlockHeight(heightAfterMultiStakingAllowListExpiration)

		h.GenAndApplyCustomParams(r, 100, 200, 2)

		randConsumer := h.RegisterAndVerifyConsumer(t, r)

		_, fpPk, _ := h.CreateFinalityProvider(r)

		_, fpConsumerPK, consumerFp, err := h.CreateConsumerFinalityProvider(r, randConsumer.ConsumerId)
		h.NoError(err)

		usePreApproval := datagen.OneInN(r, 2)
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)

		_, _, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{fpConsumerPK, fpPk},
			stakingValue,
			1000,
			0,
			0,
			usePreApproval,
			false,
			10,
			30,
		)
		h.NoError(err)

		// bad fp btc pk
		fpSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		badBtcPK := fpSK.PubKey()
		bip340PK := bbn.NewBIP340PubKeyFromBTCPK(badBtcPK)
		_, _, err = h.BTCStakingKeeper.DistributeFpCommissionAndBtcDelRewards(h.Ctx, h.Ctx.ChainID(), *bip340PK, datagen.GenRandomCoins(r))
		require.EqualError(t, err, types.ErrFpNotFound.Wrapf("finality provider %s: %s", bip340PK.MarshalHex(), "the finality provider is not found").Error())

		// bad consumer id for consumer FP
		_, _, err = h.BTCStakingKeeper.DistributeFpCommissionAndBtcDelRewards(h.Ctx, h.Ctx.ChainID(), *consumerFp.BtcPk, datagen.GenRandomCoins(r))
		require.EqualError(t, err, types.ErrFpInvalidBsnID.Wrapf("finality provider %s belongs to BSN consumer %s, not %s", consumerFp.BtcPk.MarshalHex(), consumerFp.BsnId, h.Ctx.ChainID()).Error())

		// valid distribution
		coinsToFp := datagen.GenRandomCoins(r)
		fpCommission := ictvtypes.GetCoinsPortion(coinsToFp, *consumerFp.Commission)
		delegatorRewards := coinsToFp.Sub(fpCommission...)

		fpAddr := consumerFp.Address()

		// Set up expectations for both calls made by the function
		ictvK.MockBtcStk.EXPECT().AccumulateRewardGaugeForFP(gomock.Any(), gomock.Eq(fpAddr), gomock.Eq(fpCommission)).Times(1)
		ictvK.MockBtcStk.EXPECT().AddFinalityProviderRewardsForBtcDelegations(gomock.Any(), gomock.Eq(fpAddr), gomock.Eq(delegatorRewards)).Return(nil).Times(1)

		actualFpCommission, actualDelegatorRewards, err := h.BTCStakingKeeper.DistributeFpCommissionAndBtcDelRewards(h.Ctx, randConsumer.ConsumerId, *consumerFp.BtcPk, coinsToFp)
		h.NoError(err)

		// Verify the returned values match expected calculations
		require.Equal(t, fpCommission, actualFpCommission)
		require.Equal(t, delegatorRewards, actualDelegatorRewards)
	})
}

func FuzzCollectBabylonCommission(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 500)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		bankKeeper := types.NewMockBankKeeper(ctrl)
		ictvK := testutil.NewMockIctvKeeperK(ctrl)
		chK := mocks.NewMockZoneConciergeChannelKeeper(ctrl)

		h := testutil.NewHelperWithBankMock(t, btclcKeeper, btccKeeper, bankKeeper, chK, ictvK, nil)

		h.GenAndApplyCustomParams(r, 100, 200, 2)

		totalRewards := datagen.GenRandomCoins(r)
		_, _, err := h.BTCStakingKeeper.CollectBabylonCommission(h.Ctx, invalidBsnConsumerID, totalRewards)
		require.EqualError(t, err, types.ErrConsumerIDNotRegistered.Wrapf("consumer %s: %s", invalidBsnConsumerID, "consumer not registered").Error())

		randConsumer := h.RegisterAndVerifyConsumer(t, r)

		expectedBabylonCommission := ictvtypes.GetCoinsPortion(totalRewards, randConsumer.BabylonRewardsCommission)
		expectedRemainingRewards := totalRewards.Sub(expectedBabylonCommission...)

		if expectedBabylonCommission.IsAllPositive() {
			bankKeeper.EXPECT().SendCoinsFromModuleToModule(
				gomock.Any(),
				ictvtypes.ModuleName,
				gomock.Any(),
				gomock.Eq(expectedBabylonCommission),
			).Return(nil).Times(1)
		}

		actualBabylonCommission, actualRemainingRewards, err := h.BTCStakingKeeper.CollectBabylonCommission(h.Ctx, randConsumer.ConsumerId, totalRewards)
		h.NoError(err)
		require.Equal(t, expectedBabylonCommission, actualBabylonCommission)
		require.Equal(t, expectedRemainingRewards, actualRemainingRewards)
	})
}

func FuzzCollectComissionAndDistributeBsnRewards(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 500)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(217))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		bankKeeper := types.NewMockBankKeeper(ctrl)
		ictvK := testutil.NewMockIctvKeeperK(ctrl)
		chK := mocks.NewMockZoneConciergeChannelKeeper(ctrl)

		h := testutil.NewHelperWithBankMock(t, btclcKeeper, btccKeeper, bankKeeper, chK, ictvK, nil)

		h.GenAndApplyCustomParams(r, 100, 200, 2)

		totalRewards := datagen.GenRandomCoins(r)
		emptyFpRatios := []types.FpRatio{}
		_, _, err := h.BTCStakingKeeper.CollectComissionAndDistributeBsnRewards(h.Ctx, invalidBsnConsumerID, totalRewards, emptyFpRatios)
		require.EqualError(t, err, types.ErrConsumerIDNotRegistered.Wrapf("consumer %s: %s", invalidBsnConsumerID, "consumer not registered").Error())

		randConsumer := h.RegisterAndVerifyConsumer(t, r)

		// Create multiple finality providers
		numFPs := 2 + r.Intn(3)
		fpRatios := make([]types.FpRatio, numFPs)
		fpAddresses := make([]string, numFPs)

		equalRatio := math.LegacyOneDec().QuoInt64(int64(numFPs))
		for i := 0; i < numFPs; i++ {
			_, _, fp, err := h.CreateConsumerFinalityProvider(r, randConsumer.ConsumerId)
			h.NoError(err)

			fpRatios[i] = types.FpRatio{
				BtcPk: fp.BtcPk,
				Ratio: equalRatio,
			}
			fpAddresses[i] = fp.Address().String()
		}

		expectedBabylonCommission := ictvtypes.GetCoinsPortion(totalRewards, randConsumer.BabylonRewardsCommission)
		remainingRewards := totalRewards.Sub(expectedBabylonCommission...)

		if expectedBabylonCommission.IsAllPositive() {
			bankKeeper.EXPECT().SendCoinsFromModuleToModule(
				gomock.Any(),
				ictvtypes.ModuleName,
				gomock.Any(),
				gomock.Eq(expectedBabylonCommission),
			).Return(nil).Times(1)
		}

		for _, fpRatio := range fpRatios {
			fpRewards := ictvtypes.GetCoinsPortion(remainingRewards, fpRatio.Ratio)

			fp, err := h.BTCStakingKeeper.GetFinalityProvider(h.Ctx, fpRatio.BtcPk.MustMarshal())
			h.NoError(err)

			fpCommission := ictvtypes.GetCoinsPortion(fpRewards, *fp.Commission)
			delegatorRewards := fpRewards.Sub(fpCommission...)

			if fpCommission.IsAllPositive() {
				ictvK.MockBtcStk.EXPECT().AccumulateRewardGaugeForFP(gomock.Any(), gomock.Any(), gomock.Eq(fpCommission)).Times(1)
				ictvK.MockBtcStk.EXPECT().AddFinalityProviderRewardsForBtcDelegations(gomock.Any(), gomock.Any(), gomock.Eq(delegatorRewards)).Return(nil).Times(1)
			}
		}

		actualEvtFpRatios, actualBbnCommission, err := h.BTCStakingKeeper.CollectComissionAndDistributeBsnRewards(h.Ctx, randConsumer.ConsumerId, totalRewards, fpRatios)
		h.NoError(err)

		require.Equal(t, expectedBabylonCommission, actualBbnCommission)
		require.Len(t, actualEvtFpRatios, numFPs)

		for i, evtFpRatio := range actualEvtFpRatios {
			fpRewards := ictvtypes.GetCoinsPortion(remainingRewards, fpRatios[i].Ratio)

			require.Equal(t, fpRatios[i].BtcPk.MarshalHex(), evtFpRatio.FpBtcPkHex)
			require.Equal(t, fpRewards.String(), evtFpRatio.TotalRewards.String())
		}
	})
}
