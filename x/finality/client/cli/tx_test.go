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

	finalitytypes "github.com/babylonlabs-io/babylon/v3/x/finality/types"
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
		// fp_btc_pk_hex
		"f1618a00dd6a56e4ebe20c68e0b6b0a8c57c3c2c5b1f8b3a4d2e9f8c7b6a5d4e3f2",
		"12345", // block_height
		// pub_rand_hex
		"a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2",
		// canonical_app_hash_hex
		"1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		// fork_app_hash_hex
		"fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321",
		// canonical_finality_sig_hex
		"a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2",
		// fork_finality_sig_hex
		"d0c9b8a7f6e5d4c3b2a1f0e9d8c7b6a5c4b3a2f1e0d9c8b7a6c5b4a3f2e1d0c9",
		"test-context",
		"--generate-only",
	}

	cmd.SetArgs(args)
	err := cmd.Execute()

	require.NoError(t, err)

}
