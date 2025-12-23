package tmanager

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	checkpointingtypes "github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
	costktypes "github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
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

func (n *Node) IncentiveQuery(f func(ictvtypes.QueryClient)) {
	n.GrpcConn(func(conn *grpc.ClientConn) {
		incentiveClient := ictvtypes.NewQueryClient(conn)
		f(incentiveClient)
	})
}

func (n *Node) EpochingQuery(f func(epochingtypes.QueryClient)) {
	n.GrpcConn(func(conn *grpc.ClientConn) {
		incentiveClient := epochingtypes.NewQueryClient(conn)
		f(incentiveClient)
	})
}

func (n *Node) BtcStkQuery(f func(btcstktypes.QueryClient)) {
	n.GrpcConn(func(conn *grpc.ClientConn) {
		btcStakingClient := btcstktypes.NewQueryClient(conn)
		f(btcStakingClient)
	})
}

func (n *Node) FinalityQuery(f func(ftypes.QueryClient)) {
	n.GrpcConn(func(conn *grpc.ClientConn) {
		qc := ftypes.NewQueryClient(conn)
		f(qc)
	})
}

func (n *Node) CheckpointingQuery(f func(checkpointingtypes.QueryClient)) {
	n.GrpcConn(func(conn *grpc.ClientConn) {
		qc := checkpointingtypes.NewQueryClient(conn)
		f(qc)
	})
}

func (n *Node) CostkQuery(f func(costktypes.QueryClient)) {
	n.GrpcConn(func(conn *grpc.ClientConn) {
		qc := costktypes.NewQueryClient(conn)
		f(qc)
	})
}

func (n *Node) GovQuery(f func(govtypes.QueryClient)) {
	n.GrpcConn(func(conn *grpc.ClientConn) {
		govClient := govtypes.NewQueryClient(conn)
		f(govClient)
	})
}

func (n *Node) StakingQuery(f func(stktypes.QueryClient)) {
	n.GrpcConn(func(conn *grpc.ClientConn) {
		govClient := stktypes.NewQueryClient(conn)
		f(govClient)
	})
}

