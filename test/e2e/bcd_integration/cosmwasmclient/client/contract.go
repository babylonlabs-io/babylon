package client

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"strings"

	wasmdtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdkquery "github.com/cosmos/cosmos-sdk/types/query"
)

func (cwClient *Client) StoreWasmCode(wasmFile string) error {
	wasmCode, err := os.ReadFile(wasmFile)
	if err != nil {
		return err
	}
	if strings.HasSuffix(wasmFile, "wasm") { // compress for gas limit
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, err = gz.Write(wasmCode)
		if err != nil {
			return err
		}
		err = gz.Close()
		if err != nil {
			return err
		}
		wasmCode = buf.Bytes()
	}

	storeMsg := &wasmdtypes.MsgStoreCode{
		Sender:       cwClient.MustGetAddr(),
		WASMByteCode: wasmCode,
	}
	_, err = cwClient.ReliablySendMsg(context.Background(), storeMsg, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

func (cwClient *Client) InstantiateContract(codeID uint64, initMsg []byte) error {
	instantiateMsg := &wasmdtypes.MsgInstantiateContract{
		Sender: cwClient.MustGetAddr(),
		Admin:  cwClient.MustGetAddr(),
		CodeID: codeID,
		Label:  "cw",
		Msg:    initMsg,
		Funds:  nil,
	}

	_, err := cwClient.ReliablySendMsg(context.Background(), instantiateMsg, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

// returns the latest wasm code id.
func (cwClient *Client) GetLatestCodeId() (uint64, error) {
	pagination := &sdkquery.PageRequest{
		Limit:   1,
		Reverse: true,
	}
	resp, err := cwClient.ListCodes(pagination)
	if err != nil {
		return 0, err
	}

	if len(resp.CodeInfos) == 0 {
		return 0, fmt.Errorf("no codes found")
	}

	return resp.CodeInfos[0].CodeID, nil
}
