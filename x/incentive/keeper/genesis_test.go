package keeper_test

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	"github.com/babylonlabs-io/babylon/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
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
			},
			akMockResp: func(m *types.MockAccountKeeper) {
				// mock account keeper to return an account on GetAccount call
				m.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(acc).Times(1)
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
			name: "Invalid address (account does not exist)",
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
			ctrl       = gomock.NewController(t)
			r          = rand.New(rand.NewSource(seed))
			ak         = types.NewMockAccountKeeper(ctrl)
			k, ctx     = keepertest.IncentiveKeeper(t, nil, ak, nil)
			len        = int(math.Abs(float64(r.Int() % 50))) // cap it to 50 entries
			bsg        = make([]types.BTCStakingGaugeEntry, len)
			rg         = make([]types.RewardGaugeEntry, len)
			currHeight = datagen.RandomInt(r, 100000)
		)
		defer ctrl.Finish()
		ctx = ctx.WithBlockHeight(int64(currHeight))

		for i := 0; i < len; i++ {
			bsg[i] = types.BTCStakingGaugeEntry{
				Height: uint64(rand.Intn(int(currHeight))),
				Gauge:  datagen.GenRandomGauge(r),
			}
			acc := datagen.GenRandomAccount()
			// mock account keeper to return an account on GetAccount call
			ak.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(acc).Times(1)

			rg[i] = types.RewardGaugeEntry{
				StakeholderType: datagen.GenRandomStakeholderType(r),
				Address:         acc.Address,
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
