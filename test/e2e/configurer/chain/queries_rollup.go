package chain

import (
	"encoding/json"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/stretchr/testify/require"

	"github.com/btcsuite/btcd/btcec/v2"

	bbn "github.com/babylonlabs-io/babylon/v4/types"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/rollup"
)

// QueryLastPublicRandCommitRollup returns the last public randomness
// commitments from a rollup's finality contract.
func (n *NodeConfig) QueryLastPublicRandCommitRollup(finalityContractAddr string, fpPk *btcec.PublicKey) *rollup.PubRandCommitResponse {
	fpPubKey := bbn.NewBIP340PubKeyFromBTCPK(fpPk)
	queryMsg := &rollup.QueryMsg{
		LastPubRandCommit: &rollup.PubRandCommit{
			BtcPkHex: fpPubKey.MarshalHex(),
		},
	}

	msg, err := json.Marshal(queryMsg)
	require.NoError(n.t, err)

	var queryResult *rollup.PubRandCommitResponse
	var smartContractResponse *wasmtypes.QuerySmartContractStateResponse
	smartContractResponse, err = n.QueryWasmSmart(finalityContractAddr, string(msg))
	require.NoError(n.t, err)

	require.NotNil(n.t, smartContractResponse)
	require.NotNil(n.t, smartContractResponse.Data)
	err = json.Unmarshal(smartContractResponse.Data, &queryResult)
	require.NoError(n.t, err)

	return queryResult
}
