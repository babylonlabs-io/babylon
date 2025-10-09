package keeper

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/stretchr/testify/require"
)

func TestKeeperEndBlock(t *testing.T) {
	k, ctx := NewKeeperWithMockIncentiveKeeper(t, nil)

	delAddr, valAddr := datagen.GenRandomAddress(), datagen.GenRandomValidatorAddress()
	shares := math.LegacyNewDec(1500)

	k.stkCache.SetStakedInfo(delAddr, valAddr, shares, shares)

	cachedInfo := k.stkCache.GetStakedInfo(delAddr, valAddr)
	require.True(t, shares.Equal(cachedInfo.Amount))

	k.stkCache.SetStakedInfo(delAddr, valAddr, shares, shares)

	err := k.EndBlock(ctx)
	require.NoError(t, err)

	cachedInfo = k.stkCache.GetStakedInfo(delAddr, valAddr)
	require.True(t, cachedInfo.Amount.IsZero())
}
