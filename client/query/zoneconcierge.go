package query

import (
	"context"

	zctypes "github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
	"github.com/cosmos/cosmos-sdk/client"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
)

// QueryZoneConcierge queries the ZoneConcierge module of the Babylon node
// according to the given function
func (c *QueryClient) QueryZoneConcierge(f func(ctx context.Context, queryClient zctypes.QueryClient) error) error {
	ctx, cancel := c.getQueryContext()
	defer cancel()

	clientCtx := client.Context{Client: c.RPCClient}
	queryClient := zctypes.NewQueryClient(clientCtx)

	return f(ctx, queryClient)
}

// FinalizedConnectedChainsInfo queries the zoneconcierge module to get the finalization information for a connected chain
func (c *QueryClient) FinalizedConnectedChainsInfo(consumerIds []string) (*zctypes.QueryFinalizedChainsInfoResponse, error) {
	var resp *zctypes.QueryFinalizedChainsInfoResponse
	err := c.QueryZoneConcierge(func(ctx context.Context, queryClient zctypes.QueryClient) error {
		var err error
		req := &zctypes.QueryFinalizedChainsInfoRequest{
			ConsumerIds: consumerIds,
		}
		resp, err = queryClient.FinalizedChainsInfo(ctx, req)
		return err
	})

	return resp, err
}

// ConnectedChainsInfo queries the zoneconcierge module to get information for a connected chain
func (c *QueryClient) ConnectedChainsInfo(consumerIds []string) (*zctypes.QueryChainsInfoResponse, error) {
	var resp *zctypes.QueryChainsInfoResponse
	err := c.QueryZoneConcierge(func(ctx context.Context, queryClient zctypes.QueryClient) error {
		var err error
		req := &zctypes.QueryChainsInfoRequest{
			ConsumerIds: consumerIds,
		}
		resp, err = queryClient.ChainsInfo(ctx, req)
		return err
	})

	return resp, err
}

// ConnectedChainList queries the zoneconierge module for the chain IDs of the connected chains
func (c *QueryClient) ConnectedChainList() (*zctypes.QueryChainListResponse, error) {
	var resp *zctypes.QueryChainListResponse
	err := c.QueryZoneConcierge(func(ctx context.Context, queryClient zctypes.QueryClient) error {
		var err error
		req := &zctypes.QueryChainListRequest{}
		resp, err = queryClient.ChainList(ctx, req)
		return err
	})

	return resp, err
}

// ConnectedChainHeaders queries the zoneconcierge module for the headers of a connected chain
func (c *QueryClient) ConnectedChainHeaders(consumerID string, pagination *sdkquerytypes.PageRequest) (*zctypes.QueryListHeadersResponse, error) {
	var resp *zctypes.QueryListHeadersResponse
	err := c.QueryZoneConcierge(func(ctx context.Context, queryClient zctypes.QueryClient) error {
		var err error
		req := &zctypes.QueryListHeadersRequest{
			ConsumerId: consumerID,
			Pagination: pagination,
		}
		resp, err = queryClient.ListHeaders(ctx, req)
		return err
	})

	return resp, err
}

// ConnectedChainsEpochInfo queries the zoneconcierge module for the chain information of a connected chain at a particular epoch
func (c *QueryClient) ConnectedChainsEpochInfo(consumerIds []string, epochNum uint64) (*zctypes.QueryEpochChainsInfoResponse, error) {
	var resp *zctypes.QueryEpochChainsInfoResponse
	err := c.QueryZoneConcierge(func(ctx context.Context, queryClient zctypes.QueryClient) error {
		var err error
		req := &zctypes.QueryEpochChainsInfoRequest{
			ConsumerIds: consumerIds,
			EpochNum:    epochNum,
		}
		resp, err = queryClient.EpochChainsInfo(ctx, req)
		return err
	})

	return resp, err
}
