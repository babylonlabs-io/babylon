package e2e2

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"cosmossdk.io/math"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
	"github.com/babylonlabs-io/babylon/v4/test/e2ev2/tmanager"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	sdksigning "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
)

// TestWasmIBCNilHandler demonstrates that the wasm IBC module handler is nil,
// causing channel creation to the wasm port to fail with a nil pointer panic.
//
// Bug: In app/keepers/keepers.go, `wasmStack` is declared as nil on line 651
// and never assigned the actual handler. The nil is registered in the IBC router.
//
// This test deploys an IBC-capable wasm contract (ibc_reflect_send), which binds
// an IBC port, then attempts to open a channel on that port. Before the fix,
// this causes a nil pointer dereference. After the fix, the channel open
// handshake proceeds normally through the wasm handler.
func TestWasmIBCNilHandler(t *testing.T) {
	t.Parallel()
	tm := tmanager.NewTmWithIbc(t)
	tm.Start()

	bbn, _ := tm.ChainNodes()

	// Verify the transfer channel was created successfully (sanity check)
	bbnChannels := bbn.QueryIBCChannels()
	require.Len(t, bbnChannels.Channels, 1, "transfer channel should exist")

	connectionID := bbnChannels.Channels[0].ConnectionHops[0]
	t.Logf("Reusing connection: %s", connectionID)

	// Fund a wallet for deploying the contract and submitting txs
	sender := bbn.CreateWallet("wasm_ibc_test")
	fundCoins := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, math.NewInt(10_000_000)))
	bbn.DefaultWallet().VerifySentTx = true
	bbn.SendCoins(sender.Addr(), fundCoins)
	bbn.DefaultWallet().VerifySentTx = false
	bbn.UpdateWalletAccSeqNumber(sender.KeyName)

	// Load the IBC-capable wasm contract (ibc_reflect_send from wasmd testdata)
	wasmBytecode := loadWasmBytecode(t, "bytecode/ibc_reflect_send.wasm")

	// Store the wasm contract code (needs high gas for large bytecode)
	storeMsg := &wasmtypes.MsgStoreCode{
		Sender:       sender.Addr(),
		WASMByteCode: wasmBytecode,
	}
	storeTx := signMsgWithGas(t, sender, 10_000_000, storeMsg)
	storeTxHash, err := bbn.SubmitTx(storeTx)
	require.NoError(t, err, "store code broadcast should succeed")
	bbn.WaitForNextBlock()
	bbn.RequireTxSuccess(storeTxHash)
	sender.VerifySentTx = true

	// Query the code ID
	codeID := queryLatestCodeID(t, bbn)
	t.Logf("Stored wasm code with ID: %d", codeID)
	require.Positive(t, codeID)

	// Instantiate the contract (ibc_reflect_send takes empty init msg)
	instantiateMsg := &wasmtypes.MsgInstantiateContract{
		Sender: sender.Addr(),
		Admin:  sender.Addr(),
		CodeID: codeID,
		Label:  "ibc-reflect-send",
		Msg:    []byte(`{}`),
	}
	sender.SubmitMsgs(instantiateMsg)

	// Query the contract address and IBC port
	contractAddr := queryContractByCodeID(t, bbn, codeID)
	t.Logf("Instantiated contract at: %s", contractAddr)
	require.NotEmpty(t, contractAddr)

	contractInfo := queryContractInfo(t, bbn, contractAddr)
	ibcPortID := contractInfo.IBCPortID
	t.Logf("Contract IBC port: %s", ibcPortID)
	require.NotEmpty(t, ibcPortID, "IBC-capable contract should have a port ID")

	// Now attempt to open an IBC channel on the contract's port.
	// Before the fix: nil pointer dereference panic (wasmStack is nil)
	// After the fix: the wasm handler processes the channel open init
	msg := channeltypes.NewMsgChannelOpenInit(
		ibcPortID,
		"ibc-reflect-v1",
		channeltypes.ORDERED,
		[]string{connectionID},
		ibcPortID,
		sender.Addr(),
	)

	signedTx := sender.SignMsg(msg)
	txHash, err := bbn.SubmitTx(signedTx)

	if err != nil {
		// If the tx fails at CheckTx, check whether it's the nil pointer
		// panic (bug present) or a legitimate wasm handler error (fix applied)
		t.Logf("MsgChannelOpenInit failed at broadcast: %v", err)
		require.NotContains(t, err.Error(), "nil pointer dereference",
			"should NOT get nil pointer dereference — wasmStack must be properly assigned")
	} else {
		// Tx passed broadcast — check DeliverTx result
		t.Logf("Tx broadcast succeeded (hash: %s), checking DeliverTx result...", txHash)
		bbn.WaitForNextBlock()
		txResp := bbn.QueryTxByHash(txHash)

		// The channel open may succeed (code 0) or fail with a wasm-level
		// error (e.g., contract rejects the version). Either is fine — the
		// key assertion is that there's no nil pointer panic.
		t.Logf("DeliverTx result: code=%d log=%s", txResp.TxResponse.Code, txResp.TxResponse.RawLog)
		require.NotContains(t, txResp.TxResponse.RawLog, "nil pointer dereference",
			"should NOT get nil pointer dereference — wasmStack must be properly assigned")

		if txResp.TxResponse.Code == 0 {
			t.Log("Channel open init succeeded — wasm IBC handler is working correctly")
			// Verify the channel was created
			bbnChannelsAfter := bbn.QueryIBCChannels()
			require.Greater(t, len(bbnChannelsAfter.Channels), 1,
				"a new wasm IBC channel should have been created")
			t.Logf("New channel created: %s on port %s",
				bbnChannelsAfter.Channels[len(bbnChannelsAfter.Channels)-1].ChannelId,
				bbnChannelsAfter.Channels[len(bbnChannelsAfter.Channels)-1].PortId)
		} else {
			t.Logf("Channel open init returned error (code %d) — but it's a wasm handler error, not a nil panic",
				txResp.TxResponse.Code)
		}
	}

	// Verify chain is still running
	bbn.WaitForNextBlock()
	t.Log("Chain is still producing blocks")
}

