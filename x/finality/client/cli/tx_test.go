package cli

import (
	"context"
	"encoding/hex"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"io"
	"math/rand"
	"strconv"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtbytes "github.com/cometbft/cometbft/libs/bytes"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpcclientmock "github.com/cometbft/cometbft/rpc/client/mock"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v3/app/params"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
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

	params.SetAddressPrefixes()

	interfaceRegistry := types.NewInterfaceRegistry()
	finalitytypes.RegisterInterfaces(interfaceRegistry)
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	txConfig := tx.NewTxConfig(marshaler, tx.DefaultSignModes)

	kr := keyring.NewInMemory(marshaler)

	_, _, err := kr.NewMnemonic("testkey", keyring.English,
		sdk.FullFundraiserPath, keyring.DefaultBIP39Passphrase, hd.Secp256k1)
	require.NoError(t, err)

	bz, _ := marshaler.Marshal(&sdk.TxResponse{})
	c := newMockCometRPC(abci.ResponseQuery{Value: bz})

	clientCtx := client.Context{}.
		WithKeyring(kr).
		WithTxConfig(txConfig).
		WithCodec(marshaler).
		WithClient(c).
		WithAccountRetriever(client.MockAccountRetriever{}).
		WithOutput(io.Discard).
		WithChainID("test-chain")

	return clientCtx
}

func TestAddEvidenceOfEquivocationCmd(t *testing.T) {
	clientCtx := setupClientCtx(t)
	cmd := AddEvidenceOfEquivocationCmd()

	cmd.SetContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx))

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	sk, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	blockHeight := uint64(12345)
	evidence, err := datagen.GenRandomEvidence(r, sk, blockHeight)
	require.NoError(t, err)

	args := []string{
		hex.EncodeToString(evidence.FpBtcPk.MustMarshal()),
		strconv.FormatUint(evidence.BlockHeight, 10),
		hex.EncodeToString(evidence.PubRand.MustMarshal()),
		hex.EncodeToString(evidence.CanonicalAppHash),
		hex.EncodeToString(evidence.ForkAppHash),
		hex.EncodeToString(evidence.CanonicalFinalitySig.MustMarshal()),
		hex.EncodeToString(evidence.ForkFinalitySig.MustMarshal()),
		"test-context",
		"--signer", "bbn1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqya3wcy",
		"--generate-only",
	}

	cmd.SetArgs(args)
	err = cmd.Execute()

	require.NoError(t, err)
}
