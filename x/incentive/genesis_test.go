package incentive_test

import (
	"testing"

	keepertest "github.com/babylonlabs-io/babylon/v3/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v3/testutil/nullify"
	"github.com/babylonlabs-io/babylon/v3/x/incentive"
	"github.com/babylonlabs-io/babylon/v3/x/incentive/types"
	"github.com/stretchr/testify/require"
)

func TestGenesis(t *testing.T) {
	genesisState := types.DefaultGenesis()

	k, ctx := keepertest.IncentiveKeeper(t, nil, nil, nil)
	incentive.InitGenesis(ctx, *k, *genesisState)
	got := incentive.ExportGenesis(ctx, *k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)
}
