package keeper_test

import (
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/collections"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v3/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v3/x/incentive/keeper"
	"github.com/babylonlabs-io/babylon/v3/x/incentive/types"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestInitGenesis(t *testing.T) {
	var (
		r    = rand.New(rand.NewSource(time.Now().Unix()))
		acc1 = datagen.GenRandomAccount()
		acc2 = datagen.GenRandomAccount()
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
						Address:     acc1.Address,
						RewardGauge: datagen.GenRandomRewardGauge(r),
					},
				},
				WithdrawAddresses: []types.WithdrawAddressEntry{
					{
						DelegatorAddress: acc1.Address,
						WithdrawAddress:  datagen.GenRandomAccount().Address,
					},
				},
				RefundableMsgHashes: []string{
					genRandomMsgHashStr(),
					genRandomMsgHashStr(),
				},
				FinalityProvidersCurrentRewards: []types.FinalityProviderCurrentRewardsEntry{
					{
						Address: acc1.Address,
						Rewards: func() *types.FinalityProviderCurrentRewards {
							val := datagen.GenRandomFinalityProviderCurrentRewards(r)
							return &val
						}(),
					},
					{
						Address: acc2.Address,
						Rewards: func() *types.FinalityProviderCurrentRewards {
							val := datagen.GenRandomFinalityProviderCurrentRewards(r)
							return &val
						}(),
					},
				},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{
					{
						Address: acc1.Address,
						Period:  datagen.RandomInt(r, 100000),
						Rewards: func() *types.FinalityProviderHistoricalRewards {
							val := datagen.GenRandomFPHistRwd(r)
							return &val
						}(),
					},
					{
						Address: acc2.Address,
						Period:  datagen.RandomInt(r, 100000),
						Rewards: func() *types.FinalityProviderHistoricalRewards {
							val := datagen.GenRandomFPHistRwd(r)
							return &val
						}(),
					},
				},
				BtcDelegationRewardsTrackers: []types.BTCDelegationRewardsTrackerEntry{
					{
						FinalityProviderAddress: acc1.Address,
						DelegatorAddress:        acc2.Address,
						Tracker: func() *types.BTCDelegationRewardsTracker {
							val := datagen.GenRandomBTCDelegationRewardsTracker(r)
							return &val
						}(),
					},
					{
						FinalityProviderAddress: acc2.Address,
						DelegatorAddress:        acc1.Address,
						Tracker: func() *types.BTCDelegationRewardsTracker {
							val := datagen.GenRandomBTCDelegationRewardsTracker(r)
							return &val
						}(),
					},
				},
				BtcDelegatorsToFps: []types.BTCDelegatorToFpEntry{
					{
						DelegatorAddress:        acc2.Address,
						FinalityProviderAddress: acc1.Address,
					},
					{
						DelegatorAddress:        acc1.Address,
						FinalityProviderAddress: acc2.Address,
					},
				},
				EventRewardTracker: []types.EventsPowerUpdateAtHeightEntry{
					{
						Height: datagen.RandomInt(r, 100000) + 1,
						Events: &types.EventsPowerUpdateAtHeight{
							Events: []*types.EventPowerUpdate{
								types.NewEventBtcDelegationActivated(acc1.Address, acc2.Address, datagen.RandomMathInt(r, 1000).AddRaw(20)),
							},
						},
					},
				},
				LastProcessedHeightEventRewardTracker: 0, // current block height is zero
			},
			akMockResp: func(m *types.MockAccountKeeper) {
				// mock account keeper to return an account on GetAccount call
				m.EXPECT().GetAccount(gomock.Any(), acc1.GetAddress()).Return(acc1).AnyTimes()
				m.EXPECT().GetAccount(gomock.Any(), acc2.GetAddress()).Return(acc2).AnyTimes()
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
						Address:     acc1.Address,
						RewardGauge: datagen.GenRandomRewardGauge(r),
					},
				},
			},
			akMockResp: func(m *types.MockAccountKeeper) {
				// mock account does not exist
				m.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			expectErr: true,
			errMsg:    fmt.Sprintf("account in rewards gauge with address %s does not exist", acc1.Address),
		},
		{
			name: "Invalid delegator address (account does not exist)",
			gs: types.GenesisState{
				Params: types.DefaultParams(),
				WithdrawAddresses: []types.WithdrawAddressEntry{
					{
						DelegatorAddress: acc1.Address,
						WithdrawAddress:  datagen.GenRandomAccount().Address,
					},
				},
			},
			akMockResp: func(m *types.MockAccountKeeper) {
				// mock account does not exist
				m.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			expectErr: true,
			errMsg:    fmt.Sprintf("delegator account with address %s does not exist", acc1.Address),
		},
		{
			name: "Invalid msg hash string",
			gs: types.GenesisState{
				Params:              types.DefaultParams(),
				RefundableMsgHashes: []string{"invalid_hash"},
			},
			akMockResp: func(m *types.MockAccountKeeper) {},
			expectErr:  true,
			errMsg:     "error decoding msg hash",
		},
		{
			name: "Invalid block height (future height)",
			gs: types.GenesisState{
				Params: types.DefaultParams(),
				BtcStakingGauges: []types.BTCStakingGaugeEntry{
					{
						Height: uint64(0),
						Gauge:  datagen.GenRandomGauge(r),
					},
				},
				LastProcessedHeightEventRewardTracker: 10, // current block height is zero
			},
			akMockResp: func(m *types.MockAccountKeeper) {},
			expectErr:  true,
			errMsg:     "invalid latest processed block height",
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
		ctx, k, sk, gs, ctrl := setupTest(t, seed)
		defer ctrl.Finish()

		var (
			encConfig    = appparams.DefaultEncodingConfig()
			storeService = runtime.NewKVStoreService(sk)
			store        = storeService.OpenKVStore(ctx)
			storeAdaptor = runtime.KVStoreAdapter(store)
		)

		// Setup current state
		require.NoError(t, k.SetParams(ctx, gs.Params))
		l := len(gs.BtcStakingGauges)
		for i := 0; i < l; i++ {
			k.SetBTCStakingGauge(ctx, gs.BtcStakingGauges[i].Height, gs.BtcStakingGauges[i].Gauge)
			k.SetRewardGauge(ctx, gs.RewardGauges[i].StakeholderType, sdk.MustAccAddressFromBech32(gs.RewardGauges[i].Address), gs.RewardGauges[i].RewardGauge)
			k.SetWithdrawAddr(ctx, sdk.MustAccAddressFromBech32(gs.WithdrawAddresses[i].DelegatorAddress), sdk.MustAccAddressFromBech32(gs.WithdrawAddresses[i].WithdrawAddress))
			bz, err := hex.DecodeString(gs.RefundableMsgHashes[i])
			require.NoError(t, err)
			k.RefundableMsgKeySet.Set(ctx, bz)

			// FP current rewards
			fpCurrRwdKeyBz, err := collections.EncodeKeyWithPrefix(types.FinalityProviderCurrentRewardsKeyPrefix.Bytes(), collections.BytesKey, sdk.MustAccAddressFromBech32(gs.FinalityProvidersCurrentRewards[i].Address).Bytes())
			require.NoError(t, err)

			currRwdBz, err := codec.CollValue[types.FinalityProviderCurrentRewards](encConfig.Codec).Encode(*gs.FinalityProvidersCurrentRewards[i].Rewards)
			require.NoError(t, err)
			require.NoError(t, store.Set(fpCurrRwdKeyBz, currRwdBz))

			// BTCDelegationRewardsTracker
			bdrtKey := collections.Join(
				sdk.MustAccAddressFromBech32(gs.BtcDelegationRewardsTrackers[i].FinalityProviderAddress).Bytes(),
				sdk.MustAccAddressFromBech32(gs.BtcDelegationRewardsTrackers[i].DelegatorAddress).Bytes(),
			)

			bdrtKeyBz, err := collections.EncodeKeyWithPrefix(types.BTCDelegationRewardsTrackerKeyPrefix.Bytes(), collections.PairKeyCodec(collections.BytesKey, collections.BytesKey), bdrtKey)
			require.NoError(t, err)

			bdrtBz, err := codec.CollValue[types.BTCDelegationRewardsTracker](encConfig.Codec).Encode(*gs.BtcDelegationRewardsTrackers[i].Tracker)
			require.NoError(t, err)
			require.NoError(t, store.Set(bdrtKeyBz, bdrtBz))

			// btcDel2FP
			st := prefix.NewStore(storeAdaptor, types.BTCDelegatorToFPKey)
			delAcc := sdk.MustAccAddressFromBech32(gs.BtcDelegatorsToFps[i].DelegatorAddress)
			fpAcc := sdk.MustAccAddressFromBech32(gs.BtcDelegatorsToFps[i].FinalityProviderAddress)
			delStore := prefix.NewStore(st, delAcc.Bytes())
			delStore.Set(fpAcc.Bytes(), []byte{0x00})

			require.NoError(t, k.SetRewardTrackerEvent(ctx, gs.EventRewardTracker[i].Height, gs.EventRewardTracker[i].Events))
		}

		for _, fpHistRwd := range gs.FinalityProvidersHistoricalRewards {
			fphrKey := collections.Join(
				sdk.MustAccAddressFromBech32(fpHistRwd.Address).Bytes(),
				fpHistRwd.Period,
			)

			fphrKeyBz, err := collections.EncodeKeyWithPrefix(types.FinalityProviderHistoricalRewardsKeyPrefix.Bytes(), collections.PairKeyCodec(collections.BytesKey, collections.Uint64Key), fphrKey)
			require.NoError(t, err)

			histRwdBz, err := codec.CollValue[types.FinalityProviderHistoricalRewards](encConfig.Codec).Encode(*fpHistRwd.Rewards)
			require.NoError(t, err)
			require.NoError(t, store.Set(fphrKeyBz, histRwdBz))
		}

		require.NoError(t, k.SetRewardTrackerEventLastProcessedHeight(ctx, gs.LastProcessedHeightEventRewardTracker))

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
		ctx, k, _, gs, ctrl := setupTest(t, seed)
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
func setupTest(t *testing.T, seed int64) (sdk.Context, *keeper.Keeper, *storetypes.KVStoreKey, *types.GenesisState, *gomock.Controller) {
	var (
		ctrl       = gomock.NewController(t)
		r          = rand.New(rand.NewSource(seed))
		ak         = types.NewMockAccountKeeper(ctrl)
		storeKey   = storetypes.NewKVStoreKey(types.StoreKey)
		k, ctx     = keepertest.IncentiveKeeperWithStoreKey(t, storeKey, nil, ak, nil)
		l          = int(math.Abs(float64(r.Int() % 50))) // cap it to 50 entries
		bsg        = make([]types.BTCStakingGaugeEntry, l)
		rg         = make([]types.RewardGaugeEntry, l)
		wa         = make([]types.WithdrawAddressEntry, l)
		mh         = make([]string, l)
		fpCurrRwd  = make([]types.FinalityProviderCurrentRewardsEntry, l)
		bdrt       = make([]types.BTCDelegationRewardsTrackerEntry, l)
		d2fp       = make([]types.BTCDelegatorToFpEntry, l)
		eventsRwd  = make([]types.EventsPowerUpdateAtHeightEntry, l)
		currHeight = datagen.RandomInt(r, 100000) + 100
		fpHistRwd  []types.FinalityProviderHistoricalRewardsEntry
	)
	defer ctrl.Finish()
	ctx = ctx.WithBlockHeight(int64(currHeight))

	// make sure that BTC staking gauge are unique per height
	usedHeights := make(map[uint64]bool)

	// Pre-generate all current periods to ensure consistency
	currentPeriods := make([]uint64, l)
	totalHistoricalRewards := 0
	for i := 0; i < l; i++ {
		currentPeriods[i] = uint64(r.Intn(4)) + 1
		totalHistoricalRewards += int(currentPeriods[i])
	}

	fpHistRwd = make([]types.FinalityProviderHistoricalRewardsEntry, totalHistoricalRewards)

	histIdx := 0
	for i := 0; i < l; i++ {
		blkHeight := getUniqueHeight(currHeight, usedHeights)
		acc1 := datagen.GenRandomAccount()
		acc2 := datagen.GenRandomAccount()
		currentPeriod := currentPeriods[i]

		// mock account keeper to return an account on GetAccount calls
		ak.EXPECT().GetAccount(gomock.Any(), acc1.GetAddress()).Return(acc1).AnyTimes()
		ak.EXPECT().GetAccount(gomock.Any(), acc2.GetAddress()).Return(acc2).AnyTimes()

		bsg[i] = types.BTCStakingGaugeEntry{
			Height: blkHeight,
			Gauge:  datagen.GenRandomGauge(r),
		}

		rg[i] = types.RewardGaugeEntry{
			StakeholderType: datagen.GenRandomStakeholderType(r),
			Address:         acc1.Address,
			RewardGauge:     datagen.GenRandomRewardGauge(r),
		}
		wa[i] = types.WithdrawAddressEntry{
			DelegatorAddress: acc1.Address,
			WithdrawAddress:  datagen.GenRandomAccount().Address,
		}
		mh[i] = genRandomMsgHashStr()

		fpCurrRwd[i] = types.FinalityProviderCurrentRewardsEntry{
			Address: acc1.Address,
			Rewards: func() *types.FinalityProviderCurrentRewards {
				val := datagen.GenRandomFinalityProviderCurrentRewards(r)
				val.Period = currentPeriod
				return &val
			}(),
		}

		for period := uint64(0); period < currentPeriod; period++ {
			fpHistRwd[histIdx] = types.FinalityProviderHistoricalRewardsEntry{
				Address: acc1.Address,
				Period:  period,
				Rewards: func() *types.FinalityProviderHistoricalRewards {
					val := datagen.GenRandomFPHistRwd(r)
					return &val
				}(),
			}
			histIdx++
		}

		bdrt[i] = types.BTCDelegationRewardsTrackerEntry{
			FinalityProviderAddress: acc1.Address,
			DelegatorAddress:        acc2.Address,
			Tracker: func() *types.BTCDelegationRewardsTracker {
				val := datagen.GenRandomBTCDelegationRewardsTracker(r)
				if currentPeriod > 0 {
					val.StartPeriodCumulativeReward = uint64(r.Intn(int(currentPeriod)))
				} else {
					val.StartPeriodCumulativeReward = 0
				}
				return &val
			}(),
		}
		d2fp[i] = types.BTCDelegatorToFpEntry{
			DelegatorAddress:        acc2.Address,
			FinalityProviderAddress: acc1.Address,
		}
		eventsRwd[i] = types.EventsPowerUpdateAtHeightEntry{
			Height: blkHeight,
			Events: &types.EventsPowerUpdateAtHeight{
				Events: []*types.EventPowerUpdate{
					types.NewEventBtcDelegationActivated(acc2.Address, acc1.Address, datagen.RandomMathInt(r, 2000).AddRaw(100)),
				},
			},
		}
	}

	gs := &types.GenesisState{
		Params: types.Params{
			BtcStakingPortion: datagen.RandomLegacyDec(r, 10, 1),
		},
		BtcStakingGauges:                      bsg,
		RewardGauges:                          rg,
		WithdrawAddresses:                     wa,
		RefundableMsgHashes:                   mh,
		FinalityProvidersCurrentRewards:       fpCurrRwd,
		FinalityProvidersHistoricalRewards:    fpHistRwd,
		BtcDelegationRewardsTrackers:          bdrt,
		BtcDelegatorsToFps:                    d2fp,
		EventRewardTracker:                    eventsRwd,
		LastProcessedHeightEventRewardTracker: currHeight,
	}

	require.NoError(t, gs.Validate())
	return ctx, k, storeKey, gs, ctrl
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

func genRandomMsgHashStr() string {
	msg := types.MsgWithdrawReward{
		Address: datagen.GenRandomAccount().Address,
	}
	msgHash := types.HashMsg(&msg)
	return hex.EncodeToString(msgHash)
}
