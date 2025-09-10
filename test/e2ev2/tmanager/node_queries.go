package tmanager

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strings"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btclighttypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	bsctypes "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	zoneconciergetype "github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ParseBTCHeaderInfoResponseToInfo converts BTCHeaderInfoResponse to BTCHeaderInfo
func ParseBTCHeaderInfoResponseToInfo(r *btclighttypes.BTCHeaderInfoResponse) (*btclighttypes.BTCHeaderInfo, error) {
	header, err := bbn.NewBTCHeaderBytesFromHex(r.HeaderHex)
	if err != nil {
		return nil, err
	}

	hash, err := bbn.NewBTCHeaderHashBytesFromHex(r.HashHex)
	if err != nil {
		return nil, err
	}

	return &btclighttypes.BTCHeaderInfo{
		Header: &header,
		Hash:   &hash,
		Height: r.Height,
		Work:   &r.Work,
	}, nil
}

func (n *Node) GrpcConn(f func(conn *grpc.ClientConn)) {
	conn, err := grpc.NewClient(n.GrpcEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(n.T(), err)
	defer conn.Close()

	f(conn)
}

func (n *Node) AuthQuery(f func(authtypes.QueryClient)) {
	n.GrpcConn(func(conn *grpc.ClientConn) {
		authClient := authtypes.NewQueryClient(conn)
		f(authClient)
	})
}

func (n *Node) TxQuery(f func(sdktx.ServiceClient)) {
	n.GrpcConn(func(conn *grpc.ClientConn) {
		txClient := sdktx.NewServiceClient(conn)
		f(txClient)
	})
}

func (n *Node) BankQuery(f func(banktypes.QueryClient)) {
	n.GrpcConn(func(conn *grpc.ClientConn) {
		bankClient := banktypes.NewQueryClient(conn)
		f(bankClient)
	})
}

func (n *Node) BtcStkConsumerQuery(f func(bsctypes.QueryClient)) {
	n.GrpcConn(func(conn *grpc.ClientConn) {
		bscClient := bsctypes.NewQueryClient(conn)
		f(bscClient)
	})
}

func (n *Node) IncentiveQuery(f func(ictvtypes.QueryClient)) {
	n.GrpcConn(func(conn *grpc.ClientConn) {
		incentiveClient := ictvtypes.NewQueryClient(conn)
		f(incentiveClient)
	})
}

func (n *Node) LatestBlockNumber() (uint64, error) {
	status, err := n.RpcClient.Status(context.Background())
	if err != nil {
		return 0, err
	}
	return uint64(status.SyncInfo.LatestBlockHeight), nil
}

// QueryIBCChannels queries all IBC channels on this node
func (n *Node) QueryIBCChannels() *channeltypes.QueryChannelsResponse {
	path := "/ibc/core/channel/v1/channels"
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.T(), err)

	var resp channeltypes.QueryChannelsResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.T(), err)

	return &resp
}

// QueryAccountInfo queries the account number and sequence number for a given address from a running node
func (n *Node) QueryAccountInfo(address string) (accountNumber, sequenceNumber uint64) {
	var (
		resp *authtypes.QueryAccountResponse
		err  error
	)

	n.AuthQuery(func(qc authtypes.QueryClient) {
		resp, err = qc.Account(context.Background(), &authtypes.QueryAccountRequest{
			Address: address,
		})
		require.NoError(n.T(), err)
	})

	var account sdk.AccountI
	err = util.Cdc.UnpackAny(resp.Account, &account)
	require.NoError(n.T(), err)

	return account.GetAccountNumber(), account.GetSequence()
}

// QueryAllAccountInfo queries the accounts number and sequence number for the given addresses from a running node
func (n *Node) QueryAllAccountInfo(address ...string) map[string]sdk.AccountI {
	var (
		resp *authtypes.QueryAccountResponse
		err  error
	)

	ret := make(map[string]sdk.AccountI, len(address))
	n.AuthQuery(func(qc authtypes.QueryClient) {
		for _, addr := range address {
			resp, err = qc.Account(context.Background(), &authtypes.QueryAccountRequest{
				Address: addr,
			})
			require.NoError(n.T(), err)

			var account sdk.AccountI
			err = util.Cdc.UnpackAny(resp.Account, &account)
			require.NoError(n.T(), err)

			ret[addr] = account
		}
	})

	return ret
}

