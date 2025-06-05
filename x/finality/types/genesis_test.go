package types_test

import (
	"math/rand"
	"testing"
	time "time"

	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v2/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v2/x/finality/types"
)

func TestGenesisState_Validate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	gs, err := datagen.GenRandomFinalityGenesisState(r)
	require.NoError(t, err)
	// Skip validation of voting power distribution cache for testing
	gs.VpDstCache = nil

	// Create a valid finality provider for testing
	btcPk, err := datagen.GenRandomBIP340PubKey(r)
	require.NoError(t, err)
	validComm = sdkmath.LegacyMustNewDecFromStr("0.5")
	fpDistInfo := &types.FinalityProviderDistInfo{
		BtcPk:          btcPk,
		Addr:           fpAddr1,
		TotalBondedSat: 100,
		IsTimestamped:  true,
		Commission:     &validComm,
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
			desc:     "valid full genesis state",
			genState: gs,
			valid:    true,
		},
		{
			desc: "invalid genesis state",
			genState: &types.GenesisState{
				Params: types.Params{
					MinPubRand: 200,
				},
			},
			valid:  false,
			errMsg: "max finality providers must be positive",
		},
		{
			desc:     "invalid genesis state - empty",
			genState: &types.GenesisState{},
			valid:    false,
			errMsg:   "max finality providers must be positive",
		},
		{
			desc: "invalid genesis state - duplicate block",
			genState: &types.GenesisState{
				Params:        gs.Params,
				IndexedBlocks: append(gs.IndexedBlocks, gs.IndexedBlocks[0]),
			},
			valid:  false,
			errMsg: "duplicate entry",
		},
		{
			desc: "duplicate vote sigs",
			genState: &types.GenesisState{
				Params:   gs.Params,
				VoteSigs: append(gs.VoteSigs, gs.VoteSigs[0]),
			},
			valid:  false,
			errMsg: "duplicate entry",
		},
		{
			desc: "duplicate public randomness",
			genState: &types.GenesisState{
				Params:           gs.Params,
				PublicRandomness: append(gs.PublicRandomness, gs.PublicRandomness[0]),
			},
			valid:  false,
			errMsg: "duplicate entry",
		},
		{
			desc: "duplicate pub rand commit",
			genState: &types.GenesisState{
				Params:        gs.Params,
				PubRandCommit: append(gs.PubRandCommit, gs.PubRandCommit[0]),
			},
			valid:  false,
			errMsg: "duplicate entry",
		},
		{
			desc: "duplicate signing infos",
			genState: &types.GenesisState{
				Params:       gs.Params,
				SigningInfos: append(gs.SigningInfos, gs.SigningInfos[0]),
			},
			valid:  false,
			errMsg: "duplicate entry",
		},
		{
			desc: "duplicate missed blocks",
			genState: &types.GenesisState{
				Params:       gs.Params,
				MissedBlocks: append(gs.MissedBlocks, gs.MissedBlocks[0]),
			},
			valid:  false,
			errMsg: "duplicate entry",
		},
		{
			desc: "duplicate voting powers",
			genState: &types.GenesisState{
				Params:       gs.Params,
				VotingPowers: append(gs.VotingPowers, gs.VotingPowers[0]),
			},
			valid:  false,
			errMsg: "duplicate entry",
		},
		{
			desc: "duplicate vp dist cache",
			genState: &types.GenesisState{
				Params: gs.Params,
				VpDstCache: []*types.VotingPowerDistCacheBlkHeight{
					{
						BlockHeight: 1,
						VpDistribution: &types.VotingPowerDistCache{
							TotalVotingPower:  100,
							NumActiveFps:      1,
							FinalityProviders: []*types.FinalityProviderDistInfo{fpDistInfo},
						},
					},
					{
						BlockHeight: 1,
						VpDistribution: &types.VotingPowerDistCache{
							TotalVotingPower:  100,
							NumActiveFps:      1,
							FinalityProviders: []*types.FinalityProviderDistInfo{fpDistInfo},
						},
					},
				},
			},
			valid:  false,
			errMsg: "duplicate entry",
		},
		{
			desc: "next height to reward is greater than next height to finalize",
			genState: &types.GenesisState{
				Params:               gs.Params,
				NextHeightToReward:   100,
				NextHeightToFinalize: 50,
			},
			valid:  false,
			errMsg: "Next height to reward 100 is higher than next height to finalize 50",
		},
	}
	for _, tc := range tests {
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
