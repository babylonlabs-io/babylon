package types_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
)

func TestDefaultGenesis(t *testing.T) {
	genesis := types.DefaultGenesis()
	require.NotNil(t, genesis)
	require.Equal(t, types.DefaultParams(), genesis.Params)
	require.Empty(t, genesis.HistoricalRewards)
	require.Empty(t, genesis.CoostakersRewardsTracker)
	require.NotNil(t, genesis.CurrentRewards)

	err := genesis.Validate()
	require.NoError(t, err)
}

func TestGenesisValidate(t *testing.T) {
	validAddress := datagen.GenRandomAccount().Address
	validCoins := sdk.NewCoins(sdk.NewCoin("ubbn", math.NewInt(1000)))
	validScore := math.NewInt(100)

	tests := []struct {
		name    string
		genesis types.GenesisState
		expErr  bool
	}{
		{
			name:    "default genesis is valid",
			genesis: *types.DefaultGenesis(),
			expErr:  false,
		},
		{
			name: "valid genesis with current rewards",
			genesis: types.GenesisState{
				Params: types.DefaultParams(),
				CurrentRewards: types.CurrentRewardsEntry{
					Rewards: &types.CurrentRewards{
						Rewards:    validCoins,
						Period:     1,
						TotalScore: validScore,
					},
				},
				HistoricalRewards:        []types.HistoricalRewardsEntry{},
				CoostakersRewardsTracker: []types.CoostakerRewardsTrackerEntry{},
			},
			expErr: false,
		},
		{
			name: "valid genesis with historical rewards",
			genesis: types.GenesisState{
				Params:         types.DefaultParams(),
				CurrentRewards: types.CurrentRewardsEntry{},
				HistoricalRewards: []types.HistoricalRewardsEntry{
					{
						Period: 1,
						Rewards: &types.HistoricalRewards{
							CumulativeRewardsPerScore: validCoins,
						},
					},
				},
				CoostakersRewardsTracker: []types.CoostakerRewardsTrackerEntry{},
			},
			expErr: false,
		},
		{
			name: "valid genesis with coostaker rewards tracker",
			genesis: types.GenesisState{
				Params:            types.DefaultParams(),
				CurrentRewards:    types.CurrentRewardsEntry{},
				HistoricalRewards: []types.HistoricalRewardsEntry{},
				CoostakersRewardsTracker: []types.CoostakerRewardsTrackerEntry{
					{
						CoostakerAddress: validAddress,
						Tracker: &types.CoostakerRewardsTracker{
							StartPeriodCumulativeReward: 0,
							TotalScore:                  validScore,
						},
					},
				},
			},
			expErr: false,
		},
		{
			name: "invalid current rewards - negative coins",
			genesis: types.GenesisState{
				Params: types.DefaultParams(),
				CurrentRewards: types.CurrentRewardsEntry{
					Rewards: &types.CurrentRewards{
						Rewards:    sdk.Coins{sdk.Coin{Denom: "ubbn", Amount: math.NewInt(-100)}},
						Period:     1,
						TotalScore: validScore,
					},
				},
				HistoricalRewards:        []types.HistoricalRewardsEntry{},
				CoostakersRewardsTracker: []types.CoostakerRewardsTrackerEntry{},
			},
			expErr: true,
		},
		{
			name: "invalid current rewards - negative total score",
			genesis: types.GenesisState{
				Params: types.DefaultParams(),
				CurrentRewards: types.CurrentRewardsEntry{
					Rewards: &types.CurrentRewards{
						Rewards:    validCoins,
						Period:     1,
						TotalScore: math.NewInt(-100),
					},
				},
				HistoricalRewards:        []types.HistoricalRewardsEntry{},
				CoostakersRewardsTracker: []types.CoostakerRewardsTrackerEntry{},
			},
			expErr: true,
		},
		{
			name: "duplicate historical rewards periods",
			genesis: types.GenesisState{
				Params:         types.DefaultParams(),
				CurrentRewards: types.CurrentRewardsEntry{},
				HistoricalRewards: []types.HistoricalRewardsEntry{
					{
						Period: 1,
						Rewards: &types.HistoricalRewards{
							CumulativeRewardsPerScore: validCoins,
						},
					},
					{
						Period: 1, // duplicate period
						Rewards: &types.HistoricalRewards{
							CumulativeRewardsPerScore: validCoins,
						},
					},
				},
				CoostakersRewardsTracker: []types.CoostakerRewardsTrackerEntry{},
			},
			expErr: true,
		},
		{
			name: "historical rewards with nil rewards",
			genesis: types.GenesisState{
				Params:         types.DefaultParams(),
				CurrentRewards: types.CurrentRewardsEntry{},
				HistoricalRewards: []types.HistoricalRewardsEntry{
					{
						Period:  1,
						Rewards: nil, // nil rewards
					},
				},
				CoostakersRewardsTracker: []types.CoostakerRewardsTrackerEntry{},
			},
			expErr: true,
		},
		{
			name: "duplicate coostaker addresses",
			genesis: types.GenesisState{
				Params:            types.DefaultParams(),
				CurrentRewards:    types.CurrentRewardsEntry{},
				HistoricalRewards: []types.HistoricalRewardsEntry{},
				CoostakersRewardsTracker: []types.CoostakerRewardsTrackerEntry{
					{
						CoostakerAddress: validAddress,
						Tracker: &types.CoostakerRewardsTracker{
							StartPeriodCumulativeReward: 0,
							TotalScore:                  validScore,
						},
					},
					{
						CoostakerAddress: validAddress, // duplicate address
						Tracker: &types.CoostakerRewardsTracker{
							StartPeriodCumulativeReward: 1,
							TotalScore:                  validScore,
						},
					},
				},
			},
			expErr: true,
		},
		{
			name: "invalid coostaker address",
			genesis: types.GenesisState{
				Params:            types.DefaultParams(),
				CurrentRewards:    types.CurrentRewardsEntry{},
				HistoricalRewards: []types.HistoricalRewardsEntry{},
				CoostakersRewardsTracker: []types.CoostakerRewardsTrackerEntry{
					{
						CoostakerAddress: "invalid-address", // invalid address
						Tracker: &types.CoostakerRewardsTracker{
							StartPeriodCumulativeReward: 0,
							TotalScore:                  validScore,
						},
					},
				},
			},
			expErr: true,
		},
		{
			name: "coostaker tracker with nil tracker",
			genesis: types.GenesisState{
				Params:            types.DefaultParams(),
				CurrentRewards:    types.CurrentRewardsEntry{},
				HistoricalRewards: []types.HistoricalRewardsEntry{},
				CoostakersRewardsTracker: []types.CoostakerRewardsTrackerEntry{
					{
						CoostakerAddress: validAddress,
						Tracker:          nil, // nil tracker
					},
				},
			},
			expErr: true,
		},
		{
			name: "coostaker tracker with negative total score",
			genesis: types.GenesisState{
				Params:            types.DefaultParams(),
				CurrentRewards:    types.CurrentRewardsEntry{},
				HistoricalRewards: []types.HistoricalRewardsEntry{},
				CoostakersRewardsTracker: []types.CoostakerRewardsTrackerEntry{
					{
						CoostakerAddress: validAddress,
						Tracker: &types.CoostakerRewardsTracker{
							StartPeriodCumulativeReward: 0,
							TotalScore:                  math.NewInt(-100), // negative score
						},
					},
				},
			},
			expErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.genesis.Validate()
			if tc.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCurrentRewardsEntryValidate(t *testing.T) {
	validCoins := sdk.NewCoins(sdk.NewCoin("ubbn", math.NewInt(1000)))
	validScore := math.NewInt(100)

	tests := []struct {
		name   string
		entry  types.CurrentRewardsEntry
		expErr bool
	}{
		{
			name:   "empty entry is valid",
			entry:  types.CurrentRewardsEntry{},
			expErr: false,
		},
		{
			name: "valid entry",
			entry: types.CurrentRewardsEntry{
				Rewards: &types.CurrentRewards{
					Rewards:    validCoins,
					Period:     1,
					TotalScore: validScore,
				},
			},
			expErr: false,
		},
		{
			name: "invalid negative coins",
			entry: types.CurrentRewardsEntry{
				Rewards: &types.CurrentRewards{
					Rewards:    sdk.Coins{sdk.Coin{Denom: "ubbn", Amount: math.NewInt(-100)}},
					Period:     1,
					TotalScore: validScore,
				},
			},
			expErr: true,
		},
		{
			name: "invalid negative total score",
			entry: types.CurrentRewardsEntry{
				Rewards: &types.CurrentRewards{
					Rewards:    validCoins,
					Period:     1,
					TotalScore: math.NewInt(-100),
				},
			},
			expErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Validate()
			if tc.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHistoricalRewardsEntryValidate(t *testing.T) {
	validCoins := sdk.NewCoins(sdk.NewCoin("ubbn", math.NewInt(1000)))

	tests := []struct {
		name   string
		entry  types.HistoricalRewardsEntry
		expErr bool
	}{
		{
			name: "valid entry",
			entry: types.HistoricalRewardsEntry{
				Period: 1,
				Rewards: &types.HistoricalRewards{
					CumulativeRewardsPerScore: validCoins,
				},
			},
			expErr: false,
		},
		{
			name: "nil rewards",
			entry: types.HistoricalRewardsEntry{
				Period:  1,
				Rewards: nil,
			},
			expErr: true,
		},
		{
			name: "invalid negative coins",
			entry: types.HistoricalRewardsEntry{
				Period: 1,
				Rewards: &types.HistoricalRewards{
					CumulativeRewardsPerScore: sdk.Coins{sdk.Coin{Denom: "ubbn", Amount: math.NewInt(-100)}},
				},
			},
			expErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Validate()
			if tc.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCoostakerRewardsTrackerEntryValidate(t *testing.T) {
	validAddress := datagen.GenRandomAccount().Address
	validScore := math.NewInt(100)

	tests := []struct {
		name   string
		entry  types.CoostakerRewardsTrackerEntry
		expErr bool
	}{
		{
			name: "valid entry",
			entry: types.CoostakerRewardsTrackerEntry{
				CoostakerAddress: validAddress,
				Tracker: &types.CoostakerRewardsTracker{
					StartPeriodCumulativeReward: 0,
					TotalScore:                  validScore,
				},
			},
			expErr: false,
		},
		{
			name: "invalid address",
			entry: types.CoostakerRewardsTrackerEntry{
				CoostakerAddress: "invalid-address",
				Tracker: &types.CoostakerRewardsTracker{
					StartPeriodCumulativeReward: 0,
					TotalScore:                  validScore,
				},
			},
			expErr: true,
		},
		{
			name: "empty address",
			entry: types.CoostakerRewardsTrackerEntry{
				CoostakerAddress: "",
				Tracker: &types.CoostakerRewardsTracker{
					StartPeriodCumulativeReward: 0,
					TotalScore:                  validScore,
				},
			},
			expErr: true,
		},
		{
			name: "nil tracker",
			entry: types.CoostakerRewardsTrackerEntry{
				CoostakerAddress: validAddress,
				Tracker:          nil,
			},
			expErr: true,
		},
		{
			name: "negative total score",
			entry: types.CoostakerRewardsTrackerEntry{
				CoostakerAddress: validAddress,
				Tracker: &types.CoostakerRewardsTracker{
					StartPeriodCumulativeReward: 0,
					TotalScore:                  math.NewInt(-100),
				},
			},
			expErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Validate()
			if tc.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
