package keeper_test

import (
	"math/rand"
	"sort"
	"testing"

	testutil "github.com/babylonlabs-io/babylon/v3/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	bsctypes "github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
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
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)
		h.GenAndApplyParams(r)

		// register a random consumer on Babylon
		randomConsumer := h.RegisterAndVerifyConsumer(t, r)

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
			require.Equal(t, fp.BsnId, evFp.BsnId)
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
		heightAfterMultiStakingAllowListExpiration := int64(10)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil).WithBlockHeight(heightAfterMultiStakingAllowListExpiration)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyCustomParams(r, 100, 200, 2)

		// register a random consumer on Babylon
		randomConsumer := h.RegisterAndVerifyConsumer(t, r)
		// create a new consumer finality provider
		_, consumerFpPK, _, err := h.CreateConsumerFinalityProvider(r, randomConsumer.ConsumerId)
		require.NoError(t, err)
		// create new Babylon finality provider
		_, babylonFpPK, _ := h.CreateFinalityProvider(r)

		// generate and insert new BTC delegation, multi-stake to 1 consumer fp and 1 babylon fp
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTxHash, _, _, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
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
		hasQuorum, err := h.BTCStakingKeeper.BtcDelHasCovenantQuorums(h.Ctx, actualDel, h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum)
		h.NoError(err)
		require.False(t, hasQuorum)
		// create cov sigs to activate the delegation
		msgs := h.GenerateCovenantSignaturesMessages(r, covenantSKs, actualDel)
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
		require.True(t, actualDel.HasCovenantQuorums(h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum, 0))
		require.True(t, actualDel.BtcUndelegation.HasCovenantQuorums(h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum))
		votingPower := actualDel.VotingPower(
			h.BTCLightClientKeeper.GetTipInfo(h.Ctx).Height,
			h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum, 0,
		)
		require.Equal(t, uint64(stakingValue), votingPower)

		// event store related assertions
		evs := h.BTCStakingKeeper.GetBTCStakingConsumerIBCPacket(h.Ctx, randomConsumer.ConsumerId)
		require.NotNil(t, evs)
		require.NotNil(t, evs.GetActiveDel())
		require.NotNil(t, evs.GetNewFp())
		// we created 1 consumer fp, so expect 1 fp in the event store
		require.Equal(t, 1, len(evs.GetNewFp()))
		require.Equal(t, 1, len(evs.GetActiveDel()))
		// there should be no other events in the store
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
		heightAfterMultiStakingAllowListExpiration := int64(10)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil).WithBlockHeight(heightAfterMultiStakingAllowListExpiration)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyCustomParams(r, 100, 200, 2)

		bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

		// register a random consumer on Babylon
		randomConsumer := h.RegisterAndVerifyConsumer(t, r)
		// create a new consumer finality provider
		_, consumerFpPK, _, err := h.CreateConsumerFinalityProvider(r, randomConsumer.ConsumerId)
		require.NoError(t, err)
		// create new Babylon finality provider
		_, babylonFpPK, _ := h.CreateFinalityProvider(r)

		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTime := uint16(1000)
		stakingTxHash, msgCreateBTCDel, actualDel, _, _, unbondingInfo, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{consumerFpPK, babylonFpPK},
			stakingValue,
			stakingTime,
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
		status, err := h.BTCStakingKeeper.BtcDelStatus(h.Ctx, actualDel, bsParams.CovenantQuorum, btcTip)
		h.NoError(err)
		require.Equal(t, types.BTCDelegationStatus_ACTIVE, status)

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

		unbondingTx := actualDel.MustGetUnbondingTx()
		stakingTx := actualDel.MustGetStakingTx()

		serializedUnbondingTxWithWitness, _ := datagen.AddWitnessToUnbondingTx(
			t,
			stakingTx.TxOut[0],
			delSK,
			covenantSKs,
			bsParams.CovenantQuorum,
			[]*btcec.PublicKey{consumerFpPK, babylonFpPK},
			stakingTime,
			stakingValue,
			unbondingTx,
			h.Net,
		)

		msg = &types.MsgBTCUndelegate{
			Signer:                        datagen.GenRandomAccount().Address,
			StakingTxHash:                 stakingTxHash,
			StakeSpendingTx:               serializedUnbondingTxWithWitness,
			StakeSpendingTxInclusionProof: unbondingInfo.UnbondingTxInclusionProof,
			FundingTransactions: [][]byte{
				actualDel.StakingTx,
			},
		}

		// unbond
		_, err = h.MsgServer.BTCUndelegate(h.Ctx, msg)
		h.NoError(err)

		// ensure the BTC delegation is unbonded
		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)

		status, err = h.BTCStakingKeeper.BtcDelStatus(h.Ctx, actualDel, bsParams.CovenantQuorum, btcTip)
		h.NoError(err)
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
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)
		h.GenAndApplyParams(r)

		// register random number of consumers on Babylon
		// and create 1 finality provider for each consumer
		randNum := int(datagen.RandomInt(r, 10)) + 1
		var consumerIds []string
		for i := 0; i < randNum; i++ {
			randomConsumer := h.RegisterAndVerifyConsumer(t, r)
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

func TestDeterministicOrdering(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mock BTC light client and BTC checkpoint modules
	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
	h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)
	h.GenAndApplyParams(r)

	bsnIds := []string{"bsn-z", "bsn-a", "bsn-m", "bsn-b"}

	// register consumers as cosmos consumers
	for _, bsnId := range bsnIds {
		consumerRegister := &bsctypes.ConsumerRegister{
			ConsumerId: bsnId,
			ConsumerMetadata: &bsctypes.ConsumerRegister_CosmosConsumerMetadata{
				CosmosConsumerMetadata: &bsctypes.CosmosConsumerMetadata{
					ChannelId: bsnId,
				},
			},
		}
		err := h.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, consumerRegister)
		require.NoError(t, err)
	}

	// Add consumer events in random order
	for _, bsnId := range bsnIds {
		event := &types.BTCStakingConsumerEvent{
			Event: &types.BTCStakingConsumerEvent_NewFp{
				NewFp: &types.NewFinalityProvider{
					BtcPkHex: "test-pk-" + bsnId,
					BsnId:    bsnId,
				},
			},
		}
		err := h.BTCStakingKeeper.AddBTCStakingConsumerEvent(h.Ctx, bsnId, event)
		require.NoError(t, err)
	}

	// Simulate the pattern used in BroadcastBTCStakingConsumerEvents
	// 1. Get all consumer IBC packets (returns map)
	consumerIBCPacketMap := h.BTCStakingKeeper.GetAllBTCStakingConsumerIBCPackets(h.Ctx)

	// 2. Simulate the fixed iteration pattern (extract keys, sort, then iterate)
	var processOrder1 []string
	{
		// Extract keys and sort them for deterministic iteration (this is our fix)
		consumerIDKeys := make([]string, 0, len(consumerIBCPacketMap))
		for consumerID := range consumerIBCPacketMap {
			consumerIDKeys = append(consumerIDKeys, consumerID)
		}
		sort.Strings(consumerIDKeys)

		// Iterate through consumer IDs in sorted order
		processOrder1 = append(processOrder1, consumerIDKeys...)
	}

	// 3. Repeat the same process to ensure consistent ordering
	var processOrder2 []string
	{
		consumerIBCPacketMap2 := h.BTCStakingKeeper.GetAllBTCStakingConsumerIBCPackets(h.Ctx)

		// Extract keys and sort them for deterministic iteration
		consumerIDKeys := make([]string, 0, len(consumerIBCPacketMap2))
		for consumerID := range consumerIBCPacketMap2 {
			consumerIDKeys = append(consumerIDKeys, consumerID)
		}
		sort.Strings(consumerIDKeys)

		// Iterate through consumer IDs in sorted order
		processOrder2 = append(processOrder2, consumerIDKeys...)
	}

	// Verify that both processing orders are identical
	require.Equal(t, processOrder1, processOrder2, "Processing order should be deterministic")

	// Verify that the processing order is sorted
	expectedOrder := []string{"bsn-a", "bsn-b", "bsn-m", "bsn-z"}
	require.Equal(t, expectedOrder, processOrder1, "Processing should happen in sorted order")
}

