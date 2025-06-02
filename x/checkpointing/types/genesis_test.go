package types_test

import (
	"math/rand"
	"testing"
	time "time"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"

	"github.com/stretchr/testify/require"
)

func TestGenesisStateValidate(t *testing.T) {
	var (
		r                  = rand.New(rand.NewSource(time.Now().Unix()))
		entriesCount       = int(datagen.RandomIntOtherThan(r, 0, 10))
		vs, _              = datagen.GenerateValidatorSetWithBLSPrivKeys(entriesCount)
		gk                 = make([]*types.GenesisKey, entriesCount)
		vSets              = make([]*types.ValidatorSetEntry, entriesCount)
		chkpts             = make([]*types.RawCheckpointWithMeta, entriesCount)
		lastFinalizedEpoch = uint64(entriesCount - 1)
	)

	for i := range entriesCount {
		epochNum := uint64(i) + 1
		gk[i] = datagen.GenerateGenesisKey()
		vSets[i] = &types.ValidatorSetEntry{
			EpochNumber:  epochNum,
			ValidatorSet: vs,
		}
		chkpts[i] = datagen.GenRandomRawCheckpointWithMeta(r)
		chkpts[i].Ckpt.EpochNum = epochNum
	}

	testCases := []struct {
		name   string
		gs     types.GenesisState
		valid  bool
		errMsg string
	}{
		{
			name:  "empty genesis state - valid",
			gs:    types.GenesisState{},
			valid: true,
		},
		{
			name: "duplicate validator address in GenesisKeys",
			gs: types.GenesisState{
				GenesisKeys: []*types.GenesisKey{
					gk[0],
					gk[0],
				},
			},
			valid:  false,
			errMsg: "duplicate entry",
		},
		{
			name: "duplicate epoch in ValidatorSets",
			gs: types.GenesisState{
				ValidatorSets: []*types.ValidatorSetEntry{
					vSets[0],
					vSets[0],
				},
			},
			valid:  false,
			errMsg: "duplicate entry",
		},
		{
			name: "duplicate epoch in Checkpoints",
			gs: types.GenesisState{
				Checkpoints: []*types.RawCheckpointWithMeta{
					chkpts[0],
					chkpts[0],
				},
			},
			valid:  false,
			errMsg: "duplicate entry",
		},
		{
			name: "nil checkpoint should not be valid",
			gs: types.GenesisState{
				Checkpoints: []*types.RawCheckpointWithMeta{
					{Ckpt: nil},
				},
			},
			valid:  false,
			errMsg: "null checkpoint",
		},
		{
			name: "valid full genesis state",
			gs: types.GenesisState{
				GenesisKeys:        gk,
				ValidatorSets:      vSets,
				Checkpoints:        chkpts,
				LastFinalizedEpoch: lastFinalizedEpoch,
			},
			valid: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.gs.Validate()
			if tc.valid {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.errMsg)
		})
	}
}
