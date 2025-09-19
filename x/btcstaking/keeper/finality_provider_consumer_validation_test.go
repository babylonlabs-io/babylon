package keeper_test

import (
	"math/rand"
	"testing"

	"cosmossdk.io/math"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	testutil "github.com/babylonlabs-io/babylon/v4/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/testutil/mocks"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	btcstkconsumertypes "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
)

// TestAddFinalityProvider_ConsumerValidation tests the consumer validation logic
// in the AddFinalityProvider function
func TestAddFinalityProvider_ConsumerValidation(t *testing.T) {
	r := rand.New(rand.NewSource(12345))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
	bankKeeper := types.NewMockBankKeeper(ctrl)
	chanKeeper := mocks.NewMockZoneConciergeChannelKeeper(ctrl)
	ictvKeeper := testutil.NewMockIctvKeeperK(ctrl)

	// Allow any calls to the incentive keeper methods
	ictvKeeper.MockIncentiveKeeper.EXPECT().IncRefundableMsgCount().AnyTimes()

	h := testutil.NewHelperWithBankMock(t, btclcKeeper, btccKeeper, bankKeeper, chanKeeper, ictvKeeper, nil)
	h.GenAndApplyParams(r)

	babylonChainID := h.Ctx.ChainID()
	unregisteredConsumerID := "unregistered-consumer"
	registeredCosmosConsumerID := "registered-cosmos-consumer"
	registeredRollupConsumerID := "registered-rollup-consumer"

	t.Run("babylon_chain_skips_consumer_validation", func(t *testing.T) {
		// Create finality provider for Babylon chain (empty BSN ID defaults to Babylon chain ID)
		fp, err := datagen.GenRandomFinalityProvider(r, h.FpPopContext(), "")
		require.NoError(t, err)

		msg := &types.MsgCreateFinalityProvider{
			Addr:        fp.Addr,
			Description: fp.Description,
			Commission: types.NewCommissionRates(
				*fp.Commission,
				fp.CommissionInfo.MaxRate,
				fp.CommissionInfo.MaxChangeRate,
			),
			BtcPk: fp.BtcPk,
			Pop:   fp.Pop,
			BsnId: "", // Empty BSN ID should default to Babylon chain ID
		}

		// No mock expectations needed as consumer validation should be skipped

		_, err = h.MsgServer.CreateFinalityProvider(h.Ctx, msg)
		require.NoError(t, err)

		// Verify FP was created successfully
		storedFp, err := h.BTCStakingKeeper.GetFinalityProvider(h.Ctx, fp.BtcPk.MustMarshal())
		require.NoError(t, err)
		require.Equal(t, babylonChainID, storedFp.BsnId)
	})

	t.Run("consumer_not_registered_returns_ErrFpBSNIdNotRegistered", func(t *testing.T) {
		// Create finality provider for unregistered consumer
		fp, err := datagen.GenRandomFinalityProvider(r, h.FpPopContext(), unregisteredConsumerID)
		require.NoError(t, err)

		msg := &types.MsgCreateFinalityProvider{
			Addr:        fp.Addr,
			Description: fp.Description,
			Commission: types.NewCommissionRates(
				*fp.Commission,
				fp.CommissionInfo.MaxRate,
				fp.CommissionInfo.MaxChangeRate,
			),
			BtcPk: fp.BtcPk,
			Pop:   fp.Pop,
			BsnId: unregisteredConsumerID,
		}

		// Consumer is not registered, so GetConsumerRegister will fail
		_, err = h.MsgServer.CreateFinalityProvider(h.Ctx, msg)
		require.ErrorIs(t, err, types.ErrFpBSNIdNotRegistered)
	})

	t.Run("cosmos_consumer_without_channel_id_returns_ErrFpConsumerNoIBCChannelOpen", func(t *testing.T) {
		// Create finality provider for registered Cosmos consumer
		fp, err := datagen.GenRandomFinalityProvider(r, h.FpPopContext(), registeredCosmosConsumerID)
		require.NoError(t, err)

		msg := &types.MsgCreateFinalityProvider{
			Addr:        fp.Addr,
			Description: fp.Description,
			Commission: types.NewCommissionRates(
				*fp.Commission,
				fp.CommissionInfo.MaxRate,
				fp.CommissionInfo.MaxChangeRate,
			),
			BtcPk: fp.BtcPk,
			Pop:   fp.Pop,
			BsnId: registeredCosmosConsumerID,
		}

		// Create cosmos consumer register without channel ID
		cosmosConsumerRegister := btcstkconsumertypes.NewCosmosConsumerRegister(
			registeredCosmosConsumerID,
			"Test Cosmos Consumer",
			"Test Description",
			math.LegacyNewDecWithPrec(5, 2), // 0.05
		)
		// Case for channel ID is empty

		// Register the consumer
		err = h.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, cosmosConsumerRegister)
		require.NoError(t, err)

		_, err = h.MsgServer.CreateFinalityProvider(h.Ctx, msg)
		require.ErrorIs(t, err, types.ErrFpConsumerNoIBCChannelOpen)
	})

	t.Run("cosmos_consumer_with_closed_ibc_channel_returns_ErrFpConsumerNoIBCChannelOpen", func(t *testing.T) {
		consumerIDWithClosedChannel := registeredCosmosConsumerID + "-closed"

		// Create finality provider for registered Cosmos consumer
		fp, err := datagen.GenRandomFinalityProvider(r, h.FpPopContext(), consumerIDWithClosedChannel)
		require.NoError(t, err)

		msg := &types.MsgCreateFinalityProvider{
			Addr:        fp.Addr,
			Description: fp.Description,
			Commission: types.NewCommissionRates(
				*fp.Commission,
				fp.CommissionInfo.MaxRate,
				fp.CommissionInfo.MaxChangeRate,
			),
			BtcPk: fp.BtcPk,
			Pop:   fp.Pop,
			BsnId: consumerIDWithClosedChannel,
		}

		// Create cosmos consumer register with channel ID
		cosmosConsumerRegister := btcstkconsumertypes.NewCosmosConsumerRegister(
			consumerIDWithClosedChannel,
			"Test Cosmos Consumer with Closed Channel",
			"Test Description",
			math.LegacyNewDecWithPrec(5, 2), // 0.05
		)
		cosmosConsumerRegister.GetCosmosConsumerMetadata().ChannelId = "channel-123"

		// Register the consumer
		err = h.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, cosmosConsumerRegister)
		require.NoError(t, err)

		// Mock: ConsumerHasIBCChannelOpen returns false (channel is closed)
		chanKeeper.EXPECT().
			ConsumerHasIBCChannelOpen(gomock.Any(), consumerIDWithClosedChannel, "channel-123").
			Return(false).
			Times(1)

		_, err = h.MsgServer.CreateFinalityProvider(h.Ctx, msg)
		require.ErrorIs(t, err, types.ErrFpConsumerNoIBCChannelOpen)
	})

	t.Run("cosmos_consumer_with_open_ibc_channel_succeeds", func(t *testing.T) {
		consumerIDWithOpenChannel := registeredCosmosConsumerID + "-open"

		// Create finality provider for registered Cosmos consumer
		fp, err := datagen.GenRandomFinalityProvider(r, h.FpPopContext(), consumerIDWithOpenChannel)
		require.NoError(t, err)

		msg := &types.MsgCreateFinalityProvider{
			Addr:        fp.Addr,
			Description: fp.Description,
			Commission: types.NewCommissionRates(
				*fp.Commission,
				fp.CommissionInfo.MaxRate,
				fp.CommissionInfo.MaxChangeRate,
			),
			BtcPk: fp.BtcPk,
			Pop:   fp.Pop,
			BsnId: consumerIDWithOpenChannel,
		}

		// Create cosmos consumer register with channel ID
		cosmosConsumerRegister := btcstkconsumertypes.NewCosmosConsumerRegister(
			consumerIDWithOpenChannel,
			"Test Cosmos Consumer with Open Channel",
			"Test Description",
			math.LegacyNewDecWithPrec(5, 2), // 0.05
		)
		cosmosConsumerRegister.GetCosmosConsumerMetadata().ChannelId = "channel-456"

		// Register the consumer
		err = h.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, cosmosConsumerRegister)
		require.NoError(t, err)

		// Mock: ConsumerHasIBCChannelOpen returns true (channel is open)
		chanKeeper.EXPECT().
			ConsumerHasIBCChannelOpen(gomock.Any(), consumerIDWithOpenChannel, "channel-456").
			Return(true).
			Times(1)

		_, err = h.MsgServer.CreateFinalityProvider(h.Ctx, msg)
		require.NoError(t, err)

		// Verify FP was created successfully
		storedFp, err := h.BTCStakingKeeper.GetFinalityProvider(h.Ctx, fp.BtcPk.MustMarshal())
		require.NoError(t, err)
		require.Equal(t, consumerIDWithOpenChannel, storedFp.BsnId)
	})

	t.Run("rollup_consumer_skips_ibc_channel_validation", func(t *testing.T) {
		// Create finality provider for registered rollup consumer
		fp, err := datagen.GenRandomFinalityProvider(r, h.FpPopContext(), registeredRollupConsumerID)
		require.NoError(t, err)

		msg := &types.MsgCreateFinalityProvider{
			Addr:        fp.Addr,
			Description: fp.Description,
			Commission: types.NewCommissionRates(
				*fp.Commission,
				fp.CommissionInfo.MaxRate,
				fp.CommissionInfo.MaxChangeRate,
			),
			BtcPk: fp.BtcPk,
			Pop:   fp.Pop,
			BsnId: registeredRollupConsumerID,
		}

		// Create rollup consumer register
		rollupConsumerRegister := btcstkconsumertypes.NewRollupConsumerRegister(
			registeredRollupConsumerID,
			"Test Rollup Consumer",
			"Test Description",
			"0x1234567890abcdef",
			math.LegacyNewDecWithPrec(5, 2), // 0.05
		)

		// Register the consumer
		err = h.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, rollupConsumerRegister)
		require.NoError(t, err)

		// No ConsumerHasIBCChannelOpen expectation - should be skipped for rollup consumers

		_, err = h.MsgServer.CreateFinalityProvider(h.Ctx, msg)
		require.NoError(t, err)

		// Verify FP was created successfully
		storedFp, err := h.BTCStakingKeeper.GetFinalityProvider(h.Ctx, fp.BtcPk.MustMarshal())
		require.NoError(t, err)
		require.Equal(t, registeredRollupConsumerID, storedFp.BsnId)
	})
}