func TestHasBTCStakingConsumerIBCPackets(t *testing.T) {
	r := rand.New(rand.NewSource(11))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mock BTC light client and BTC checkpoint modules
	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
	btclcKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(&btclctypes.BTCHeaderInfo{}).AnyTimes()

	h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)
	h.GenAndApplyParams(r)

	// Test: Initially no BTC staking consumer IBC packets should exist
	hasPackets := h.BTCStakingKeeper.HasBTCStakingConsumerIBCPackets(h.Ctx)
	require.False(t, hasPackets, "Should return false when no packets exist")

	// Register and verify first consumer
	consumer1 := h.RegisterAndVerifyConsumer(t, r)

	// Create consumer finality provider, this will add events to the store
	_, _, _, err := h.CreateConsumerFinalityProvider(r, consumer1.ConsumerId)
	require.NoError(t, err)

	// Test: Now packets should exist
	hasPackets = h.BTCStakingKeeper.HasBTCStakingConsumerIBCPackets(h.Ctx)
	require.True(t, hasPackets, "Should return true when at least one packet exists")

	// Register and verify second consumer
	consumer2 := h.RegisterAndVerifyConsumer(t, r)

	// Create another consumer finality provider
	_, _, _, err = h.CreateConsumerFinalityProvider(r, consumer2.ConsumerId)
	require.NoError(t, err)

	// Test: Still should return true (multiple packets exist)
	hasPackets = h.BTCStakingKeeper.HasBTCStakingConsumerIBCPackets(h.Ctx)
	require.True(t, hasPackets, "Should return true when multiple packets exist")

	// Verify consistency with GetAllBTCStakingConsumerIBCPackets
	allPackets := h.BTCStakingKeeper.GetAllBTCStakingConsumerIBCPackets(h.Ctx)
	expectedHasPackets := len(allPackets) > 0
	require.Equal(t, expectedHasPackets, hasPackets, "HasBTCStakingConsumerIBCPackets should be consistent with GetAllBTCStakingConsumerIBCPackets")

	// Delete all packets and verify HasBTCStakingConsumerIBCPackets returns false
	h.BTCStakingKeeper.DeleteBTCStakingConsumerIBCPacket(h.Ctx, consumer1.ConsumerId)
	h.BTCStakingKeeper.DeleteBTCStakingConsumerIBCPacket(h.Ctx, consumer2.ConsumerId)

	hasPackets = h.BTCStakingKeeper.HasBTCStakingConsumerIBCPackets(h.Ctx)
	require.False(t, hasPackets, "Should return false after deleting all packets")

	// Verify consistency again
	allPackets = h.BTCStakingKeeper.GetAllBTCStakingConsumerIBCPackets(h.Ctx)
	expectedHasPackets = len(allPackets) > 0
	require.Equal(t, expectedHasPackets, hasPackets, "HasBTCStakingConsumerIBCPackets should be consistent with GetAllBTCStakingConsumerIBCPackets after deletion")
}

