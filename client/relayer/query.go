package relayerclient

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/x/params/types/proposal"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"strconv"
	"strings"
	"time"
)

func (cc *CosmosProvider) queryParamsSubspaceTime(ctx context.Context, subspace string, key string) (time.Duration, error) {
	queryClient := proposal.NewQueryClient(cc)

	params := proposal.QueryParamsRequest{Subspace: subspace, Key: key}

	res, err := queryClient.Params(ctx, &params)

	if err != nil {
		return 0, fmt.Errorf("failed to make %s params request: %w", subspace, err)
	}

	if res.Param.Value == "" {
		return 0, fmt.Errorf("%s %s is empty", subspace, key)
	}

	unbondingValue, err := strconv.ParseUint(strings.ReplaceAll(res.Param.Value, `"`, ""), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse %s from %s param: %w", key, subspace, err)
	}

	return time.Duration(unbondingValue), nil
}

// QueryUnbondingPeriod returns the unbonding period of the chain
func (cc *CosmosProvider) QueryUnbondingPeriod(ctx context.Context) (time.Duration, error) {

	// Attempt ICS query
	consumerUnbondingPeriod, consumerErr := cc.queryParamsSubspaceTime(ctx, "ccvconsumer", "UnbondingPeriod")
	if consumerErr == nil {
		return consumerUnbondingPeriod, nil
	}

	//Attempt Staking query.
	unbondingPeriod, stakingParamsErr := cc.queryParamsSubspaceTime(ctx, "staking", "UnbondingTime")
	if stakingParamsErr == nil {
		return unbondingPeriod, nil
	}

	// Fallback
	req := stakingtypes.QueryParamsRequest{}
	queryClient := stakingtypes.NewQueryClient(cc)
	res, err := queryClient.Params(ctx, &req)
	if err == nil {
		return res.Params.UnbondingTime, nil

	}

	return 0,
		fmt.Errorf("failed to query unbonding period from ccvconsumer, staking & fallback : %w: %s : %s", consumerErr, stakingParamsErr.Error(), err.Error())
}

// QueryTx takes a transaction hash and returns the transaction
func (cc *CosmosProvider) QueryTx(ctx context.Context, hashHex string) (*RelayerTxResponse, error) {
	hash, err := hex.DecodeString(hashHex)
	if err != nil {
		return nil, err
	}

	resp, err := cc.RPCClient.Tx(ctx, hash, true)
	if err != nil {
		return nil, err
	}

	events := parseEventsFromResponseDeliverTx(resp.TxResult.Events)

	return &RelayerTxResponse{
		Height: resp.Height,
		TxHash: string(hash),
		Code:   resp.TxResult.Code,
		Data:   string(resp.TxResult.Data),
		Events: events,
	}, nil
}

// QueryTxs returns an array of transactions given a tag
func (cc *CosmosProvider) QueryTxs(ctx context.Context, page, limit int, events []string) ([]*RelayerTxResponse, error) {
	if len(events) == 0 {
		return nil, errors.New("must declare at least one event to search")
	}

	if page <= 0 {
		return nil, errors.New("page must greater than 0")
	}

	if limit <= 0 {
		return nil, errors.New("limit must greater than 0")
	}

	res, err := cc.RPCClient.TxSearch(ctx, strings.Join(events, " AND "), true, &page, &limit, "")
	if err != nil {
		return nil, err
	}

	// Currently, we only call QueryTxs() in two spots and in both of them we are expecting there to only be,
	// at most, one tx in the response. Because of this we don't want to initialize the slice with an initial size.
	var txResps []*RelayerTxResponse
	for _, tx := range res.Txs {
		relayerEvents := parseEventsFromResponseDeliverTx(tx.TxResult.Events)
		txResps = append(txResps, &RelayerTxResponse{
			Height: tx.Height,
			TxHash: string(tx.Hash),
			Code:   tx.TxResult.Code,
			Data:   string(tx.TxResult.Data),
			Events: relayerEvents,
		})
	}
	return txResps, nil
}

// parseEventsFromResponseDeliverTx parses the events from a ResponseDeliverTx and builds a slice
// of provider.RelayerEvent's.
func parseEventsFromResponseDeliverTx(events []abci.Event) []RelayerEvent {
	var rlyEvents []RelayerEvent

	for _, event := range events {
		attributes := make(map[string]string)
		for _, attribute := range event.Attributes {
			attributes[attribute.Key] = attribute.Value
		}

		rlyEvents = append(rlyEvents, RelayerEvent{
			EventType:  event.Type,
			Attributes: attributes,
		})
	}

	return rlyEvents
}
