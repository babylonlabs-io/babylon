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

func TestBSNBTCState_GetSet(t *testing.T) {
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

	expectedState := &types.BSNBTCState{
		BaseHeader:      header,
		LastSentSegment: segment,
	}

	state := zcKeeper.GetBSNBTCState(ctx, consumerID)
	require.Nil(t, state)

	zcKeeper.SetBSNBTCState(ctx, consumerID, expectedState)
	state = zcKeeper.GetBSNBTCState(ctx, consumerID)
	require.NotNil(t, state)
	require.Equal(t, expectedState.BaseHeader, state.BaseHeader)
	require.Equal(t, expectedState.LastSentSegment, state.LastSentSegment)

	newHeader := datagen.GenRandomBTCHeaderInfo(r)
	expectedState.BaseHeader = newHeader
	zcKeeper.SetBSNBTCState(ctx, consumerID, expectedState)
	state = zcKeeper.GetBSNBTCState(ctx, consumerID)
	require.Equal(t, newHeader, state.BaseHeader)
}

func TestConsumerBaseBTCHeader_GetSet(t *testing.T) {
	babylonApp := app.Setup(t, false)
	zcKeeper := babylonApp.ZoneConciergeKeeper
	ctx := babylonApp.NewContext(false)

	r := rand.New(rand.NewSource(10))
	consumerID := testConsumerID
	header := datagen.GenRandomBTCHeaderInfo(r)

	result := zcKeeper.GetBSNBaseBTCHeader(ctx, consumerID)
	require.Nil(t, result)

	zcKeeper.SetBSNBaseBTCHeader(ctx, consumerID, header)
	result = zcKeeper.GetBSNBaseBTCHeader(ctx, consumerID)
	require.NotNil(t, result)
	require.Equal(t, header, result)

	newHeader := datagen.GenRandomBTCHeaderInfo(r)
	zcKeeper.SetBSNBaseBTCHeader(ctx, consumerID, newHeader)
	result = zcKeeper.GetBSNBaseBTCHeader(ctx, consumerID)
	require.Equal(t, newHeader, result)

	segment := &types.BTCChainSegment{
		BtcHeaders: []*btclctypes.BTCHeaderInfo{datagen.GenRandomBTCHeaderInfo(r)},
	}
	zcKeeper.SetBSNLastSentSegment(ctx, consumerID, segment)

	anotherHeader := datagen.GenRandomBTCHeaderInfo(r)
	zcKeeper.SetBSNBaseBTCHeader(ctx, consumerID, anotherHeader)

	resultHeader := zcKeeper.GetBSNBaseBTCHeader(ctx, consumerID)
	resultSegment := zcKeeper.GetBSNLastSentSegment(ctx, consumerID)
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

	result := zcKeeper.GetBSNLastSentSegment(ctx, consumerID)
	require.Nil(t, result)

	zcKeeper.SetBSNLastSentSegment(ctx, consumerID, segment)
	result = zcKeeper.GetBSNLastSentSegment(ctx, consumerID)
	require.NotNil(t, result)
	require.Equal(t, segment, result)

	newHeaders := []*btclctypes.BTCHeaderInfo{
		datagen.GenRandomBTCHeaderInfo(r),
	}
	newSegment := &types.BTCChainSegment{
		BtcHeaders: newHeaders,
	}
	zcKeeper.SetBSNLastSentSegment(ctx, consumerID, newSegment)
	result = zcKeeper.GetBSNLastSentSegment(ctx, consumerID)
	require.Equal(t, newSegment, result)

	baseHeader := datagen.GenRandomBTCHeaderInfo(r)
	zcKeeper.SetBSNBaseBTCHeader(ctx, consumerID, baseHeader)

	anotherSegment := &types.BTCChainSegment{
		BtcHeaders: []*btclctypes.BTCHeaderInfo{datagen.GenRandomBTCHeaderInfo(r)},
	}
	zcKeeper.SetBSNLastSentSegment(ctx, consumerID, anotherSegment)

	resultHeader := zcKeeper.GetBSNBaseBTCHeader(ctx, consumerID)
	resultSegment := zcKeeper.GetBSNLastSentSegment(ctx, consumerID)
	require.Equal(t, baseHeader, resultHeader)
	require.Equal(t, anotherSegment, resultSegment)
}

