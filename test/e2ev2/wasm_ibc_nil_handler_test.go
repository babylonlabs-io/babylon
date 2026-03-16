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
// Bug context: In app/keepers/keepers.go, `wasmStack` was declared nil and
// registered in the IBC router without being assigned the actual handler.
// Any IBC channel open targeting the wasm port caused a nil pointer panic.
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

		// MsgChannelOpenInit on the bare "wasm" port (no contract bound)
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
				"should NOT get nil pointer dereference — wasmStack must be properly assigned")
		} else {
			bbn.WaitForNextBlock()
			txResp := bbn.QueryTxByHash(txHash)
			t.Logf("DeliverTx result: code=%d log=%s", txResp.TxResponse.Code, txResp.TxResponse.RawLog)
			require.NotContains(t, txResp.TxResponse.RawLog, "nil pointer dereference",
				"should NOT get nil pointer dereference — wasmStack must be properly assigned")
		}

		bbn.WaitForNextBlock()
		t.Log("Chain is still producing blocks after raw wasm port channel open attempt")
	})

	t.Run("contract_ibc_port", func(t *testing.T) {
		sender := bbn.CreateWallet("wasm_ibc_test")
		fundCoins := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, math.NewInt(10_000_000)))
		bbn.DefaultWallet().VerifySentTx = true
		bbn.SendCoins(sender.Addr(), fundCoins)
		bbn.DefaultWallet().VerifySentTx = false
		bbn.UpdateWalletAccSeqNumber(sender.KeyName)

		// Store the IBC-capable wasm contract (needs high gas for large bytecode)
		wasmBytecode := tmanager.LoadWasmBytecode(t, ibcReflectSendWasmPath)
		storeMsg := &wasmtypes.MsgStoreCode{
			Sender:       sender.Addr(),
			WASMByteCode: wasmBytecode,
		}
		storeTx := sender.SignMsgWithGas(10_000_000, storeMsg)
		storeTxHash, err := bbn.SubmitTx(storeTx)
		require.NoError(t, err, "store code broadcast should succeed")
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

		// Query the contract's IBC port
		contractAddr := bbn.QueryWasmContractByCodeID(codeID)
		t.Logf("Instantiated contract at: %s", contractAddr)

		contractInfo := bbn.QueryWasmContractInfo(contractAddr)
		ibcPortID := contractInfo.IBCPortID
		t.Logf("Contract IBC port: %s", ibcPortID)
		require.NotEmpty(t, ibcPortID, "IBC-capable contract should have a port ID")

		// Open an IBC channel on the contract's port
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
			t.Logf("MsgChannelOpenInit failed at broadcast: %v", err)
			require.NotContains(t, err.Error(), "nil pointer dereference",
				"should NOT get nil pointer dereference — wasmStack must be properly assigned")
		} else {
			t.Logf("Tx broadcast succeeded (hash: %s), checking DeliverTx result...", txHash)
			bbn.WaitForNextBlock()
			txResp := bbn.QueryTxByHash(txHash)

			t.Logf("DeliverTx result: code=%d log=%s", txResp.TxResponse.Code, txResp.TxResponse.RawLog)
			require.NotContains(t, txResp.TxResponse.RawLog, "nil pointer dereference",
				"should NOT get nil pointer dereference — wasmStack must be properly assigned")

			if txResp.TxResponse.Code == 0 {
				t.Log("Channel open init succeeded — wasm IBC handler is working correctly")
				bbnChannelsAfter := bbn.QueryIBCChannels()
				require.Greater(t, len(bbnChannelsAfter.Channels), 1,
					"a new wasm IBC channel should have been created")
				lastCh := bbnChannelsAfter.Channels[len(bbnChannelsAfter.Channels)-1]
				t.Logf("New channel created: %s on port %s", lastCh.ChannelId, lastCh.PortId)
			} else {
				t.Logf("Channel open returned wasm handler error (code %d) — not a nil panic", txResp.TxResponse.Code)
			}
		}

		bbn.WaitForNextBlock()
		t.Log("Chain is still producing blocks after contract IBC channel open")
	})
}

