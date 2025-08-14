package cli_test

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	abci "github.com/cometbft/cometbft/abci/types"
	cmtbytes "github.com/cometbft/cometbft/libs/bytes"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpcclientmock "github.com/cometbft/cometbft/rpc/client/mock"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/app"
	testutilcli "github.com/babylonlabs-io/babylon/v4/testutil/cli"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btcstakingcli "github.com/babylonlabs-io/babylon/v4/x/btcstaking/client/cli"
)

type mockCometRPC struct {
	rpcclientmock.Client
	responseQuery abci.ResponseQuery
}

func newMockCometRPC(respQuery abci.ResponseQuery) mockCometRPC {
	return mockCometRPC{responseQuery: respQuery}
}

func (mockCometRPC) BroadcastTxSync(_ context.Context, _ cmttypes.Tx) (*coretypes.ResultBroadcastTx, error) {
	return &coretypes.ResultBroadcastTx{}, nil
}

func (m mockCometRPC) ABCIQueryWithOptions(
	_ context.Context,
	_ string, _ cmtbytes.HexBytes,
	_ rpcclient.ABCIQueryOptions,
) (*coretypes.ResultABCIQuery, error) {
	return &coretypes.ResultABCIQuery{Response: m.responseQuery}, nil
}

// setupClientCtx creates a test client context for CLI testing
func setupClientCtx(t *testing.T) (client.Context, []sdk.AccAddress) {
	t.Helper()
	encCfg := app.GetEncodingConfig()
	kr := keyring.NewInMemory(encCfg.Codec)

	bz, _ := encCfg.Codec.Marshal(&sdk.TxResponse{})
	c := newMockCometRPC(abci.ResponseQuery{Value: bz})

	clientCtx := client.Context{}.
		WithKeyring(kr).
		WithTxConfig(encCfg.TxConfig).
		WithCodec(encCfg.Codec).
		WithClient(c).
		WithAccountRetriever(client.MockAccountRetriever{}).
		WithOutput(io.Discard).
		WithChainID("test-chain")

	// Create test addresses
	addrs := make([]sdk.AccAddress, 0)
	for i := 0; i < 3; i++ {
		k, _, err := clientCtx.Keyring.NewMnemonic("NewValidator", keyring.English, sdk.FullFundraiserPath, keyring.DefaultBIP39Passphrase, hd.Secp256k1)
		require.NoError(t, err)

		pub, err := k.GetPubKey()
		require.NoError(t, err)

		newAddr := sdk.AccAddress(pub.Address())
		addrs = append(addrs, newAddr)
	}

	return clientCtx, addrs
}

