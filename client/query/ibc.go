package query

import (
	"context"

	"github.com/cosmos/cosmos-sdk/client"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	connectiontypes "github.com/cosmos/ibc-go/v10/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
)

func (c *QueryClient) QueryIBCChannel(f func(ctx context.Context, queryClient channeltypes.QueryClient) error) error {
	ctx, cancel := c.getQueryContext()
	defer cancel()

	clientCtx := client.Context{Client: c.RPCClient}
	queryClient := channeltypes.NewQueryClient(clientCtx)

	return f(ctx, queryClient)
}

func (c *QueryClient) QueryIBCClient(f func(ctx context.Context, queryClient clienttypes.QueryClient) error) error {
	ctx, cancel := c.getQueryContext()
	defer cancel()

	clientCtx := client.Context{Client: c.RPCClient}
	queryClient := clienttypes.NewQueryClient(clientCtx)

	return f(ctx, queryClient)
}

func (c *QueryClient) QueryIBCConnection(f func(ctx context.Context, queryClient connectiontypes.QueryClient) error) error {
	ctx, cancel := c.getQueryContext()
	defer cancel()

	clientCtx := client.Context{Client: c.RPCClient}
	queryClient := connectiontypes.NewQueryClient(clientCtx)

	return f(ctx, queryClient)
}

// IBCChannels queries all IBC channels
func (c *QueryClient) IBCChannels() (*channeltypes.QueryChannelsResponse, error) {
	var resp *channeltypes.QueryChannelsResponse
	err := c.QueryIBCChannel(func(ctx context.Context, queryClient channeltypes.QueryClient) error {
		var err error
		req := &channeltypes.QueryChannelsRequest{}
		resp, err = queryClient.Channels(ctx, req)
		return err
	})

	return resp, err
}

// IBCClientStates queries all IBC client states
func (c *QueryClient) IBCClientStates() (*clienttypes.QueryClientStatesResponse, error) {
	var resp *clienttypes.QueryClientStatesResponse
	err := c.QueryIBCClient(func(ctx context.Context, queryClient clienttypes.QueryClient) error {
		var err error
		req := &clienttypes.QueryClientStatesRequest{}
		resp, err = queryClient.ClientStates(ctx, req)
		return err
	})

	return resp, err
}

// IBCConnections queries all IBC connections
func (c *QueryClient) IBCConnections() (*connectiontypes.QueryConnectionsResponse, error) {
	var resp *connectiontypes.QueryConnectionsResponse
	err := c.QueryIBCConnection(func(ctx context.Context, queryClient connectiontypes.QueryClient) error {
		var err error
		req := &connectiontypes.QueryConnectionsRequest{}
		resp, err = queryClient.Connections(ctx, req)
		return err
	})

	return resp, err
}
