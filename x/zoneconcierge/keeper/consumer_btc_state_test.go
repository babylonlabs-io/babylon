package keeper_test

import (
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v3/app"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
	"github.com/stretchr/testify/require"
)

const testConsumerID = "test-consumer-1"

func TestConsumerBTCState_GetSet(t *testing.T) {
	babylonApp := app.Setup(t, false)
	zcKeeper := babylonApp.ZoneConciergeKeeper
	ctx := babylonApp.NewContext(false)

	r := rand.New(rand.NewSource(10))
	consumerID := testConsumerID

	header := datagen.GenRandomBTCHeaderInfo(r)

	headers := []*btclctypes.BTCHeaderInfo{
		datagen.GenRandomBTCHeaderInfo(r),
		datagen.GenRandomBTCHeaderInfo(r),
	}
	segment := &types.BTCChainSegment{
		BtcHeaders: headers,
	}

	expectedState := &types.ConsumerBTCState{
		BaseHeader:      header,
		LastSentSegment: segment,
	}

	state := zcKeeper.GetConsumerBTCState(ctx, consumerID)
	require.Nil(t, state)

	zcKeeper.SetConsumerBTCState(ctx, consumerID, expectedState)
	state = zcKeeper.GetConsumerBTCState(ctx, consumerID)
	require.NotNil(t, state)
	require.Equal(t, expectedState.BaseHeader, state.BaseHeader)
	require.Equal(t, expectedState.LastSentSegment, state.LastSentSegment)

	newHeader := datagen.GenRandomBTCHeaderInfo(r)
	expectedState.BaseHeader = newHeader
	zcKeeper.SetConsumerBTCState(ctx, consumerID, expectedState)
	state = zcKeeper.GetConsumerBTCState(ctx, consumerID)
	require.Equal(t, newHeader, state.BaseHeader)
}

func TestConsumerBaseBTCHeader_GetSet(t *testing.T) {
	babylonApp := app.Setup(t, false)
	zcKeeper := babylonApp.ZoneConciergeKeeper
	ctx := babylonApp.NewContext(false)

	r := rand.New(rand.NewSource(10))
	consumerID := testConsumerID
	header := datagen.GenRandomBTCHeaderInfo(r)

	result := zcKeeper.GetConsumerBaseBTCHeader(ctx, consumerID)
	require.Nil(t, result)

	zcKeeper.SetConsumerBaseBTCHeader(ctx, consumerID, header)
	result = zcKeeper.GetConsumerBaseBTCHeader(ctx, consumerID)
	require.NotNil(t, result)
	require.Equal(t, header, result)

	newHeader := datagen.GenRandomBTCHeaderInfo(r)
	zcKeeper.SetConsumerBaseBTCHeader(ctx, consumerID, newHeader)
	result = zcKeeper.GetConsumerBaseBTCHeader(ctx, consumerID)
	require.Equal(t, newHeader, result)

	segment := &types.BTCChainSegment{
		BtcHeaders: []*btclctypes.BTCHeaderInfo{datagen.GenRandomBTCHeaderInfo(r)},
	}
	zcKeeper.SetConsumerLastSentSegment(ctx, consumerID, segment)

	anotherHeader := datagen.GenRandomBTCHeaderInfo(r)
	zcKeeper.SetConsumerBaseBTCHeader(ctx, consumerID, anotherHeader)

	resultHeader := zcKeeper.GetConsumerBaseBTCHeader(ctx, consumerID)
	resultSegment := zcKeeper.GetConsumerLastSentSegment(ctx, consumerID)
	require.Equal(t, anotherHeader, resultHeader)
	require.Equal(t, segment, resultSegment)
}

