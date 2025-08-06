package keeper_test

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	testkeeper "github.com/babylonlabs-io/babylon/v3/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
)

func TestGetParams(t *testing.T) {
	k, ctx := testkeeper.BTCStakingKeeper(t, nil, nil, nil, nil)
	params := types.DefaultParams()

	currParams := k.GetParams(ctx)
	params.BtcActivationHeight = currParams.BtcActivationHeight + 1
	err := k.SetParams(ctx, params)
	require.NoError(t, err)

	require.EqualValues(t, params, k.GetParams(ctx))
}

func TestAddNewPairParams(t *testing.T) {
	htvm := types.NewHeightToVersionMap()
	// btc start height, version of params
	err := htvm.AddNewPair(10, 0)
	require.NoError(t, err)

	err = htvm.AddNewPair(11, 1)
	require.NoError(t, err)

	err = htvm.AddNewPair(11, 2)
	require.EqualError(t, err, fmt.Errorf("pairs must be sorted by start height in ascending order, got %d <= %d", 11, 11).Error())

	err = htvm.AddNewPair(15, 1)
	require.EqualError(t, err, fmt.Errorf("versions must be strictly increasing, got %d != %d + 1", 1, 1).Error())
}

func TestGetParamsVersions(t *testing.T) {
	k, ctx := testkeeper.BTCStakingKeeper(t, nil, nil, nil, nil)
	params := types.DefaultParams()

	pv := k.GetParamsWithVersion(ctx)

	require.EqualValues(t, params, pv.Params)
	require.EqualValues(t, uint32(0), pv.Version)

	params1 := types.DefaultParams()
	params1.MinSlashingTxFeeSat = 23400
	params1.BtcActivationHeight = pv.Params.BtcActivationHeight + 1

	err := k.SetParams(ctx, params1)
	require.NoError(t, err)

	pv = k.GetParamsWithVersion(ctx)
	p := k.GetParams(ctx)
	require.EqualValues(t, params1, pv.Params)
	require.EqualValues(t, params1, p)
	require.EqualValues(t, uint32(1), pv.Version)

	pv0 := k.GetParamsByVersion(ctx, 0)
	require.NotNil(t, pv0)
	require.EqualValues(t, params, *pv0)
	pv1 := k.GetParamsByVersion(ctx, 1)
	require.NotNil(t, pv1)
	require.EqualValues(t, params1, *pv1)
}

// Property: All public methods related to params are consistent with each other
func FuzzParamsVersioning(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		k, ctx := testkeeper.BTCStakingKeeper(t, nil, nil, nil, nil)
		numVersionsToGenerate := r.Intn(100) + 1
		params0 := k.GetParams(ctx)
		var generatedParams []*types.Params
		generatedParams = append(generatedParams, &params0)

		var btcActivationHeights []uint32
		for i := 0; i < numVersionsToGenerate; i++ {
			params := types.DefaultParams()
			// randomize two parameters so each params are slightly different
			params.MinSlashingTxFeeSat = r.Int63()
			params.UnbondingTimeBlocks = uint32(r.Intn(math.MaxUint16))
			params.BtcActivationHeight = uint32(i) + 1
			err := k.SetParams(ctx, params)
			require.NoError(t, err)
			generatedParams = append(generatedParams, &params)
			btcActivationHeights = append(btcActivationHeights, params.BtcActivationHeight)
		}

		allParams := k.GetAllParams(ctx)

		require.Equal(t, len(generatedParams), len(allParams))

		for i := 0; i < len(generatedParams); i++ {
			// Check that params from aggregate query are ok
			require.EqualValues(t, *generatedParams[i], *allParams[i])

			// Check retrieval by version is ok
			paramByVersion := k.GetParamsByVersion(ctx, uint32(i))
			require.NotNil(t, paramByVersion)
			require.EqualValues(t, *generatedParams[i], *paramByVersion)
		}

		lastParams := k.GetParams(ctx)
		lastVer := k.GetParamsByVersion(ctx, uint32(len(generatedParams)-1))
		require.EqualValues(t, *generatedParams[len(generatedParams)-1], lastParams)
		require.EqualValues(t, lastParams, *lastVer)

		heightToVersionMap := k.GetHeightToVersionMap(ctx)
		require.NotNil(t, heightToVersionMap)
		require.EqualValues(t, len(generatedParams), len(heightToVersionMap.Pairs))

		for _, btcActivationHeight := range btcActivationHeights {
			paramsBTCActivation, version, err := k.GetParamsForBtcHeight(ctx, uint64(btcActivationHeight))
			require.NoError(t, err)
			require.NotNil(t, paramsBTCActivation)

			paramsByVersion := k.GetParamsByVersion(ctx, version)
			require.NotNil(t, paramsByVersion)
			require.EqualValues(t, *paramsBTCActivation, *paramsByVersion)
		}
	})
}