func loadWasmBytecode(t *testing.T, relativePath string) []byte {
	t.Helper()
	absPath, err := filepath.Abs(relativePath)
	require.NoError(t, err)
	bz, err := os.ReadFile(absPath)
	require.NoError(t, err, "failed to read wasm file at %s", absPath)
	return bz
}

func queryLatestCodeID(t *testing.T, n *tmanager.Node) uint64 {
	t.Helper()
	var codeID uint64
	n.GrpcConn(func(conn *grpc.ClientConn) {
		client := wasmtypes.NewQueryClient(conn)
		resp, err := client.Codes(context.Background(), &wasmtypes.QueryCodesRequest{})
		require.NoError(t, err)
		require.NotEmpty(t, resp.CodeInfos, "no wasm codes found")
		codeID = resp.CodeInfos[len(resp.CodeInfos)-1].CodeID
	})
	return codeID
}

func queryContractByCodeID(t *testing.T, n *tmanager.Node, codeID uint64) string {
	t.Helper()
	var addr string
	n.GrpcConn(func(conn *grpc.ClientConn) {
		client := wasmtypes.NewQueryClient(conn)
		resp, err := client.ContractsByCode(context.Background(), &wasmtypes.QueryContractsByCodeRequest{
			CodeId: codeID,
		})
		require.NoError(t, err)
		require.NotEmpty(t, resp.Contracts, "no contracts found for code ID %d", codeID)
		addr = resp.Contracts[len(resp.Contracts)-1]
	})
	return addr
}

func queryContractInfo(t *testing.T, n *tmanager.Node, contractAddr string) *wasmtypes.ContractInfo {
	t.Helper()
	var info *wasmtypes.ContractInfo
	n.GrpcConn(func(conn *grpc.ClientConn) {
		client := wasmtypes.NewQueryClient(conn)
		resp, err := client.ContractInfo(context.Background(), &wasmtypes.QueryContractInfoRequest{
			Address: contractAddr,
		})
		require.NoError(t, err)
		ci := resp.ContractInfo
		info = &ci
	})
	return info
}

