package keeper_test

import (
	"testing"

	testkeeper "github.com/babylonlabs-io/babylon/testutil/keeper"
	"github.com/babylonlabs-io/babylon/x/incentive/types"
	"github.com/stretchr/testify/require"
)

func TestGetParams(t *testing.T) {
	k, ctx := testkeeper.IncentiveKeeper(t, nil, nil, nil)
	params := types.DefaultParams()

	err := k.SetParams(ctx, params)
	require.NoError(t, err)

	require.EqualValues(t, params, k.GetParams(ctx))
}
