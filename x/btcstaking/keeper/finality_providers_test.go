package keeper_test

import (
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/math"
	testutil "github.com/babylonlabs-io/babylon/v4/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/require"
)

func TestUpdateFinalityProviderCommission(t *testing.T) {
	t.Parallel()
	var (
		now       = time.Now()
		r         = rand.New(rand.NewSource(10))
		randomDec = func() *math.LegacyDec {
			val := datagen.RandomLegacyDec(r, 10, 1)
			return &val
		}
	)

	testCases := []struct {
		name          string
		newCommission *math.LegacyDec
		fp            types.FinalityProvider
		minCommission math.LegacyDec
		expectedErr   error
	}{
		{
			name:          "nil commission (no-op)",
			newCommission: nil,
			fp: types.FinalityProvider{
				Commission: randomDec(),
				CommissionInfo: types.NewCommissionInfoWithTime(
					math.LegacyOneDec(),
					math.LegacyOneDec(),
					now.Add(-48*time.Hour).UTC(),
				),
			},
			minCommission: math.LegacyZeroDec(),
			expectedErr:   nil,
		},
		{
			name:          "commission updated within 24h (fails)",
			newCommission: randomDec(),
			fp: types.FinalityProvider{
				Commission: randomDec(),
				CommissionInfo: types.NewCommissionInfoWithTime(
					math.LegacyOneDec(),
					math.LegacyOneDec(),
					now.Add(-12*time.Hour).UTC(),
				),
			},
			minCommission: math.LegacyZeroDec(),
			expectedErr:   stktypes.ErrCommissionUpdateTime,
		},
		{
			name: "commission below min rate (fails)",
			newCommission: func() *math.LegacyDec {
				val := math.LegacyNewDecWithPrec(1, 2)
				return &val
			}(), // 0.01
			fp: types.FinalityProvider{
				Commission: func() *math.LegacyDec {
					val := math.LegacyNewDecWithPrec(5, 1)
					return &val
				}(), // 0.5
				CommissionInfo: types.NewCommissionInfoWithTime(
					math.LegacyOneDec(),
					math.LegacyOneDec(),
					now.Add(-48*time.Hour).UTC(),
				),
			},
			minCommission: math.LegacyNewDecWithPrec(2, 1), // 0.2
			expectedErr:   types.ErrCommissionLTMinRate,
		},
		{
			name: "commission change exceeds max allowed change rate (fails)",
			newCommission: func() *math.LegacyDec {
				val := math.LegacyNewDecWithPrec(9, 1)
				return &val
			}(), // 0.9
			fp: types.FinalityProvider{
				Commission: func() *math.LegacyDec {
					val := math.LegacyNewDecWithPrec(5, 1)
					return &val
				}(), // 0.5
				CommissionInfo: types.NewCommissionInfoWithTime(
					math.LegacyOneDec(),
					math.LegacyNewDecWithPrec(3, 1), // 0.3 max rate change
					now.Add(-48*time.Hour).UTC(),
				),
			},
			minCommission: math.LegacyZeroDec(),
			expectedErr:   stktypes.ErrCommissionGTMaxChangeRate,
		},
		{
			name: "valid commission update (success)",
			newCommission: func() *math.LegacyDec {
				val := math.LegacyNewDecWithPrec(6, 1)
				return &val
			}(), // 0.6
			fp: types.FinalityProvider{
				Commission: func() *math.LegacyDec {
					val := math.LegacyNewDecWithPrec(5, 1)
					return &val
				}(), // 0.5
				CommissionInfo: types.NewCommissionInfoWithTime(
					math.LegacyOneDec(),
					math.LegacyNewDecWithPrec(2, 1), // 0.2 max rate change
					now.Add(-48*time.Hour).UTC(),
				),
			},
			minCommission: math.LegacyNewDecWithPrec(1, 2), // 0.01
			expectedErr:   nil,
		},
		{
			name: "commission change exceeds max allowed change rate (fail)",
			newCommission: func() *math.LegacyDec {
				val := math.LegacyNewDecWithPrec(2, 1)
				return &val
			}(), // 0.2
			fp: types.FinalityProvider{
				Commission: func() *math.LegacyDec {
					val := math.LegacyNewDecWithPrec(8, 1)
					return &val
				}(), // 0.8
				CommissionInfo: types.NewCommissionInfoWithTime(
					math.LegacyOneDec(),
					math.LegacyNewDecWithPrec(3, 1),
					now.Add(-48*time.Hour).UTC(),
				),
			},
			minCommission: math.LegacyZeroDec(),
			expectedErr:   stktypes.ErrCommissionGTMaxChangeRate,
		},
		{
			name: "commission above max rate (fail)",
			newCommission: func() *math.LegacyDec {
				val := math.LegacyNewDecWithPrec(9, 1)
				return &val
			}(), // 0.9
			fp: types.FinalityProvider{
				Commission: func() *math.LegacyDec {
					val := math.LegacyNewDecWithPrec(7, 1)
					return &val
				}(), // 0.7
				CommissionInfo: types.NewCommissionInfoWithTime(
					math.LegacyNewDecWithPrec(8, 1),
					math.LegacyNewDecWithPrec(3, 1),
					now.Add(-48*time.Hour).UTC(),
				),
			},
			minCommission: math.LegacyZeroDec(),
			expectedErr:   stktypes.ErrCommissionGTMaxRate,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := testutil.NewHelper(t, nil, nil, nil)

			params := h.BTCStakingKeeper.GetParams(h.Ctx)
			params.MinCommissionRate = tc.minCommission
			params.BtcActivationHeight = 1
			require.NoError(t, h.BTCStakingKeeper.SetParams(h.Ctx, params))

			ctx := h.Ctx.WithBlockTime(now.UTC())
			err := h.BTCStakingKeeper.UpdateFinalityProviderCommission(ctx, tc.newCommission, &tc.fp)

			if tc.expectedErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
				if tc.newCommission != nil {
					require.Equal(t, tc.newCommission, tc.fp.Commission)
					require.Equal(t, now.UTC(), tc.fp.CommissionInfo.UpdateTime)
				}
			}
		})
	}
}

