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
				ValidatorsPortion:   math.LegacyMustNewDecFromStr("0.001"),
			},
			expErr: nil,
		},
		{
			name: "valid params with minimum values",
			params: types.Params{
				CoostakingPortion:   math.LegacyNewDec(0),
				ScoreRatioBtcByBaby: math.OneInt(),
				ValidatorsPortion:   math.LegacyNewDec(0),
			},
			expErr: nil,
		},
		{
			name: "valid params with maximum coostaking portion",
			params: types.Params{
				CoostakingPortion:   math.LegacyMustNewDecFromStr("0.5"),
				ScoreRatioBtcByBaby: math.NewInt(50),
				ValidatorsPortion:   math.LegacyMustNewDecFromStr("0.4"),
			},
			expErr: nil,
		},
		{
			name: "nil coostaking portion",
			params: types.Params{
				CoostakingPortion:   math.LegacyDec{},
				ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
				ValidatorsPortion:   types.DefaultValidatorsPortion,
			},
			expErr: types.ErrInvalidPercentage,
		},
		{
			name: "coostaking portion equal to 1",
			params: types.Params{
				CoostakingPortion:   math.LegacyOneDec(),
				ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
				ValidatorsPortion:   types.DefaultValidatorsPortion,
			},
			expErr: types.ErrPercentageTooHigh,
		},
		{
			name: "coostaking portion greater than 1",
			params: types.Params{
				CoostakingPortion:   math.LegacyMustNewDecFromStr("1.5"),
				ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
				ValidatorsPortion:   types.DefaultValidatorsPortion,
			},
			expErr: types.ErrPercentageTooHigh,
		},
		{
			name: "negative coostaking portion",
			params: types.Params{
				CoostakingPortion:   math.LegacyMustNewDecFromStr("-0.1"),
				ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
				ValidatorsPortion:   types.DefaultValidatorsPortion,
			},
			expErr: types.ErrInvalidPercentage.Wrap("lower than zero"),
		},
		{
			name: "nil score ratio btc by baby",
			params: types.Params{
				CoostakingPortion:   types.DefaultCoostakingPortion,
				ScoreRatioBtcByBaby: math.Int{},
				ValidatorsPortion:   types.DefaultValidatorsPortion,
			},
			expErr: types.ErrInvalidScoreRatioBtcByBaby,
		},
		{
			name: "score ratio btc by baby less than 1",
			params: types.Params{
				CoostakingPortion:   types.DefaultCoostakingPortion,
				ScoreRatioBtcByBaby: math.ZeroInt(),
				ValidatorsPortion:   types.DefaultValidatorsPortion,
			},
			expErr: types.ErrScoreRatioTooLow,
		},
		{
			name: "negative score ratio btc by baby",
			params: types.Params{
				CoostakingPortion:   types.DefaultCoostakingPortion,
				ScoreRatioBtcByBaby: math.NewInt(-10),
				ValidatorsPortion:   types.DefaultValidatorsPortion,
			},
			expErr: types.ErrScoreRatioTooLow,
		},
		{
			name: "both fields invalid",
			params: types.Params{
				CoostakingPortion:   math.LegacyDec{},
				ScoreRatioBtcByBaby: math.Int{},
				ValidatorsPortion:   types.DefaultValidatorsPortion,
			},
			expErr: types.ErrInvalidPercentage,
		},
		{
			name: "nil validators portion",
			params: types.Params{
				CoostakingPortion:   types.DefaultCoostakingPortion,
				ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
				ValidatorsPortion:   math.LegacyDec{},
			},
			expErr: types.ErrInvalidPercentage,
		},
		{
			name: "validators portion equal to 1",
			params: types.Params{
				CoostakingPortion:   types.DefaultCoostakingPortion,
				ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
				ValidatorsPortion:   math.LegacyOneDec(),
			},
			expErr: types.ErrPercentageTooHigh,
		},
		{
			name: "validators portion greater than 1",
			params: types.Params{
				CoostakingPortion:   types.DefaultCoostakingPortion,
				ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
				ValidatorsPortion:   math.LegacyMustNewDecFromStr("1.5"),
			},
			expErr: types.ErrPercentageTooHigh,
		},
		{
			name: "negative validators portion",
			params: types.Params{
				CoostakingPortion:   types.DefaultCoostakingPortion,
				ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
				ValidatorsPortion:   math.LegacyMustNewDecFromStr("-0.01"),
			},
			expErr: types.ErrInvalidPercentage.Wrap("lower than zero"),
		},
		{
			name: "coostaking + validators portion equal to 1",
			params: types.Params{
				CoostakingPortion:   math.LegacyMustNewDecFromStr("0.5"),
				ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
				ValidatorsPortion:   math.LegacyMustNewDecFromStr("0.5"),
			},
			expErr: types.ErrPercentageTooHigh,
		},
		{
			name: "coostaking + validators portion greater than 1",
			params: types.Params{
				CoostakingPortion:   math.LegacyMustNewDecFromStr("0.6"),
				ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
				ValidatorsPortion:   math.LegacyMustNewDecFromStr("0.5"),
			},
			expErr: types.ErrPercentageTooHigh,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.params.Validate()

			if tc.expErr != nil {
				require.ErrorContains(t, err, tc.expErr.Error())
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
	require.Equal(t, types.DefaultValidatorsPortion, params.ValidatorsPortion)

	err := params.Validate()
	require.NoError(t, err)
}