func FuzzMultiStaking_ConsumerEvents_ActiveDel(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()

		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		heightAfterMultiStakingAllowListExpiration := int64(10)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil).WithBlockHeight(heightAfterMultiStakingAllowListExpiration)

		covenantSKs, _ := h.GenAndApplyCustomParams(r, 100, 200, 2)

		randomConsumer := h.RegisterAndVerifyConsumer(t, r)
		_, consumerFpPK, _, err := h.CreateConsumerFinalityProvider(r, randomConsumer.ConsumerId)
		require.NoError(t, err)
		_, babylonFpPK, _ := h.CreateFinalityProvider(r)

		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTxHash, msgCreateBTCDel, actualDel, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{babylonFpPK, consumerFpPK},
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

		h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel, 10)

		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		require.True(t, actualDel.HasCovenantQuorums(h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum, 0))
		votingPower := actualDel.VotingPower(
			h.BTCLightClientKeeper.GetTipInfo(h.Ctx).Height,
			h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum, 0,
		)
		require.Equal(t, uint64(stakingValue), votingPower)

		// event store related assertions for multi-staking
		evs := h.BTCStakingKeeper.GetBTCStakingConsumerIBCPacket(h.Ctx, randomConsumer.ConsumerId)
		require.NotNil(t, evs)
		require.NotNil(t, evs.GetActiveDel())
		require.NotNil(t, evs.GetNewFp())
		require.Equal(t, 1, len(evs.GetNewFp()))
		require.Equal(t, 1, len(evs.GetActiveDel()))
		require.Nil(t, evs.GetUnbondedDel())

		activeDel := evs.GetActiveDel()[0]
		require.NotNil(t, activeDel)
		require.Equal(t, actualDel.BtcPk.MarshalHex(), activeDel.BtcPkHex)
		require.Equal(t, actualDel.StartHeight, activeDel.StartHeight)
		require.Equal(t, actualDel.EndHeight, activeDel.EndHeight)
		require.Equal(t, actualDel.TotalSat, activeDel.TotalSat)
		require.Equal(t, actualDel.StakingTx, activeDel.StakingTx)
		require.Equal(t, actualDel.StakingOutputIdx, activeDel.StakingOutputIdx)
		require.Equal(t, actualDel.UnbondingTime, activeDel.UnbondingTime)

		expFpList := make([]string, len(actualDel.FpBtcPkList))
		for i, fpBtcPk := range actualDel.FpBtcPkList {
			expFpList[i] = fpBtcPk.MarshalHex()
		}
		require.Equal(t, expFpList, activeDel.FpBtcPkList)
	})
}

