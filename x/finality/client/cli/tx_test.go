package cli

import (
	"context"
	"io"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtbytes "github.com/cometbft/cometbft/libs/bytes"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpcclientmock "github.com/cometbft/cometbft/rpc/client/mock"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/stretchr/testify/require"

	finalitytypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
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

func setupClientCtx(t *testing.T) client.Context {
	t.Helper()
	interfaceRegistry := types.NewInterfaceRegistry()
	finalitytypes.RegisterInterfaces(interfaceRegistry)
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	txConfig := tx.NewTxConfig(marshaler, tx.DefaultSignModes)

	kr := keyring.NewInMemory(marshaler)

	bz, _ := marshaler.Marshal(&sdk.TxResponse{})
	c := newMockCometRPC(abci.ResponseQuery{Value: bz})

	k, _, err := kr.NewMnemonic("testkey", keyring.English,
		sdk.FullFundraiserPath, keyring.DefaultBIP39Passphrase, hd.Secp256k1)
	require.NoError(t, err)

	pub, err := k.GetPubKey()
	require.NoError(t, err)

	clientCtx := client.Context{}.
		WithKeyring(kr).
		WithTxConfig(txConfig).
		WithCodec(marshaler).
		WithClient(c).
		WithAccountRetriever(client.MockAccountRetriever{}).
		WithOutput(io.Discard).
		WithChainID("test-chain").
		WithFromName("testkey").
		WithFromAddress(sdk.AccAddress(pub.Address()))

	return clientCtx
}

func TestAddEvidenceOfEquivocationCmd(t *testing.T) {
	clientCtx := setupClientCtx(t)
	cmd := AddEvidenceOfEquivocationCmd()

	cmd.SetContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx))

	args := []string{
		"1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", // fp_btc_pk_hex
		"12345", // block_height
		"1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", // pub_rand_hex
		"1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", // canonical_app_hash_hex
		"fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321", // fork_app_hash_hex
		"1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", // canonical_finality_sig_hex
		"fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321", // fork_finality_sig_hex
		"test-context",
		"--generate-only",
	}

	cmd.SetArgs(args)
	err := cmd.Execute()

	require.NoError(t, err)
}
