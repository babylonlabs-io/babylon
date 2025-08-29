package chain

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	sdkmath "cosmossdk.io/math"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	cmtabcitypes "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	grpctypes "github.com/cosmos/cosmos-sdk/types/grpc"
	"github.com/cosmos/cosmos-sdk/types/query"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	dstrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypesv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	icacontrollertypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/controller/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
	blc "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	ct "github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
	etypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	minttypes "github.com/babylonlabs-io/babylon/v4/x/mint/types"
	mtypes "github.com/babylonlabs-io/babylon/v4/x/monitor/types"
)

func (n *NodeConfig) QueryGRPCGateway(path string, queryParams url.Values) ([]byte, error) {
	return n.QueryGRPCGatewayWithHeaders(path, queryParams, nil)
}

func (n *NodeConfig) QueryGRPCGatewayWithHeaders(path string, queryParams url.Values, headers map[string]string) ([]byte, error) {
	// add the URL for the given validator ID, and prepend to path.
	hostPort, err := n.containerManager.GetHostPort(n.Name, "1317/tcp")
	require.NoError(n.t, err)
	endpoint := fmt.Sprintf("http://%s", hostPort)
	fullQueryPath := fmt.Sprintf("%s/%s", endpoint, path)

	var resp *http.Response
	require.Eventually(n.t, func() bool {
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			fullQueryPath,
			nil,
		)
		if err != nil {
			return false
		}

		if len(queryParams) > 0 {
			req.URL.RawQuery = queryParams.Encode()
		}

		// Add custom headers
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			n.t.Logf("error while executing HTTP request: %s", err.Error())
			return false
		}

		return resp.StatusCode != http.StatusServiceUnavailable
	}, time.Minute, time.Millisecond*10, "failed to execute HTTP request")

	defer resp.Body.Close()

	bz, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bz))
	}
	return bz, nil
}

// QueryModuleAddress returns the address of a given module
func (n *NodeConfig) QueryModuleAddress(name string) (sdk.AccAddress, error) {
	path := fmt.Sprintf("/cosmos/auth/v1beta1/module_accounts/%s", name)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp authtypes.QueryModuleAccountByNameResponse
	if err := util.Cdc.UnmarshalJSON(bz, &resp); err != nil {
		return sdk.AccAddress{}, err
	}
	// cast to account
	var account sdk.AccountI
	if err := util.EncodingConfig.InterfaceRegistry.UnpackAny(resp.Account, &account); err != nil {
		return sdk.AccAddress{}, err
	}

	return account.GetAddress(), nil
}

// QueryAccount returns the account given the address
func (n *NodeConfig) QueryAccount(address string) (sdk.AccountI, error) {
	path := fmt.Sprintf("/cosmos/auth/v1beta1/accounts/%s", address)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp authtypes.QueryAccountResponse
	if err := util.Cdc.UnmarshalJSON(bz, &resp); err != nil {
		return nil, err
	}

	var account sdk.AccountI
	if err := util.EncodingConfig.InterfaceRegistry.UnpackAny(resp.Account, &account); err != nil {
		return nil, err
	}

	return account, nil
}

// QueryBalances returns balances at the address.
func (n *NodeConfig) QueryBalances(address string) (sdk.Coins, error) {
	return n.QueryBalancesAtHeight(address, 0)
}

// QueryBalancesAtHeight returns balances at the address at a specific block height.
// If height is 0, queries the latest block.
func (n *NodeConfig) QueryBalancesAtHeight(address string, height uint64) (sdk.Coins, error) {
	path := fmt.Sprintf("cosmos/bank/v1beta1/balances/%s", address)

	var headers map[string]string
	if height > 0 {
		headers = map[string]string{
			grpctypes.GRPCBlockHeightHeader: strconv.FormatUint(height, 10),
		}
	}

	bz, err := n.QueryGRPCGatewayWithHeaders(path, url.Values{}, headers)
	require.NoError(n.t, err)

	var balancesResp banktypes.QueryAllBalancesResponse
	if err := util.Cdc.UnmarshalJSON(bz, &balancesResp); err != nil {
		return sdk.Coins{}, err
	}
	return balancesResp.GetBalances(), nil
}