func FuzzMultiStaking_ConsumerEvents_UnbondedDel(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()

		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		heightAfterMultiStakingAllowListExpiration := int64(10)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil).WithBlockHeight(heightAfterMultiStakingAllowListExpiration)

		covenantSKs, _ := h.GenAndApplyCustomParams(r, 100, 200, 2)
		bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

		// register a random consumer on Babylon and create finality providers
		randomConsumer := h.RegisterAndVerifyConsumer(t, r)
		_, consumerFpPK, _, err := h.CreateConsumerFinalityProvider(r, randomConsumer.ConsumerId)
		require.NoError(t, err)
		_, babylonFpPK, _ := h.CreateFinalityProvider(r)

		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTime := uint16(1000)
		stakingTxHash, msgCreateBTCDel, actualDel, _, _, unbondingInfo, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{babylonFpPK, consumerFpPK},
			stakingValue,
			stakingTime,
			0,
			0,
			false,
			false,
			10,
			30,
		)
		h.NoError(err)

		h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel, 10)

		// ensure the multi-staked BTC delegation is active
		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		btcTip := uint32(30)
		status, err := h.BTCStakingKeeper.BtcDelStatus(h.Ctx, actualDel, bsParams.CovenantQuorum, btcTip)
		h.NoError(err)
		require.Equal(t, types.BTCDelegationStatus_ACTIVE, status)

		// prepare unbonding message
		unbondingTx := actualDel.MustGetUnbondingTx()
		stakingTx := actualDel.MustGetStakingTx()

		serializedUnbondingTxWithWitness, _ := datagen.AddWitnessToUnbondingTx(
			t,
			stakingTx.TxOut[0],
			delSK,
			covenantSKs,
			bsParams.CovenantQuorum,
			[]*btcec.PublicKey{babylonFpPK, consumerFpPK},
			stakingTime,
			stakingValue,
			unbondingTx,
			h.Net,
		)

		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: 30}).AnyTimes()

		msg := &types.MsgBTCUndelegate{
			Signer:                        datagen.GenRandomAccount().Address,
			StakingTxHash:                 stakingTxHash,
			StakeSpendingTx:               serializedUnbondingTxWithWitness,
			StakeSpendingTxInclusionProof: unbondingInfo.UnbondingTxInclusionProof,
			FundingTransactions: [][]byte{
				actualDel.StakingTx,
			},
		}

		// unbond the multi-staked delegation
		_, err = h.MsgServer.BTCUndelegate(h.Ctx, msg)
		h.NoError(err)

		// ensure the multi-staked BTC delegation is unbonded
		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		status, err = h.BTCStakingKeeper.BtcDelStatus(h.Ctx, actualDel, bsParams.CovenantQuorum, btcTip)
		h.NoError(err)
		require.Equal(t, types.BTCDelegationStatus_UNBONDED, status)

		// event store related assertions for multi-staking unbonding
		evs := h.BTCStakingKeeper.GetBTCStakingConsumerIBCPacket(h.Ctx, randomConsumer.ConsumerId)
		require.NotNil(t, evs)
		require.NotNil(t, evs.GetActiveDel())
		require.NotNil(t, evs.GetNewFp())
		require.NotNil(t, evs.GetUnbondedDel())
		require.Equal(t, 1, len(evs.GetNewFp()))
		require.Equal(t, 1, len(evs.GetActiveDel()))
		require.Equal(t, 1, len(evs.GetUnbondedDel()))

		// verify the unbonded delegation event for multi-staking
		unbondedDel := evs.GetUnbondedDel()[0]
		require.NotNil(t, unbondedDel)
		require.Equal(t, actualDel.MustGetStakingTxHash().String(), unbondedDel.StakingTxHash)
	})
}

