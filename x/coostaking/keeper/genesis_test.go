package keeper

import (
	"math/rand"
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
)

func FuzzInitExportGenesis(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)

		// Generate random genesis state
		numHistoricalRewards := int(datagen.RandomInt(r, 10))
		numCoostakerTrackers := int(datagen.RandomInt(r, 10))

		historicalRewards := []types.HistoricalRewardsEntry{}
		for i := 0; i < numHistoricalRewards; i++ {
			historicalRewards = append(historicalRewards, types.HistoricalRewardsEntry{
				Period: uint64(i),
				Rewards: &types.HistoricalRewards{
					CumulativeRewardsPerScore: datagen.GenRandomCoins(r),
				},
			})
		}

		coostakerTrackers := []types.CoostakerRewardsTrackerEntry{}
		addresses := make(map[string]bool) // To avoid duplicates
		for i := 0; i < numCoostakerTrackers; i++ {
			addr := datagen.GenRandomAccount().Address
			if !addresses[addr] {
				addresses[addr] = true
				coostakerTrackers = append(coostakerTrackers, types.CoostakerRewardsTrackerEntry{
					CoostakerAddress: addr,
					Tracker: &types.CoostakerRewardsTracker{
						StartPeriodCumulativeReward: datagen.RandomInt(r, 100),
						TotalScore:                  math.NewInt(int64(datagen.RandomInt(r, 1000) + 1)),
						ActiveSatoshis:              math.NewInt(int64(datagen.RandomInt(r, 1000) + 1)),
						ActiveBaby:                  math.NewInt(int64(datagen.RandomInt(r, 1000) + 1)),
					},
				})
			}
		}

		// Create genesis state
		genState := &types.GenesisState{
			Params: types.DefaultParams(),
			CurrentRewards: types.CurrentRewardsEntry{
				Rewards: &types.CurrentRewards{
					Rewards:    datagen.GenRandomCoins(r),
					Period:     datagen.RandomInt(r, 100),
					TotalScore: math.NewInt(int64(datagen.RandomInt(r, 1000) + 1)),
				},
			},
			HistoricalRewards:        historicalRewards,
			CoostakersRewardsTracker: coostakerTrackers,
		}

		// Validate genesis state
		err := genState.Validate()
		require.NoError(t, err)

		// Initialize genesis
		err = k.InitGenesis(ctx, *genState)
		require.NoError(t, err)

		// Export genesis
		exportedGenState, err := k.ExportGenesis(ctx)
		require.NoError(t, err)
		require.NotNil(t, exportedGenState)

		// Sort exported genesis state for deterministic comparison
		types.SortData(genState)
		types.SortData(exportedGenState)

		// Validate exported genesis
		err = exportedGenState.Validate()
		require.NoError(t, err)

		// Verify exported state matches original
		require.Equal(t, genState, exportedGenState)
	})
}

func TestInitGenesisEmpty(t *testing.T) {
	k, ctx := NewKeeperWithCtx(t)

	// Test with default genesis
	genState := *types.DefaultGenesis()
	err := k.InitGenesis(ctx, genState)
	require.NoError(t, err)

	// Export and verify
	exportedGenState, err := k.ExportGenesis(ctx)
	require.NoError(t, err)
	require.NotNil(t, exportedGenState)

	require.Equal(t, genState.Params, exportedGenState.Params)
	require.Empty(t, exportedGenState.HistoricalRewards)
	require.Empty(t, exportedGenState.CoostakersRewardsTracker)
}

func TestInitGenesisWithCurrentRewards(t *testing.T) {
	k, ctx := NewKeeperWithCtx(t)

	currentRewards := types.CurrentRewards{
		Rewards:    sdk.NewCoins(sdk.NewCoin("ubbn", math.NewInt(1000))),
		Period:     5,
		TotalScore: math.NewInt(100),
	}

	genState := types.GenesisState{
		Params: types.DefaultParams(),
		CurrentRewards: types.CurrentRewardsEntry{
			Rewards: &currentRewards,
		},
		HistoricalRewards:        []types.HistoricalRewardsEntry{},
		CoostakersRewardsTracker: []types.CoostakerRewardsTrackerEntry{},
	}

	err := k.InitGenesis(ctx, genState)
	require.NoError(t, err)

	// Verify current rewards were set
	retrievedRewards, err := k.GetCurrentRewards(ctx)
	require.NoError(t, err)
	require.Equal(t, currentRewards.Rewards.String(), retrievedRewards.Rewards.String())
	require.Equal(t, currentRewards.Period, retrievedRewards.Period)
	require.Equal(t, currentRewards.TotalScore.String(), retrievedRewards.TotalScore.String())

	// Export and verify
	exportedGenState, err := k.ExportGenesis(ctx)
	require.NoError(t, err)

	// Sort for deterministic comparison
	types.SortData(exportedGenState)

	require.NotNil(t, exportedGenState.CurrentRewards.Rewards)
	require.Equal(t, currentRewards.Rewards.String(), exportedGenState.CurrentRewards.Rewards.Rewards.String())
	require.Equal(t, currentRewards.Period, exportedGenState.CurrentRewards.Rewards.Period)
	require.Equal(t, currentRewards.TotalScore.String(), exportedGenState.CurrentRewards.Rewards.TotalScore.String())
}