// QueryTxByHash queries a transaction by its hash from a running node
func (n *Node) QueryTxByHash(txHash string) *sdktx.GetTxResponse {
	var (
		resp *sdktx.GetTxResponse
		err  error
	)

	n.TxQuery(func(txClient sdktx.ServiceClient) {
		resp, err = txClient.GetTx(context.Background(), &sdktx.GetTxRequest{
			Hash: txHash,
		})
		require.NoError(n.T(), err)
	})

	return resp
}

// QueryAllBalances queries all coin balances for a given address from a running node
func (n *Node) QueryAllBalances(address string) sdk.Coins {
	var (
		resp *banktypes.QueryAllBalancesResponse
		err  error
	)

	n.BankQuery(func(bankClient banktypes.QueryClient) {
		resp, err = bankClient.AllBalances(context.Background(), &banktypes.QueryAllBalancesRequest{
			Address: address,
		})
		require.NoError(n.T(), err)
	})

	return resp.Balances
}

// QueryBTCStkConsumerConsumers queries all registered BTC staking consumer chains
func (n *Node) QueryBTCStkConsumerConsumers() []*bsctypes.ConsumerRegisterResponse {
	path := "/babylon/btcstkconsumer/v1/consumer_registry_list"
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.T(), err)

	var resp bsctypes.QueryConsumerRegistryListResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.T(), err)

	return resp.ConsumerRegisters
}

// QueryFinalityProviderRewards queries rewards for multiple finality providers
func (n *Node) QueryFinalityProviderRewards(fpAddrs []string) map[string]sdk.Coins {
	return n.QueryIctvRewardGauges(fpAddrs, ictvtypes.FINALITY_PROVIDER)
}

// QueryDelegatorRewards queries rewards for multiple delegators
func (n *Node) QueryDelegatorRewards(delAddrs []string) map[string]sdk.Coins {
	return n.QueryIctvRewardGauges(delAddrs, ictvtypes.BTC_STAKER)
}

// QueryIctvRewardGauges queries rewards for multiple delegators
func (n *Node) QueryIctvRewardGauges(addrs []string, holderType ictvtypes.StakeholderType) map[string]sdk.Coins {
	rewards := make(map[string]sdk.Coins, len(addrs))

	n.IncentiveQuery(func(ictvClient ictvtypes.QueryClient) {
		for _, addr := range addrs {
			resp, err := ictvClient.RewardGauges(context.Background(), &ictvtypes.QueryRewardGaugesRequest{
				Address: addr,
			})
			require.NoError(n.T(), err)

			rewards[addr] = resp.RewardGauges[holderType.String()].Coins
		}
	})

	return rewards
}

// QueryLatestEpochHeader retrieves the latest epoch header for the specified consumer ID
func (n *Node) QueryLatestEpochHeader(consumerID string) *zoneconciergetype.QueryLatestEpochHeaderResponse {
	path := fmt.Sprintf("/babylon/zoneconcierge/v1/latest_epoch_header/%s", consumerID)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.T(), err)

	var resp zoneconciergetype.QueryLatestEpochHeaderResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.T(), err)

	return &resp
}

// QueryBSNLastSentSegment retrieves the last sent segment information for the specified consumer ID
func (n *Node) QueryBSNLastSentSegment(consumerID string) *zoneconciergetype.QueryBSNLastSentSegmentResponse {
	path := fmt.Sprintf("/babylon/zoneconcierge/v1/bsn_last_sent_segment/%s", consumerID)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.T(), err)

	var resp zoneconciergetype.QueryBSNLastSentSegmentResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.T(), err)

	return &resp
}

// QueryGetSealedEpochProof retrieves the sealed epoch proof for the specified epoch number
func (n *Node) QueryGetSealedEpochProof(epochNum uint64) *zoneconciergetype.QueryGetSealedEpochProofResponse {
	path := fmt.Sprintf("/babylon/zoneconcierge/v1/sealed_epoch_proof/%d", epochNum)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.T(), err)

	var resp zoneconciergetype.QueryGetSealedEpochProofResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.T(), err)

	return &resp
}

