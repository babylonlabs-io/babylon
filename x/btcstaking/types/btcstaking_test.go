package types

import (
	"testing"

	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/stretchr/testify/assert"
)

func TestSortFinalityProvidersWithZeroedVotingPower(t *testing.T) {
	tests := []struct {
		name     string
		fps      []*FinalityProviderDistInfo
		expected []*FinalityProviderDistInfo
	}{
		{
			name: "Sort by voting power",
			fps: []*FinalityProviderDistInfo{
				{Addr: "fp1", TotalVotingPower: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x03}},
				{Addr: "fp2", TotalVotingPower: 200, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x02}},
				{Addr: "fp3", TotalVotingPower: 150, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x01}},
			},
			expected: []*FinalityProviderDistInfo{
				{Addr: "fp2", TotalVotingPower: 200, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x02}},
				{Addr: "fp3", TotalVotingPower: 150, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x01}},
				{Addr: "fp1", TotalVotingPower: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x03}},
			},
		},
		{
			name: "Jailed and non-timestamped providers at the end",
			fps: []*FinalityProviderDistInfo{
				{Addr: "fp1", TotalVotingPower: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x04}},
				{Addr: "fp2", TotalVotingPower: 200, IsJailed: true, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x02}},
				{Addr: "fp3", TotalVotingPower: 150, IsJailed: false, IsTimestamped: false, BtcPk: &bbn.BIP340PubKey{0x03}},
				{Addr: "fp4", TotalVotingPower: 50, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x01}},
			},
			expected: []*FinalityProviderDistInfo{
				{Addr: "fp1", TotalVotingPower: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x04}},
				{Addr: "fp4", TotalVotingPower: 50, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x01}},
				{Addr: "fp2", TotalVotingPower: 200, IsJailed: true, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x02}},
				{Addr: "fp3", TotalVotingPower: 150, IsJailed: false, IsTimestamped: false, BtcPk: &bbn.BIP340PubKey{0x03}},
			},
		},
		{
			name: "Equal voting power, sort by BTC public key",
			fps: []*FinalityProviderDistInfo{
				{Addr: "fp1", TotalVotingPower: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x03}},
				{Addr: "fp2", TotalVotingPower: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x01}},
				{Addr: "fp3", TotalVotingPower: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x02}},
			},
			expected: []*FinalityProviderDistInfo{
				{Addr: "fp2", TotalVotingPower: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x01}},
				{Addr: "fp3", TotalVotingPower: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x02}},
				{Addr: "fp1", TotalVotingPower: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x03}},
			},
		},
		{
			name: "Zeroed voting power, sort by BTC public key",
			fps: []*FinalityProviderDistInfo{
				{Addr: "fp1", TotalVotingPower: 200, IsJailed: true, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x03}},
				{Addr: "fp2", TotalVotingPower: 150, IsJailed: false, IsTimestamped: false, BtcPk: &bbn.BIP340PubKey{0x01}},
				{Addr: "fp3", TotalVotingPower: 100, IsJailed: true, IsTimestamped: false, BtcPk: &bbn.BIP340PubKey{0x02}},
			},
			expected: []*FinalityProviderDistInfo{
				{Addr: "fp2", TotalVotingPower: 150, IsJailed: false, IsTimestamped: false, BtcPk: &bbn.BIP340PubKey{0x01}},
				{Addr: "fp3", TotalVotingPower: 100, IsJailed: true, IsTimestamped: false, BtcPk: &bbn.BIP340PubKey{0x02}},
				{Addr: "fp1", TotalVotingPower: 200, IsJailed: true, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x03}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SortFinalityProvidersWithZeroedVotingPower(tt.fps)
			assert.Equal(t, tt.expected, tt.fps, "Sorted slice should match expected order")
		})
	}
}
