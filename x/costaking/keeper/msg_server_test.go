package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	testkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
)

func TestMsgUpdateParams(t *testing.T) {
	t.Parallel()
	tcs := []struct {
		name     string
		setupMsg func() *types.MsgUpdateParams
		expErr   error
	}{
		{
			name: "valid params update",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: appparams.AccGov.String(),
					Params: types.Params{
						CostakingPortion:   math.LegacyMustNewDecFromStr("0.5"),
						ScoreRatioBtcByBaby: math.NewInt(100),
						ValidatorsPortion:   math.LegacyMustNewDecFromStr("0.001"),
					},
				}
			},
			expErr: nil,
		},
		{
			name: "valid params update with default values",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: appparams.AccGov.String(),
					Params:    types.DefaultParams(),
				}
			},
			expErr: nil,
		},
		{
			name: "invalid authority",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: "invalid_authority",
					Params:    types.DefaultParams(),
				}
			},
			expErr: govtypes.ErrInvalidSigner,
		},
		{
			name: "nil costaking portion",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: appparams.AccGov.String(),
					Params: types.Params{
						CostakingPortion:   math.LegacyDec{},
						ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
						ValidatorsPortion:   types.DefaultValidatorsPortion,
					},
				}
			},
			expErr: govtypes.ErrInvalidProposalMsg,
		},
		{
			name: "costaking portion too high",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: appparams.AccGov.String(),
					Params: types.Params{
						CostakingPortion:   math.LegacyMustNewDecFromStr("1.5"),
						ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
						ValidatorsPortion:   types.DefaultValidatorsPortion,
					},
				}
			},
			expErr: govtypes.ErrInvalidProposalMsg,
		},
		{
			name: "costaking portion equal to 1",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: appparams.AccGov.String(),
					Params: types.Params{
						CostakingPortion:   math.LegacyOneDec(),
						ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
						ValidatorsPortion:   types.DefaultValidatorsPortion,
					},
				}
			},
			expErr: govtypes.ErrInvalidProposalMsg,
		},
		{
			name: "nil score ratio btc by baby",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: appparams.AccGov.String(),
					Params: types.Params{
						CostakingPortion:   types.DefaultCostakingPortion,
						ScoreRatioBtcByBaby: math.Int{},
						ValidatorsPortion:   types.DefaultValidatorsPortion,
					},
				}
			},
			expErr: govtypes.ErrInvalidProposalMsg,
		},
		{
			name: "score ratio too low",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: appparams.AccGov.String(),
					Params: types.Params{
						CostakingPortion:   types.DefaultCostakingPortion,
						ScoreRatioBtcByBaby: math.ZeroInt(),
						ValidatorsPortion:   types.DefaultValidatorsPortion,
					},
				}
			},
			expErr: govtypes.ErrInvalidProposalMsg,
		},
		{
			name: "negative score ratio",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: appparams.AccGov.String(),
					Params: types.Params{
						CostakingPortion:   types.DefaultCostakingPortion,
						ScoreRatioBtcByBaby: math.NewInt(-10),
						ValidatorsPortion:   types.DefaultValidatorsPortion,
					},
				}
			},
			expErr: govtypes.ErrInvalidProposalMsg,
		},
		{
			name: "valid params with minimum values",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: appparams.AccGov.String(),
					Params: types.Params{
						CostakingPortion:   math.LegacyNewDec(0),
						ScoreRatioBtcByBaby: math.OneInt(),
						ValidatorsPortion:   math.LegacyNewDec(0),
					},
				}
			},
			expErr: nil,
		},
		{
			name: "valid params with maximum costaking portion",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: appparams.AccGov.String(),
					Params: types.Params{
						CostakingPortion:   math.LegacyMustNewDecFromStr("0.9"),
						ScoreRatioBtcByBaby: math.NewInt(50),
						ValidatorsPortion:   math.LegacyMustNewDecFromStr("0.099"),
					},
				}
			},
			expErr: nil,
		},
		{
			name: "total portion equal to 1",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: appparams.AccGov.String(),
					Params: types.Params{
						CostakingPortion:   math.LegacyMustNewDecFromStr("0.5"),
						ScoreRatioBtcByBaby: math.OneInt(),
						ValidatorsPortion:   math.LegacyMustNewDecFromStr("0.5"),
					},
				}
			},
			expErr: govtypes.ErrInvalidProposalMsg,
		},
		{
			name: "total portion exceeds 1",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: appparams.AccGov.String(),
					Params: types.Params{
						CostakingPortion:   math.LegacyMustNewDecFromStr("0.6"),
						ScoreRatioBtcByBaby: math.OneInt(),
						ValidatorsPortion:   math.LegacyMustNewDecFromStr("0.5"),
					},
				}
			},
			expErr: govtypes.ErrInvalidProposalMsg,
		},
		{
			name: "nil validators portion",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: appparams.AccGov.String(),
					Params: types.Params{
						CostakingPortion:   types.DefaultCostakingPortion,
						ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
						ValidatorsPortion:   math.LegacyDec{},
					},
				}
			},
			expErr: govtypes.ErrInvalidProposalMsg,
		},
		{
			name: "validators portion equal to 1",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: appparams.AccGov.String(),
					Params: types.Params{
						CostakingPortion:   types.DefaultCostakingPortion,
						ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
						ValidatorsPortion:   math.LegacyOneDec(),
					},
				}
			},
			expErr: govtypes.ErrInvalidProposalMsg,
		},
		{
			name: "validators portion greater than 1",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: appparams.AccGov.String(),
					Params: types.Params{
						CostakingPortion:   types.DefaultCostakingPortion,
						ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
						ValidatorsPortion:   math.LegacyMustNewDecFromStr("1.5"),
					},
				}
			},
			expErr: govtypes.ErrInvalidProposalMsg,
		},
		{
			name: "negative validators portion",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: appparams.AccGov.String(),
					Params: types.Params{
						CostakingPortion:   types.DefaultCostakingPortion,
						ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
						ValidatorsPortion:   math.LegacyMustNewDecFromStr("-0.01"),
					},
				}
			},
			expErr: govtypes.ErrInvalidProposalMsg,
		},
		{
			name: "valid params with same ratio",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: appparams.AccGov.String(),
					Params: types.Params{
						CostakingPortion:   math.LegacyMustNewDecFromStr("0.5"),
						ScoreRatioBtcByBaby: math.OneInt(),
						ValidatorsPortion:   math.LegacyMustNewDecFromStr("0.4"),
					},
				}
			},
			expErr: nil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			k, ctrl, ctx := testkeeper.CostakingKeeperWithMocks(t, nil)
			defer ctrl.Finish()
			msgServer := keeper.NewMsgServerImpl(*k)

			// Initialize CurrentRewards to avoid collections error
			initialCurrentRewards := types.CurrentRewards{
				TotalScore: math.OneInt(),
				Period:     1,
			}
			err := k.SetCurrentRewards(ctx, initialCurrentRewards)
			require.NoError(t, err)

			msg := tc.setupMsg()
			_, err = msgServer.UpdateParams(ctx, msg)
			if tc.expErr != nil {
				require.ErrorIs(t, err, tc.expErr)
				return
			}
			require.NoError(t, err)

			updatedParams := k.GetParams(ctx)
			require.Equal(t, msg.Params.CostakingPortion, updatedParams.CostakingPortion)
			require.Equal(t, msg.Params.ScoreRatioBtcByBaby, updatedParams.ScoreRatioBtcByBaby)
			require.Equal(t, msg.Params.ValidatorsPortion, updatedParams.ValidatorsPortion)
		})
	}
}
