package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	keepertest "github.com/babylonchain/babylon/testutil/keeper"
	"github.com/babylonchain/babylon/x/btcstkconsumer/types"
)

func TestGetParams(t *testing.T) {
	k, ctx := keepertest.BTCStkConsumerKeeper(t)
	params := types.DefaultParams()

	require.NoError(t, k.SetParams(ctx, params))
	require.EqualValues(t, params, k.GetParams(ctx))
}
