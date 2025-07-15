package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"

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