// signMsgWithGas signs a tx with a custom gas limit
func signMsgWithGas(t *testing.T, ws *tmanager.WalletSender, gasLimit uint64, msgs ...sdk.Msg) *sdktx.Tx {
	t.Helper()
	txBuilder := util.EncodingConfig.TxConfig.NewTxBuilder()
	err := txBuilder.SetMsgs(msgs...)
	require.NoError(t, err)

	fee := math.NewIntFromUint64(gasLimit / 10) // 0.1ubbn per gas
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, fee)))
	txBuilder.SetGasLimit(gasLimit)

	pubKey := ws.PrivKey.PubKey()
	signerData := authsigning.SignerData{
		ChainID:       ws.ChainID(),
		AccountNumber: ws.AccountNumber,
		Sequence:      ws.SequenceNumber,
		Address:       ws.Address.String(),
		PubKey:        pubKey,
	}

	sig := sdksigning.SignatureV2{
		PubKey: pubKey,
		Data: &sdksigning.SingleSignatureData{
			SignMode:  sdksigning.SignMode_SIGN_MODE_DIRECT,
			Signature: nil,
		},
		Sequence: ws.SequenceNumber,
	}
	err = txBuilder.SetSignatures(sig)
	require.NoError(t, err)

	bytesToSign, err := authsigning.GetSignBytesAdapter(
		sdk.Context{},
		util.EncodingConfig.TxConfig.SignModeHandler(),
		sdksigning.SignMode_SIGN_MODE_DIRECT,
		signerData,
		txBuilder.GetTx(),
	)
	require.NoError(t, err)

	sigBytes, err := ws.PrivKey.Sign(bytesToSign)
	require.NoError(t, err)

	sig = sdksigning.SignatureV2{
		PubKey: pubKey,
		Data: &sdksigning.SingleSignatureData{
			SignMode:  sdksigning.SignMode_SIGN_MODE_DIRECT,
			Signature: sigBytes,
		},
		Sequence: ws.SequenceNumber,
	}
	err = txBuilder.SetSignatures(sig)
	require.NoError(t, err)

	ws.IncSeq()

	bz, err := util.EncodingConfig.TxConfig.TxEncoder()(txBuilder.GetTx())
	require.NoError(t, err)
	txDecoded, err := tmanager.DecodeTx(bz)
	require.NoError(t, err)
	return txDecoded
}

// TestWasmIBCNilHandlerRawPort is a simpler variant that tests the raw "wasm"
// port without deploying a contract. This verifies the handler is not nil.
func TestWasmIBCNilHandlerRawPort(t *testing.T) {
	t.Parallel()
	tm := tmanager.NewTmWithIbc(t)
	tm.Start()

	bbn, _ := tm.ChainNodes()

	bbnChannels := bbn.QueryIBCChannels()
	require.Len(t, bbnChannels.Channels, 1)
	connectionID := bbnChannels.Channels[0].ConnectionHops[0]

	sender := bbn.CreateWallet("wasm_raw_test")
	fundCoins := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, math.NewInt(1_000_000)))
	bbn.DefaultWallet().VerifySentTx = true
	bbn.SendCoins(sender.Addr(), fundCoins)
	bbn.DefaultWallet().VerifySentTx = false
	bbn.UpdateWalletAccSeqNumber(sender.KeyName)

	// Submit MsgChannelOpenInit targeting bare "wasm" port (no contract)
	msg := channeltypes.NewMsgChannelOpenInit(
		"wasm",
		"wasm-ibc-1",
		channeltypes.UNORDERED,
		[]string{connectionID},
		"wasm",
		sender.Addr(),
	)

	signedTx := sender.SignMsg(msg)
	txHash, err := bbn.SubmitTx(signedTx)

	if err != nil {
		t.Logf("MsgChannelOpenInit to raw wasm port failed at broadcast: %v", err)
		require.NotContains(t, err.Error(), "nil pointer dereference",
			"should NOT get nil pointer dereference after fix")
	} else {
		bbn.WaitForNextBlock()
		txResp := bbn.QueryTxByHash(txHash)
		t.Logf("DeliverTx result: code=%d log=%s", txResp.TxResponse.Code, txResp.TxResponse.RawLog)
		require.NotContains(t, txResp.TxResponse.RawLog, "nil pointer dereference",
			"should NOT get nil pointer dereference after fix")
	}

	bbn.WaitForNextBlock()
	t.Log("Chain is still producing blocks")
}
