package types_test

import (
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v3/app"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v3/testutil/nullify"
	"github.com/babylonlabs-io/babylon/v3/x/epoching"
	"github.com/babylonlabs-io/babylon/v3/x/epoching/types"
	"github.com/stretchr/testify/require"
)

func TestGenesis(t *testing.T) {
	// This test requires setting up the staking module
	// Otherwise the epoching module cannot initialise the genesis validator set
	app := app.Setup(t, false)
	ctx := app.BaseApp.NewContext(false)
	keeper := app.EpochingKeeper

	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
	}

	epoching.InitGenesis(ctx, keeper, genesisState)
	got := epoching.ExportGenesis(ctx, keeper)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)
}

func TestGenesisState_Validate(t *testing.T) {
	var (
		r                                                   = rand.New(rand.NewSource(time.Now().Unix()))
		gs                                                  = datagen.GenRandomEpochingGenesisState(r)
		epochs, qs, valSets, slashedValSets, valsLc, delsLc = gs.Epochs, gs.Queues, gs.ValidatorSets, gs.SlashedValidatorSets, gs.ValidatorsLifecycle, gs.DelegationsLifecycle
		entriesCount                                        = len(epochs)
	)
	for _, tc := range []struct {
		desc     string
		genState *types.GenesisState
		valid    bool
		errMsg   string
	}{
		{
			desc:     "default is valid",
			genState: types.DefaultGenesis(),
			valid:    true,
		},
		{
			desc: "valid genesis state",
			genState: &types.GenesisState{
				Params: types.Params{
					EpochInterval: 100,
				},
			},
			valid: true,
		},
		{
			desc:     "valid full genesis state",
			genState: gs,
			valid:    true,
		},
		{
			desc:     "invalid genesis state - empty",
			genState: &types.GenesisState{},
			valid:    false,
			errMsg:   "epoch interval must be at least 2",
		},
		{
			desc: "invalid genesis state - duplicate epoch number",
			genState: types.NewGenesis(
				types.DefaultParams(),
				append(epochs, epochs[0]),
				qs,
				valSets,
				slashedValSets,
				valsLc,
				delsLc,
			),
			valid:  false,
			errMsg: "duplicate EpochNumber",
		},
		{
			desc: "invalid genesis state - duplicate epoch FirstBlockHeight",
			genState: types.NewGenesis(
				types.DefaultParams(),
				append(epochs, &types.Epoch{
					EpochNumber:      uint64(entriesCount) + 1,
					FirstBlockHeight: epochs[0].FirstBlockHeight,
				}),
				qs,
				valSets,
				slashedValSets,
				valsLc,
				delsLc,
			),
			valid:  false,
			errMsg: "duplicate FirstBlockHeight",
		},
		{
			desc: "invalid genesis state - duplicate epoch SealerBlockHash",
			genState: types.NewGenesis(
				types.DefaultParams(),
				append(epochs, &types.Epoch{
					EpochNumber:     uint64(entriesCount) + 1,
					SealerBlockHash: epochs[0].SealerBlockHash,
				}),
				qs,
				valSets,
				slashedValSets,
				valsLc,
				delsLc,
			),
			valid:  false,
			errMsg: "duplicate SealerBlockHash",
		},
		{
			desc: "invalid genesis state - duplicate queue",
			genState: types.NewGenesis(
				types.DefaultParams(),
				epochs,
				append(qs, qs[0]),
				valSets,
				slashedValSets,
				valsLc,
				delsLc,
			),
			valid:  false,
			errMsg: "duplicate entry",
		},
		{
			desc: "invalid genesis state - duplicate val set",
			genState: types.NewGenesis(
				types.DefaultParams(),
				epochs,
				qs,
				append(valSets, valSets[0]),
				slashedValSets,
				valsLc,
				delsLc,
			),
			valid:  false,
			errMsg: "duplicate entry",
		},
		{
			desc: "invalid genesis state - duplicate val lyfecycle",
			genState: types.NewGenesis(
				types.DefaultParams(),
				epochs,
				qs,
				valSets,
				slashedValSets,
				append(valsLc, valsLc[0]),
				delsLc,
			),
			valid:  false,
			errMsg: "duplicate entry",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.genState.Validate()
			if tc.valid {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorContains(t, err, tc.errMsg)
		})
	}
}