// QueryDistributionRewards returns distribution module rewards available at the address.
func (n *NodeConfig) QueryDistributionRewards(address string) (sdk.Coins, error) {
	path := fmt.Sprintf("/cosmos/distribution/v1beta1/delegators/%s/rewards", address)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var rwdsResp dstrtypes.QueryDelegationTotalRewardsResponse
	if err := util.Cdc.UnmarshalJSON(bz, &rwdsResp); err != nil {
		return sdk.Coins{}, err
	}

	return sdk.NormalizeCoins(rwdsResp.Total), nil
}

// QueryBalance returns balance of some address.
func (n *NodeConfig) QueryBalance(address, denom string) (*sdk.Coin, error) {
	return n.QueryBalanceAtHeight(address, denom, 0)
}

// QueryBalanceAtHeight returns balance of some address at a specific block height.
// If height is 0, queries the latest block.
func (n *NodeConfig) QueryBalanceAtHeight(address, denom string, height int64) (*sdk.Coin, error) {
	path := fmt.Sprintf("cosmos/bank/v1beta1/balances/%s/by_denom", address)

	params := url.Values{}
	params.Set("denom", denom)

	var headers map[string]string
	if height > 0 {
		headers = map[string]string{
			grpctypes.GRPCBlockHeightHeader: strconv.FormatInt(height, 10),
		}
	}

	bz, err := n.QueryGRPCGatewayWithHeaders(path, params, headers)
	require.NoError(n.t, err)

	var balancesResp banktypes.QueryBalanceResponse
	if err := util.Cdc.UnmarshalJSON(bz, &balancesResp); err != nil {
		return nil, err
	}
	return balancesResp.GetBalance(), nil
}

// QueryBankSendEnabled returns the status of the denom if it is possible to send bank tx transactions.
func (n *NodeConfig) QueryBankSendEnabled(denoms ...string) ([]*banktypes.SendEnabled, error) {
	path := "cosmos/bank/v1beta1/send_enabled"

	params := url.Values{}
	params.Set("denoms", strings.Join(denoms, " "))
	bz, err := n.QueryGRPCGateway(path, params)
	require.NoError(n.t, err)

	var resp banktypes.QuerySendEnabledResponse
	if err := util.Cdc.UnmarshalJSON(bz, &resp); err != nil {
		return nil, err
	}
	return resp.SendEnabled, nil
}

func (n *NodeConfig) QuerySupplyOf(denom string) (sdkmath.Int, error) {
	path := fmt.Sprintf("cosmos/bank/v1beta1/supply/%s", denom)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var supplyResp banktypes.QuerySupplyOfResponse
	if err := util.Cdc.UnmarshalJSON(bz, &supplyResp); err != nil {
		return sdkmath.NewInt(0), err
	}
	return supplyResp.Amount.Amount, nil
}

// QueryBlock gets block at a specific height
func (n *NodeConfig) QueryBlock(height int64) (*cmttypes.Block, error) {
	block, err := n.rpcClient.Block(context.Background(), &height)
	if err != nil {
		return nil, err
	}
	return block.Block, nil
}

// QueryHashFromBlock gets block hash at a specific height. Otherwise, error.
func (n *NodeConfig) QueryHashFromBlock(height int64) (string, error) {
	block, err := n.rpcClient.Block(context.Background(), &height)
	if err != nil {
		return "", err
	}
	return block.BlockID.Hash.String(), nil
}

// QueryCurrentHeight returns the current block height of the node or error.
func (n *NodeConfig) QueryCurrentHeight() (int64, error) {
	status, err := n.rpcClient.Status(context.Background())
	if err != nil {
		return 0, err
	}
	return status.SyncInfo.LatestBlockHeight, nil
}

// QueryLatestBlockTime returns the latest block time.
func (n *NodeConfig) QueryLatestBlockTime() time.Time {
	status, err := n.rpcClient.Status(context.Background())
	require.NoError(n.t, err)
	return status.SyncInfo.LatestBlockTime
}

// QueryListSnapshots gets all snapshots currently created for a node.
func (n *NodeConfig) QueryListSnapshots() ([]*cmtabcitypes.Snapshot, error) {
	abciResponse, err := n.rpcClient.ABCIQuery(context.Background(), "/app/snapshots", nil)
	if err != nil {
		return nil, err
	}

	var listSnapshots cmtabcitypes.ResponseListSnapshots
	if err := json.Unmarshal(abciResponse.Response.Value, &listSnapshots); err != nil {
		return nil, err
	}

	return listSnapshots.Snapshots, nil
}

