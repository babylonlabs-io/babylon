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

func TestChainInfo_Validate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	validHeader := datagen.GenRandomIndexedHeader(r)

	testCases := []struct {
		name        string
		chainInfo   types.ChainInfo
		expectError string
	}{
		{
			name:        "Empty ConsumerId",
			chainInfo:   types.ChainInfo{},
			expectError: "ConsumerId is empty",
		},
		{
			name: "Nil LatestHeader",
			chainInfo: types.ChainInfo{
				ConsumerId: "chain-A",
			},
			expectError: "LatestHeader is nil",
		},
		{
			name: "Nil LatestForks",
			chainInfo: types.ChainInfo{
				ConsumerId:   "chain-A",
				LatestHeader: validHeader,
			},
			expectError: "LatestForks is nil",
		},
		{
			name: "Invalid LatestHeader",
			chainInfo: types.ChainInfo{
				ConsumerId:   "chain-A",
				LatestHeader: &types.IndexedHeader{},
				LatestForks:  &types.Forks{},
			},
			expectError: "empty ConsumerID",
		},
		{
			name: "Invalid ForkHeader",
			chainInfo: types.ChainInfo{
				ConsumerId:   "chain-A",
				LatestHeader: validHeader,
				LatestForks: &types.Forks{
					Headers: []*types.IndexedHeader{&types.IndexedHeader{}},
				},
			},
			expectError: "empty ConsumerID",
		},
		{
			name: "Valid ChainInfo",
			chainInfo: types.ChainInfo{
				ConsumerId:   "chain-A",
				LatestHeader: validHeader,
				LatestForks: &types.Forks{
					Headers: []*types.IndexedHeader{validHeader},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.chainInfo.Validate()
			if tc.expectError == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorContains(t, err, tc.expectError)
		})
	}
}

func TestForks_Validate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	testCases := []struct {
		name        string
		forks       types.Forks
		expectError string
	}{
		{
			name: "Single valid header",
			forks: types.Forks{
				Headers: []*types.IndexedHeader{
					datagen.GenRandomIndexedHeaderWithConsumerAndEpoch(r, "chain-A", 1),
				},
			},
		},
		{
			name: "Multiple unique headers",
			forks: types.Forks{
				Headers: []*types.IndexedHeader{
					datagen.GenRandomIndexedHeaderWithConsumerAndEpoch(r, "chain-A", 1),
					datagen.GenRandomIndexedHeaderWithConsumerAndEpoch(r, "chain-B", 2),
					datagen.GenRandomIndexedHeaderWithConsumerAndEpoch(r, "chain-A", 2),
				},
			},
		},
		{
			name:        "Empty headers",
			forks:       types.Forks{Headers: []*types.IndexedHeader{}},
			expectError: "empty headers",
		},
		{
			name: "Duplicate ConsumerId + Epoch",
			forks: types.Forks{
				Headers: []*types.IndexedHeader{
					datagen.GenRandomIndexedHeaderWithConsumerAndEpoch(r, "chain-A", 1),
					datagen.GenRandomIndexedHeaderWithConsumerAndEpoch(r, "chain-A", 1),
				},
			},
			expectError: "duplicate entry",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.forks.Validate()
			if tc.expectError == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorContains(t, err, tc.expectError)
		})
	}
}

func TestChainInfoWithProof_Validate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	validChainInfo := &types.ChainInfo{
		ConsumerId:   "chain-A",
		LatestHeader: datagen.GenRandomIndexedHeader(r),
		LatestForks:  &types.Forks{},
	}

	invalidChainInfo := &types.ChainInfo{
		ConsumerId:   "",
		LatestHeader: nil,
		LatestForks:  nil,
	}

	testCases := []struct {
		name        string
		input       types.ChainInfoWithProof
		expectError string
	}{
		{
			name: "Nil ChainInfo",
			input: types.ChainInfoWithProof{
				ChainInfo:          nil,
				ProofHeaderInEpoch: &crypto.ProofOps{},
			},
			expectError: "empty chain info",
		},
		{
			name: "Nil ProofHeaderInEpoch",
			input: types.ChainInfoWithProof{
				ChainInfo:          validChainInfo,
				ProofHeaderInEpoch: nil,
			},
			expectError: "empty proof",
		},
		{
			name: "Invalid ChainInfo (fails its Validate)",
			input: types.ChainInfoWithProof{
				ChainInfo:          invalidChainInfo,
				ProofHeaderInEpoch: &crypto.ProofOps{},
			},
			expectError: "ConsumerId is empty",
		},
		{
			name: "Valid types.ChainInfoWithProof",
			input: types.ChainInfoWithProof{
				ChainInfo:          validChainInfo,
				ProofHeaderInEpoch: &crypto.ProofOps{},
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