func TestEpochQueue_Validate(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		queue    types.EpochQueue
		valid    bool
		errorMsg string
	}{
		{
			name:     "nil Msgs",
			queue:    types.EpochQueue{EpochNumber: 1, Msgs: nil},
			valid:    false,
			errorMsg: "empty Msgs in epoch queue. EpochNum: 1",
		},
		{
			name:     "empty Msgs slice",
			queue:    types.EpochQueue{EpochNumber: 2, Msgs: []*types.QueuedMessage{}},
			valid:    false,
			errorMsg: "empty Msgs in epoch queue.",
		},
		{
			name: "single message with nil Msg",
			queue: types.EpochQueue{
				EpochNumber: 3,
				Msgs: []*types.QueuedMessage{
					{Msg: nil},
				},
			},
			valid:    false,
			errorMsg: "null Msg in epoch queue. EpochNum: 3",
		},
		{
			name: "single valid message",
			queue: types.EpochQueue{
				EpochNumber: 4,
				Msgs: []*types.QueuedMessage{
					{
						TxId:        []byte("tx"),
						MsgId:       []byte("msg"),
						BlockHeight: 100,
						BlockTime:   &now,
						Msg:         &types.QueuedMessage_MsgDelegate{},
					},
				},
			},
			valid: true,
		},
		{
			name: "multiple messages with one nil",
			queue: types.EpochQueue{
				EpochNumber: 5,
				Msgs: []*types.QueuedMessage{
					{Msg: &types.QueuedMessage_MsgDelegate{}},
					{Msg: nil},
				},
			},
			valid:    false,
			errorMsg: "null Msg in epoch queue. EpochNum: 5",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.queue.Validate()
			if tc.valid {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorContains(t, err, tc.errorMsg)
		})
	}
}

func TestValidateSequentialEpochs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		epochNumbers []uint64
		wantErr      bool
		errContains  string
	}{
		{
			name:         "empty slice",
			epochNumbers: []uint64{},
			wantErr:      false,
		},
		{
			name:         "single epoch",
			epochNumbers: []uint64{0},
			wantErr:      false,
		},
		{
			name:         "single epoch non-zero",
			epochNumbers: []uint64{5},
			wantErr:      false,
		},
		{
			name:         "consecutive from zero",
			epochNumbers: []uint64{0, 1, 2, 3, 4},
			wantErr:      false,
		},
		{
			name:         "consecutive unordered input",
			epochNumbers: []uint64{3, 1, 4, 2, 5},
			wantErr:      false,
		},
		{
			name:         "gap at beginning",
			epochNumbers: []uint64{0, 2, 3, 4},
			wantErr:      true,
			errContains:  "found gap between 0 and 2",
		},
		{
			name:         "gap in middle",
			epochNumbers: []uint64{1, 2, 4, 5},
			wantErr:      true,
			errContains:  "found gap between 2 and 4",
		},
		{
			name:         "gap at end",
			epochNumbers: []uint64{1, 2, 3, 6},
			wantErr:      true,
			errContains:  "found gap between 3 and 6",
		},
		{
			name:         "multiple gaps",
			epochNumbers: []uint64{1, 3, 5, 7},
			wantErr:      true,
			errContains:  "found gap between 1 and 3",
		},
		{
			name:         "large gap",
			epochNumbers: []uint64{1, 2, 100},
			wantErr:      true,
			errContains:  "found gap between 2 and 100",
		},
		{
			name:         "unordered with gap",
			epochNumbers: []uint64{5, 1, 3, 2},
			wantErr:      true,
			errContains:  "found gap between 3 and 5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := types.ValidateSequentialEpochs(tt.epochNumbers)

			if tt.wantErr {
				if err == nil {
					t.Errorf("validateSequentialEpochs() expected error but got nil")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("validateSequentialEpochs() error = %v, expected to contain %q", err, tt.errContains)
				}
			} else if err != nil {
				t.Errorf("validateSequentialEpochs() unexpected error = %v", err)
			}
		})
	}
}
