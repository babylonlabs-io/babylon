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
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v3/app"
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

func SetupClientContext(t *testing.T) (client.Context, []sdk.AccAddress) {
	encCfg := app.GetEncodingConfig()
	kr := keyring.NewInMemory(encCfg.Codec)

	baseCtx := client.Context{}.
		WithKeyring(kr).
		WithTxConfig(encCfg.TxConfig).
		WithCodec(encCfg.Codec).
		WithClient(mockCometRPC{Client: rpcclientmock.Client{}}).
		WithAccountRetriever(client.MockAccountRetriever{}).
		WithOutput(io.Discard).
		WithChainID("test-chain")

	bz, _ := encCfg.Codec.Marshal(&sdk.TxResponse{})
	c := newMockCometRPC(abci.ResponseQuery{Value: bz})
	clientCtx := baseCtx.WithClient(c)

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

// ExecTestCLICmd builds the client context, mocks the output and executes the command.
func ExecTestCLICmd(clientCtx client.Context, cmd *cobra.Command, extraArgs []string) (testutil.BufferWriter, error) {
	cmd.SetArgs(extraArgs)

	_, out := testutil.ApplyMockIO(cmd)
	clientCtx = clientCtx.WithOutput(out)

	ctx := context.Background()
	ctx = context.WithValue(ctx, client.ClientContextKey, &clientCtx)

	if err := cmd.ExecuteContext(ctx); err != nil {
		return out, err
	}

	return out, nil
}