func FuzzSlashConsumerFinalityProvider(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()

		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)
		h.GenAndApplyParams(r)

		// create a consumer finality provider
		_, _, fp := h.CreateFinalityProvider(r)
		fpBTCPK := fp.BtcPk

		// Verify consumer FP exists and is not slashed initially
		retrievedConsumerFP, err := h.BTCStakingKeeper.GetFinalityProvider(h.Ctx, fpBTCPK.MustMarshal())
		require.NoError(t, err)
		require.False(t, retrievedConsumerFP.IsSlashed())

		// Set up BTC tip info for slashing
		btcTip := &btclctypes.BTCHeaderInfo{
			Height: 100,
		}
		btclcKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(btcTip).AnyTimes()

		// Slash the consumer finality provider using SlashFinalityProvider
		// This tests the fix for issue #948 - SlashFinalityProvider should handle consumer FPs
		err = h.BTCStakingKeeper.SlashFinalityProvider(h.Ctx, fpBTCPK.MustMarshal())
		require.NoError(t, err)

		// Verify the consumer FP is slashed
		slashedConsumerFP, err := h.BTCStakingKeeper.GetFinalityProvider(h.Ctx, fpBTCPK.MustMarshal())
		require.NoError(t, err)
		require.True(t, slashedConsumerFP.IsSlashed())
		require.Greater(t, slashedConsumerFP.SlashedBabylonHeight, uint64(0))
		require.Equal(t, btcTip.Height, slashedConsumerFP.SlashedBtcHeight)
	})
}

func FuzzHasFpRegistered(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		h := testutil.NewHelper(t, nil, nil)

		randAddr := datagen.GenRandomAddress()

		registered, err := h.BTCStakingKeeper.HasFpRegistered(h.Ctx, randAddr)
		require.NoError(t, err)
		require.False(t, registered)

		err = h.BTCStakingKeeper.SetFpBbnAddr(h.Ctx, randAddr)
		require.NoError(t, err)

		registered, err = h.BTCStakingKeeper.HasFpRegistered(h.Ctx, randAddr)
		require.NoError(t, err)
		require.True(t, registered)
	})
}

func FuzzIsFinalityProviderDeleted(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))
		h := testutil.NewHelper(t, nil, nil)

		randFpBtcPk, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(t, err)

		deleted := h.BTCStakingKeeper.IsFinalityProviderDeleted(h.Ctx, randFpBtcPk)
		require.False(t, deleted)

		err = h.BTCStakingKeeper.SoftDeleteFinalityProvider(h.Ctx, randFpBtcPk)
		require.NoError(t, err)

		deleted = h.BTCStakingKeeper.IsFinalityProviderDeleted(h.Ctx, randFpBtcPk)
		require.NoError(t, err)
		require.True(t, deleted)
	})
}

func FuzzIterateFinalityProvider(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))
		h := testutil.NewHelper(t, nil, nil)

		numFps := datagen.RandomInt(r, 10) + 1

		fpByBtcPk := make(map[string]struct{}, numFps)
		for i := 0; i < int(numFps); i++ {
			fp, err := datagen.GenRandomFinalityProvider(r)
			require.NoError(t, err)
			msg := &types.MsgCreateFinalityProvider{
				Addr:        fp.Addr,
				Description: fp.Description,
				Commission: types.NewCommissionRates(
					*fp.Commission,
					fp.CommissionInfo.MaxRate,
					fp.CommissionInfo.MaxChangeRate,
				),
				BtcPk: fp.BtcPk,
				Pop:   fp.Pop,
			}
			_, err = h.MsgServer.CreateFinalityProvider(h.Ctx, msg)
			require.NoError(t, err)
			fpByBtcPk[fp.BtcPk.MarshalHex()] = struct{}{}
		}

		iter := uint64(0)
		err := h.BTCStakingKeeper.IterateFinalityProvider(h.Ctx, func(fp types.FinalityProvider) error {
			delete(fpByBtcPk, fp.BtcPk.MarshalHex())
			iter++
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, iter, numFps)
		require.Len(t, fpByBtcPk, 0)
	})
}
