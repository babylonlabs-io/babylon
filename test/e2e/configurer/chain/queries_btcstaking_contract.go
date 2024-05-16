package chain

import (
	"encoding/json"
	"fmt"
	"time"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/stretchr/testify/require"
)

type ConsumerFpResponse struct {
	ConsumerFps []ConsumerFp `json:"fps"`
}

// ConsumerFp represents the finality provider data returned by the contract query.
// For more details, refer to the following links:
// https://github.com/babylonchain/babylon-contract/blob/v0.5.3/packages/apis/src/btc_staking_api.rs
// https://github.com/babylonchain/babylon-contract/blob/v0.5.3/contracts/btc-staking/src/msg.rs
// https://github.com/babylonchain/babylon-contract/blob/v0.5.3/contracts/btc-staking/schema/btc-staking.json
type ConsumerFp struct {
	BtcPkHex             string `json:"btc_pk_hex"`
	SlashedBabylonHeight uint64 `json:"slashed_babylon_height"`
	SlashedBtcHeight     uint64 `json:"slashed_btc_height"`
	ChainID              string `json:"chain_id"`
}

type ConsumerDelegationResponse struct {
	ConsumerDelegations []ConsumerDelegation `json:"delegations"`
}

type ConsumerDelegation struct {
	BtcPkHex    string   `json:"btc_pk_hex"`
	FpBtcPkList []string `json:"fp_btc_pk_list"`
	StartHeight uint64   `json:"start_height"`
	EndHeight   uint64   `json:"end_height"`
	TotalSat    uint64   `json:"total_sat"`
	StakingTx   []byte   `json:"staking_tx"`
	SlashingTx  []byte   `json:"slashing_tx"`
}

type ConsumerDelegationsByFpResponse struct {
	StakingTxHashes []string `json:"hashes"`
}

// QueryConsumerFps queries all finality providers stored in the staking contract
func (n *NodeConfig) QueryConsumerFps(stakingContractAddr string) (*ConsumerFpResponse, error) {
	queryMsg := `{"finality_providers":{}}`
	var (
		smartContractResponse *wasmtypes.QuerySmartContractStateResponse
		result                ConsumerFpResponse
		err                   error
	)
	require.Eventually(n.t, func() bool {
		smartContractResponse, err = n.QueryWasmSmart(stakingContractAddr, queryMsg)
		if err != nil || smartContractResponse == nil || smartContractResponse.Data == nil {
			return false
		}

		err = json.Unmarshal(smartContractResponse.Data, &result)
		return err == nil
	}, time.Second*20, time.Second)

	return &result, err
}

// QueryConsumerDelegations queries all BTC delegations stored in the staking contract
func (n *NodeConfig) QueryConsumerDelegations(stakingContractAddr string) (*ConsumerDelegationResponse, error) {
	queryMsg := `{"delegations":{}}`
	var (
		smartContractResponse *wasmtypes.QuerySmartContractStateResponse
		result                ConsumerDelegationResponse
		err                   error
	)
	require.Eventually(n.t, func() bool {
		smartContractResponse, err = n.QueryWasmSmart(stakingContractAddr, queryMsg)
		if err != nil || smartContractResponse == nil || smartContractResponse.Data == nil {
			return false
		}

		err = json.Unmarshal(smartContractResponse.Data, &result)
		return err == nil
	}, time.Second*20, time.Second)

	return &result, err
}

// QueryConsumerDelegationsByFp queries delegations by a specific finality provider
func (n *NodeConfig) QueryConsumerDelegationsByFp(stakingContractAddr string, fpBtcPkHex string) (*ConsumerDelegationsByFpResponse, error) {
	queryMsg := fmt.Sprintf(`{"delegations_by_f_p":{"btc_pk_hex":"%s"}}`, fpBtcPkHex)
	var (
		smartContractResponse *wasmtypes.QuerySmartContractStateResponse
		result                ConsumerDelegationsByFpResponse
		err                   error
	)
	require.Eventually(n.t, func() bool {
		smartContractResponse, err = n.QueryWasmSmart(stakingContractAddr, queryMsg)
		if err != nil || smartContractResponse == nil || smartContractResponse.Data == nil {
			return false
		}

		err = json.Unmarshal(smartContractResponse.Data, &result)
		return err == nil
	}, time.Second*20, time.Second)

	return &result, err
}

// QuerySingleConsumerFp queries a specific finality provider by Bitcoin public key hex
func (n *NodeConfig) QuerySingleConsumerFp(stakingContractAddr string, btcPkHex string) (*ConsumerFp, error) {
	queryMsg := fmt.Sprintf(`{"finality_provider":{"btc_pk_hex":"%s"}}`, btcPkHex)
	var (
		smartContractResponse *wasmtypes.QuerySmartContractStateResponse
		result                ConsumerFp
		err                   error
	)
	require.Eventually(n.t, func() bool {
		smartContractResponse, err = n.QueryWasmSmart(stakingContractAddr, queryMsg)
		if err != nil || smartContractResponse == nil || smartContractResponse.Data == nil {
			return false
		}

		err = json.Unmarshal(smartContractResponse.Data, &result)
		return err == nil
	}, time.Second*20, time.Second)

	return &result, err
}

// QuerySingleConsumerDelegation queries a specific BTC delegation by staking tx hash hex
func (n *NodeConfig) QuerySingleConsumerDelegation(stakingContractAddr string, stakingTxHashHex string) (*ConsumerDelegation, error) {
	queryMsg := fmt.Sprintf(`{"delegation":{"staking_tx_hash_hex":"%s"}}`, stakingTxHashHex)
	var (
		smartContractResponse *wasmtypes.QuerySmartContractStateResponse
		result                ConsumerDelegation
		err                   error
	)
	require.Eventually(n.t, func() bool {
		smartContractResponse, err = n.QueryWasmSmart(stakingContractAddr, queryMsg)
		if err != nil || smartContractResponse == nil || smartContractResponse.Data == nil {
			return false
		}

		err = json.Unmarshal(smartContractResponse.Data, &result)
		return err == nil
	}, time.Second*20, time.Second)

	return &result, err
}