func (n *NodeConfig) QueryRawCheckpoint(epoch uint64) (*ct.RawCheckpointWithMetaResponse, error) {
	path := fmt.Sprintf("/babylon/checkpointing/v1/raw_checkpoint/%d", epoch)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	if err != nil {
		return nil, err
	}

	var checkpointingResponse ct.QueryRawCheckpointResponse
	if err := util.Cdc.UnmarshalJSON(bz, &checkpointingResponse); err != nil {
		return nil, err
	}

	return checkpointingResponse.RawCheckpoint, nil
}

func (n *NodeConfig) QueryRawCheckpoints(pagination *query.PageRequest) (*ct.QueryRawCheckpointsResponse, error) {
	queryParams := url.Values{}
	if pagination != nil {
		queryParams.Set("pagination.key", base64.URLEncoding.EncodeToString(pagination.Key))
		queryParams.Set("pagination.limit", strconv.Itoa(int(pagination.Limit)))
	}

	bz, err := n.QueryGRPCGateway("/babylon/checkpointing/v1/raw_checkpoints", queryParams)
	require.NoError(n.t, err)

	var checkpointingResponse ct.QueryRawCheckpointsResponse
	if err := util.Cdc.UnmarshalJSON(bz, &checkpointingResponse); err != nil {
		return nil, err
	}

	return &checkpointingResponse, nil
}

func (n *NodeConfig) QueryLastFinalizedEpoch() (uint64, error) {
	queryParams := url.Values{}
	queryParams.Add("status", fmt.Sprintf("%d", ct.Finalized))

	bz, err := n.QueryGRPCGateway(fmt.Sprintf("/babylon/checkpointing/v1/last_raw_checkpoint/%d", ct.Finalized), queryParams)
	require.NoError(n.t, err)
	var res ct.QueryLastCheckpointWithStatusResponse
	if err := util.Cdc.UnmarshalJSON(bz, &res); err != nil {
		return 0, err
	}
	return res.RawCheckpoint.EpochNum, nil
}

func (n *NodeConfig) QueryBtcBaseHeader() (*blc.BTCHeaderInfoResponse, error) {
	bz, err := n.QueryGRPCGateway("/babylon/btclightclient/v1/baseheader", url.Values{})
	require.NoError(n.t, err)

	var blcResponse blc.QueryBaseHeaderResponse
	if err := util.Cdc.UnmarshalJSON(bz, &blcResponse); err != nil {
		return nil, err
	}

	return blcResponse.Header, nil
}

func (n *NodeConfig) QueryTip() (*blc.BTCHeaderInfoResponse, error) {
	bz, err := n.QueryGRPCGateway("/babylon/btclightclient/v1/tip", url.Values{})
	require.NoError(n.t, err)

	var blcResponse blc.QueryTipResponse
	if err := util.Cdc.UnmarshalJSON(bz, &blcResponse); err != nil {
		return nil, err
	}

	return blcResponse.Header, nil
}

func (n *NodeConfig) QueryHeaderDepth(hash string) (uint32, error) {
	path := fmt.Sprintf("/babylon/btclightclient/v1/depth/%s", hash)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var blcResponse blc.QueryHeaderDepthResponse
	if err := util.Cdc.UnmarshalJSON(bz, &blcResponse); err != nil {
		return 0, err
	}

	return blcResponse.Depth, nil
}

func (n *NodeConfig) QueryCurrentEpoch() (uint64, error) {
	bz, err := n.QueryGRPCGateway("/babylon/epoching/v1/current_epoch", url.Values{})
	require.NoError(n.t, err)
	var epochResponse etypes.QueryCurrentEpochResponse
	if err := util.Cdc.UnmarshalJSON(bz, &epochResponse); err != nil {
		return 0, err
	}
	return epochResponse.CurrentEpoch, nil
}

func (n *NodeConfig) QueryLightClientHeightEpochEnd(epoch uint64) (uint32, error) {
	monitorPath := fmt.Sprintf("/babylon/monitor/v1/epochs/%d", epoch)
	bz, err := n.QueryGRPCGateway(monitorPath, url.Values{})
	require.NoError(n.t, err)
	var mResponse mtypes.QueryEndedEpochBtcHeightResponse
	if err := util.Cdc.UnmarshalJSON(bz, &mResponse); err != nil {
		return 0, err
	}
	return mResponse.BtcLightClientHeight, nil
}

