package types_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bbntypes "github.com/babylonlabs-io/babylon/v4/types"
	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/v4/x/finality/types"

	"github.com/stretchr/testify/require"
)

func TestEvidence_ValidateBasic(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	sk, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	validEvidence, err := datagen.GenRandomEvidence(r, sk, datagen.RandomIntOtherThan(r, 0, 10000))
	require.NoError(t, err)

	invalidPk := bbntypes.BIP340PubKey(make([]byte, 10))    // wrong size
	invalidSig := bbntypes.SchnorrEOTSSig(make([]byte, 10)) // wrong size

	testCases := []struct {
		name   string
		ev     types.Evidence
		expErr string
	}{
		{
			name:   "nil FpBtcPk",
			ev:     types.Evidence{},
			expErr: "empty FpBtcPk",
		},
		{
			name: "invalid FpBtcPk",
			ev: types.Evidence{
				FpBtcPk:              &invalidPk,
				BlockHeight:          validEvidence.BlockHeight,
				PubRand:              validEvidence.PubRand,
				CanonicalAppHash:     validEvidence.CanonicalAppHash,
				ForkAppHash:          validEvidence.ForkAppHash,
				CanonicalFinalitySig: validEvidence.CanonicalFinalitySig,
				ForkFinalitySig:      validEvidence.ForkFinalitySig,
			},
			expErr: "bad pubkey byte string size",
		},
		{
			name: "nil PubRand",
			ev: types.Evidence{
				FpBtcPk:              validEvidence.FpBtcPk,
				BlockHeight:          validEvidence.BlockHeight,
				PubRand:              nil,
				CanonicalAppHash:     validEvidence.CanonicalAppHash,
				ForkAppHash:          validEvidence.ForkAppHash,
				CanonicalFinalitySig: validEvidence.CanonicalFinalitySig,
				ForkFinalitySig:      validEvidence.ForkFinalitySig,
			},
			expErr: "empty PubRand",
		},
		{
			name: "malformed CanonicalAppHash",
			ev: types.Evidence{
				FpBtcPk:              validEvidence.FpBtcPk,
				BlockHeight:          validEvidence.BlockHeight,
				PubRand:              validEvidence.PubRand,
				CanonicalAppHash:     []byte("short"),
				ForkAppHash:          validEvidence.ForkAppHash,
				CanonicalFinalitySig: validEvidence.CanonicalFinalitySig,
				ForkFinalitySig:      validEvidence.ForkFinalitySig,
			},
			expErr: "malformed CanonicalAppHash",
		},
		{
			name: "malformed ForkAppHash",
			ev: types.Evidence{
				FpBtcPk:              validEvidence.FpBtcPk,
				BlockHeight:          validEvidence.BlockHeight,
				PubRand:              validEvidence.PubRand,
				CanonicalAppHash:     validEvidence.CanonicalAppHash,
				ForkAppHash:          []byte("short"),
				CanonicalFinalitySig: validEvidence.CanonicalFinalitySig,
				ForkFinalitySig:      validEvidence.ForkFinalitySig,
			},
			expErr: "malformed ForkAppHash",
		},
		{
			name: "nil ForkFinalitySig",
			ev: types.Evidence{
				FpBtcPk:              validEvidence.FpBtcPk,
				BlockHeight:          validEvidence.BlockHeight,
				PubRand:              validEvidence.PubRand,
				CanonicalAppHash:     validEvidence.CanonicalAppHash,
				ForkAppHash:          validEvidence.ForkAppHash,
				CanonicalFinalitySig: validEvidence.CanonicalFinalitySig,
				ForkFinalitySig:      nil,
			},
			expErr: "empty ForkFinalitySig",
		},
		{
			name: "malformed ForkFinalitySig",
			ev: types.Evidence{
				FpBtcPk:              validEvidence.FpBtcPk,
				BlockHeight:          validEvidence.BlockHeight,
				PubRand:              validEvidence.PubRand,
				CanonicalAppHash:     validEvidence.CanonicalAppHash,
				ForkAppHash:          validEvidence.ForkAppHash,
				CanonicalFinalitySig: validEvidence.CanonicalFinalitySig,
				ForkFinalitySig:      &invalidSig,
			},
			expErr: "malformed ForkFinalitySig",
		},
		{
			name:   "valid evidence",
			ev:     *validEvidence,
			expErr: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.ev.ValidateBasic()
			if tc.expErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestFinalityProviderDistInfoFpStatus(t *testing.T) {
	tcs := []struct {
		name           string
		fp             types.FinalityProviderDistInfo
		canBeActive    bool
		expectedStatus btcstktypes.FinalityProviderStatus
	}{
		{
			name: "slashed fp",
			fp: types.FinalityProviderDistInfo{
				IsSlashed:      true,
				IsJailed:       false,
				IsTimestamped:  true,
				TotalBondedSat: 1000,
			},
			canBeActive:    true,
			expectedStatus: btcstktypes.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_SLASHED,
		},
		{
			name: "jailed fp (not slashed)",
			fp: types.FinalityProviderDistInfo{
				IsSlashed:      false,
				IsJailed:       true,
				IsTimestamped:  true,
				TotalBondedSat: 1000,
			},
			canBeActive:    true,
			expectedStatus: btcstktypes.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_JAILED,
		},
		{
			name: "slashed and jailed fp (slashed takes precedence)",
			fp: types.FinalityProviderDistInfo{
				IsSlashed:      true,
				IsJailed:       true,
				IsTimestamped:  true,
				TotalBondedSat: 1000,
			},
			canBeActive:    true,
			expectedStatus: btcstktypes.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_SLASHED,
		},
		{
			name: "active fp - all conditions met",
			fp: types.FinalityProviderDistInfo{
				IsSlashed:      false,
				IsJailed:       false,
				IsTimestamped:  true,
				TotalBondedSat: 1000,
			},
			canBeActive:    true,
			expectedStatus: btcstktypes.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_ACTIVE,
		},
		{
			name: "inactive fp - canBeActive is false",
			fp: types.FinalityProviderDistInfo{
				IsSlashed:      false,
				IsJailed:       false,
				IsTimestamped:  true,
				TotalBondedSat: 1000,
			},
			canBeActive:    false,
			expectedStatus: btcstktypes.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_INACTIVE,
		},
		{
			name: "inactive fp - not timestamped",
			fp: types.FinalityProviderDistInfo{
				IsSlashed:      false,
				IsJailed:       false,
				IsTimestamped:  false,
				TotalBondedSat: 1000,
			},
			canBeActive:    true,
			expectedStatus: btcstktypes.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_INACTIVE,
		},
		{
			name: "inactive fp - zero bonded sats",
			fp: types.FinalityProviderDistInfo{
				IsSlashed:      false,
				IsJailed:       false,
				IsTimestamped:  true,
				TotalBondedSat: 0,
			},
			canBeActive:    true,
			expectedStatus: btcstktypes.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_INACTIVE,
		},
		{
			name: "inactive fp - multiple conditions failing",
			fp: types.FinalityProviderDistInfo{
				IsSlashed:      false,
				IsJailed:       false,
				IsTimestamped:  false,
				TotalBondedSat: 0,
			},
			canBeActive:    false,
			expectedStatus: btcstktypes.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_INACTIVE,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			actualStatus := tc.fp.FpStatus(tc.canBeActive)
			require.Equal(t, tc.expectedStatus, actualStatus)
		})
	}
}
