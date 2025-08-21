package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	testkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/coostaking/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
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
						CoostakingPortion:   math.LegacyMustNewDecFromStr("0.5"),
						ScoreRatioBtcByBaby: math.NewInt(100),
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
			name: "nil coostaking portion",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: appparams.AccGov.String(),
					Params: types.Params{
						CoostakingPortion:   math.LegacyDec{},
						ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
					},
				}
			},
			expErr: govtypes.ErrInvalidProposalMsg,
		},
		{
			name: "coostaking portion too high",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: appparams.AccGov.String(),
					Params: types.Params{
						CoostakingPortion:   math.LegacyMustNewDecFromStr("1.5"),
						ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
					},
				}
			},
			expErr: govtypes.ErrInvalidProposalMsg,
		},
		{
			name: "coostaking portion equal to 1",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: appparams.AccGov.String(),
					Params: types.Params{
						CoostakingPortion:   math.LegacyOneDec(),
						ScoreRatioBtcByBaby: types.DefaultScoreRatioBtcByBaby,
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
						CoostakingPortion:   types.DefaultCoostakingPortion,
						ScoreRatioBtcByBaby: math.Int{},
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
						CoostakingPortion:   types.DefaultCoostakingPortion,
						ScoreRatioBtcByBaby: math.ZeroInt(),
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
						CoostakingPortion:   types.DefaultCoostakingPortion,
						ScoreRatioBtcByBaby: math.NewInt(-10),
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
						CoostakingPortion:   math.LegacyNewDec(0),
						ScoreRatioBtcByBaby: math.OneInt(),
					},
				}
			},
			expErr: nil,
		},
		{
			name: "valid params with maximum coostaking portion",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: appparams.AccGov.String(),
					Params: types.Params{
						CoostakingPortion:   math.LegacyMustNewDecFromStr("0.999999999999999999"),
						ScoreRatioBtcByBaby: math.NewInt(50),
					},
				}
			},
			expErr: nil,
		},
		{
			name: "valid params with same ratio",
			setupMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: appparams.AccGov.String(),
					Params: types.Params{
						CoostakingPortion:   math.LegacyMustNewDecFromStr("0.999999999999999999"),
						ScoreRatioBtcByBaby: math.OneInt(),
					},
				}
			},
			expErr: nil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx := testkeeper.CoostakingKeeper(t)
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
			require.Equal(t, msg.Params.CoostakingPortion, updatedParams.CoostakingPortion)
			require.Equal(t, msg.Params.ScoreRatioBtcByBaby, updatedParams.ScoreRatioBtcByBaby)
		})
	}
}