func TestConsumerLastSentSegment_GetSet(t *testing.T) {
	babylonApp := app.Setup(t, false)
	zcKeeper := babylonApp.ZoneConciergeKeeper
	ctx := babylonApp.NewContext(false)

	r := rand.New(rand.NewSource(10))
	consumerID := testConsumerID

	headers := []*btclctypes.BTCHeaderInfo{
		datagen.GenRandomBTCHeaderInfo(r),
		datagen.GenRandomBTCHeaderInfo(r),
	}
	segment := &types.BTCChainSegment{
		BtcHeaders: headers,
	}

	result := zcKeeper.GetConsumerLastSentSegment(ctx, consumerID)
	require.Nil(t, result)

	zcKeeper.SetConsumerLastSentSegment(ctx, consumerID, segment)
	result = zcKeeper.GetConsumerLastSentSegment(ctx, consumerID)
	require.NotNil(t, result)
	require.Equal(t, segment, result)

	newHeaders := []*btclctypes.BTCHeaderInfo{
		datagen.GenRandomBTCHeaderInfo(r),
	}
	newSegment := &types.BTCChainSegment{
		BtcHeaders: newHeaders,
	}
	zcKeeper.SetConsumerLastSentSegment(ctx, consumerID, newSegment)
	result = zcKeeper.GetConsumerLastSentSegment(ctx, consumerID)
	require.Equal(t, newSegment, result)

	baseHeader := datagen.GenRandomBTCHeaderInfo(r)
	zcKeeper.SetConsumerBaseBTCHeader(ctx, consumerID, baseHeader)

	anotherSegment := &types.BTCChainSegment{
		BtcHeaders: []*btclctypes.BTCHeaderInfo{datagen.GenRandomBTCHeaderInfo(r)},
	}
	zcKeeper.SetConsumerLastSentSegment(ctx, consumerID, anotherSegment)

	resultHeader := zcKeeper.GetConsumerBaseBTCHeader(ctx, consumerID)
	resultSegment := zcKeeper.GetConsumerLastSentSegment(ctx, consumerID)
	require.Equal(t, baseHeader, resultHeader)
	require.Equal(t, anotherSegment, resultSegment)
}

func TestInitializeConsumerBTCState(t *testing.T) {
	babylonApp := app.Setup(t, false)
	zcKeeper := babylonApp.ZoneConciergeKeeper
	btclcKeeper := babylonApp.BTCLightClientKeeper
	ctx := babylonApp.NewContext(false)

	r := rand.New(rand.NewSource(10))
	consumerID := testConsumerID

	chain := datagen.NewBTCHeaderChainWithLength(r, 1, 100, 5)
	err := btclcKeeper.InsertHeadersWithHookAndEvents(ctx, chain.ChainToBytes())
	require.NoError(t, err)

	tipInfo := btclcKeeper.GetTipInfo(ctx)
	require.NotNil(t, tipInfo)

	err = zcKeeper.InitializeConsumerBTCState(ctx, consumerID)
	require.NoError(t, err)

	state := zcKeeper.GetConsumerBTCState(ctx, consumerID)
	require.NotNil(t, state)
	require.Equal(t, tipInfo, state.BaseHeader)
	require.Nil(t, state.LastSentSegment) // Should be nil initially

	err = zcKeeper.InitializeConsumerBTCState(ctx, consumerID)
	require.NoError(t, err)

	newState := zcKeeper.GetConsumerBTCState(ctx, consumerID)
	require.Equal(t, state, newState)

	existingHeader := datagen.GenRandomBTCHeaderInfo(r)
	zcKeeper.SetConsumerBaseBTCHeader(ctx, consumerID, existingHeader)

	err = zcKeeper.InitializeConsumerBTCState(ctx, consumerID)
	require.NoError(t, err)

	finalHeader := zcKeeper.GetConsumerBaseBTCHeader(ctx, consumerID)
	require.Equal(t, existingHeader, finalHeader)
}

