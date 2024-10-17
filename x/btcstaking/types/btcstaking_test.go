package types_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

func TestSortFinalityProvidersWithZeroedVotingPower(t *testing.T) {
	tests := []struct {
		name     string
		fps      []*types.FinalityProviderDistInfo
		expected []*types.FinalityProviderDistInfo
	}{
		{
			name: "Sort by voting power",
			fps: []*types.FinalityProviderDistInfo{
				{Addr: "fp1", TotalBondedSat: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x03}},
				{Addr: "fp2", TotalBondedSat: 200, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x02}},
				{Addr: "fp3", TotalBondedSat: 150, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x01}},
			},
			expected: []*types.FinalityProviderDistInfo{
				{Addr: "fp2", TotalBondedSat: 200, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x02}},
				{Addr: "fp3", TotalBondedSat: 150, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x01}},
				{Addr: "fp1", TotalBondedSat: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x03}},
			},
		},
		{
			name: "Jailed and non-timestamped providers at the end",
			fps: []*types.FinalityProviderDistInfo{
				{Addr: "fp1", TotalBondedSat: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x04}},
				{Addr: "fp2", TotalBondedSat: 200, IsJailed: true, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x02}},
				{Addr: "fp3", TotalBondedSat: 150, IsJailed: false, IsTimestamped: false, BtcPk: &bbn.BIP340PubKey{0x03}},
				{Addr: "fp4", TotalBondedSat: 50, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x01}},
			},
			expected: []*types.FinalityProviderDistInfo{
				{Addr: "fp1", TotalBondedSat: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x04}},
				{Addr: "fp4", TotalBondedSat: 50, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x01}},
				{Addr: "fp2", TotalBondedSat: 200, IsJailed: true, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x02}},
				{Addr: "fp3", TotalBondedSat: 150, IsJailed: false, IsTimestamped: false, BtcPk: &bbn.BIP340PubKey{0x03}},
			},
		},
		{
			name: "Equal voting power, sort by BTC public key",
			fps: []*types.FinalityProviderDistInfo{
				{Addr: "fp1", TotalBondedSat: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x03}},
				{Addr: "fp2", TotalBondedSat: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x01}},
				{Addr: "fp3", TotalBondedSat: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x02}},
			},
			expected: []*types.FinalityProviderDistInfo{
				{Addr: "fp2", TotalBondedSat: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x01}},
				{Addr: "fp3", TotalBondedSat: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x02}},
				{Addr: "fp1", TotalBondedSat: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x03}},
			},
		},
		{
			name: "Zeroed voting power, sort by BTC public key",
			fps: []*types.FinalityProviderDistInfo{
				{Addr: "fp1", TotalBondedSat: 200, IsJailed: true, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x03}},
				{Addr: "fp2", TotalBondedSat: 150, IsJailed: false, IsTimestamped: false, BtcPk: &bbn.BIP340PubKey{0x01}},
				{Addr: "fp3", TotalBondedSat: 100, IsJailed: true, IsTimestamped: false, BtcPk: &bbn.BIP340PubKey{0x02}},
			},
			expected: []*types.FinalityProviderDistInfo{
				{Addr: "fp2", TotalBondedSat: 150, IsJailed: false, IsTimestamped: false, BtcPk: &bbn.BIP340PubKey{0x01}},
				{Addr: "fp3", TotalBondedSat: 100, IsJailed: true, IsTimestamped: false, BtcPk: &bbn.BIP340PubKey{0x02}},
				{Addr: "fp1", TotalBondedSat: 200, IsJailed: true, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x03}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			types.SortFinalityProvidersWithZeroedVotingPower(tt.fps)
			require.Equal(t, tt.expected, tt.fps, "Sorted slice should match expected order")
		})
	}
}

// FuzzSortingDeterminism tests the property of the sorting algorithm that the result should
// be deterministic
func FuzzSortingDeterminism(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 1000)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		max_vp := 10000

		vp0 := datagen.RandomInt(r, max_vp) + 10
		vp1 := vp0 // this is for the case voting power is the same
		vp2 := datagen.RandomInt(r, max_vp) + 10
		vp3 := datagen.RandomInt(r, max_vp) + 10
		vp4 := datagen.RandomInt(r, max_vp) + 10

		pk1, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(t, err)
		pk2, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(t, err)
		pk3, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(t, err)
		pk4, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(t, err)
		pk5, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(t, err)

		fpsWithMeta := []*types.FinalityProviderDistInfo{
			{TotalBondedSat: vp0, IsJailed: false, IsTimestamped: true, Addr: "addr0", BtcPk: pk1},
			{TotalBondedSat: vp1, IsJailed: false, IsTimestamped: true, Addr: "addr1", BtcPk: pk2},
			{TotalBondedSat: vp2, IsJailed: false, IsTimestamped: true, Addr: "addr2", BtcPk: pk3},
			{TotalBondedSat: vp3, IsJailed: false, IsTimestamped: true, Addr: "addr3", BtcPk: pk4},
			{TotalBondedSat: vp4, IsJailed: false, IsTimestamped: true, Addr: "addr4", BtcPk: pk5},
		}
		jailedIdx := datagen.RandomInt(r, len(fpsWithMeta))
		noTimestampedIdx := datagen.RandomIntOtherThan(r, int(jailedIdx), len(fpsWithMeta))

		fpsWithMeta[jailedIdx].IsJailed = true
		fpsWithMeta[jailedIdx].IsTimestamped = false
		fpsWithMeta[noTimestampedIdx].IsJailed = false
		fpsWithMeta[noTimestampedIdx].IsTimestamped = false

		fpsWithMeta1 := []*types.FinalityProviderDistInfo{
			{TotalBondedSat: vp0, IsJailed: false, IsTimestamped: true, Addr: "addr0", BtcPk: pk1},
			{TotalBondedSat: vp1, IsJailed: false, IsTimestamped: true, Addr: "addr1", BtcPk: pk2},
			{TotalBondedSat: vp2, IsJailed: false, IsTimestamped: true, Addr: "addr2", BtcPk: pk3},
			{TotalBondedSat: vp3, IsJailed: false, IsTimestamped: true, Addr: "addr3", BtcPk: pk4},
			{TotalBondedSat: vp4, IsJailed: false, IsTimestamped: true, Addr: "addr4", BtcPk: pk5},
		}

		fpsWithMeta1[jailedIdx].IsJailed = true
		fpsWithMeta1[jailedIdx].IsTimestamped = false
		fpsWithMeta1[noTimestampedIdx].IsJailed = false
		fpsWithMeta1[noTimestampedIdx].IsTimestamped = false

		// Shuffle the fpsWithMeta1 slice
		r.Shuffle(len(fpsWithMeta1), func(i, j int) {
			fpsWithMeta1[i], fpsWithMeta1[j] = fpsWithMeta1[j], fpsWithMeta1[i]
		})

		types.SortFinalityProvidersWithZeroedVotingPower(fpsWithMeta)
		types.SortFinalityProvidersWithZeroedVotingPower(fpsWithMeta1)

		for i := 0; i < len(fpsWithMeta); i++ {
			// our lists should be sorted in same order
			require.Equal(t, fpsWithMeta[i].BtcPk.MarshalHex(), fpsWithMeta1[i].BtcPk.MarshalHex())
		}
	})
}