func FuzzMultiStaking_ConsumerEvents_MultipleFPs(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()

		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		heightAfterMultiStakingAllowListExpiration := int64(10)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil).WithBlockHeight(heightAfterMultiStakingAllowListExpiration)
		covenantSKs, _ := h.GenAndApplyCustomParams(r, 100, 200, 2)

		numConsumers := r.Intn(5) + 1 // rand 1-5 consumers
		consumers := make([]*bsctypes.ConsumerRegister, numConsumers)
		for i := 0; i < numConsumers; i++ {
			consumers[i] = h.RegisterAndVerifyConsumer(t, r)
		}

		numConsumersToTest := r.Intn(numConsumers) + 1
		var consumerFpPKs []*btcec.PublicKey
		var testConsumers []*bsctypes.ConsumerRegister

		for i := 0; i < numConsumersToTest; i++ {
			consumer := consumers[r.Intn(numConsumers)]
			testConsumers = append(testConsumers, consumer)
			_, consumerFpPK, _, err := h.CreateConsumerFinalityProvider(r, consumer.ConsumerId)
			require.NoError(t, err)
			consumerFpPKs = append(consumerFpPKs, consumerFpPK)
		}

		_, babylonFpPK1, _ := h.CreateFinalityProvider(r)
		_, babylonFpPK2, _ := h.CreateFinalityProvider(r)

		stakingValue := int64(2 * 10e8)

		var stakingTxHashes []string

		for i := 0; i < numConsumersToTest; i++ {
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			h.NoError(err)

			babylonFpPK := babylonFpPK1
			if r.Intn(2) == 0 {
				babylonFpPK = babylonFpPK2
			}

			stakingTxHash, msgBTCDel, actualDel, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
				r,
				delSK,
				[]*btcec.PublicKey{babylonFpPK, consumerFpPKs[i]},
				stakingValue,
				1000, 0, 0, false, false, 10, 30,
			)
			h.NoError(err)

			stakingTxHashes = append(stakingTxHashes, stakingTxHash)

			h.CreateCovenantSigs(r, covenantSKs, msgBTCDel, actualDel, 10)
		}

		for i, consumer := range testConsumers {
			evs := h.BTCStakingKeeper.GetBTCStakingConsumerIBCPacket(h.Ctx, consumer.ConsumerId)
			require.NotNil(t, evs)
			require.NotNil(t, evs.GetActiveDel())
			require.True(t, len(evs.GetActiveDel()) >= 1)
			require.Nil(t, evs.GetUnbondedDel())

			actualDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHashes[i])
			h.NoError(err)

			delPkHex := actualDel.BtcPk.MarshalHex()
			found := false
			for _, activeDel := range evs.GetActiveDel() {
				if activeDel.BtcPkHex == delPkHex {
					found = true
					require.Equal(t, actualDel.StakingTx, activeDel.StakingTx)
					break
				}
			}
			require.True(t, found)
		}
	})
}
