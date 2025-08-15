package types_test

import (
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	crypto "github.com/cometbft/cometbft/proto/tendermint/crypto"

	bbntypes "github.com/babylonlabs-io/babylon/v4/types"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"

	"github.com/stretchr/testify/require"
)

func TestIndexedHeader_Validate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	validHeader := datagen.GenRandomIndexedHeader(r)

	testCases := []struct {
		name        string
		header      types.IndexedHeader
		expectError string
	}{
		{
			name:        "Empty ConsumerId",
			header:      types.IndexedHeader{},
			expectError: "empty ConsumerID",
		},
		{
			name:   "Valid IndexedHeader",
			header: *validHeader,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.header.Validate()
			if tc.expectError == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorContains(t, err, tc.expectError)
		})
	}
}

func TestIndexedHeaderWithProof_Validate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	validHeader := datagen.GenRandomIndexedHeader(r)

	invalidHeader := &types.IndexedHeader{
		ConsumerId: "",
	}

	testCases := []struct {
		name        string
		input       types.IndexedHeaderWithProof
		expectError string
	}{
		{
			name: "Nil Header",
			input: types.IndexedHeaderWithProof{
				Header: nil,
				Proof:  &crypto.ProofOps{},
			},
			expectError: "empty header",
		},
		{
			name: "Nil Proof (should pass validation since Proof is not validated)",
			input: types.IndexedHeaderWithProof{
				Header: validHeader,
				Proof:  nil,
			},
			expectError: "",
		},
		{
			name: "Invalid Header (fails its Validate)",
			input: types.IndexedHeaderWithProof{
				Header: invalidHeader,
				Proof:  &crypto.ProofOps{},
			},
			expectError: "empty ConsumerID",
		},
		{
			name: "Valid IndexedHeaderWithProof",
			input: types.IndexedHeaderWithProof{
				Header: validHeader,
				Proof:  &crypto.ProofOps{},
			},
			expectError: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			if tc.expectError == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorContains(t, err, tc.expectError)
		})
	}
}

func TestBTCChainSegment_Validate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	btcHeader := datagen.GenRandomBtcdHeader(r)
	blkHash := btcHeader.BlockHash()
	validHashBytes := bbntypes.NewBTCHeaderHashBytesFromChainhash(&blkHash)
	validHeaderBytes := bbntypes.NewBTCHeaderBytesFromBlockHeader(btcHeader)
	nonZeroWork := math.NewUint(100)
	zeroWork := math.ZeroUint()

	testCases := []struct {
		name        string
		input       types.BTCChainSegment
		expectError string
	}{
		{
			name: "Empty BTC headers",
			input: types.BTCChainSegment{
				BtcHeaders: []*btclctypes.BTCHeaderInfo{},
			},
			expectError: "empty headers",
		},
		{
			name: "Header is nil",
			input: types.BTCChainSegment{
				BtcHeaders: []*btclctypes.BTCHeaderInfo{
					{Header: nil, Hash: &validHashBytes, Work: &nonZeroWork},
				},
			},
			expectError: "header is nil",
		},
		{
			name: "Work is zero",
			input: types.BTCChainSegment{
				BtcHeaders: []*btclctypes.BTCHeaderInfo{
					{Header: &validHeaderBytes, Hash: &validHashBytes, Work: &zeroWork},
				},
			},
			expectError: "work is zero",
		},
		{
			name: "Valid BTC header info",
			input: types.BTCChainSegment{
				BtcHeaders: []*btclctypes.BTCHeaderInfo{
					{Header: &validHeaderBytes, Hash: &validHashBytes, Work: &nonZeroWork},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			if tc.expectError == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorContains(t, err, tc.expectError)
		})
	}
}
