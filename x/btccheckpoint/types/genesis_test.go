package types_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	"github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	"github.com/stretchr/testify/require"
)

func TestGenesisState_Validate(t *testing.T) {
	var (
		r             = rand.New(rand.NewSource(time.Now().Unix()))
		defaultParams = types.DefaultParams()
		entriesCount  = 3
		epochs        = make([]types.EpochEntry, 0, entriesCount)
		submissions   = make([]types.SubmissionEntry, 0, entriesCount)
	)

	for i := range entriesCount {
		epochNum := uint64(i + 1)
		validTxKey := datagen.RandomTxKey(r)
		validEpoch := types.EpochEntry{
			EpochNumber: epochNum,
			Data: &types.EpochData{
				Status: datagen.RandomBtcStatus(r),
				Keys:   []*types.SubmissionKey{{Key: []*types.TransactionKey{validTxKey}}},
			},
		}
		epochs = append(epochs, validEpoch)
		validSubmission := types.SubmissionEntry{
			SubmissionKey: &types.SubmissionKey{
				Key: []*types.TransactionKey{validTxKey},
			},
			Data: &types.SubmissionData{
				VigilanteAddresses: &types.CheckpointAddresses{
					Submitter: datagen.GenRandomAddress().Bytes(),
					Reporter:  datagen.GenRandomAddress().Bytes(),
				},
				TxsInfo: []*types.TransactionInfo{{
					Key:         validTxKey,
					Transaction: datagen.GenRandomByteArray(r, 32),
					Proof:       datagen.GenRandomByteArray(r, 32),
				}},
			},
		}
		submissions = append(submissions, validSubmission)
	}

	tests := []struct {
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
				Params:                   defaultParams,
				LastFinalizedEpochNumber: uint64(entriesCount - 1),
				Epochs:                   epochs,
				Submissions:              submissions,
			},
			valid: true,
		},
		{
			desc: "duplicate epochs",
			genState: &types.GenesisState{
				Epochs: []types.EpochEntry{epochs[0], epochs[0]},
				Params: defaultParams,
			},
			valid:  false,
			errMsg: "duplicate entry for key",
		},
		{
			desc: "duplicate submissions",
			genState: &types.GenesisState{
				Submissions: []types.SubmissionEntry{submissions[0], submissions[0]},
				Params:      defaultParams,
			},
			valid:  false,
			errMsg: "duplicate entry for key",
		},
		{
			desc: "last finalized epoch greater than highest epoch",
			genState: &types.GenesisState{
				LastFinalizedEpochNumber: 2,
				Epochs:                   []types.EpochEntry{epochs[0]}, // max is 1
				Params:                   defaultParams,
			},
			valid: false,
			errMsg: fmt.Sprintf("last finalized epoch number (%d) cannot be greater than the highest epoch number (%d)",
				2, 1),
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.genState.Validate()
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.errMsg)
			}
		})
	}
}