func TestParseFpRatios(t *testing.T) {
	// Generate valid BTC public keys for testing
	r := rand.New(rand.NewSource(time.Now().Unix()))
	_, fpPK1, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	_, fpPK2, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	fp1Hex := bbn.NewBIP340PubKeyFromBTCPK(fpPK1).MarshalHex()
	fp2Hex := bbn.NewBIP340PubKeyFromBTCPK(fpPK2).MarshalHex()

	testCases := []struct {
		name        string
		input       string
		expectError bool
		errorMsg    string
		expectedLen int
	}{
		{
			name:        "empty input",
			input:       "",
			expectError: true,
			errorMsg:    "FP ratios cannot be empty",
		},
		{
			name:        "valid single ratio",
			input:       fmt.Sprintf("%s:1.0", fp1Hex),
			expectError: false,
			expectedLen: 1,
		},
		{
			name:        "valid multiple ratios",
			input:       fmt.Sprintf("%s:0.6,%s:0.4", fp1Hex, fp2Hex),
			expectError: false,
			expectedLen: 2,
		},
		{
			name:        "invalid format - missing colon",
			input:       fmt.Sprintf("%s_0.5", fp1Hex),
			expectError: true,
			errorMsg:    "invalid FP ratio format",
		},
		{
			name:        "invalid format - too many parts",
			input:       fmt.Sprintf("%s:0.5:extra", fp1Hex),
			expectError: true,
			errorMsg:    "invalid FP ratio format",
		},
		{
			name:        "invalid BTC public key",
			input:       "invalid_hex:0.5",
			expectError: true,
			errorMsg:    "invalid BTC public key",
		},
		{
			name:        "invalid ratio - negative",
			input:       fmt.Sprintf("%s:-0.1", fp1Hex),
			expectError: true,
			errorMsg:    "ratio must be between 0 and 1",
		},
		{
			name:        "invalid ratio - greater than 1",
			input:       fmt.Sprintf("%s:1.5", fp1Hex),
			expectError: true,
			errorMsg:    "ratio must be between 0 and 1",
		},
		{
			name:        "invalid ratio - not a number",
			input:       fmt.Sprintf("%s:invalid", fp1Hex),
			expectError: true,
			errorMsg:    "invalid ratio",
		},
		{
			name:        "valid zero ratio",
			input:       fmt.Sprintf("%s:0.0", fp1Hex),
			expectError: false,
			expectedLen: 1,
		},
		{
			name:        "valid edge case - exactly 1.0",
			input:       fmt.Sprintf("%s:1.0", fp1Hex),
			expectError: false,
			expectedLen: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := btcstakingcli.ParseFpRatios(tc.input)

			if tc.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errorMsg)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.Len(t, result, tc.expectedLen)

				// Verify all ratios are within valid range
				for _, ratio := range result {
					require.False(t, ratio.Ratio.IsNegative())
					require.True(t, ratio.Ratio.LTE(sdkmath.LegacyOneDec()))
				}
			}
		})
	}
}

func TestNewAddBsnRewardsCmd(t *testing.T) {
	clientCtx, addrs := setupClientCtx(t)

	// Generate valid BTC public keys for testing
	r := rand.New(rand.NewSource(time.Now().Unix()))
	_, fpPK1, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	_, fpPK2, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	fp1Hex := bbn.NewBIP340PubKeyFromBTCPK(fpPK1).MarshalHex()
	fp2Hex := bbn.NewBIP340PubKeyFromBTCPK(fpPK2).MarshalHex()

	cmd := btcstakingcli.NewAddBsnRewardsCmd()

	// Helper function to build command args with standard flags
	buildArgs := func(consumerID, rewards, fpRatios string) []string {
		return []string{
			consumerID,
			rewards,
			fpRatios,
			fmt.Sprintf("--%s=%s", flags.FlagFrom, addrs[0]),
			fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
			fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
			fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10)).String()),
		}
	}

	testCases := []struct {
		name         string
		consumerID   string
		rewards      string
		fpRatios     string
		expectErrMsg string
	}{
		{
			name:         "valid command with single FP",
			consumerID:   "test-consumer",
			rewards:      "1000ubbn",
			fpRatios:     fmt.Sprintf("%s:1.0", fp1Hex),
			expectErrMsg: "",
		},
		{
			name:         "valid command with multiple FPs",
			consumerID:   "test-consumer",
			rewards:      "1000ubbn",
			fpRatios:     fmt.Sprintf("%s:0.6,%s:0.4", fp1Hex, fp2Hex),
			expectErrMsg: "",
		},
		{
			name:         "empty BSN consumer ID",
			consumerID:   "", // empty consumer ID
			rewards:      "1000ubbn",
			fpRatios:     fmt.Sprintf("%s:1.0", fp1Hex),
			expectErrMsg: "BSN consumer ID cannot be empty",
		},
		{
			name:         "invalid total rewards format",
			consumerID:   "test-consumer",
			rewards:      "invalid_coins", // invalid coin format
			fpRatios:     fmt.Sprintf("%s:1.0", fp1Hex),
			expectErrMsg: "invalid total rewards",
		},
		{
			name:         "invalid FP ratios format",
			consumerID:   "test-consumer",
			rewards:      "1000ubbn",
			fpRatios:     "invalid_ratios", // invalid ratio format
			expectErrMsg: "invalid FP ratios",
		},
		{
			name:         "ratios do not sum to 1.0",
			consumerID:   "test-consumer",
			rewards:      "1000ubbn",
			fpRatios:     fmt.Sprintf("%s:0.3,%s:0.3", fp1Hex, fp2Hex), // sum = 0.6, not 1.0
			expectErrMsg: "FP ratios must sum to 1.0",
		},
		{
			name:         "invalid BTC public key in ratios",
			consumerID:   "test-consumer",
			rewards:      "1000ubbn",
			fpRatios:     "invalid_btc_key:1.0", // invalid BTC public key
			expectErrMsg: "invalid FP ratios",
		},
		{
			name:         "negative ratio",
			consumerID:   "test-consumer",
			rewards:      "1000ubbn",
			fpRatios:     fmt.Sprintf("%s:-0.5", fp1Hex), // negative ratio
			expectErrMsg: "invalid FP ratios",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := buildArgs(tc.consumerID, tc.rewards, tc.fpRatios)
			out, err := testutilcli.ExecTestCLICmd(clientCtx, cmd, args)

			if tc.expectErrMsg != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectErrMsg)
			} else {
				require.NoError(t, err, "test: %s\noutput: %s", tc.name, out.String())
				resp := &sdk.TxResponse{}
				err = clientCtx.Codec.UnmarshalJSON(out.Bytes(), resp)
				require.NoError(t, err, out.String(), "test: %s, output\n:", tc.name, out.String())
			}
		})
	}
}

