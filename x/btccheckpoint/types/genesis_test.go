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
	r := rand.New(rand.NewSource(time.Now().Unix()))
	defaultParams := types.DefaultParams()

	header := datagen.GenRandomBTCHeaderBytes(r, nil, nil)

	var epochNum uint64 = 1

	validTxKey := &types.TransactionKey{Hash: header.Hash()}
	validEpoch := types.EpochEntry{
		EpochNumber: epochNum,
		Data: &types.EpochData{
			Keys: []*types.SubmissionKey{{Key: []*types.TransactionKey{validTxKey}}},
		},
	}

	validSubmission := types.SubmissionEntry{
		SubmissionKey: &types.SubmissionKey{
			Key: []*types.TransactionKey{validTxKey},
		},
		Data: &types.SubmissionData{
			VigilanteAddresses: &types.CheckpointAddresses{
				Submitter: make([]byte, 20),
				Reporter:  make([]byte, 20),
			},
			TxsInfo: []*types.TransactionInfo{{
				Key:         validTxKey,
				Transaction: []byte{0x2},
				Proof:       []byte{0x3},
			}},
		},
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
				Params: defaultParams,
			},
			valid: true,
		},
		{
			desc: "duplicate epochs",
			genState: &types.GenesisState{
				Epochs: []types.EpochEntry{validEpoch, validEpoch}, // duplicate
				Params: defaultParams,
			},
			valid:  false,
			errMsg: "duplicate entry for key",
		},
		{
			desc: "duplicate submissions",
			genState: &types.GenesisState{
				Submissions: []types.SubmissionEntry{validSubmission, validSubmission}, // duplicate
				Params:      defaultParams,
			},
			valid:  false,
			errMsg: "duplicate entry for key",
		},
		{
			desc: "last finalized epoch greater than highest epoch",
			genState: &types.GenesisState{
				LastFinalizedEpochNumber: 2,
				Epochs:                   []types.EpochEntry{validEpoch}, // max is 1
				Params:                   defaultParams,
			},
			valid: false,
			errMsg: fmt.Sprintf("last finalized epoch number (%d) cannot be greater than the highest epoch number (%d)",
				2, epochNum),
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