func TestConsumerBTCState_MultipleConsumers(t *testing.T) {
	babylonApp := app.Setup(t, false)
	zcKeeper := babylonApp.ZoneConciergeKeeper
	ctx := babylonApp.NewContext(false)

	r := rand.New(rand.NewSource(10))

	consumer1 := "consumer-1"
	consumer2 := "consumer-2"
	consumer3 := "consumer-3"

	header1 := datagen.GenRandomBTCHeaderInfo(r)
	header2 := datagen.GenRandomBTCHeaderInfo(r)
	header3 := datagen.GenRandomBTCHeaderInfo(r)

	segment1 := &types.BTCChainSegment{
		BtcHeaders: []*btclctypes.BTCHeaderInfo{datagen.GenRandomBTCHeaderInfo(r)},
	}
	segment2 := &types.BTCChainSegment{
		BtcHeaders: []*btclctypes.BTCHeaderInfo{
			datagen.GenRandomBTCHeaderInfo(r),
			datagen.GenRandomBTCHeaderInfo(r),
		},
	}

	zcKeeper.SetConsumerBaseBTCHeader(ctx, consumer1, header1)
	zcKeeper.SetConsumerBaseBTCHeader(ctx, consumer2, header2)
	zcKeeper.SetConsumerBaseBTCHeader(ctx, consumer3, header3)

	zcKeeper.SetConsumerLastSentSegment(ctx, consumer1, segment1)
	zcKeeper.SetConsumerLastSentSegment(ctx, consumer2, segment2)

	result1 := zcKeeper.GetConsumerBaseBTCHeader(ctx, consumer1)
	result2 := zcKeeper.GetConsumerBaseBTCHeader(ctx, consumer2)
	result3 := zcKeeper.GetConsumerBaseBTCHeader(ctx, consumer3)

	require.Equal(t, header1, result1)
	require.Equal(t, header2, result2)
	require.Equal(t, header3, result3)

	seg1 := zcKeeper.GetConsumerLastSentSegment(ctx, consumer1)
	seg2 := zcKeeper.GetConsumerLastSentSegment(ctx, consumer2)
	seg3 := zcKeeper.GetConsumerLastSentSegment(ctx, consumer3)

	require.Equal(t, segment1, seg1)
	require.Equal(t, segment2, seg2)
	require.Nil(t, seg3)

	newHeader1 := datagen.GenRandomBTCHeaderInfo(r)
	zcKeeper.SetConsumerBaseBTCHeader(ctx, consumer1, newHeader1)

	result1 = zcKeeper.GetConsumerBaseBTCHeader(ctx, consumer1)
	result2 = zcKeeper.GetConsumerBaseBTCHeader(ctx, consumer2)
	result3 = zcKeeper.GetConsumerBaseBTCHeader(ctx, consumer3)

	require.Equal(t, newHeader1, result1)
	require.Equal(t, header2, result2)
	require.Equal(t, header3, result3)
}

func TestConsumerBTCState_EdgeCases(t *testing.T) {
	babylonApp := app.Setup(t, false)
	zcKeeper := babylonApp.ZoneConciergeKeeper
	ctx := babylonApp.NewContext(false)

	// Test non-existent consumer
	nonExistentID := "non-existent-consumer"
	result := zcKeeper.GetConsumerBTCState(ctx, nonExistentID)
	require.Nil(t, result)

	baseHeader := zcKeeper.GetConsumerBaseBTCHeader(ctx, nonExistentID)
	require.Nil(t, baseHeader)

	lastSegment := zcKeeper.GetConsumerLastSentSegment(ctx, nonExistentID)
	require.Nil(t, lastSegment)

	// Test empty segment
	consumerID := "test-consumer"
	emptySegment := &types.BTCChainSegment{
		BtcHeaders: nil,
	}
	zcKeeper.SetConsumerLastSentSegment(ctx, consumerID, emptySegment)
	resultSegment := zcKeeper.GetConsumerLastSentSegment(ctx, consumerID)
	require.NotNil(t, resultSegment)
	require.Nil(t, resultSegment.BtcHeaders)
}
