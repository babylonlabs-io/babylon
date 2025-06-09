package types_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/v3/x/monitor/types"

	"github.com/stretchr/testify/require"
)

func TestGenesisState_Validate(t *testing.T) {
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
			desc:     "empty is valid",
			genState: &types.GenesisState{},
			valid:    true,
		},
		{
			desc: "valid EpochEndRecords and CheckpointsReported",
			genState: &types.GenesisState{
				EpochEndRecords: []*types.EpochEndLightClient{
					{Epoch: 1, BtcLightClientHeight: 100},
					{Epoch: 2, BtcLightClientHeight: 101},
				},
				CheckpointsReported: []*types.CheckpointReportedLightClient{
					{CkptHash: "deadbeef01", BtcLightClientHeight: 200},
					{CkptHash: "deadbeef02", BtcLightClientHeight: 201},
				},
			},
			valid: true,
		},
		{
			desc: "valid EpochEndRecord with height 0 (default uin64)",
			genState: &types.GenesisState{
				EpochEndRecords: []*types.EpochEndLightClient{
					{Epoch: 1, BtcLightClientHeight: 0},
				},
			},
			valid: true,
		},
		{
			desc: "CheckpointReported with height 0",
			genState: &types.GenesisState{
				CheckpointsReported: []*types.CheckpointReportedLightClient{
					{CkptHash: "deadbeef", BtcLightClientHeight: 0},
				},
			},
			valid: true,
		},
		{
			desc: "invalid CheckpointReported with empty hash",
			genState: &types.GenesisState{
				CheckpointsReported: []*types.CheckpointReportedLightClient{
					{CkptHash: "", BtcLightClientHeight: 123},
				},
			},
			valid:  false,
			errMsg: "checkpoint hash cannot be empty",
		},
		{
			desc: "duplicate EpochEndRecord epochs",
			genState: &types.GenesisState{
				EpochEndRecords: []*types.EpochEndLightClient{
					{Epoch: 1, BtcLightClientHeight: 100},
					{Epoch: 1, BtcLightClientHeight: 101},
				},
			},
			valid:  false,
			errMsg: "duplicate entry",
		},
		{
			desc: "duplicate CheckpointsReported hashes",
			genState: &types.GenesisState{
				CheckpointsReported: []*types.CheckpointReportedLightClient{
					{CkptHash: "deadbeef", BtcLightClientHeight: 200},
					{CkptHash: "deadbeef", BtcLightClientHeight: 201},
				},
			},
			valid:  false,
			errMsg: "duplicate entry",
		},
		{
			desc: "invalid hash string in CheckpointsReported",
			genState: &types.GenesisState{
				EpochEndRecords: []*types.EpochEndLightClient{
					{Epoch: 1, BtcLightClientHeight: 100},
					{Epoch: 2, BtcLightClientHeight: 101},
				},
				CheckpointsReported: []*types.CheckpointReportedLightClient{
					{CkptHash: "deadbeef01", BtcLightClientHeight: 200},
					{CkptHash: "deadbeefg2", BtcLightClientHeight: 201},
				},
			},
			valid:  false,
			errMsg: "invalid hash string deadbeefg2",
		},
	} {
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
