package types_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
)

func TestParamsValidate(t *testing.T) {
	tests := []struct {
		name   string
		params types.Params
		expErr error
	}{
		{
			name:   "valid params with default values",
			params: types.DefaultParams(),
			expErr: nil,
		},
		{
			name: "valid params with custom values",
			params: types.Params{
				CoostakingPortion:   math.LegacyMustNewDecFromStr("0.5"),
				ScoreRatioBtcByBaby: math.NewInt(100),
			},
			expErr: nil,
		},
		{
			name: "valid params with minimum values",
			params: types.Params{
				CoostakingPortion:   math.LegacyNewDec(0),
				ScoreRatioBtcByBaby: math.OneInt(),
			},
			expErr: nil,
		},
		{
			name: "valid params with maximum coostaking portion",
			params: types.Params{
				CoostakingPortion:   math.LegacyMustNewDecFromStr("0.999999999999999999"),
				ScoreRatioBtcByBaby: math.NewInt(50),
			},
			expErr: nil,
		},
		{
			name: "nil coostaking portion",
			params: types.Params{
				CoostakingPortion:   math.LegacyDec{},
				ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
			},
			expErr: types.ErrInvalidCoostakingPortion,
		},
		{
			name: "coostaking portion equal to 1",
			params: types.Params{
				CoostakingPortion:   math.LegacyOneDec(),
				ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
			},
			expErr: types.ErrCoostakingPortionTooHigh,
		},
		{
			name: "coostaking portion greater than 1",
			params: types.Params{
				CoostakingPortion:   math.LegacyMustNewDecFromStr("1.5"),
				ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
			},
			expErr: types.ErrCoostakingPortionTooHigh,
		},
		{
			name: "nil score ratio btc by baby",
			params: types.Params{
				CoostakingPortion:   types.DefaultCoostakingPortion,
				ScoreRatioBtcByBaby: math.Int{},
			},
			expErr: types.ErrInvalidScoreRatioBtcByBaby,
		},
		{
			name: "score ratio btc by baby less than 1",
			params: types.Params{
				CoostakingPortion:   types.DefaultCoostakingPortion,
				ScoreRatioBtcByBaby: math.ZeroInt(),
			},
			expErr: types.ErrScoreRatioTooLow,
		},
		{
			name: "negative score ratio btc by baby",
			params: types.Params{
				CoostakingPortion:   types.DefaultCoostakingPortion,
				ScoreRatioBtcByBaby: math.NewInt(-10),
			},
			expErr: types.ErrScoreRatioTooLow,
		},
		{
			name: "both fields invalid",
			params: types.Params{
				CoostakingPortion:   math.LegacyDec{},
				ScoreRatioBtcByBaby: math.Int{},
			},
			expErr: types.ErrInvalidCoostakingPortion,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.params.Validate()

			if tc.expErr != nil {
				require.EqualError(t, err, tc.expErr.Error())
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestDefaultParams(t *testing.T) {
	params := types.DefaultParams()

	require.Equal(t, types.DefaultCoostakingPortion, params.CoostakingPortion)
	require.Equal(t, types.DefaultScoreRatioBtcByBaby, params.ScoreRatioBtcByBaby)

	err := params.Validate()
	require.NoError(t, err)
}
