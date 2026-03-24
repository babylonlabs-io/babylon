package e2e2

import (
	"testing"

	"cosmossdk.io/math"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/test/e2ev2/tmanager"
	sdk "github.com/cosmos/cosmos-sdk/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	"github.com/stretchr/testify/require"
)

const ibcReflectSendWasmPath = "bytecode/ibc_reflect_send.wasm"

// TestWasmIBCHandler verifies that the wasm IBC module handler is properly
// wired in the IBC router.
//
// This test covers two cases:
//   - Raw port: MsgChannelOpenInit on the bare "wasm" port (no contract)
//   - Contract port: deploy an IBC contract, then open a channel on its port
func TestWasmIBCHandler(t *testing.T) {
	t.Parallel()
	tm := tmanager.NewTmWithIbc(t)
	tm.Start()

	bbn, _ := tm.ChainNodes()

	bbnChannels := bbn.QueryIBCChannels()
	require.Len(t, bbnChannels.Channels, 1, "transfer channel should exist")
	connectionID := bbnChannels.Channels[0].ConnectionHops[0]
	t.Logf("Reusing connection: %s", connectionID)

	t.Run("raw_wasm_port", func(t *testing.T) {
		sender := bbn.CreateWallet("wasm_raw_test")
		fundCoins := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, math.NewInt(1_000_000)))
		bbn.DefaultWallet().VerifySentTx = true
		bbn.SendCoins(sender.Addr(), fundCoins)
		bbn.DefaultWallet().VerifySentTx = false
		bbn.UpdateWalletAccSeqNumber(sender.KeyName)

		msg := channeltypes.NewMsgChannelOpenInit(
			"wasm",
			"wasm-ibc-1",
			channeltypes.UNORDERED,
			[]string{connectionID},
			"wasm",
			sender.Addr(),
		)

		// Channel open on bare "wasm" port should broadcast successfully
		// but fail in DeliverTx because no contract is bound to that port
		signedTx := sender.SignMsg(msg)
		txHash, err := bbn.SubmitTx(signedTx)
		require.NoError(t, err)

		bbn.WaitForNextBlock()
		txResp := bbn.QueryTxByHash(txHash)
		require.NotZero(t, txResp.TxResponse.Code, "channel open on bare wasm port should fail")
		require.Equal(t, txResp.TxResponse.RawLog, "failed to execute message; message index: 0: channel open init callback failed for port ID: wasm, channel ID: channel-1: contract port id: without prefix: invalid")
		t.Logf("Raw wasm port correctly rejected: %s", txResp.TxResponse.RawLog)
	})

	t.Run("contract_ibc_port", func(t *testing.T) {
		sender := bbn.CreateWallet("wasm_ibc_test")
		fundCoins := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, math.NewInt(10_000_000)))
		bbn.DefaultWallet().VerifySentTx = true
		bbn.SendCoins(sender.Addr(), fundCoins)
		bbn.DefaultWallet().VerifySentTx = false
		bbn.UpdateWalletAccSeqNumber(sender.KeyName)

		// Store the IBC-capable wasm contract
		wasmBytecode := tmanager.LoadWasmBytecode(t, ibcReflectSendWasmPath)
		storeMsg := &wasmtypes.MsgStoreCode{
			Sender:       sender.Addr(),
			WASMByteCode: wasmBytecode,
		}
		storeTx := sender.SignMsgWithGas(10_000_000, storeMsg)
		storeTxHash, err := bbn.SubmitTx(storeTx)
		require.NoError(t, err)
		bbn.WaitForNextBlock()
		bbn.RequireTxSuccess(storeTxHash)

		codeID := bbn.QueryWasmLatestCodeID()
		t.Logf("Stored wasm code with ID: %d", codeID)

		// Instantiate (ibc_reflect_send takes empty init msg)
		sender.VerifySentTx = true
		instantiateMsg := &wasmtypes.MsgInstantiateContract{
			Sender: sender.Addr(),
			Admin:  sender.Addr(),
			CodeID: codeID,
			Label:  "ibc-reflect-send",
			Msg:    []byte(`{}`),
		}
		sender.SubmitMsgs(instantiateMsg)

		contractAddr := bbn.QueryWasmContractByCodeID(codeID)
		t.Logf("Instantiated contract at: %s", contractAddr)

		contractInfo := bbn.QueryWasmContractInfo(contractAddr)
		ibcPortID := contractInfo.IBCPortID
		t.Logf("Contract IBC port: %s", ibcPortID)
		require.NotEmpty(t, ibcPortID, "IBC-capable contract should have a port ID")

		// Open an IBC channel on the contract's port — should succeed
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
		require.NoError(t, err)

		bbn.WaitForNextBlock()
		txResp := bbn.QueryTxByHash(txHash)
		require.Zero(t, txResp.TxResponse.Code, "channel open on contract port should succeed: %s", txResp.TxResponse.RawLog)

		bbnChannelsAfter := bbn.QueryIBCChannels()
		require.Greater(t, len(bbnChannelsAfter.Channels), 1, "a new wasm IBC channel should have been created")
		lastCh := bbnChannelsAfter.Channels[len(bbnChannelsAfter.Channels)-1]
		t.Logf("New channel created: %s on port %s", lastCh.ChannelId, lastCh.PortId)
	})
}
