package keeper_test

import (
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v3/app"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/stretchr/testify/require"
)

func TestConsumerBaseBTCHeader_GetSet(t *testing.T) {
	r := rand.New(rand.NewSource(123))
	babylonApp := app.Setup(t, false)
	zcKeeper := babylonApp.ZoneConciergeKeeper
	btclcKeeper := babylonApp.BTCLightClientKeeper
	ctx := babylonApp.NewContext(false)

	chainLength := uint32(10)
	genRandomChain(t, r, &btclcKeeper, ctx, 0, chainLength)

	headerHeight := uint32(datagen.RandomInt(r, int(chainLength)))
	header := btclcKeeper.GetHeaderByHeight(ctx, headerHeight)
	require.NotNil(t, header)

	consumerID := datagen.GenRandomHexStr(r, 32)

	baseHeader := zcKeeper.GetConsumerBaseBTCHeader(ctx, consumerID)
	require.Nil(t, baseHeader)

	zcKeeper.SetConsumerBaseBTCHeader(ctx, consumerID, header)

	retrievedHeader := zcKeeper.GetConsumerBaseBTCHeader(ctx, consumerID)
	require.NotNil(t, retrievedHeader)
	require.True(t, allFieldsEqual(header, retrievedHeader))

	consumerID2 := datagen.GenRandomHexStr(r, 32)
	baseHeader2 := zcKeeper.GetConsumerBaseBTCHeader(ctx, consumerID2)
	require.Nil(t, baseHeader2)

	header2 := btclcKeeper.GetHeaderByHeight(ctx, headerHeight+1)
	zcKeeper.SetConsumerBaseBTCHeader(ctx, consumerID2, header2)

	// Verify both consumers have their own base headers
	retrievedHeader1 := zcKeeper.GetConsumerBaseBTCHeader(ctx, consumerID)
	retrievedHeader2 := zcKeeper.GetConsumerBaseBTCHeader(ctx, consumerID2)
	require.True(t, allFieldsEqual(header, retrievedHeader1))
	require.True(t, allFieldsEqual(header2, retrievedHeader2))
	require.False(t, allFieldsEqual(retrievedHeader1, retrievedHeader2))
}

func TestConsumerBaseBTCHeader_Update(t *testing.T) {
	r := rand.New(rand.NewSource(456))
	babylonApp := app.Setup(t, false)
	zcKeeper := babylonApp.ZoneConciergeKeeper
	btclcKeeper := babylonApp.BTCLightClientKeeper
	ctx := babylonApp.NewContext(false)

	chainLength := uint32(10)
	genRandomChain(t, r, &btclcKeeper, ctx, 0, chainLength)

	consumerID := datagen.GenRandomHexStr(r, 32)

	header1 := btclcKeeper.GetHeaderByHeight(ctx, 5)
	zcKeeper.SetConsumerBaseBTCHeader(ctx, consumerID, header1)

	retrievedHeader := zcKeeper.GetConsumerBaseBTCHeader(ctx, consumerID)
	require.True(t, allFieldsEqual(header1, retrievedHeader))

	header2 := btclcKeeper.GetHeaderByHeight(ctx, 8)
	zcKeeper.SetConsumerBaseBTCHeader(ctx, consumerID, header2)

	retrievedHeader = zcKeeper.GetConsumerBaseBTCHeader(ctx, consumerID)
	require.True(t, allFieldsEqual(header2, retrievedHeader))
	require.False(t, allFieldsEqual(header1, retrievedHeader))
}

func TestInitializeConsumerBaseBTCHeader(t *testing.T) {
	r := rand.New(rand.NewSource(789))
	babylonApp := app.Setup(t, false)
	zcKeeper := babylonApp.ZoneConciergeKeeper
	btclcKeeper := babylonApp.BTCLightClientKeeper
	ctx := babylonApp.NewContext(false)

	chainLength := uint32(10)
	genRandomChain(t, r, &btclcKeeper, ctx, 0, chainLength)

	consumerID := datagen.GenRandomHexStr(r, 32)

	tip := btclcKeeper.GetTipInfo(ctx)
	require.NotNil(t, tip)

	baseHeader := zcKeeper.GetConsumerBaseBTCHeader(ctx, consumerID)
	require.Nil(t, baseHeader)

	zcKeeper.SetConsumerBaseBTCHeader(ctx, consumerID, tip)

	// Verify the base header is now set to the tip
	retrievedHeader := zcKeeper.GetConsumerBaseBTCHeader(ctx, consumerID)
	require.NotNil(t, retrievedHeader)
	require.True(t, allFieldsEqual(tip, retrievedHeader))
}

func TestConsumerBaseBTCHeader_EmptyConsumerID(t *testing.T) {
	r := rand.New(rand.NewSource(999))
	babylonApp := app.Setup(t, false)
	zcKeeper := babylonApp.ZoneConciergeKeeper
	btclcKeeper := babylonApp.BTCLightClientKeeper
	ctx := babylonApp.NewContext(false)

	chainLength := uint32(5)
	genRandomChain(t, r, &btclcKeeper, ctx, 0, chainLength)

	emptyConsumerID := ""
	header := btclcKeeper.GetHeaderByHeight(ctx, 2)

	require.Panics(t, func() {
		zcKeeper.SetConsumerBaseBTCHeader(ctx, emptyConsumerID, header)
	})

	retrievedHeader := zcKeeper.GetConsumerBaseBTCHeader(ctx, emptyConsumerID)
	require.Nil(t, retrievedHeader)
}

func TestConsumerBaseBTCHeader_MultipleConsumers(t *testing.T) {
	r := rand.New(rand.NewSource(111))
	babylonApp := app.Setup(t, false)
	zcKeeper := babylonApp.ZoneConciergeKeeper
	btclcKeeper := babylonApp.BTCLightClientKeeper
	ctx := babylonApp.NewContext(false)

	chainLength := uint32(20)
	genRandomChain(t, r, &btclcKeeper, ctx, 0, chainLength)

	numConsumers := 5
	consumerIDs := make([]string, numConsumers)
	headers := make([]*btclctypes.BTCHeaderInfo, numConsumers)

	// Set different base headers for different consumers
	for i := 0; i < numConsumers; i++ {
		consumerIDs[i] = datagen.GenRandomHexStr(r, 32)
		headers[i] = btclcKeeper.GetHeaderByHeight(ctx, uint32(i*3+1))
		zcKeeper.SetConsumerBaseBTCHeader(ctx, consumerIDs[i], headers[i])
	}

	// Verify all consumers have their correct base headers
	for i := 0; i < numConsumers; i++ {
		retrievedHeader := zcKeeper.GetConsumerBaseBTCHeader(ctx, consumerIDs[i])
		require.NotNil(t, retrievedHeader)
		require.True(t, allFieldsEqual(headers[i], retrievedHeader))
	}

	// Verify headers are different for different consumers
	for i := 0; i < numConsumers; i++ {
		for j := i + 1; j < numConsumers; j++ {
			header1 := zcKeeper.GetConsumerBaseBTCHeader(ctx, consumerIDs[i])
			header2 := zcKeeper.GetConsumerBaseBTCHeader(ctx, consumerIDs[j])
			require.False(t, allFieldsEqual(header1, header2))
		}
	}
}