func (n *Node) UpgradeQuery(f func(upgradetypes.QueryClient)) {
	n.GrpcConn(func(conn *grpc.ClientConn) {
		upgradeClient := upgradetypes.NewQueryClient(conn)
		f(upgradeClient)
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

// QueryFinalityProviderRewards queries rewards for multiple finality providers
func (n *Node) QueryFinalityProviderRewards(fpAddrs []string) map[string]sdk.Coins {
	return n.QueryIctvRewardGauges(fpAddrs, ictvtypes.FINALITY_PROVIDER)
}

// QueryDelegatorRewards queries rewards for multiple delegators
func (n *Node) QueryDelegatorRewards(delAddrs []string) map[string]sdk.Coins {
	return n.QueryIctvRewardGauges(delAddrs, ictvtypes.BTC_STAKER)
}

// QueryCurrentEpoch queries the current epoch
func (n *Node) QueryCurrentEpoch() *epochingtypes.QueryCurrentEpochResponse {
	var (
		resp *epochingtypes.QueryCurrentEpochResponse
		err  error
	)

	n.EpochingQuery(func(qc epochingtypes.QueryClient) {
		resp, err = qc.CurrentEpoch(context.Background(), &epochingtypes.QueryCurrentEpochRequest{})
		require.NoError(n.T(), err)
	})
	return resp
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

func (n *Node) QueryBtcStakingParams() *btcstktypes.Params {
	var (
		resp *btcstktypes.QueryParamsResponse
		err  error
	)

	n.BtcStkQuery(func(btcStkClient btcstktypes.QueryClient) {
		resp, err = btcStkClient.Params(context.Background(), &btcstktypes.QueryParamsRequest{})
		require.NoError(n.T(), err)
	})

	return &resp.Params
}

func (n *Node) QueryRawCheckpoints(pagination *query.PageRequest) *checkpointingtypes.QueryRawCheckpointsResponse {
	var (
		resp *checkpointingtypes.QueryRawCheckpointsResponse
		err  error
	)

	n.CheckpointingQuery(func(qc checkpointingtypes.QueryClient) {
		resp, err = qc.RawCheckpoints(context.Background(), &checkpointingtypes.QueryRawCheckpointsRequest{
			Pagination: pagination,
		})
		require.NoError(n.T(), err)
	})

	return resp
}

func (n *Node) QueryRawCheckpoint(epochNum uint64) *checkpointingtypes.RawCheckpointWithMetaResponse {
	rawCkpt, err := n.QueryRawCheckpointWithErr(epochNum)
	require.NoError(n.T(), err)
	return rawCkpt
}

func (n *Node) QueryRawCheckpointWithErr(epochNum uint64) (*checkpointingtypes.RawCheckpointWithMetaResponse, error) {
	var (
		resp *checkpointingtypes.QueryRawCheckpointResponse
		err  error
	)

	n.CheckpointingQuery(func(qc checkpointingtypes.QueryClient) {
		resp, err = qc.RawCheckpoint(context.Background(), &checkpointingtypes.QueryRawCheckpointRequest{
			EpochNum: epochNum,
		})
	})

	if err != nil {
		return nil, err
	}

	return resp.RawCheckpoint, nil
}

func (n *Node) QueryLastCheckpointWithStatusResponse() *checkpointingtypes.RawCheckpointResponse {
	var (
		resp *checkpointingtypes.QueryLastCheckpointWithStatusResponse
		err  error
	)

	n.CheckpointingQuery(func(qc checkpointingtypes.QueryClient) {
		resp, err = qc.LastCheckpointWithStatus(context.Background(), &checkpointingtypes.QueryLastCheckpointWithStatusRequest{})
		require.NoError(n.T(), err)
	})

	return resp.RawCheckpoint
}

func (n *Node) QueryStakingParams() stktypes.Params {
	var (
		resp *stktypes.QueryParamsResponse
		err  error
	)

	n.StakingQuery(func(qc stktypes.QueryClient) {
		resp, err = qc.Params(context.Background(), &stktypes.QueryParamsRequest{})
		require.NoError(n.T(), err)
	})

	return resp.Params
}

func (n *Node) QueryValidator(valAddr sdk.ValAddress) stktypes.Validator {
	var (
		resp *stktypes.QueryValidatorResponse
		err  error
	)

	n.StakingQuery(func(qc stktypes.QueryClient) {
		resp, err = qc.Validator(context.Background(), &stktypes.QueryValidatorRequest{
			ValidatorAddr: valAddr.String(),
		})
		require.NoError(n.T(), err)
	})

	return resp.Validator
}

func (n *Node) QueryDelegation(delAddr sdk.AccAddress, valAddr sdk.ValAddress) stktypes.DelegationResponse {
	var (
		resp *stktypes.QueryDelegationResponse
		err  error
	)

	n.StakingQuery(func(qc stktypes.QueryClient) {
		resp, err = qc.Delegation(context.Background(), &stktypes.QueryDelegationRequest{
			DelegatorAddr: delAddr.String(),
			ValidatorAddr: valAddr.String(),
		})
		require.NoError(n.T(), err)
	})

	return *resp.DelegationResponse
}

func (n *Node) QueryBTCDelegation(stakingTxHash string) *btcstktypes.BTCDelegationResponse {
	var (
		resp *btcstktypes.QueryBTCDelegationResponse
		err  error
	)

	n.BtcStkQuery(func(btcStkClient btcstktypes.QueryClient) {
		resp, err = btcStkClient.BTCDelegation(context.Background(), &btcstktypes.QueryBTCDelegationRequest{
			StakingTxHashHex: stakingTxHash,
		})
		require.NoError(n.T(), err)
	})

	return resp.BtcDelegation
}

func (n *Node) QueryBTCDelegations(status btcstktypes.BTCDelegationStatus) []*btcstktypes.BTCDelegationResponse {
	var (
		resp *btcstktypes.QueryBTCDelegationsResponse
		err  error
	)

	n.BtcStkQuery(func(btcStkClient btcstktypes.QueryClient) {
		resp, err = btcStkClient.BTCDelegations(context.Background(), &btcstktypes.QueryBTCDelegationsRequest{
			Status: status,
		})
		require.NoError(n.T(), err)
	})

	return resp.BtcDelegations
}

func (n *Node) QueryFinalityProvider(fpBtcPkHex string) *btcstktypes.FinalityProviderResponse {
	var (
		resp *btcstktypes.QueryFinalityProviderResponse
		err  error
	)

	n.BtcStkQuery(func(btcStkClient btcstktypes.QueryClient) {
		resp, err = btcStkClient.FinalityProvider(context.Background(), &btcstktypes.QueryFinalityProviderRequest{
			FpBtcPkHex: fpBtcPkHex,
		})
		require.NoError(n.T(), err)
	})

	return resp.FinalityProvider
}

func (n *Node) QueryActivatedHeight() uint64 {
	var (
		resp *ftypes.QueryActivatedHeightResponse
		err  error
	)

	n.FinalityQuery(func(qc ftypes.QueryClient) {
		resp, err = qc.ActivatedHeight(context.Background(), &ftypes.QueryActivatedHeightRequest{})
		require.NoError(n.T(), err)
	})

	return resp.Height
}

func (n *Node) QueryProposals() *govtypes.QueryProposalsResponse {
	var (
		resp *govtypes.QueryProposalsResponse
		err  error
	)

	n.GovQuery(func(govClient govtypes.QueryClient) {
		resp, err = govClient.Proposals(context.Background(), &govtypes.QueryProposalsRequest{})
		require.NoError(n.T(), err)
	})

	return resp
}

func (n *Node) QueryCostkRwdTrck(addr sdk.AccAddress) *costktypes.QueryCostakerRewardsTrackerResponse {
	var (
		resp *costktypes.QueryCostakerRewardsTrackerResponse
		err  error
	)

	n.CostkQuery(func(qc costktypes.QueryClient) {
		resp, err = qc.CostakerRewardsTracker(context.Background(), &costktypes.QueryCostakerRewardsTrackerRequest{
			CostakerAddress: addr.String(),
		})
		require.NoError(n.T(), err)
	})

	return resp
}

func (n *Node) QueryCostkCurrRwd() *costktypes.QueryCurrentRewardsResponse {
	var (
		resp *costktypes.QueryCurrentRewardsResponse
		err  error
	)

	n.CostkQuery(func(qc costktypes.QueryClient) {
		resp, err = qc.CurrentRewards(context.Background(), &costktypes.QueryCurrentRewardsRequest{})
		require.NoError(n.T(), err)
	})

	return resp
}

func (n *Node) QueryCostkParams() *costktypes.Params {
	var (
		resp *costktypes.QueryParamsResponse
		err  error
	)

	n.CostkQuery(func(qc costktypes.QueryClient) {
		resp, err = qc.Params(context.Background(), &costktypes.QueryParamsRequest{})
		require.NoError(n.T(), err)
	})

	return &resp.Params
}

func (n *Node) QueryTallyResult(propID uint64) *govtypes.TallyResult {
	var (
		resp *govtypes.QueryTallyResultResponse
		err  error
	)

	n.GovQuery(func(govClient govtypes.QueryClient) {
		resp, err = govClient.TallyResult(context.Background(), &govtypes.QueryTallyResultRequest{
			ProposalId: propID,
		})
		require.NoError(n.T(), err)
	})

	return resp.Tally
}

func (n *Node) QueryAppliedPlan(planName string) int64 {
	var (
		resp *upgradetypes.QueryAppliedPlanResponse
		err  error
	)

	n.UpgradeQuery(func(upgradeClient upgradetypes.QueryClient) {
		resp, err = upgradeClient.AppliedPlan(context.Background(), &upgradetypes.QueryAppliedPlanRequest{
			Name: planName,
		})
		require.NoError(n.T(), err)
	})

	return resp.Height
}

func (n *Node) QueryCostkRwdTrckCli(addr sdk.AccAddress) *costktypes.QueryCostakerRewardsTrackerResponse {
	cmd := []string{"babylond", "query", "costaking", "costaker-rewards-tracker", addr.String(), "--output=json", "--node", n.GetRpcEndpoint()}
	outBuf, _, err := n.Tm.ContainerManager.ExecCmd(n.T(), n.Container.Name, cmd, "")
	require.NoError(n.T(), err)

	resp := &costktypes.QueryCostakerRewardsTrackerResponse{}
	err = util.Cdc.UnmarshalJSON(outBuf.Bytes(), resp)
	require.NoError(n.T(), err)

	return resp
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
