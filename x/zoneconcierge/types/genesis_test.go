package types_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
	"github.com/stretchr/testify/require"
)

func TestGenesisState_Validate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	gs := datagen.GenRandomZoneconciergeGenState(r)
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
			desc:     "empty is invalid",
			genState: &types.GenesisState{},
			valid:    false,
			errMsg:   "identifier cannot be blank",
		},
		{
			desc: "valid genesis state",
			genState: &types.GenesisState{
				PortId: types.PortID,
				Params: types.Params{IbcPacketTimeoutSeconds: 100},
			},
			valid: true,
		},
		{
			desc: "invalid port id",
			genState: &types.GenesisState{
				PortId: "invalid!port",
				Params: types.DefaultParams(),
			},
			valid:  false,
			errMsg: "invalid identifier",
		},
		{
			desc: "duplicate chain info entries",
			genState: &types.GenesisState{
				PortId:     types.PortID,
				ChainsInfo: append(gs.ChainsInfo, gs.ChainsInfo[0]),
			},
			valid:  false,
			errMsg: "duplicate entry",
		},
		{
			desc: "duplicate indexed headers",
			genState: &types.GenesisState{
				PortId:               types.PortID,
				ChainsIndexedHeaders: append(gs.ChainsIndexedHeaders, gs.ChainsIndexedHeaders[0]),
			},
			valid:  false,
			errMsg: "duplicate entry",
		},
		{
			desc: "invalid epoch chain info entry (nil chain info)",
			genState: &types.GenesisState{
				PortId: types.PortID,
				ChainsEpochsInfo: []*types.EpochChainInfoEntry{
					{EpochNumber: 1, ChainInfo: nil},
				},
			},
			valid:  false,
			errMsg: "empty chain info",
		},
		{
			desc: "invalid sealed epoch proof (nil proof)",
			genState: &types.GenesisState{
				PortId: types.PortID,
				SealedEpochsProofs: []*types.SealedEpochProofEntry{
					{EpochNumber: 1, Proof: nil},
				},
			},
			valid:  false,
			errMsg: "empty proof",
		},
		{
			desc: "invalid params",
			genState: &types.GenesisState{
				PortId: types.PortID,
				Params: types.Params{IbcPacketTimeoutSeconds: 0},
			},
			valid:  false,
			errMsg: "IbcPacketTimeoutSeconds must be positive",
		},
		{
			desc:     "valid full genesis state",
			genState: gs,
			valid:    true,
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
