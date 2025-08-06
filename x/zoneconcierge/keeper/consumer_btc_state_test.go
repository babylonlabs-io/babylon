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

	headers := []*btclctypes.BTCHeaderInfo{
		datagen.GenRandomBTCHeaderInfo(r),
		datagen.GenRandomBTCHeaderInfo(r),
	}
	segment := &types.BTCChainSegment{
		BtcHeaders: headers,
	}

	expectedState := &types.BSNBTCState{
		LastSentSegment: segment,
	}

	state := zcKeeper.GetBSNBTCState(ctx, consumerID)
	require.Nil(t, state)

	zcKeeper.SetBSNBTCState(ctx, consumerID, expectedState)
	state = zcKeeper.GetBSNBTCState(ctx, consumerID)
	require.NotNil(t, state)
	require.Equal(t, expectedState.LastSentSegment, state.LastSentSegment)
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

	anotherSegment := &types.BTCChainSegment{
		BtcHeaders: []*btclctypes.BTCHeaderInfo{datagen.GenRandomBTCHeaderInfo(r)},
	}
	zcKeeper.SetBSNLastSentSegment(ctx, consumerID, anotherSegment)

	resultSegment := zcKeeper.GetBSNLastSentSegment(ctx, consumerID)
	require.Equal(t, anotherSegment, resultSegment)
}

func TestBSNBTCState_EdgeCases(t *testing.T) {
	babylonApp := app.Setup(t, false)
	zcKeeper := babylonApp.ZoneConciergeKeeper
	ctx := babylonApp.NewContext(false)

	// Test non-existent consumer
	nonExistentID := "non-existent-consumer"
	result := zcKeeper.GetBSNBTCState(ctx, nonExistentID)
	require.Nil(t, result)

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