// QueryLatestEpochHeaderCLI retrieves the latest epoch header for the specified consumer ID using CLI
func (n *Node) QueryLatestEpochHeaderCLI(consumerID string) string {
	cmd := []string{"babylond", "query", "zc", "latest-epoch-header", consumerID, "--output=json", "--node", n.GetRpcEndpoint()}
	outBuf, _, err := n.Tm.ContainerManager.ExecCmd(n.T(), n.Container.Name, cmd, "")
	require.NoError(n.T(), err)
	return outBuf.String()
}

// QueryBSNLastSentSegmentCLI retrieves the last sent segment information for the specified consumer ID using CLI
func (n *Node) QueryBSNLastSentSegmentCLI(consumerID string) string {
	cmd := []string{"babylond", "query", "zc", "bsn-last-sent-seg", consumerID, "--output=json", "--node", n.GetRpcEndpoint()}
	outBuf, _, err := n.Tm.ContainerManager.ExecCmd(n.T(), n.Container.Name, cmd, "")
	require.NoError(n.T(), err)
	return outBuf.String()
}

// QueryGetSealedEpochProofCLI retrieves the sealed epoch proof for the specified epoch number using CLI
func (n *Node) QueryGetSealedEpochProofCLI(epochNum uint64) string {
	cmd := []string{"babylond", "query", "zc", "get-sealed-epoch-proof", fmt.Sprintf("%d", epochNum), "--output=json", "--node", n.GetRpcEndpoint()}
	outBuf, _, err := n.Tm.ContainerManager.ExecCmd(n.T(), n.Container.Name, cmd, "")
	require.NoError(n.T(), err)
	return outBuf.String()
}

// GetRpcEndpoint returns the RPC endpoint of the node
func (n *Node) GetRpcEndpoint() string {
	return "tcp://" + net.JoinHostPort(n.Container.Name, fmt.Sprintf("%d", n.Ports.RPC))
}

func (n *Node) QueryConsumerActive(consumerID string) (bool, error) {
	var (
		resp *bsctypes.QueryConsumerActiveResponse
		err  error
	)

	n.BtcStkConsumerQuery(func(bscClient bsctypes.QueryClient) {
		resp, err = bscClient.ConsumerActive(context.Background(), &bsctypes.QueryConsumerActiveRequest{
			ConsumerId: consumerID,
		})
	})
	if err != nil {
		if strings.Contains(err.Error(), "consumer cannot be found") {
			return false, nil
		}
		return false, err
	}

	return resp.Active, nil
}

func (n *Node) QueryConsumerActiveWithError(consumerID string) (bool, error) {
	var (
		resp *bsctypes.QueryConsumerActiveResponse
		err  error
	)

	n.BtcStkConsumerQuery(func(bscClient bsctypes.QueryClient) {
		resp, err = bscClient.ConsumerActive(context.Background(), &bsctypes.QueryConsumerActiveRequest{
			ConsumerId: consumerID,
		})
	})

	if err != nil {
		return false, err
	}
	return resp.Active, nil
}

// QueryLatestWasmCodeID returns the latest WASM code ID
func (n *Node) QueryLatestWasmCodeID() uint64 {
	path := "/cosmwasm/wasm/v1/code"
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.T(), err)

	var response wasmtypes.QueryCodesResponse
	err = util.Cdc.UnmarshalJSON(bz, &response)
	require.NoError(n.T(), err)

	if len(response.CodeInfos) == 0 {
		return 0
	}
	return response.CodeInfos[len(response.CodeInfos)-1].CodeID
}

// QueryContractsFromId returns all contract addresses for a given code ID
func (n *Node) QueryContractsFromId(codeId int) ([]string, error) {
	path := fmt.Sprintf("/cosmwasm/wasm/v1/code/%d/contracts", codeId)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	if err != nil {
		return nil, err
	}

	var contractsResponse wasmtypes.QueryContractsByCodeResponse
	if err := util.Cdc.UnmarshalJSON(bz, &contractsResponse); err != nil {
		return nil, err
	}
	return contractsResponse.Contracts, nil
}

// QueryWasmSmart executes a smart query on a WASM contract
func (n *Node) QueryWasmSmart(contract string, queryMsg string) (*wasmtypes.QuerySmartContractStateResponse, error) {
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