func (n *NodeConfig) QueryLightClientHeightCheckpointReported(ckptHash []byte) (uint32, error) {
	monitorPath := fmt.Sprintf("/babylon/monitor/v1/checkpoints/%x", ckptHash)
	bz, err := n.QueryGRPCGateway(monitorPath, url.Values{})
	require.NoError(n.t, err)
	var mResponse mtypes.QueryReportedCheckpointBtcHeightResponse
	if err := util.Cdc.UnmarshalJSON(bz, &mResponse); err != nil {
		return 0, err
	}
	return mResponse.BtcLightClientHeight, nil
}

func (n *NodeConfig) QueryLatestWasmCodeID() uint64 {
	path := "/cosmwasm/wasm/v1/code"

	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var response wasmtypes.QueryCodesResponse
	err = util.Cdc.UnmarshalJSON(bz, &response)
	require.NoError(n.t, err)
	if len(response.CodeInfos) == 0 {
		return 0
	}
	return response.CodeInfos[len(response.CodeInfos)-1].CodeID
}

func (n *NodeConfig) QueryContractsFromId(codeId int) ([]string, error) {
	path := fmt.Sprintf("/cosmwasm/wasm/v1/code/%d/contracts", codeId)
	bz, err := n.QueryGRPCGateway(path, url.Values{})

	require.NoError(n.t, err)

	var contractsResponse wasmtypes.QueryContractsByCodeResponse
	if err := util.Cdc.UnmarshalJSON(bz, &contractsResponse); err != nil {
		return nil, err
	}

	return contractsResponse.Contracts, nil
}

