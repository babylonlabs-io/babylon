package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVotingPowerDistCache(t *testing.T) {
	tests := []struct {
		desc             string
		maxActiveFPs     uint32
		numActiveFps     uint32
		totalVotingPower uint64
		fps              []*FinalityProviderDistInfo
	}{
		{
			desc:             "all not timestamped",
			maxActiveFPs:     80,
			numActiveFps:     0,
			totalVotingPower: 0,
			fps: []*FinalityProviderDistInfo{
				{
					TotalVotingPower: 1000,
					IsTimestamped:    false,
				},
				{
					TotalVotingPower: 2000,
					IsTimestamped:    false,
				},
			},
		},
		{
			desc:             "all timestamped",
			maxActiveFPs:     80,
			numActiveFps:     2,
			totalVotingPower: 3000,
			fps: []*FinalityProviderDistInfo{
				{
					TotalVotingPower: 1000,
					IsTimestamped:    true,
				},
				{
					TotalVotingPower: 2000,
					IsTimestamped:    true,
				},
			},
		},
		{
			desc:             "partly timestamped",
			maxActiveFPs:     80,
			numActiveFps:     1,
			totalVotingPower: 1000,
			fps: []*FinalityProviderDistInfo{
				{
					TotalVotingPower: 1000,
					IsTimestamped:    true,
				},
				{
					TotalVotingPower: 2000,
					IsTimestamped:    false,
				},
			},
		},
		{
			desc:             "small max active fps",
			maxActiveFPs:     1,
			numActiveFps:     1,
			totalVotingPower: 2000,
			fps: []*FinalityProviderDistInfo{
				{
					TotalVotingPower: 1000,
					IsTimestamped:    true,
				},
				{
					TotalVotingPower: 2000,
					IsTimestamped:    true,
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			dc := NewVotingPowerDistCache()
			for _, fp := range tc.fps {
				dc.AddFinalityProviderDistInfo(fp)
			}
			dc.ApplyActiveFinalityProviders(tc.maxActiveFPs)
			require.Equal(t, tc.totalVotingPower, dc.TotalVotingPower)
			require.Equal(t, tc.numActiveFps, dc.NumActiveFps)
		})
	}
}
