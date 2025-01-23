package keeper_test

import (
	"math/rand"
	"testing"

	testutil "github.com/babylonlabs-io/babylon/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	btclctypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
	bsctypes "github.com/babylonlabs-io/babylon/x/btcstkconsumer/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func FuzzSetBTCStakingEventStore_NewFp(f *testing.F) {
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
		h.GenAndApplyParams(r)

		// register a random consumer on Babylon
		randomConsumer := registerAndVerifyConsumer(t, r, h)

		// create new consumer finality providers, this will create on Babylon and insert
		// events in the events store
		var fps []*types.FinalityProvider
		for i := 0; i < int(datagen.RandomInt(r, 10))+1; i++ {
			_, _, fp, err := h.CreateConsumerFinalityProvider(r, randomConsumer.ConsumerId)
			require.NoError(t, err)
			fps = append(fps, fp)
		}

		// fetch the events from kv store and expect only events related to new finality provider
		evs := h.BTCStakingKeeper.GetBTCStakingConsumerIBCPacket(h.Ctx, randomConsumer.ConsumerId)
		require.NotNil(t, evs)
		require.NotNil(t, evs.GetNewFp())
		require.Equal(t, len(evs.GetNewFp()), len(fps))

		// there should be no other events in the store
		require.Nil(t, evs.GetActiveDel())
		require.Nil(t, evs.GetSlashedDel())
		require.Nil(t, evs.GetUnbondedDel())

		// Prepare a map of finality providers based on btc pk hex
		fpsMap := make(map[string]*types.FinalityProvider)
		for _, fp := range fps {
			fpsMap[fp.BtcPk.MarshalHex()] = fp
		}
		// Assert the contents of staking events
		for _, evFp := range evs.GetNewFp() {
			fp := fpsMap[evFp.BtcPkHex]
			require.NotNil(t, fp)

			// Assert individual fields
			require.Equal(t, fp.Description.Moniker, evFp.Description.Moniker)
			require.Equal(t, fp.Commission.String(), evFp.Commission)
			require.Equal(t, fp.BtcPk.MarshalHex(), evFp.BtcPkHex)
			require.Equal(t, fp.Pop, evFp.Pop)
			require.Equal(t, fp.ConsumerId, evFp.ConsumerId)
		}
	})
}

func FuzzSetBTCStakingEventStore_ActiveDel(f *testing.F) {
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

		// register a random consumer on Babylon
		randomConsumer := registerAndVerifyConsumer(t, r, h)
		// create a new consumer finality provider
		_, consumerFpPK, _, err := h.CreateConsumerFinalityProvider(r, randomConsumer.ConsumerId)
		require.NoError(t, err)
		// create new Babylon finality provider
		_, babylonFpPK, _ := h.CreateFinalityProvider(r)

		// generate and insert new BTC delegation, restake to 1 consumer fp and 1 babylon fp
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTxHash, msgCreateBTCDel, _, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{consumerFpPK, babylonFpPK},
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

		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: 30}).AnyTimes()

		// delegation related assertions
		actualDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		require.False(t, actualDel.HasCovenantQuorums(h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum))
		// create cov sigs to activate the delegation
		msgs := h.GenerateCovenantSignaturesMessages(r, covenantSKs, msgCreateBTCDel, actualDel)
		bogusMsg := *msgs[0]
		bogusMsg.StakingTxHash = datagen.GenRandomBtcdHash(r).String()
		_, err = h.MsgServer.AddCovenantSigs(h.Ctx, &bogusMsg)
		h.Error(err)
		for _, msg := range msgs {
			_, err = h.MsgServer.AddCovenantSigs(h.Ctx, msg)
			h.NoError(err)
		}
		// ensure the BTC delegation now has voting power as it has been activated
		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		require.True(t, actualDel.HasCovenantQuorums(h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum))
		require.True(t, actualDel.BtcUndelegation.HasCovenantQuorums(h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum))
		votingPower := actualDel.VotingPower(
			h.BTCLightClientKeeper.GetTipInfo(h.Ctx).Height,
			h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum,
		)
		require.Equal(t, uint64(stakingValue), votingPower)

		// event store related assertions
		evs := h.BTCStakingKeeper.GetBTCStakingConsumerIBCPacket(h.Ctx, randomConsumer.ConsumerId)
		require.NotNil(t, evs)
		require.NotNil(t, evs.GetActiveDel())
		require.NotNil(t, evs.GetNewFp())
		// we created 2 finality providers but only 1 of them is consumer fp, so expect only 1 fp in the event store
		require.Equal(t, 1, len(evs.GetNewFp()))
		require.Equal(t, 1, len(evs.GetActiveDel()))
		// there should be no other events in the store
		require.Nil(t, evs.GetSlashedDel())
		require.Nil(t, evs.GetUnbondedDel())
		// Assert the contents of the staking event
		ev := evs.GetActiveDel()[0]
		require.NotNil(t, ev)
		require.Equal(t, actualDel.BtcPk.MarshalHex(), ev.BtcPkHex)
		require.Equal(t, actualDel.StartHeight, ev.StartHeight)
		require.Equal(t, actualDel.EndHeight, ev.EndHeight)
		require.Equal(t, actualDel.TotalSat, ev.TotalSat)
		require.Equal(t, actualDel.StakingTx, ev.StakingTx)
		require.Equal(t, actualDel.StakingOutputIdx, ev.StakingOutputIdx)
		require.Equal(t, actualDel.UnbondingTime, ev.UnbondingTime)
	})
}

