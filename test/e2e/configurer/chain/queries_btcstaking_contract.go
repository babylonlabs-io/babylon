package chain

import (
	"encoding/json"
	"fmt"
	"time"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/stretchr/testify/require"
)

type ConsumerFpsResponse struct {
	ConsumerFps []ConsumerFpResponse `json:"fps"`
}

// ConsumerFpResponse represents the finality provider data returned by the contract query.
// For more details, refer to the following links:
// https://github.com/babylonchain/babylon-contract/blob/v0.7.0/contracts/btc-staking/src/msg.rs
// https://github.com/babylonchain/babylon-contract/blob/v0.7.0/contracts/btc-staking/schema/btc-staking.json
type ConsumerFpResponse struct {
	BtcPkHex         string `json:"btc_pk_hex"`
	SlashedHeight    uint64 `json:"slashed_height"`
	SlashedBtcHeight uint64 `json:"slashed_btc_height"`
	ConsumerID       string `json:"consumer_id"`
}

type ConsumerDelegationsResponse struct {
	ConsumerDelegations []SingleConsumerDelegationResponse `json:"delegations"`
}

type SingleConsumerDelegationResponse struct {
	BtcPkHex             string                      `json:"btc_pk_hex"`
	FpBtcPkList          []string                    `json:"fp_btc_pk_list"`
	StartHeight          uint64                      `json:"start_height"`
	EndHeight            uint64                      `json:"end_height"`
	TotalSat             uint64                      `json:"total_sat"`
	StakingTx            []byte                      `json:"staking_tx"`
	SlashingTx           []byte                      `json:"slashing_tx"`
	DelegatorSlashingSig []byte                      `json:"delegator_slashing_sig"`
	CovenantSigs         []CovenantAdaptorSignatures `json:"covenant_sigs"`
	StakingOutputIdx     uint32                      `json:"staking_output_idx"`
	UnbondingTime        uint32                      `json:"unbonding_time"`
	UndelegationInfo     *BtcUndelegationInfo        `json:"undelegation_info"`
	ParamsVersion        uint32                      `json:"params_version"`
}

type BtcUndelegationInfo struct {
	UnbondingTx              []byte                      `json:"unbonding_tx"`
	DelegatorUnbondingSig    []byte                      `json:"delegator_unbonding_sig"`
	CovenantUnbondingSigList []SignatureInfo             `json:"covenant_unbonding_sig_list"`
	SlashingTx               []byte                      `json:"slashing_tx"`
	DelegatorSlashingSig     []byte                      `json:"delegator_slashing_sig"`
	CovenantSlashingSigs     []CovenantAdaptorSignatures `json:"covenant_slashing_sigs"`
}

type SignatureInfo struct {
	Pk  []byte `json:"pk"`
	Sig []byte `json:"sig"`
}

type CovenantAdaptorSignatures struct {
	CovPk       []byte   `json:"cov_pk"`
	AdaptorSigs [][]byte `json:"adaptor_sigs"`
}

type ConsumerDelegationsByFpResponse struct {
	StakingTxHashes []string `json:"hashes"`
}

type ConsumerFpInfo struct {
	BtcPkHex string `json:"btc_pk_hex"`
	Power    uint64 `json:"power"`
}

type ConsumerFpsByPowerResponse struct {
	Fps []ConsumerFpInfo `json:"fps"`
}

// QueryConsumerFps queries all finality providers stored in the staking contract
func (n *NodeConfig) QueryConsumerFps(stakingContractAddr string) (*ConsumerFpsResponse, error) {
	queryMsg := `{"finality_providers":{}}`
	var (
		smartContractResponse *wasmtypes.QuerySmartContractStateResponse
		result                ConsumerFpsResponse
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
func (n *NodeConfig) QueryConsumerDelegations(stakingContractAddr string) (*ConsumerDelegationsResponse, error) {
	queryMsg := `{"delegations":{}}`
	var (
		smartContractResponse *wasmtypes.QuerySmartContractStateResponse
		result                ConsumerDelegationsResponse
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
func (n *NodeConfig) QuerySingleConsumerFp(stakingContractAddr string, btcPkHex string) (*ConsumerFpResponse, error) {
	queryMsg := fmt.Sprintf(`{"finality_provider":{"btc_pk_hex":"%s"}}`, btcPkHex)
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

// QuerySingleConsumerDelegation queries a specific BTC delegation by staking tx hash hex
func (n *NodeConfig) QuerySingleConsumerDelegation(stakingContractAddr string, stakingTxHashHex string) (*SingleConsumerDelegationResponse, error) {
	queryMsg := fmt.Sprintf(`{"delegation":{"staking_tx_hash_hex":"%s"}}`, stakingTxHashHex)
	var (
		smartContractResponse *wasmtypes.QuerySmartContractStateResponse
		result                SingleConsumerDelegationResponse
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

// QueryConsumerFpsByPower returns all finality providers with their respective power
func (n *NodeConfig) QueryConsumerFpsByPower(stakingContractAddr string) (*ConsumerFpsByPowerResponse, error) {
	queryMsg := `{"finality_providers_by_power":{}}`
	var (
		smartContractResponse *wasmtypes.QuerySmartContractStateResponse
		result                ConsumerFpsByPowerResponse
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

// QueryConsumerFpInfo queries a specific finality provider by Bitcoin public key hex and returns its power
func (n *NodeConfig) QueryConsumerFpInfo(stakingContractAddr string, fpBtcPkHex string, height *uint64) (*ConsumerFpInfo, error) {
	var queryMsg string
	if height != nil {
		queryMsg = fmt.Sprintf(`{"finality_provider_info":{"btc_pk_hex":"%s", "height": "%d"}}`, fpBtcPkHex, *height)
	} else {
		queryMsg = fmt.Sprintf(`{"finality_provider_info":{"btc_pk_hex":"%s"}}`, fpBtcPkHex)
	}

	var (
		smartContractResponse *wasmtypes.QuerySmartContractStateResponse
		result                ConsumerFpInfo
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
