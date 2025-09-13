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

	k.stkCache.SetStakedAmount(delAddr, valAddr, shares)

	cachedAmount := k.stkCache.GetStakedAmount(delAddr, valAddr)
	require.True(t, shares.Equal(cachedAmount))

	k.stkCache.SetStakedAmount(delAddr, valAddr, shares)

	err := k.EndBlock(ctx)
	require.NoError(t, err)

	cachedAmount = k.stkCache.GetStakedAmount(delAddr, valAddr)
	require.True(t, cachedAmount.IsZero())
}
