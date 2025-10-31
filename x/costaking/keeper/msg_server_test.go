package keeper_test

import (
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	dbm "github.com/cosmos/cosmos-db"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	testkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
)

func formatNumber(n uint64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.0fm", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.0fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func TestMsgUpdateParamsScoreRatioBenchmark(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	accK := types.NewMockAccountKeeper(ctrl)
	accK.EXPECT().GetModuleAddress(gomock.Any()).Return(authtypes.NewModuleAddress(types.ModuleName)).AnyTimes()

	tmpDir := t.TempDir()
	backend := dbm.GoLevelDBBackend
	db, err := dbm.NewDB(t.Name(), backend, tmpDir)
	require.NoError(t, err)
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())

	t.Logf("TestMsgUpdateParamsScoreRatioBenchmark is running with file database of type: %s at %s", backend, tmpDir)

	k, ctx := testkeeper.CostakingKeeperWithStore(t, db, stateStore, nil, nil, accK, nil, nil, nil)

	params := types.DefaultParams()
	err = k.SetParams(ctx, params)
	require.NoError(t, err)

	numberOfCostakers := map[uint64]struct{}{
		1_000:         struct{}{},
		10_000:        struct{}{},
		50_000:        struct{}{},
		100_000:       struct{}{},
		1_000_000:     struct{}{},
		10_000_000:    struct{}{},
		100_000_000:   struct{}{},
		1_000_000_000: struct{}{},
	}
	costakersCreated := uint64(0)

	activeAmounts := math.NewInt(1000)

	maxValue := uint64(0)
	for num := range numberOfCostakers {
		if num > maxValue {
			maxValue = num
		}
	}

	msgServer := keeper.NewMsgServerImpl(*k)
	before := time.Now()
	prevInsert := uint64(0)

	for costakersCreated <= maxValue {
		_, reachedGoal := numberOfCostakers[costakersCreated]
		if reachedGoal {
			after := time.Now()
			timeDiffInsert := after.Sub(before)
			t.Logf("Took %s to insert %s costakers", timeDiffInsert.String(), formatNumber(costakersCreated-prevInsert))
			before = after
			prevInsert = costakersCreated

			beforeUpdate := time.Now()
			params.ScoreRatioBtcByBaby = params.ScoreRatioBtcByBaby.QuoRaw(2)
			_, err = msgServer.UpdateParams(ctx, &types.MsgUpdateParams{
				Authority: appparams.AccGov.String(),
				Params:    params,
			})
			require.NoError(t, err)
			afterUpdate := time.Now()

			timeDiff := afterUpdate.Sub(beforeUpdate)
			t.Logf("Running costaking MsgUpdateParams with different ScoreRatioBtcByBaby for %s costakers it takes about: %s", formatNumber(costakersCreated), timeDiff.String())
		}

		err = k.CostakerModifiedActiveAmounts(ctx, datagen.GenRandomAddress(), activeAmounts, activeAmounts)
		require.NoError(t, err)
		costakersCreated++
		continue
	}
}

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
						CostakingPortion:    math.LegacyMustNewDecFromStr("0.5"),
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
						CostakingPortion:    math.LegacyDec{},
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
						CostakingPortion:    math.LegacyMustNewDecFromStr("1.5"),
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
						CostakingPortion:    math.LegacyOneDec(),
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
						CostakingPortion:    types.DefaultCostakingPortion,
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
						CostakingPortion:    types.DefaultCostakingPortion,
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
						CostakingPortion:    types.DefaultCostakingPortion,
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
						CostakingPortion:    math.LegacyNewDec(0),
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
						CostakingPortion:    math.LegacyMustNewDecFromStr("0.9"),
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
						CostakingPortion:    math.LegacyMustNewDecFromStr("0.5"),
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
						CostakingPortion:    math.LegacyMustNewDecFromStr("0.6"),
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
						CostakingPortion:    types.DefaultCostakingPortion,
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
						CostakingPortion:    types.DefaultCostakingPortion,
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
						CostakingPortion:    types.DefaultCostakingPortion,
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
						CostakingPortion:    types.DefaultCostakingPortion,
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
						CostakingPortion:    math.LegacyMustNewDecFromStr("0.5"),
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

			// creates proper historical
			_, err := k.GetCurrentRewardsInitialized(ctx)
			require.NoError(t, err)

			// Initialize CurrentRewards to avoid collections error
			initialCurrentRewards := types.CurrentRewards{
				TotalScore: math.OneInt(),
				Period:     1,
			}
			err = k.SetCurrentRewards(ctx, initialCurrentRewards)
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
