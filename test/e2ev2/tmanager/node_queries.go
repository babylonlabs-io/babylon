package tmanager

import (
	"context"
	"net/url"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
	bsctypes "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	zctypes "github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

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

func (n *Node) ZoneConciergeQuery(f func(zctypes.QueryClient)) {
	n.GrpcConn(func(conn *grpc.ClientConn) {
		zcClient := zctypes.NewQueryClient(conn)
		f(zcClient)
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
	var (
		resp *bsctypes.QueryConsumerRegistryListResponse
		err  error
	)

	n.BtcStkConsumerQuery(func(bscClient bsctypes.QueryClient) {
		resp, err = bscClient.ConsumerRegistryList(context.Background(), &bsctypes.QueryConsumerRegistryListRequest{})
		require.NoError(n.T(), err)
	})

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

// QueryConsumerActive queries whether a given consumer is currently active
func (n *Node) QueryConsumerActive(consumerID string) *zctypes.QueryConsumerActiveResponse {
	var (
		resp *zctypes.QueryConsumerActiveResponse
		err  error
	)

	n.ZoneConciergeQuery(func(zcClient zctypes.QueryClient) {
		resp, err = zcClient.ConsumerActive(context.Background(), &zctypes.QueryConsumerActiveRequest{
			ConsumerId: consumerID,
		})
		require.NoError(n.T(), err)
	})

	return resp
}

// QueryConsumerActiveWithError queries whether a given consumer is currently active, returning error if any
func (n *Node) QueryConsumerActiveWithError(consumerID string) (*zctypes.QueryConsumerActiveResponse, error) {
	var (
		resp *zctypes.QueryConsumerActiveResponse
		err  error
	)

	n.ZoneConciergeQuery(func(zcClient zctypes.QueryClient) {
		resp, err = zcClient.ConsumerActive(context.Background(), &zctypes.QueryConsumerActiveRequest{
			ConsumerId: consumerID,
		})
	})

	return resp, err
}