func TestInitGenesisWithHistoricalRewards(t *testing.T) {
	k, ctx := NewKeeperWithCtx(t)

	historicalRewards := []types.HistoricalRewardsEntry{
		{
			Period: 1,
			Rewards: &types.HistoricalRewards{
				CumulativeRewardsPerScore: sdk.NewCoins(sdk.NewCoin("ubbn", math.NewInt(500))),
			},
		},
		{
			Period: 2,
			Rewards: &types.HistoricalRewards{
				CumulativeRewardsPerScore: sdk.NewCoins(sdk.NewCoin("ubbn", math.NewInt(1000))),
			},
		},
	}

	genState := types.GenesisState{
		Params:                   types.DefaultParams(),
		CurrentRewards:           types.CurrentRewardsEntry{},
		HistoricalRewards:        historicalRewards,
		CoostakersRewardsTracker: []types.CoostakerRewardsTrackerEntry{},
	}

	err := k.InitGenesis(ctx, genState)
	require.NoError(t, err)

	// Verify historical rewards were set
	for _, entry := range historicalRewards {
		retrievedRewards, err := k.GetHistoricalRewards(ctx, entry.Period)
		require.NoError(t, err)
		require.Equal(t, entry.Rewards.CumulativeRewardsPerScore.String(),
			retrievedRewards.CumulativeRewardsPerScore.String())
	}

	// Export and verify
	exportedGenState, err := k.ExportGenesis(ctx)
	require.NoError(t, err)

	// Sort for deterministic comparison
	types.SortData(exportedGenState)

	require.Len(t, exportedGenState.HistoricalRewards, len(historicalRewards))
}

func TestInitGenesisWithCoostakerTrackers(t *testing.T) {
	k, ctx := NewKeeperWithCtx(t)

	addr1 := datagen.GenRandomAccount().Address
	addr2 := datagen.GenRandomAccount().Address

	coostakerTrackers := []types.CoostakerRewardsTrackerEntry{
		{
			CoostakerAddress: addr1,
			Tracker: &types.CoostakerRewardsTracker{
				StartPeriodCumulativeReward: 0,
				TotalScore:                  math.NewInt(50),
			},
		},
		{
			CoostakerAddress: addr2,
			Tracker: &types.CoostakerRewardsTracker{
				StartPeriodCumulativeReward: 3,
				TotalScore:                  math.NewInt(75),
			},
		},
	}

	genState := types.GenesisState{
		Params:                   types.DefaultParams(),
		CurrentRewards:           types.CurrentRewardsEntry{},
		HistoricalRewards:        []types.HistoricalRewardsEntry{},
		CoostakersRewardsTracker: coostakerTrackers,
	}

	err := k.InitGenesis(ctx, genState)
	require.NoError(t, err)

	// Verify coostaker trackers were set
	for _, entry := range coostakerTrackers {
		addr, err := sdk.AccAddressFromBech32(entry.CoostakerAddress)
		require.NoError(t, err)

		retrievedTracker, err := k.GetCoostakerRewards(ctx, addr)
		require.NoError(t, err)
		require.Equal(t, entry.Tracker.StartPeriodCumulativeReward,
			retrievedTracker.StartPeriodCumulativeReward)
		require.Equal(t, entry.Tracker.TotalScore.String(),
			retrievedTracker.TotalScore.String())
	}

	// Export and verify
	exportedGenState, err := k.ExportGenesis(ctx)
	require.NoError(t, err)

	// Sort for deterministic comparison
	types.SortData(exportedGenState)

	require.Len(t, exportedGenState.CoostakersRewardsTracker, len(coostakerTrackers))
}

func TestExportGenesisEmpty(t *testing.T) {
	k, ctx := NewKeeperWithCtx(t)

	// Set default params
	err := k.SetParams(ctx, types.DefaultParams())
	require.NoError(t, err)

	exportedGenState, err := k.ExportGenesis(ctx)
	require.NoError(t, err)
	require.NotNil(t, exportedGenState)

	// Sort for deterministic comparison
	types.SortData(exportedGenState)

	require.Equal(t, types.DefaultParams(), exportedGenState.Params)
	require.Empty(t, exportedGenState.HistoricalRewards)
	require.Empty(t, exportedGenState.CoostakersRewardsTracker)
}
