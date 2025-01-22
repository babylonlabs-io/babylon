// This file is derived from the Cosmos Relayer repository (https://github.com/cosmos/relayer),
// originally licensed under the Apache License, Version 2.0.

package babylonclient

import (
	"context"
	"github.com/cometbft/cometbft/libs/bytes"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"
)

type RPCClient struct {
	c rpcclient.Client
}

func NewRPCClient(c rpcclient.Client) RPCClient {
	return RPCClient{c: c}
}

func (r RPCClient) ABCIInfo(ctx context.Context) (*coretypes.ResultABCIInfo, error) {
	return r.c.ABCIInfo(ctx)
}

func (r RPCClient) ABCIQuery(
	ctx context.Context,
	path string,
	data bytes.HexBytes,
) (*coretypes.ResultABCIQuery, error) {
	return r.c.ABCIQuery(ctx, path, (data))
}

func (r RPCClient) ABCIQueryWithOptions(
	ctx context.Context,
	path string,
	data bytes.HexBytes,
	opts rpcclient.ABCIQueryOptions,
) (*coretypes.ResultABCIQuery, error) {
	return r.c.ABCIQueryWithOptions(ctx, path, data, opts)
}

func (r RPCClient) BroadcastTxCommit(ctx context.Context, tx tmtypes.Tx) (*coretypes.ResultBroadcastTxCommit, error) {
	return r.c.BroadcastTxCommit(ctx, tx)
}

func (r RPCClient) BroadcastTxAsync(ctx context.Context, tx tmtypes.Tx) (*coretypes.ResultBroadcastTx, error) {
	return r.c.BroadcastTxAsync(ctx, tx)
}

func (r RPCClient) BroadcastTxSync(ctx context.Context, tx tmtypes.Tx) (*coretypes.ResultBroadcastTx, error) {
	return r.c.BroadcastTxSync(ctx, tx)
}

func (r RPCClient) Validators(
	ctx context.Context,
	height *int64,
	page, perPage *int,
) (*coretypes.ResultValidators, error) {
	return r.c.Validators(ctx, height, page, perPage)
}

func (r RPCClient) Status(ctx context.Context) (*coretypes.ResultStatus, error) {
	return r.c.Status(ctx)
}

func (r RPCClient) Block(ctx context.Context, height *int64) (*coretypes.ResultBlock, error) {
	return r.c.Block(ctx, height)
}

func (r RPCClient) BlockByHash(ctx context.Context, hash []byte) (*coretypes.ResultBlock, error) {
	return r.c.BlockByHash(ctx, hash)
}

func (r RPCClient) BlockResults(ctx context.Context, height *int64) (*coretypes.ResultBlockResults, error) {
	return r.c.BlockResults(ctx, height)
}

func (r RPCClient) BlockchainInfo(
	ctx context.Context,
	minHeight, maxHeight int64,
) (*coretypes.ResultBlockchainInfo, error) {
	return r.c.BlockchainInfo(ctx, minHeight, maxHeight)
}

func (r RPCClient) Commit(ctx context.Context, height *int64) (*coretypes.ResultCommit, error) {
	return r.c.Commit(ctx, height)
}

func (r RPCClient) Tx(ctx context.Context, hash []byte, prove bool) (*coretypes.ResultTx, error) {
	return r.c.Tx(ctx, hash, prove)
}

func (r RPCClient) TxSearch(
	ctx context.Context,
	query string,
	prove bool,
	page, perPage *int,
	orderBy string,
) (*coretypes.ResultTxSearch, error) {
	return r.c.TxSearch(ctx, query, prove, page, perPage, orderBy)
}

func (r RPCClient) BlockSearch(
	ctx context.Context,
	query string,
	page, perPage *int,
	orderBy string,
) (*coretypes.ResultBlockSearch, error) {
	return r.c.BlockSearch(ctx, query, page, perPage, orderBy)
}