func (n *NodeConfig) QueryWasmSmart(contract string, queryMsg string) (*wasmtypes.QuerySmartContractStateResponse, error) {
	// base64-encode the queryMsg
	encodedMsg := base64.StdEncoding.EncodeToString([]byte(queryMsg))
	path := fmt.Sprintf("/cosmwasm/wasm/v1/contract/%s/smart/%s", contract, encodedMsg)

	bz, err := n.QueryGRPCGateway(path, url.Values{})
	if err != nil {
		return nil, err
	}

	var response wasmtypes.QuerySmartContractStateResponse
	err = util.Cdc.UnmarshalJSON(bz, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (n *NodeConfig) QueryProposal(proposalNumber int) govtypesv1.QueryProposalResponse {
	path := fmt.Sprintf("cosmos/gov/v1beta1/proposals/%d", proposalNumber)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp govtypesv1.QueryProposalResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp
}

func (n *NodeConfig) QueryProposals() govtypesv1.QueryProposalsResponse {
	bz, err := n.QueryGRPCGateway("cosmos/gov/v1beta1/proposals", url.Values{})
	require.NoError(n.t, err)

	var resp govtypesv1.QueryProposalsResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp
}

func (n *NodeConfig) QueryAppliedPlan(planName string) upgradetypes.QueryAppliedPlanResponse {
	path := fmt.Sprintf("cosmos/upgrade/v1beta1/applied_plan/%s", planName)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp upgradetypes.QueryAppliedPlanResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp
}

func (n *NodeConfig) QueryTx(txHash string, overallFlags ...string) (sdk.TxResponse, *sdktx.Tx) {
	cmd := []string{
		"babylond", "q", "tx", "--type=hash", txHash, "--output=json",
		n.FlagChainID(),
	}

	out, _, err := n.containerManager.ExecCmd(n.t, n.Name, append(cmd, overallFlags...), "")
	require.NoError(n.t, err)

	var txResp sdk.TxResponse
	err = util.Cdc.UnmarshalJSON(out.Bytes(), &txResp)
	require.NoError(n.t, err)

	txAuth := txResp.Tx.GetCachedValue().(*sdktx.Tx)
	return txResp, txAuth
}

func (n *NodeConfig) QueryTxWithError(txHash string, overallFlags ...string) (sdk.TxResponse, *sdktx.Tx, error) {
	cmd := []string{
		"babylond", "q", "tx", "--type=hash", txHash, "--output=json",
		n.FlagChainID(),
	}

	out, stderr, err := n.containerManager.ExecCmd(n.t, n.Name, append(cmd, overallFlags...), "")
	if err != nil {
		return sdk.TxResponse{}, nil, fmt.Errorf("failed to execute command: %v, stderr: %s", err, stderr.String())
	}

	var txResp sdk.TxResponse
	err = util.Cdc.UnmarshalJSON(out.Bytes(), &txResp)
	if err != nil {
		if err == io.EOF {
			return sdk.TxResponse{}, nil, fmt.Errorf("unexpected EOF while unmarshalling transaction response, output: %s", out.String())
		}
		return sdk.TxResponse{}, nil, fmt.Errorf("failed to unmarshal transaction response: %v, output: %s", err, out.String())
	}

	txAuth, ok := txResp.Tx.GetCachedValue().(*sdktx.Tx)
	if !ok {
		return sdk.TxResponse{}, nil, fmt.Errorf("failed to cast transaction to *sdktx.Tx, response: %v", txResp)
	}

	return txResp, txAuth, nil
}

func (n *NodeConfig) QueryICAAccountAddress(owner, connectionID string) string {
	path := fmt.Sprintf("ibc/apps/interchain_accounts/controller/v1/owners/%s/connections/%s", owner, connectionID)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp icacontrollertypes.QueryInterchainAccountResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp.Address
}

func (n *NodeConfig) WaitUntilCurrentEpochIsSealedAndFinalized(startEpoch uint64) (lastFinalizedEpoch uint64) {
	// finalize epochs from 1 to the current epoch
	currentEpoch, err := n.QueryCurrentEpoch()
	require.NoError(n.t, err)

	// wait until the end epoch is sealed
	require.Eventually(n.t, func() bool {
		resp, err := n.QueryRawCheckpoint(currentEpoch)
		if err != nil {
			return false
		}
		return resp.Status == ct.Sealed
	}, time.Minute*5, time.Millisecond*200)
	n.FinalizeSealedEpochs(startEpoch, currentEpoch)

	// ensure the committed epoch is finalized
	require.Eventually(n.t, func() bool {
		lastFinalizedEpoch, err = n.QueryLastFinalizedEpoch()
		if err != nil {
			return false
		}
		return lastFinalizedEpoch >= currentEpoch
	}, time.Minute*2, time.Millisecond*200)
	return lastFinalizedEpoch
}

func (n *NodeConfig) WaitFinalityIsActivated() (activatedHeight uint64) {
	var err error
	require.Eventually(n.t, func() bool {
		activatedHeight, err = n.QueryActivatedHeight()
		if err != nil {
			n.t.Logf("WaitFinalityIsActivated: err query activated height %s", err.Error())
			return false
		}
		return activatedHeight > 0
	}, time.Minute*4, 10*time.Second)
	n.t.Logf("the activated height is %d", activatedHeight)
	return activatedHeight
}

func (n *NodeConfig) QueryBalancesN(addrs ...string) map[string]sdk.Coins {
	resp := make(map[string]sdk.Coins, len(addrs))
	for _, addr := range addrs {
		coins, err := n.QueryBalances(addr)
		require.NoError(n.t, err)

		resp[addr] = coins
	}
	return resp
}

// BalancesDiff queries the balance before and after querying the func and returns it
func (n *NodeConfig) BalancesDiff(f func(), addrs ...string) map[string]sdk.Coins {
	before := n.QueryBalancesN(addrs...)

	f()

	after := n.QueryBalancesN(addrs...)

	resp := make(map[string]sdk.Coins, len(addrs))
	for _, addr := range addrs {
		resp[addr] = after[addr].Sub(before[addr]...)
	}
	return resp
}

// QueryMintedAmountFromEvents queries the actual minted amount from mint events in a specific block
func (n *NodeConfig) QueryMintedAmountFromEvents(blockHeight int64) (sdk.Coins, error) {
	if blockHeight <= 1 {
		// No minting for genesis block or block 1
		return sdk.NewCoins(), nil
	}

	// Query block results to get events
	blockResults, err := n.rpcClient.BlockResults(context.Background(), &blockHeight)
	if err != nil {
		return sdk.Coins{}, err
	}

	// Look for mint events in BeginBlockEvents
	for _, event := range blockResults.FinalizeBlockEvents {
		if event.Type == minttypes.EventTypeMint {
			// Find the amount attribute
			for _, attr := range event.Attributes {
				if attr.Key == sdk.AttributeKeyAmount {
					// Parse the amount
					if attr.Value == "" {
						continue
					}

					// Parse as integer (amount is just the number without denom)
					amount, ok := sdkmath.NewIntFromString(attr.Value)
					if !ok {
						return sdk.Coins{}, fmt.Errorf("failed to parse minted amount: %s", attr.Value)
					}

					mintedCoin := sdk.NewCoin(minttypes.DefaultBondDenom, amount)
					return sdk.NewCoins(mintedCoin), nil
				}
			}
		}
	}

	// No mint event found, return empty coins
	return sdk.NewCoins(), nil
}
