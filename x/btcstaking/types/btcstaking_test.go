package types_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"

	"github.com/stretchr/testify/require"
)

func FuzzSortingDeterminism(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 500)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		max_vp := math.MaxUint32

		vp0 := datagen.RandomInt(r, max_vp) + 10
		vp1 := datagen.RandomInt(r, max_vp) + 10
		vp2 := datagen.RandomInt(r, max_vp) + 10
		vp3 := datagen.RandomInt(r, max_vp) + 10
		vp4 := datagen.RandomInt(r, max_vp) + 10

		fpsWithMeta := []*types.FinalityProviderDistInfo{
			{TotalVotingPower: vp0, IsJailed: false, IsTimestamped: true, Addr: "addr0", BtcPk: &bbn.BIP340PubKey{0x00}},
			{TotalVotingPower: vp1, IsJailed: false, IsTimestamped: true, Addr: "addr1", BtcPk: &bbn.BIP340PubKey{0x01}},
			{TotalVotingPower: vp2, IsJailed: false, IsTimestamped: true, Addr: "addr2", BtcPk: &bbn.BIP340PubKey{0x02}},
			{TotalVotingPower: vp3, IsJailed: false, IsTimestamped: true, Addr: "addr3", BtcPk: &bbn.BIP340PubKey{0x03}},
			{TotalVotingPower: vp4, IsJailed: false, IsTimestamped: true, Addr: "addr4", BtcPk: &bbn.BIP340PubKey{0x04}},
		}
		jailedIdx1 := datagen.RandomInt(r, len(fpsWithMeta))
		jailedIdx2 := datagen.RandomIntOtherThan(r, int(jailedIdx1), len(fpsWithMeta))

		fpsWithMeta[jailedIdx1].IsJailed = true
		fpsWithMeta[jailedIdx1].IsTimestamped = false
		fpsWithMeta[jailedIdx2].IsJailed = true
		fpsWithMeta[jailedIdx2].IsTimestamped = false

		fpsWithMeta1 := []*types.FinalityProviderDistInfo{
			{TotalVotingPower: vp0, IsJailed: false, IsTimestamped: true, Addr: "addr0", BtcPk: &bbn.BIP340PubKey{0x00}},
			{TotalVotingPower: vp1, IsJailed: false, IsTimestamped: true, Addr: "addr1", BtcPk: &bbn.BIP340PubKey{0x01}},
			{TotalVotingPower: vp2, IsJailed: false, IsTimestamped: true, Addr: "addr2", BtcPk: &bbn.BIP340PubKey{0x02}},
			{TotalVotingPower: vp3, IsJailed: false, IsTimestamped: true, Addr: "addr3", BtcPk: &bbn.BIP340PubKey{0x03}},
			{TotalVotingPower: vp4, IsJailed: false, IsTimestamped: true, Addr: "addr4", BtcPk: &bbn.BIP340PubKey{0x04}},
		}

		fpsWithMeta1[jailedIdx1].IsJailed = true
		fpsWithMeta1[jailedIdx1].IsTimestamped = false
		fpsWithMeta1[jailedIdx2].IsJailed = true
		fpsWithMeta1[jailedIdx2].IsTimestamped = false

		types.SortFinalityProvidersWithZeroedVotingPower(fpsWithMeta)
		types.SortFinalityProvidersWithZeroedVotingPower(fpsWithMeta1)

		for i := 0; i < len(fpsWithMeta); i++ {
			// our lists should be sorted in same order
			require.Equal(t, fpsWithMeta[i].Addr, fpsWithMeta1[i].Addr)
		}
	})
}
