package keeper_test

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	"github.com/babylonlabs-io/babylon/x/incentive/keeper"
	"github.com/babylonlabs-io/babylon/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestInitGenesis(t *testing.T) {
	var (
		r   = rand.New(rand.NewSource(time.Now().Unix()))
		acc = datagen.GenRandomAccount()
	)
	tests := []struct {
		name       string
		gs         types.GenesisState
		akMockResp func(m *types.MockAccountKeeper)
		expectErr  bool
		errMsg     string
	}{
		{
			name: "Valid genesis state",
			gs: types.GenesisState{
				Params: types.DefaultParams(),
				BtcStakingGauges: []types.BTCStakingGaugeEntry{
					{
						Height: uint64(0),
						Gauge:  datagen.GenRandomGauge(r),
					},
				},
				RewardGauges: []types.RewardGaugeEntry{
					{
						Address:     acc.Address,
						RewardGauge: datagen.GenRandomRewardGauge(r),
					},
				},
				WithdrawAddresses: []types.WithdrawAddressEntry{
					{
						DelegatorAddress: acc.Address,
						WithdrawAddress:  datagen.GenRandomAccount().Address,
					},
				},
			},
			akMockResp: func(m *types.MockAccountKeeper) {
				// mock account keeper to return an account on GetAccount call
				m.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(acc).Times(2)
			},
			expectErr: false,
		},
		{
			name: "Invalid block height (future height)",
			gs: types.GenesisState{
				Params: types.DefaultParams(),
				BtcStakingGauges: []types.BTCStakingGaugeEntry{
					{
						Height: uint64(100),
						Gauge:  datagen.GenRandomGauge(r),
					},
				},
			},
			akMockResp: func(m *types.MockAccountKeeper) {},
			expectErr:  true,
			errMsg:     "is higher than current block height",
		},
		{
			name: "Invalid address in gauge (account does not exist)",
			gs: types.GenesisState{
				Params: types.DefaultParams(),
				RewardGauges: []types.RewardGaugeEntry{
					{
						Address:     acc.Address,
						RewardGauge: datagen.GenRandomRewardGauge(r),
					},
				},
			},
			akMockResp: func(m *types.MockAccountKeeper) {
				// mock account does not exist
				m.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			expectErr: true,
			errMsg:    fmt.Sprintf("account in rewards gauge with address %s does not exist", acc.Address),
		},
		{
			name: "Invalid delegator address (account does not exist)",
			gs: types.GenesisState{
				Params: types.DefaultParams(),
				WithdrawAddresses: []types.WithdrawAddressEntry{
					{
						DelegatorAddress: acc.Address,
						WithdrawAddress:  datagen.GenRandomAccount().Address,
					},
				},
			},
			akMockResp: func(m *types.MockAccountKeeper) {
				// mock account does not exist
				m.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			expectErr: true,
			errMsg:    fmt.Sprintf("delegator account with address %s does not exist", acc.Address),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				ctrl   = gomock.NewController(t)
				ak     = types.NewMockAccountKeeper(ctrl)
				k, ctx = keepertest.IncentiveKeeper(t, nil, ak, nil)
			)
			defer ctrl.Finish()
			tc.akMockResp(ak)

			err := k.InitGenesis(ctx, tc.gs)
			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func FuzzTestExportGenesis(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		ctx, k, gs, ctrl := setupTest(t, seed)
		defer ctrl.Finish()

		// Setup current state
		require.NoError(t, k.SetParams(ctx, gs.Params))
		l := len(gs.BtcStakingGauges)
		for i := 0; i < l; i++ {
			k.SetBTCStakingGauge(ctx, gs.BtcStakingGauges[i].Height, gs.BtcStakingGauges[i].Gauge)
			k.SetRewardGauge(ctx, gs.RewardGauges[i].StakeholderType, sdk.MustAccAddressFromBech32(gs.RewardGauges[i].Address), gs.RewardGauges[i].RewardGauge)
			k.SetWithdrawAddr(ctx, sdk.MustAccAddressFromBech32(gs.WithdrawAddresses[i].DelegatorAddress), sdk.MustAccAddressFromBech32(gs.WithdrawAddresses[i].WithdrawAddress))
		}

		// Run the ExportGenesis
		exported, err := k.ExportGenesis(ctx)

		require.NoError(t, err)
		types.SortData(gs)
		types.SortData(exported)
		require.Equal(t, gs, exported, fmt.Sprintf("Found diff: %s | seed %d", cmp.Diff(gs, exported), seed))
	})
}

func FuzzTestInitGenesis(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		ctx, k, gs, ctrl := setupTest(t, seed)
		defer ctrl.Finish()
		// Run the InitGenesis
		err := k.InitGenesis(ctx, *gs)
		require.NoError(t, err)

		// get the current state
		exported, err := k.ExportGenesis(ctx)
		require.NoError(t, err)

		types.SortData(gs)
		types.SortData(exported)
		require.Equal(t, gs, exported, fmt.Sprintf("Found diff: %s | seed %d", cmp.Diff(gs, exported), seed))
	})
}

// setupTest is a helper function to generate a random genesis state
// and setup the incentive keeper with the accounts keeper mock
func setupTest(t *testing.T, seed int64) (sdk.Context, *keeper.Keeper, *types.GenesisState, *gomock.Controller) {
	var (
		ctrl       = gomock.NewController(t)
		r          = rand.New(rand.NewSource(seed))
		ak         = types.NewMockAccountKeeper(ctrl)
		k, ctx     = keepertest.IncentiveKeeper(t, nil, ak, nil)
		l          = int(math.Abs(float64(r.Int() % 50))) // cap it to 50 entries
		bsg        = make([]types.BTCStakingGaugeEntry, l)
		rg         = make([]types.RewardGaugeEntry, l)
		wa         = make([]types.WithdrawAddressEntry, l)
		currHeight = datagen.RandomInt(r, 100000)
	)
	defer ctrl.Finish()
	ctx = ctx.WithBlockHeight(int64(currHeight))

	// make sure that BTC staking gauge are unique per height
	usedHeights := make(map[uint64]bool)
	for i := 0; i < l; i++ {
		bsg[i] = types.BTCStakingGaugeEntry{
			Height: getUniqueHeight(currHeight, usedHeights),
			Gauge:  datagen.GenRandomGauge(r),
		}
		acc := datagen.GenRandomAccount()
		// mock account keeper to return an account on GetAccount call (called 2 times, for the RewardsGauge, and the withdraw addr)
		ak.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(acc).AnyTimes()

		rg[i] = types.RewardGaugeEntry{
			StakeholderType: datagen.GenRandomStakeholderType(r),
			Address:         acc.Address,
			RewardGauge:     datagen.GenRandomRewardGauge(r),
		}
		wa[i] = types.WithdrawAddressEntry{
			DelegatorAddress: acc.Address,
			WithdrawAddress:  datagen.GenRandomAccount().Address,
		}
	}

	gs := &types.GenesisState{
		Params: types.Params{
			BtcStakingPortion: datagen.RandomLegacyDec(r, 10, 1),
		},
		BtcStakingGauges:  bsg,
		RewardGauges:      rg,
		WithdrawAddresses: wa,
	}

	require.NoError(t, gs.Validate())
	return ctx, k, gs, ctrl
}

// getUniqueHeight is a helper function to get a block height
// that hasn't been used yet
func getUniqueHeight(currHeight uint64, usedHeights map[uint64]bool) uint64 {
	var height uint64
	for {
		height = uint64(rand.Intn(int(currHeight)))
		if !usedHeights[height] {
			usedHeights[height] = true
			break
		}
	}
	return height
}
