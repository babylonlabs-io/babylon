package keeper_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	"github.com/babylonlabs-io/babylon/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func FuzzTestExportGenesis(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		var (
			r      = rand.New(rand.NewSource(seed))
			k, ctx = keepertest.IncentiveKeeper(t, nil, nil, nil)
			len    = int(math.Abs(float64(r.Int() % 50))) // cap it to 50 entries
			bsg    = make([]types.BTCStakingGaugeEntry, len)
			rg     = make([]types.RewardGaugeEntry, len)
		)

		for i := 0; i < len; i++ {
			bsg[i] = types.BTCStakingGaugeEntry{
				Height: datagen.RandomInt(r, 100000),
				Gauge:  datagen.GenRandomGauge(r),
			}
			rg[i] = types.RewardGaugeEntry{
				StakeholderType: datagen.GenRandomStakeholderType(r),
				Address:         datagen.GenRandomAccount().Address,
				RewardGauge:     datagen.GenRandomRewardGauge(r),
			}
		}

		gs := &types.GenesisState{
			Params: types.Params{
				BtcStakingPortion: datagen.RandomLegacyDec(r, 10, 1),
			},
			BtcStakingGauges: bsg,
			RewardGauges:     rg,
		}
		// Setup current state
		require.NoError(t, k.SetParams(ctx, gs.Params))
		for _, e := range gs.BtcStakingGauges {
			k.SetBTCStakingGauge(ctx, e.Height, e.Gauge)
		}
		for _, e := range gs.RewardGauges {
			k.SetRewardGauge(ctx, e.StakeholderType, sdk.MustAccAddressFromBech32(e.Address), e.RewardGauge)
		}

		// Run the ExportGenesis
		exported, err := k.ExportGenesis(ctx)

		require.NoError(t, err)
		types.SortGauges(gs)
		types.SortGauges(exported)
		require.Equal(t, gs, exported)
	})
}

func FuzzTestInitGenesis(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		var (
			r      = rand.New(rand.NewSource(seed))
			k, ctx = keepertest.IncentiveKeeper(t, nil, nil, nil)
			len    = int(math.Abs(float64(r.Int() % 50))) // cap it to 50 entries
			bsg    = make([]types.BTCStakingGaugeEntry, len)
			rg     = make([]types.RewardGaugeEntry, len)
		)

		for i := 0; i < len; i++ {
			bsg[i] = types.BTCStakingGaugeEntry{
				Height: datagen.RandomInt(r, 100000),
				Gauge:  datagen.GenRandomGauge(r),
			}
			rg[i] = types.RewardGaugeEntry{
				StakeholderType: datagen.GenRandomStakeholderType(r),
				Address:         datagen.GenRandomAccount().Address,
				RewardGauge:     datagen.GenRandomRewardGauge(r),
			}
		}

		gs := &types.GenesisState{
			Params: types.Params{
				BtcStakingPortion: datagen.RandomLegacyDec(r, 10, 1),
			},
			BtcStakingGauges: bsg,
			RewardGauges:     rg,
		}
		// Setup current state
		require.NoError(t, k.SetParams(ctx, gs.Params))
		for _, e := range gs.BtcStakingGauges {
			k.SetBTCStakingGauge(ctx, e.Height, e.Gauge)
		}
		for _, e := range gs.RewardGauges {
			k.SetRewardGauge(ctx, e.StakeholderType, sdk.MustAccAddressFromBech32(e.Address), e.RewardGauge)
		}

		// Run the InitGenesis
		err := k.InitGenesis(ctx, *gs)
		require.NoError(t, err)

		// get the current state
		exported, err := k.ExportGenesis(ctx)
		require.NoError(t, err)

		types.SortGauges(gs)
		types.SortGauges(exported)
		require.Equal(t, gs, exported)
	})
}