func TestAddBsnRewardsValidation(t *testing.T) {
	// Generate valid BTC public keys
	r := rand.New(rand.NewSource(time.Now().Unix()))
	_, fpPK1, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	_, fpPK2, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	fp1 := bbn.NewBIP340PubKeyFromBTCPK(fpPK1)
	fp2 := bbn.NewBIP340PubKeyFromBTCPK(fpPK2)

	// Test edge cases for ratio summation using the actual CLI parsing
	testCases := []struct {
		name           string
		ratioStr       string
		expectedSum    string
		shouldEqual1_0 bool
	}{
		{
			name:           "ratios sum exactly to 1.0",
			ratioStr:       fmt.Sprintf("%s:0.6,%s:0.4", fp1.MarshalHex(), fp2.MarshalHex()),
			expectedSum:    "1.0",
			shouldEqual1_0: true,
		},
		{
			name:           "ratios sum to slightly less than 1.0",
			ratioStr:       fmt.Sprintf("%s:0.59,%s:0.40", fp1.MarshalHex(), fp2.MarshalHex()),
			expectedSum:    "0.99",
			shouldEqual1_0: false,
		},
		{
			name:           "ratios sum to slightly more than 1.0",
			ratioStr:       fmt.Sprintf("%s:0.61,%s:0.40", fp1.MarshalHex(), fp2.MarshalHex()),
			expectedSum:    "1.01",
			shouldEqual1_0: false,
		},
		{
			name:           "single ratio equals 1.0",
			ratioStr:       fmt.Sprintf("%s:1.0", fp1.MarshalHex()),
			expectedSum:    "1.0",
			shouldEqual1_0: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse the FP ratios using the CLI function
			fpRatios, err := btcstakingcli.ParseFpRatios(tc.ratioStr)
			require.NoError(t, err)

			// Simulate the ratio sum validation from the CLI command
			ratioSum := sdkmath.LegacyZeroDec()
			for _, fpRatio := range fpRatios {
				ratioSum = ratioSum.Add(fpRatio.Ratio)
			}

			if tc.shouldEqual1_0 {
				require.True(t, ratioSum.Equal(sdkmath.LegacyOneDec()), "Expected ratio sum to equal 1.0, got: %s", ratioSum.String())
			} else {
				require.False(t, ratioSum.Equal(sdkmath.LegacyOneDec()), "Expected ratio sum to not equal 1.0, got: %s", ratioSum.String())
			}
		})
	}
}
