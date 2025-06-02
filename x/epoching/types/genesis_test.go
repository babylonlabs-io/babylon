package types_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v4/app"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/testutil/nullify"
	"github.com/babylonlabs-io/babylon/v4/x/epoching"
	"github.com/babylonlabs-io/babylon/v4/x/epoching/types"
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
