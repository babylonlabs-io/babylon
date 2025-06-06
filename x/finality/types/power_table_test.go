package types_test

import (
	"math/rand"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v2/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v2/types"
	types "github.com/babylonlabs-io/babylon/v2/x/finality/types"
)

var (
	fpPrivKey1, _ = btcec.NewPrivateKey()
	fpPrivKey2, _ = btcec.NewPrivateKey()
	fpPrivKey3, _ = btcec.NewPrivateKey()
	fpPubKey1     = bbn.NewBIP340PubKeyFromBTCPK(fpPrivKey1.PubKey())
	fpPubKey2     = bbn.NewBIP340PubKeyFromBTCPK(fpPrivKey2.PubKey())
	fpPubKey3     = bbn.NewBIP340PubKeyFromBTCPK(fpPrivKey3.PubKey())
	fpAddr1       = datagen.GenRandomAddress()
	fpAddr2       = datagen.GenRandomAddress()
	fpAddr3       = datagen.GenRandomAddress()
	negComm       = sdkmath.LegacyNewDec(-1)
	highComm      = sdkmath.LegacyNewDec(2)
	validComm     = sdkmath.LegacyMustNewDecFromStr("0.5")
)

func TestVotingPowerDistCache(t *testing.T) {
	tests := []struct {
		desc             string
		maxActiveFPs     uint32
		numActiveFps     uint32
		numInactiveFps   uint32
		totalVotingPower uint64
		prevDistCache    *types.VotingPowerDistCache
		fps              []*types.FinalityProviderDistInfo
	}{
		{
			desc:             "all not timestamped",
			maxActiveFPs:     80,
			numActiveFps:     0,
			numInactiveFps:   2,
			totalVotingPower: 0,
			prevDistCache:    types.NewVotingPowerDistCache(),
			fps: []*types.FinalityProviderDistInfo{
				{
					BtcPk:          fpPubKey1,
					TotalBondedSat: 1000,
					IsTimestamped:  false,
				},
				{
					BtcPk:          fpPubKey2,
					TotalBondedSat: 2000,
					IsTimestamped:  false,
				},
			},
		},
		{
			desc:             "all timestamped",
			maxActiveFPs:     80,
			numActiveFps:     2,
			numInactiveFps:   0,
			totalVotingPower: 3000,
			prevDistCache:    types.NewVotingPowerDistCache(),
			fps: []*types.FinalityProviderDistInfo{
				{
					BtcPk:          fpPubKey1,
					TotalBondedSat: 1000,
					IsTimestamped:  true,
				},
				{
					BtcPk:          fpPubKey2,
					TotalBondedSat: 2000,
					IsTimestamped:  true,
				},
			},
		},
		{
			desc:             "partly timestamped",
			maxActiveFPs:     80,
			numActiveFps:     1,
			numInactiveFps:   1,
			totalVotingPower: 1000,
			prevDistCache:    types.NewVotingPowerDistCache(),
			fps: []*types.FinalityProviderDistInfo{
				{
					BtcPk:          fpPubKey1,
					TotalBondedSat: 1000,
					IsTimestamped:  true,
				},
				{
					BtcPk:          fpPubKey2,
					TotalBondedSat: 2000,
					IsTimestamped:  false,
				},
			},
		},
		{
			desc:             "small max active fps",
			maxActiveFPs:     1,
			numActiveFps:     1,
			numInactiveFps:   1,
			totalVotingPower: 2000,
			prevDistCache:    types.NewVotingPowerDistCache(),
			fps: []*types.FinalityProviderDistInfo{
				{
					BtcPk:          fpPubKey1,
					TotalBondedSat: 1000,
					IsTimestamped:  true,
				},
				{
					BtcPk:          fpPubKey2,
					TotalBondedSat: 2000,
					IsTimestamped:  true,
				},
			},
		},
		{
			desc:             "one got jailed",
			maxActiveFPs:     80,
			numActiveFps:     1,
			numInactiveFps:   0,
			totalVotingPower: 1000,
			prevDistCache:    types.NewVotingPowerDistCache(),
			fps: []*types.FinalityProviderDistInfo{
				{
					BtcPk:          fpPubKey1,
					TotalBondedSat: 1000,
					IsTimestamped:  true,
				},
				{
					BtcPk:          fpPubKey2,
					TotalBondedSat: 2000,
					IsTimestamped:  true,
					IsJailed:       true,
				},
			},
		},
		{
			desc:             "one got slashed",
			maxActiveFPs:     80,
			numActiveFps:     1,
			numInactiveFps:   0,
			totalVotingPower: 1000,
			prevDistCache:    types.NewVotingPowerDistCache(),
			fps: []*types.FinalityProviderDistInfo{
				{
					BtcPk:          fpPubKey1,
					TotalBondedSat: 1000,
					IsTimestamped:  true,
				},
				{
					BtcPk:          fpPubKey2,
					TotalBondedSat: 0, // a jailed fp cannot accept delegation
					IsTimestamped:  true,
					IsSlashed:      true,
				},
			},
		},
		{
			desc:             "previous one got unjailed",
			maxActiveFPs:     80,
			numActiveFps:     1,
			numInactiveFps:   0,
			totalVotingPower: 1000,
			prevDistCache: types.NewVotingPowerDistCacheWithFinalityProviders(
				[]*types.FinalityProviderDistInfo{
					{
						BtcPk:          fpPubKey1,
						TotalBondedSat: 1000,
						IsTimestamped:  true,
						IsJailed:       true,
					}}),
			fps: []*types.FinalityProviderDistInfo{
				{
					BtcPk:          fpPubKey1,
					TotalBondedSat: 1000,
					IsTimestamped:  true,
					IsJailed:       false,
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			dc := types.NewVotingPowerDistCacheWithFinalityProviders(tc.fps)
			dc.ApplyActiveFinalityProviders(tc.maxActiveFPs)
			require.Equal(t, tc.totalVotingPower, dc.TotalVotingPower)
			require.Equal(t, tc.numActiveFps, dc.NumActiveFps)

			newActiveFps := dc.FindNewActiveFinalityProviders(tc.prevDistCache)
			require.Equal(t, tc.numActiveFps, uint32(len(newActiveFps)))

			newInactiveFps := dc.FindNewInactiveFinalityProviders(tc.prevDistCache)
			require.Equal(t, tc.numInactiveFps, uint32(len(newInactiveFps)))
		})
	}
}

func TestSortFinalityProvidersWithZeroedVotingPower(t *testing.T) {
	tests := []struct {
		name     string
		fps      []*types.FinalityProviderDistInfo
		expected []*types.FinalityProviderDistInfo
	}{
		{
			name: "Sort by voting power",
			fps: []*types.FinalityProviderDistInfo{
				{Addr: []byte("fp1"), TotalBondedSat: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x03}},
				{Addr: []byte("fp2"), TotalBondedSat: 200, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x02}},
				{Addr: []byte("fp3"), TotalBondedSat: 150, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x01}},
			},
			expected: []*types.FinalityProviderDistInfo{
				{Addr: []byte("fp2"), TotalBondedSat: 200, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x02}},
				{Addr: []byte("fp3"), TotalBondedSat: 150, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x01}},
				{Addr: []byte("fp1"), TotalBondedSat: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x03}},
			},
		},
		{
			name: "Jailed and non-timestamped providers at the end",
			fps: []*types.FinalityProviderDistInfo{
				{Addr: []byte("fp1"), TotalBondedSat: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x04}},
				{Addr: []byte("fp2"), TotalBondedSat: 200, IsJailed: true, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x02}},
				{Addr: []byte("fp3"), TotalBondedSat: 150, IsJailed: false, IsTimestamped: false, BtcPk: &bbn.BIP340PubKey{0x03}},
				{Addr: []byte("fp4"), TotalBondedSat: 50, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x01}},
			},
			expected: []*types.FinalityProviderDistInfo{
				{Addr: []byte("fp1"), TotalBondedSat: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x04}},
				{Addr: []byte("fp4"), TotalBondedSat: 50, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x01}},
				{Addr: []byte("fp2"), TotalBondedSat: 200, IsJailed: true, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x02}},
				{Addr: []byte("fp3"), TotalBondedSat: 150, IsJailed: false, IsTimestamped: false, BtcPk: &bbn.BIP340PubKey{0x03}},
			},
		},
		{
			name: "Equal voting power, sort by BTC public key",
			fps: []*types.FinalityProviderDistInfo{
				{Addr: []byte("fp1"), TotalBondedSat: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x03}},
				{Addr: []byte("fp2"), TotalBondedSat: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x01}},
				{Addr: []byte("fp3"), TotalBondedSat: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x02}},
			},
			expected: []*types.FinalityProviderDistInfo{
				{Addr: []byte("fp2"), TotalBondedSat: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x01}},
				{Addr: []byte("fp3"), TotalBondedSat: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x02}},
				{Addr: []byte("fp1"), TotalBondedSat: 100, IsJailed: false, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x03}},
			},
		},
		{
			name: "Zeroed voting power, sort by BTC public key",
			fps: []*types.FinalityProviderDistInfo{
				{Addr: []byte("fp1"), TotalBondedSat: 200, IsJailed: true, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x03}},
				{Addr: []byte("fp2"), TotalBondedSat: 150, IsJailed: false, IsTimestamped: false, BtcPk: &bbn.BIP340PubKey{0x01}},
				{Addr: []byte("fp3"), TotalBondedSat: 100, IsJailed: true, IsTimestamped: false, BtcPk: &bbn.BIP340PubKey{0x02}},
			},
			expected: []*types.FinalityProviderDistInfo{
				{Addr: []byte("fp2"), TotalBondedSat: 150, IsJailed: false, IsTimestamped: false, BtcPk: &bbn.BIP340PubKey{0x01}},
				{Addr: []byte("fp3"), TotalBondedSat: 100, IsJailed: true, IsTimestamped: false, BtcPk: &bbn.BIP340PubKey{0x02}},
				{Addr: []byte("fp1"), TotalBondedSat: 200, IsJailed: true, IsTimestamped: true, BtcPk: &bbn.BIP340PubKey{0x03}},
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

func FuzzNewFinalityProviderDistInfo(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 1000)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		fp, err := datagen.GenRandomFinalityProvider(r)
		require.NoError(t, err)

		if r.Int31n(10) > 5 {
			fp.SlashedBabylonHeight = r.Uint64()
		}
		if r.Int31n(10) > 5 {
			fp.Jailed = true
		}

		fpDstInf := types.NewFinalityProviderDistInfo(fp)
		require.Equal(t, fpDstInf.BtcPk.MarshalHex(), fp.BtcPk.MarshalHex())
		require.Equal(t, sdk.AccAddress(fpDstInf.Addr).String(), fp.Addr)
		require.Equal(t, fpDstInf.Commission.String(), fp.Commission.String())
		require.Equal(t, fpDstInf.TotalBondedSat, uint64(0))
		require.Equal(t, fpDstInf.IsJailed, fp.Jailed)
		require.Equal(t, fpDstInf.IsSlashed, fp.IsSlashed())
		require.Equal(t, fpDstInf.IsTimestamped, false)
	})
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
			{TotalBondedSat: vp0, IsJailed: false, IsTimestamped: true, Addr: []byte("addr0"), BtcPk: pk1},
			{TotalBondedSat: vp1, IsJailed: false, IsTimestamped: true, Addr: []byte("addr1"), BtcPk: pk2},
			{TotalBondedSat: vp2, IsJailed: false, IsTimestamped: true, Addr: []byte("addr2"), BtcPk: pk3},
			{TotalBondedSat: vp3, IsJailed: false, IsTimestamped: true, Addr: []byte("addr3"), BtcPk: pk4},
			{TotalBondedSat: vp4, IsJailed: false, IsTimestamped: true, Addr: []byte("addr4"), BtcPk: pk5},
		}
		jailedIdx := datagen.RandomInt(r, len(fpsWithMeta))
		noTimestampedIdx := datagen.RandomIntOtherThan(r, int(jailedIdx), len(fpsWithMeta))

		fpsWithMeta[jailedIdx].IsJailed = true
		fpsWithMeta[jailedIdx].IsTimestamped = false
		fpsWithMeta[noTimestampedIdx].IsJailed = false
		fpsWithMeta[noTimestampedIdx].IsTimestamped = false

		fpsWithMeta1 := []*types.FinalityProviderDistInfo{
			{TotalBondedSat: vp0, IsJailed: false, IsTimestamped: true, Addr: []byte("addr0"), BtcPk: pk1},
			{TotalBondedSat: vp1, IsJailed: false, IsTimestamped: true, Addr: []byte("addr1"), BtcPk: pk2},
			{TotalBondedSat: vp2, IsJailed: false, IsTimestamped: true, Addr: []byte("addr2"), BtcPk: pk3},
			{TotalBondedSat: vp3, IsJailed: false, IsTimestamped: true, Addr: []byte("addr3"), BtcPk: pk4},
			{TotalBondedSat: vp4, IsJailed: false, IsTimestamped: true, Addr: []byte("addr4"), BtcPk: pk5},
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

func TestVotingPowerDistCache_Validate(t *testing.T) {
	t.Parallel()
	tcs := []struct {
		name      string
		vpdc      types.VotingPowerDistCache
		expErrMsg string
	}{
		{
			name: "empty finality providers - valid",
			vpdc: types.VotingPowerDistCache{},
		},
		{
			name: "NumActiveFps exceeds number of providers",
			vpdc: types.VotingPowerDistCache{
				FinalityProviders: []*types.FinalityProviderDistInfo{
					{
						BtcPk:          fpPubKey1,
						TotalBondedSat: 100,
						Addr:           fpAddr1,
						Commission:     &validComm,
						IsTimestamped:  true,
					},
				},
				NumActiveFps:     2,
				TotalVotingPower: 100,
			},
			expErrMsg: "invalid voting power distribution cache. NumActiveFps 2 is higher than active FPs count 1",
		},
		{
			name: "duplicate finality providers",
			vpdc: types.VotingPowerDistCache{
				FinalityProviders: []*types.FinalityProviderDistInfo{
					{
						BtcPk:          fpPubKey1,
						TotalBondedSat: 100,
						Addr:           fpAddr1,
						Commission:     &validComm,
					},
					{
						BtcPk:          fpPubKey1,
						TotalBondedSat: 200,
						Addr:           fpAddr1,
						Commission:     &validComm,
					},
				},
				NumActiveFps:     2,
				TotalVotingPower: 300,
			},
			expErrMsg: "invalid voting power distribution cache. Duplicate finality provider entry with BTC PK",
		},
		{
			name: "invalid finality provider PK fails",
			vpdc: types.VotingPowerDistCache{
				FinalityProviders: []*types.FinalityProviderDistInfo{
					{
						BtcPk:          &bbn.BIP340PubKey{},
						TotalBondedSat: 100,
						Addr:           fpAddr1,
						Commission:     &validComm,
					},
				},
				NumActiveFps:     1,
				TotalVotingPower: 100,
			},
			expErrMsg: "invalid fp dist info. finality provider BTC public key length: got 0, want 32",
		},
		{
			name: "voting power mismatch",
			vpdc: types.VotingPowerDistCache{
				FinalityProviders: []*types.FinalityProviderDistInfo{
					{
						BtcPk:          fpPubKey1,
						TotalBondedSat: 100,
						Addr:           fpAddr1,
						Commission:     &validComm,
						IsTimestamped:  true,
					},
					{
						BtcPk:          fpPubKey2,
						TotalBondedSat: 100,
						Addr:           fpAddr2,
						Commission:     &validComm,
						IsTimestamped:  true,
					},
				},
				NumActiveFps:     2,
				TotalVotingPower: 150,
			},
			expErrMsg: "invalid voting power distribution cache. Provided TotalVotingPower 150 is different than FPs accumulated voting power 200",
		},
		{
			name: "no address",
			vpdc: types.VotingPowerDistCache{
				FinalityProviders: []*types.FinalityProviderDistInfo{
					{BtcPk: fpPubKey1, TotalBondedSat: 100, Commission: &validComm},
				},
			},
			expErrMsg: "invalid fp dist info. empty finality provider address",
		},
		{
			name: "bad address",
			vpdc: types.VotingPowerDistCache{
				FinalityProviders: []*types.FinalityProviderDistInfo{
					{BtcPk: fpPubKey1, TotalBondedSat: 100, Commission: &validComm, Addr: []byte("badaddr")},
				},
			},
			expErrMsg: "invalid bech32 address: address length must be 20 or 32 bytes, got 7: unknown address",
		},
		{
			name: "commission lower than 0",
			vpdc: types.VotingPowerDistCache{
				FinalityProviders: []*types.FinalityProviderDistInfo{
					{BtcPk: fpPubKey1, TotalBondedSat: 100, Commission: &negComm, Addr: fpAddr1},
				},
			},
			expErrMsg: "invalid fp dist info. commission is negative",
		},
		{
			name: "commission greater than 1",
			vpdc: types.VotingPowerDistCache{
				FinalityProviders: []*types.FinalityProviderDistInfo{
					{BtcPk: fpPubKey1, TotalBondedSat: 100, Commission: &highComm, Addr: fpAddr1},
				},
			},
			expErrMsg: "invalid fp dist info. commission is greater than 1",
		},
		{
			name: "one inactive case",
			vpdc: types.VotingPowerDistCache{
				FinalityProviders: []*types.FinalityProviderDistInfo{
					{
						BtcPk:          fpPubKey1,
						TotalBondedSat: 100,
						IsTimestamped:  true,
						Addr:           fpAddr1,
						Commission:     &validComm,
					},
					{
						BtcPk:          fpPubKey2,
						TotalBondedSat: 200,
						IsTimestamped:  true,
						Addr:           []byte(fpAddr2),
						Commission:     &validComm,
					},
					{
						BtcPk:          fpPubKey3,
						TotalBondedSat: 100,
						IsTimestamped:  true,
						IsJailed:       true,
						Addr:           []byte(fpAddr3),
						Commission:     &validComm,
					},
				},
				NumActiveFps:     2,
				TotalVotingPower: 300,
			},
		},
		{
			name: "valid case",
			vpdc: types.VotingPowerDistCache{
				FinalityProviders: []*types.FinalityProviderDistInfo{
					{
						BtcPk:          fpPubKey1,
						TotalBondedSat: 100,
						Addr:           fpAddr1,
						Commission:     &validComm,
						IsTimestamped:  true,
					},
					{
						BtcPk:          fpPubKey2,
						TotalBondedSat: 200,
						Addr:           fpAddr2,
						Commission:     &validComm,
						IsTimestamped:  true,
					},
				},
				NumActiveFps:     2,
				TotalVotingPower: 300,
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.vpdc.Validate()
			if tc.expErrMsg == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorContains(t, err, tc.expErrMsg)
		})
	}
}
