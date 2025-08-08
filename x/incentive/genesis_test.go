package incentive_test

import (
	"testing"

<<<<<<< HEAD
	keepertest "github.com/babylonlabs-io/babylon/v2/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v2/testutil/nullify"
	"github.com/babylonlabs-io/babylon/v2/x/incentive"
	"github.com/babylonlabs-io/babylon/v2/x/incentive/types"
=======
	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v3/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v3/testutil/nullify"
	"github.com/babylonlabs-io/babylon/v3/x/incentive"
	"github.com/babylonlabs-io/babylon/v3/x/incentive/types"
>>>>>>> 25a0cd3 (chore: allow zero len coins as fp historical rewards (#1529))
	"github.com/stretchr/testify/require"
)

func TestGenesis(t *testing.T) {
	genesisState := types.DefaultGenesis()

	k, ctx := keepertest.IncentiveKeeper(t, nil, nil, nil)
	incentive.InitGenesis(ctx, *k, *genesisState)
	fp1, fp2, del1, del2 := datagen.GenRandomAddress(), datagen.GenRandomAddress(), datagen.GenRandomAddress(), datagen.GenRandomAddress()

	// creates empty historical data
	amt := uint64(30)
	err := k.BtcDelegationActivated(ctx, fp1, del1, sdkmath.NewIntFromUint64(amt))
	require.NoError(t, err)
	err = k.BtcDelegationActivated(ctx, fp2, del2, sdkmath.NewIntFromUint64(amt))
	require.NoError(t, err)
	err = k.BtcDelegationUnbonded(ctx, fp2, del2, sdkmath.NewIntFromUint64(amt))
	require.NoError(t, err)

	got := incentive.ExportGenesis(ctx, *k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)
}
