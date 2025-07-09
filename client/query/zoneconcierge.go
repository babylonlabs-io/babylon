package query

import (
	"context"

	zctypes "github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
	"github.com/cosmos/cosmos-sdk/client"
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
