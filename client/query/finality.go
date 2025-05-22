package query

import (
	"context"

	"github.com/cosmos/cosmos-sdk/client"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"

	finalitytypes "github.com/babylonlabs-io/babylon/v3/x/finality/types"
)

// QueryFinality queries the Finality module of the Babylon node according to the given function
func (c *QueryClient) QueryFinality(f func(ctx context.Context, queryClient finalitytypes.QueryClient) error) error {
	ctx, cancel := c.getQueryContext()
	defer cancel()

	clientCtx := client.Context{Client: c.RPCClient}
	queryClient := finalitytypes.NewQueryClient(clientCtx)

	return f(ctx, queryClient)
}

// ActiveFinalityProvidersAtHeight queries the BTCStaking module for all finality providers
// with non-zero voting power at a given height
func (c *QueryClient) ActiveFinalityProvidersAtHeight(height uint64, pagination *sdkquerytypes.PageRequest) (*finalitytypes.QueryActiveFinalityProvidersAtHeightResponse, error) {
	var resp *finalitytypes.QueryActiveFinalityProvidersAtHeightResponse
	err := c.QueryFinality(func(ctx context.Context, queryClient finalitytypes.QueryClient) error {
		var err error
		req := &finalitytypes.QueryActiveFinalityProvidersAtHeightRequest{
			Height:     height,
			Pagination: pagination,
		}
		resp, err = queryClient.ActiveFinalityProvidersAtHeight(ctx, req)
		return err
	})

	return resp, err
}

// FinalityProviderPowerAtHeight queries the BTCStaking module for the power of a finality provider at a given height
func (c *QueryClient) FinalityProviderPowerAtHeight(fpBtcPkHex string, height uint64) (*finalitytypes.QueryFinalityProviderPowerAtHeightResponse, error) {
	var resp *finalitytypes.QueryFinalityProviderPowerAtHeightResponse
	err := c.QueryFinality(func(ctx context.Context, queryClient finalitytypes.QueryClient) error {
		var err error
		req := &finalitytypes.QueryFinalityProviderPowerAtHeightRequest{
			FpBtcPkHex: fpBtcPkHex,
			Height:     height,
		}
		resp, err = queryClient.FinalityProviderPowerAtHeight(ctx, req)
		return err
	})

	return resp, err
}

func (c *QueryClient) ActivatedHeight() (*finalitytypes.QueryActivatedHeightResponse, error) {
	var resp *finalitytypes.QueryActivatedHeightResponse
	err := c.QueryFinality(func(ctx context.Context, queryClient finalitytypes.QueryClient) error {
		var err error
		req := &finalitytypes.QueryActivatedHeightRequest{}
		resp, err = queryClient.ActivatedHeight(ctx, req)
		return err
	})

	return resp, err
}

// FinalityParams queries the finality module parameters
func (c *QueryClient) FinalityParams() (*finalitytypes.QueryParamsResponse, error) {
	var resp *finalitytypes.QueryParamsResponse
	err := c.QueryFinality(func(ctx context.Context, queryClient finalitytypes.QueryClient) error {
		var err error
		req := &finalitytypes.QueryParamsRequest{}
		resp, err = queryClient.Params(ctx, req)
		return err
	})

	return resp, err
}

// VotesAtHeight queries the Finality module to get signature set at a given babylon block height
func (c *QueryClient) VotesAtHeight(height uint64) (*finalitytypes.QueryVotesAtHeightResponse, error) {
	var resp *finalitytypes.QueryVotesAtHeightResponse
	err := c.QueryFinality(func(ctx context.Context, queryClient finalitytypes.QueryClient) error {
		var err error
		req := &finalitytypes.QueryVotesAtHeightRequest{
			Height: height,
		}
		resp, err = queryClient.VotesAtHeight(ctx, req)
		return err
	})

	return resp, err
}
func (c *QueryClient) ListPubRandCommit(fpBtcPkHex string, pagination *sdkquerytypes.PageRequest) (*finalitytypes.QueryListPubRandCommitResponse, error) {
	var resp *finalitytypes.QueryListPubRandCommitResponse
	err := c.QueryFinality(func(ctx context.Context, queryClient finalitytypes.QueryClient) error {
		var err error
		req := &finalitytypes.QueryListPubRandCommitRequest{
			FpBtcPkHex: fpBtcPkHex,
			Pagination: pagination,
		}
		resp, err = queryClient.ListPubRandCommit(ctx, req)
		return err
	})

	return resp, err
}

// ListBlocks queries the Finality module to get blocks with a given status.
func (c *QueryClient) ListBlocks(status finalitytypes.QueriedBlockStatus, pagination *sdkquerytypes.PageRequest) (*finalitytypes.QueryListBlocksResponse, error) {
	var resp *finalitytypes.QueryListBlocksResponse
	err := c.QueryFinality(func(ctx context.Context, queryClient finalitytypes.QueryClient) error {
		var err error
		req := &finalitytypes.QueryListBlocksRequest{
			Status:     status,
			Pagination: pagination,
		}
		resp, err = queryClient.ListBlocks(ctx, req)
		return err
	})

	return resp, err
}

// Block queries a block at a given height.
func (c *QueryClient) Block(height uint64) (*finalitytypes.QueryBlockResponse, error) {
	var resp *finalitytypes.QueryBlockResponse
	err := c.QueryFinality(func(ctx context.Context, queryClient finalitytypes.QueryClient) error {
		var err error
		req := &finalitytypes.QueryBlockRequest{
			Height: height,
		}
		resp, err = queryClient.Block(ctx, req)
		return err
	})

	return resp, err
}

// ListEvidences queries the Finality module to get evidences after a given height.
func (c *QueryClient) ListEvidences(startHeight uint64, pagination *sdkquerytypes.PageRequest) (*finalitytypes.QueryListEvidencesResponse, error) {
	var resp *finalitytypes.QueryListEvidencesResponse
	err := c.QueryFinality(func(ctx context.Context, queryClient finalitytypes.QueryClient) error {
		var err error
		req := &finalitytypes.QueryListEvidencesRequest{
			StartHeight: startHeight,
			Pagination:  pagination,
		}
		resp, err = queryClient.ListEvidences(ctx, req)
		return err
	})

	return resp, err
}
