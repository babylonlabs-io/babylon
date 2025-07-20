package keeper_test

import (
	"encoding/hex"
	"math/rand"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	testutil "github.com/babylonlabs-io/babylon/v3/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
)

func TestPropagateFPSlashingToConsumersFillSlashingEventProperly(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks and helper
	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
	heightAfterMultiStakingAllowListExpiration := int64(10)
	h := testutil.NewHelper(t, btclcKeeper, btccKeeper).WithBlockHeight(heightAfterMultiStakingAllowListExpiration)
	h.GenAndApplyCustomParams(r, 100, 200, 0, 2)

	// generate and insert new Babylon finality provider
	_, fpPK, _ := h.CreateFinalityProvider(r)

	// Register a consumer
	consumerRegister := datagen.GenRandomCosmosConsumerRegister(r)
	err := h.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, consumerRegister)
	require.NoError(t, err)
	consumerID := consumerRegister.ConsumerId

	// Create a consumer finality provider and delegation
	consumerSk, consumerFPPK, _, err := h.CreateConsumerFinalityProvider(r, consumerID)
	require.NoError(t, err)

	// Create a BTC delegation for the FP
	stakingValue := int64(2 * 10e8)
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	_, _, actualDel, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK,
		[]*btcec.PublicKey{fpPK, consumerFPPK},
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

	// Call the function under test
	err = h.BTCStakingKeeper.PropagateFPSlashingToConsumers(h.Ctx, consumerSk)
	require.NoError(t, err)

	// Assert that a slashing event was added for the consumer
	packet := h.BTCStakingKeeper.GetBTCStakingConsumerIBCPacket(h.Ctx, consumerID)
	require.NotNil(t, packet)
	require.NotEmpty(t, packet.SlashedDel)

	// Assert slashing event contains correct data
	slashedDel := packet.SlashedDel[0]
	require.Equal(t, actualDel.MustGetStakingTxHash().String(), slashedDel.StakingTxHash)
	require.Equal(t, hex.EncodeToString(consumerSk.Serialize()), slashedDel.RecoveredFpBtcSk)
}
