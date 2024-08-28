package types

import (
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/types"
)

var (
	fpPrivKey1, _ = btcec.NewPrivateKey()
	fpPrivKey2, _ = btcec.NewPrivateKey()
	fpPubKey1     = types.NewBIP340PubKeyFromBTCPK(fpPrivKey1.PubKey())
	fpPubKey2     = types.NewBIP340PubKeyFromBTCPK(fpPrivKey2.PubKey())
)

func TestVotingPowerDistCache(t *testing.T) {
	tests := []struct {
		desc             string
		maxActiveFPs     uint32
		numActiveFps     uint32
		totalVotingPower uint64
		prevDistCache    *VotingPowerDistCache
		fps              []*FinalityProviderDistInfo
	}{
		{
			desc:             "all not timestamped",
			maxActiveFPs:     80,
			numActiveFps:     0,
			totalVotingPower: 0,
			prevDistCache:    NewVotingPowerDistCache(),
			fps: []*FinalityProviderDistInfo{
				{
					BtcPk:            fpPubKey1,
					TotalVotingPower: 1000,
					IsTimestamped:    false,
				},
				{
					BtcPk:            fpPubKey2,
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
			prevDistCache:    NewVotingPowerDistCache(),
			fps: []*FinalityProviderDistInfo{
				{
					BtcPk:            fpPubKey1,
					TotalVotingPower: 1000,
					IsTimestamped:    true,
				},
				{
					BtcPk:            fpPubKey2,
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
			prevDistCache:    NewVotingPowerDistCache(),
			fps: []*FinalityProviderDistInfo{
				{
					BtcPk:            fpPubKey1,
					TotalVotingPower: 1000,
					IsTimestamped:    true,
				},
				{
					BtcPk:            fpPubKey2,
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
			prevDistCache:    NewVotingPowerDistCache(),
			fps: []*FinalityProviderDistInfo{
				{
					BtcPk:            fpPubKey1,
					TotalVotingPower: 1000,
					IsTimestamped:    true,
				},
				{
					BtcPk:            fpPubKey2,
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

			newBondedFps := dc.FindNewActiveFinalityProviders(tc.prevDistCache)
			require.Equal(t, tc.numActiveFps, uint32(len(newBondedFps)))
		})
	}
}
