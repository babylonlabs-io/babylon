package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"

	"cosmossdk.io/math"
	testutil "github.com/babylonlabs-io/babylon/v3/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v3/x/incentive/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func FuzzDistributeFpCommissionAndBtcDelRewards(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 500)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Use NoMocksCalls helper to avoid .AnyTimes() setup conflicts
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelperNoMocksCalls(t, btclcKeeper, btccKeeper)

		h.GenAndApplyCustomParams(r, 100, 200, 0, 2)

		randConsumer := registerAndVerifyConsumer(t, r, h)

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
		require.EqualError(t, err, fmt.Errorf("finality provider not found: %w", types.ErrFpNotFound).Error())

		// bad consumer id for consumer FP
		_, _, err = h.BTCStakingKeeper.DistributeFpCommissionAndBtcDelRewards(h.Ctx, h.Ctx.ChainID(), *consumerFp.BtcPk, datagen.GenRandomCoins(r))
		require.EqualError(t, err, fmt.Errorf("finality provider %s belongs to BSN consumer %s, not %s", consumerFp.BtcPk.MarshalHex(), consumerFp.BsnId, h.Ctx.ChainID()).Error())

		// valid distribution
		coinsToFp := datagen.GenRandomCoins(r)
		fpCommission := ictvtypes.GetCoinsPortion(coinsToFp, *consumerFp.Commission)
		delegatorRewards := coinsToFp.Sub(fpCommission...)

		fpAddr := consumerFp.Address()

		// Set up expectations for both calls made by the function
		h.IctvKeeperK.MockBtcStk.EXPECT().AccumulateRewardGaugeForFP(gomock.Any(), gomock.Eq(fpAddr), gomock.Eq(fpCommission)).Times(1)
		h.IctvKeeperK.MockBtcStk.EXPECT().AddFinalityProviderRewardsForBtcDelegations(gomock.Any(), gomock.Eq(fpAddr), gomock.Eq(delegatorRewards)).Return(nil).Times(1)

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

		// Use helper with bank keeper mock
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		bankKeeper := types.NewMockBankKeeper(ctrl)
		h := testutil.NewHelperWithBankMock(t, btclcKeeper, btccKeeper, bankKeeper)

		h.GenAndApplyCustomParams(r, 100, 200, 0, 2)

		// Test case 1: Consumer not found
		nonExistentConsumerId := "non-existent-consumer"
		totalRewards := datagen.GenRandomCoins(r)
		_, _, err := h.BTCStakingKeeper.CollectBabylonCommission(h.Ctx, nonExistentConsumerId, totalRewards)
		require.ErrorContains(t, err, "BSN consumer not found")

		// Test case 2: Valid commission collection
		randConsumer := registerAndVerifyConsumer(t, r, h)

		// Calculate expected values
		expectedBabylonCommission := ictvtypes.GetCoinsPortion(totalRewards, randConsumer.BabylonRewardsCommission)
		expectedRemainingRewards := totalRewards.Sub(expectedBabylonCommission...)

		// Mock the bank keeper for commission transfer
		bankKeeper.EXPECT().SendCoinsFromModuleToModule(
			gomock.Any(),
			ictvtypes.ModuleName,
			gomock.Any(),
			gomock.Eq(expectedBabylonCommission),
		).Return(nil).Times(1)

		actualBabylonCommission, actualRemainingRewards, err := h.BTCStakingKeeper.CollectBabylonCommission(h.Ctx, randConsumer.ConsumerId, totalRewards)
		h.NoError(err)

		// Verify the returned values match expected calculations
		require.Equal(t, expectedBabylonCommission, actualBabylonCommission)
		require.Equal(t, expectedRemainingRewards, actualRemainingRewards)
	})
}

func FuzzDistributeComissionAndBsnRewards(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 500)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Use helper with bank keeper mock
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		bankKeeper := types.NewMockBankKeeper(ctrl)
		h := testutil.NewHelperWithBankMock(t, btclcKeeper, btccKeeper, bankKeeper)

		h.GenAndApplyCustomParams(r, 100, 200, 0, 2)

		// Test case 1: Consumer not found
		nonExistentConsumerId := "non-existent-consumer"
		totalRewards := datagen.GenRandomCoins(r)
		emptyFpRatios := []types.FpRatio{}
		_, _, err := h.BTCStakingKeeper.DistributeComissionAndBsnRewards(h.Ctx, nonExistentConsumerId, totalRewards, emptyFpRatios)
		require.ErrorContains(t, err, "BSN consumer not found")

		// Test case 2: Valid distribution with multiple FPs
		randConsumer := registerAndVerifyConsumer(t, r, h)

		// Create multiple finality providers
		numFPs := 2 + r.Intn(3) // 2-4 FPs
		fpRatios := make([]types.FpRatio, numFPs)
		fpAddresses := make([]string, numFPs)

		// Generate FP ratios that sum to 1.0
		// Create equal ratios for simplicity
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

		// Calculate expected values
		expectedBabylonCommission := ictvtypes.GetCoinsPortion(totalRewards, randConsumer.BabylonRewardsCommission)
		remainingRewards := totalRewards.Sub(expectedBabylonCommission...)

		// Mock the bank keeper for commission transfer
		bankKeeper.EXPECT().SendCoinsFromModuleToModule(
			gomock.Any(),
			ictvtypes.ModuleName,
			gomock.Any(),
			gomock.Eq(expectedBabylonCommission),
		).Return(nil).Times(1)

		// Set up expectations for each FP
		for _, fpRatio := range fpRatios {
			fpRewards := ictvtypes.GetCoinsPortion(remainingRewards, fpRatio.Ratio)

			// Get the FP from the BTCStaking keeper to get the actual commission
			fp, err := h.BTCStakingKeeper.GetFinalityProvider(h.Ctx, fpRatio.BtcPk.MustMarshal())
			h.NoError(err)

			// For each FP, we expect both AccumulateRewardGaugeForFP and AddFinalityProviderRewardsForBtcDelegations
			fpCommission := ictvtypes.GetCoinsPortion(fpRewards, *fp.Commission)
			delegatorRewards := fpRewards.Sub(fpCommission...)

			h.IctvKeeperK.MockBtcStk.EXPECT().AccumulateRewardGaugeForFP(gomock.Any(), gomock.Any(), gomock.Eq(fpCommission)).Times(1)
			h.IctvKeeperK.MockBtcStk.EXPECT().AddFinalityProviderRewardsForBtcDelegations(gomock.Any(), gomock.Any(), gomock.Eq(delegatorRewards)).Return(nil).Times(1)
		}

		actualEvtFpRatios, actualBbnCommission, err := h.BTCStakingKeeper.DistributeComissionAndBsnRewards(h.Ctx, randConsumer.ConsumerId, totalRewards, fpRatios)
		h.NoError(err)

		// Verify the returned values
		require.Equal(t, expectedBabylonCommission, actualBbnCommission)
		require.Len(t, actualEvtFpRatios, numFPs)

		// Verify each FP's event data
		for i, evtFpRatio := range actualEvtFpRatios {
			require.Equal(t, fpRatios[i].BtcPk, evtFpRatio.BtcPk)
			require.Equal(t, fpRatios[i].Ratio, evtFpRatio.Ratio)
		}
	})
}
