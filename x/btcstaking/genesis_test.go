package btcstaking_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	"github.com/babylonlabs-io/babylon/testutil/nullify"
	"github.com/babylonlabs-io/babylon/x/btcstaking"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

func TestGenesis(t *testing.T) {
	p := types.DefaultParams()
	genesisState := types.GenesisState{
		Params: []*types.Params{&p},
	}

	k, ctx := keepertest.BTCStakingKeeper(t, nil, nil, nil)
	btcstaking.InitGenesis(ctx, *k, genesisState)
	got := btcstaking.ExportGenesis(ctx, *k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)
}
