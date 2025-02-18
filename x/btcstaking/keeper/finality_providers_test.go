package keeper_test

import (
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/math"
	testutil "github.com/babylonlabs-io/babylon/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestUpdateFinalityProviderCommission(t *testing.T) {
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
		maxRateChange math.LegacyDec
		expectedErr   error
	}{
		{
			name:          "nil commission (no-op)",
			newCommission: nil,
			fp: types.FinalityProvider{
				Commission:           randomDec(),
				CommissionUpdateTime: now.Add(-48 * time.Hour).UTC(),
			},
			minCommission: math.LegacyZeroDec(),
			maxRateChange: math.LegacyOneDec(),
			expectedErr:   nil,
		},
		{
			name:          "commission updated within 24h (fails)",
			newCommission: randomDec(),
			fp: types.FinalityProvider{
				Commission:           randomDec(),
				CommissionUpdateTime: now.Add(-12 * time.Hour).UTC(),
			},
			minCommission: math.LegacyZeroDec(),
			maxRateChange: math.LegacyOneDec(),
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
				CommissionUpdateTime: now.Add(-48 * time.Hour).UTC(),
			},
			minCommission: math.LegacyNewDecWithPrec(2, 1), // 0.2
			maxRateChange: math.LegacyOneDec(),
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
				CommissionUpdateTime: now.Add(-48 * time.Hour).UTC(),
			},
			minCommission: math.LegacyZeroDec(),
			maxRateChange: math.LegacyNewDecWithPrec(3, 1), // 0.3
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
				CommissionUpdateTime: now.Add(-48 * time.Hour).UTC(),
			},
			minCommission: math.LegacyNewDecWithPrec(1, 2), // 0.01
			maxRateChange: math.LegacyNewDecWithPrec(2, 1), // 0.2
			expectedErr:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// mock BTC light client and BTC checkpoint modules
			btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
			btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
			h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

			params := h.BTCStakingKeeper.GetParams(h.Ctx)
			params.MinCommissionRate = tc.minCommission
			params.MaxCommissionChangeRate = tc.maxRateChange
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
					require.Equal(t, now.UTC(), tc.fp.CommissionUpdateTime)
				}
			}
		})
	}
}