func FuzzSetBTCStakingEventStore_UnbondedDel(f *testing.F) {
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

		bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

		// register a random consumer on Babylon
		randomConsumer := registerAndVerifyConsumer(t, r, h)
		// create a new consumer finality provider
		_, consumerFpPK, _, err := h.CreateConsumerFinalityProvider(r, randomConsumer.ConsumerId)
		require.NoError(t, err)
		// create new Babylon finality provider
		_, babylonFpPK, _ := h.CreateFinalityProvider(r)

		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTxHash, msgCreateBTCDel, actualDel, _, _, unbondingInfo, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{consumerFpPK, babylonFpPK},
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

		// add covenant signatures to this BTC delegation
		h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel, 10)

		// ensure the BTC delegation is bonded right now
		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		btcTip := uint32(30)
		status := actualDel.GetStatus(btcTip, bsParams.CovenantQuorum)
		require.Equal(t, types.BTCDelegationStatus_ACTIVE, status)

		// construct unbonding msg
		h.NoError(err)
		msg := &types.MsgBTCUndelegate{
			Signer:                        datagen.GenRandomAccount().Address,
			StakingTxHash:                 stakingTxHash,
			StakeSpendingTx:               actualDel.BtcUndelegation.UnbondingTx,
			StakeSpendingTxInclusionProof: unbondingInfo.UnbondingTxInclusionProof,
		}

		// ensure the system does not panic due to a bogus unbonding msg
		bogusMsg := *msg
		bogusMsg.StakingTxHash = datagen.GenRandomBtcdHash(r).String()
		_, err = h.MsgServer.BTCUndelegate(h.Ctx, &bogusMsg)
		h.Error(err)

		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: 30}).AnyTimes()

		// unbond
		_, err = h.MsgServer.BTCUndelegate(h.Ctx, msg)
		h.NoError(err)

		// ensure the BTC delegation is unbonded
		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		status = actualDel.GetStatus(btcTip, bsParams.CovenantQuorum)
		require.Equal(t, types.BTCDelegationStatus_UNBONDED, status)

		// event store related assertions
		evs := h.BTCStakingKeeper.GetBTCStakingConsumerIBCPacket(h.Ctx, randomConsumer.ConsumerId)
		require.NotNil(t, evs)
		require.NotNil(t, evs.GetActiveDel())
		require.NotNil(t, evs.GetNewFp())
		require.NotNil(t, evs.GetUnbondedDel())
		// we created 2 finality providers but only 1 of them is consumer fp, so expect only 1 fp in the event store
		require.Equal(t, 1, len(evs.GetNewFp()))
		require.Equal(t, 1, len(evs.GetActiveDel()))
		require.Equal(t, 1, len(evs.GetUnbondedDel()))
		// there should be no other events in the store
		require.Nil(t, evs.GetSlashedDel())
		// Assert the contents of the staking event
		ev := evs.GetUnbondedDel()[0]
		require.NotNil(t, ev)
		require.Equal(t, actualDel.MustGetStakingTxHash().String(), ev.StakingTxHash)
	})
}