func TestInitializeBSNBTCState(t *testing.T) {
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

	err = zcKeeper.InitializeBSNBTCState(ctx, consumerID)
	require.NoError(t, err)

	state := zcKeeper.GetBSNBTCState(ctx, consumerID)
	require.NotNil(t, state)
	require.Equal(t, tipInfo, state.BaseHeader)
	require.Nil(t, state.LastSentSegment) // Should be nil initially

	err = zcKeeper.InitializeBSNBTCState(ctx, consumerID)
	require.NoError(t, err)

	newState := zcKeeper.GetBSNBTCState(ctx, consumerID)
	require.Equal(t, state, newState)

	existingHeader := datagen.GenRandomBTCHeaderInfo(r)
	zcKeeper.SetBSNBaseBTCHeader(ctx, consumerID, existingHeader)

	err = zcKeeper.InitializeBSNBTCState(ctx, consumerID)
	require.NoError(t, err)

	finalHeader := zcKeeper.GetBSNBaseBTCHeader(ctx, consumerID)
	require.Equal(t, existingHeader, finalHeader)
}

func TestBSNBTCState_MultipleBSNs(t *testing.T) {
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

	zcKeeper.SetBSNBaseBTCHeader(ctx, consumer1, header1)
	zcKeeper.SetBSNBaseBTCHeader(ctx, consumer2, header2)
	zcKeeper.SetBSNBaseBTCHeader(ctx, consumer3, header3)

	zcKeeper.SetBSNLastSentSegment(ctx, consumer1, segment1)
	zcKeeper.SetBSNLastSentSegment(ctx, consumer2, segment2)

	result1 := zcKeeper.GetBSNBaseBTCHeader(ctx, consumer1)
	result2 := zcKeeper.GetBSNBaseBTCHeader(ctx, consumer2)
	result3 := zcKeeper.GetBSNBaseBTCHeader(ctx, consumer3)

	require.Equal(t, header1, result1)
	require.Equal(t, header2, result2)
	require.Equal(t, header3, result3)

	seg1 := zcKeeper.GetBSNLastSentSegment(ctx, consumer1)
	seg2 := zcKeeper.GetBSNLastSentSegment(ctx, consumer2)
	seg3 := zcKeeper.GetBSNLastSentSegment(ctx, consumer3)

	require.Equal(t, segment1, seg1)
	require.Equal(t, segment2, seg2)
	require.Nil(t, seg3)

	newHeader1 := datagen.GenRandomBTCHeaderInfo(r)
	zcKeeper.SetBSNBaseBTCHeader(ctx, consumer1, newHeader1)

	result1 = zcKeeper.GetBSNBaseBTCHeader(ctx, consumer1)
	result2 = zcKeeper.GetBSNBaseBTCHeader(ctx, consumer2)
	result3 = zcKeeper.GetBSNBaseBTCHeader(ctx, consumer3)

	require.Equal(t, newHeader1, result1)
	require.Equal(t, header2, result2)
	require.Equal(t, header3, result3)
}

func TestBSNBTCState_EdgeCases(t *testing.T) {
	babylonApp := app.Setup(t, false)
	zcKeeper := babylonApp.ZoneConciergeKeeper
	ctx := babylonApp.NewContext(false)

	// Test non-existent consumer
	nonExistentID := "non-existent-consumer"
	result := zcKeeper.GetBSNBTCState(ctx, nonExistentID)
	require.Nil(t, result)

	baseHeader := zcKeeper.GetBSNBaseBTCHeader(ctx, nonExistentID)
	require.Nil(t, baseHeader)

	lastSegment := zcKeeper.GetBSNLastSentSegment(ctx, nonExistentID)
	require.Nil(t, lastSegment)

	// Test empty segment
	consumerID := "test-consumer"
	emptySegment := &types.BTCChainSegment{
		BtcHeaders: nil,
	}
	zcKeeper.SetBSNLastSentSegment(ctx, consumerID, emptySegment)
	resultSegment := zcKeeper.GetBSNLastSentSegment(ctx, consumerID)
	require.NotNil(t, resultSegment)
	require.Nil(t, resultSegment.BtcHeaders)
}