func FuzzDeleteBTCStakingEventStore(f *testing.F) {
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
		h.GenAndApplyParams(r)

		// register random number of consumers on Babylon
		// and create 1 finality provider for each consumer
		randNum := int(datagen.RandomInt(r, 10)) + 1
		var consumerIds []string
		for i := 0; i < randNum; i++ {
			randomConsumer := registerAndVerifyConsumer(t, r, h)
			_, _, _, err := h.CreateConsumerFinalityProvider(r, randomConsumer.ConsumerId)
			require.NoError(t, err)

			evs := h.BTCStakingKeeper.GetBTCStakingConsumerIBCPacket(h.Ctx, randomConsumer.ConsumerId)
			require.NotNil(t, evs)
			require.NotNil(t, evs.GetNewFp())
			require.Equal(t, len(evs.GetNewFp()), 1)

			consumerIds = append(consumerIds, randomConsumer.ConsumerId)
		}

		// delete events for only 1 random consumer
		h.BTCStakingKeeper.DeleteBTCStakingConsumerIBCPacket(h.Ctx, consumerIds[0])

		// assert that the events for the deleted consumer are no longer in the store
		latestStoreState := h.BTCStakingKeeper.GetBTCStakingConsumerIBCPacket(h.Ctx, consumerIds[0])
		require.Nil(t, latestStoreState)

		// assert that the events for the other consumers are still in the store
		for i := 1; i < randNum; i++ {
			latestStoreState = h.BTCStakingKeeper.GetBTCStakingConsumerIBCPacket(h.Ctx, consumerIds[i])
			require.NotNil(t, latestStoreState)
			require.NotNil(t, latestStoreState.GetNewFp())
			require.Equal(t, len(latestStoreState.GetNewFp()), 1)
		}
	})
}

// helper function: register a random consumer on Babylon and verify the registration
func registerAndVerifyConsumer(t *testing.T, r *rand.Rand, h *testutil.Helper) *bsctypes.ConsumerRegister {
	// Generate a random consumer register
	randomConsumer := datagen.GenRandomCosmosConsumerRegister(r)

	// Check that the consumer is not already registered
	isRegistered := h.BTCStkConsumerKeeper.IsConsumerRegistered(h.Ctx, randomConsumer.ConsumerId)
	require.False(t, isRegistered)

	// Attempt to fetch the consumer from the database
	dbConsumer, err := h.BTCStkConsumerKeeper.GetConsumerRegister(h.Ctx, randomConsumer.ConsumerId)
	require.Error(t, err)
	require.Nil(t, dbConsumer)

	// Register the consumer
	err = h.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, randomConsumer)
	require.NoError(t, err)

	// Verify that the consumer is now registered
	dbConsumer, err = h.BTCStkConsumerKeeper.GetConsumerRegister(h.Ctx, randomConsumer.ConsumerId)
	require.NoError(t, err)
	require.NotNil(t, dbConsumer)
	require.Equal(t, randomConsumer.ConsumerId, dbConsumer.ConsumerId)
	require.Equal(t, randomConsumer.ConsumerName, dbConsumer.ConsumerName)
	require.Equal(t, randomConsumer.ConsumerDescription, dbConsumer.ConsumerDescription)

	return dbConsumer
}
