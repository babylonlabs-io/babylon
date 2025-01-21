package relayerclient

import (
	"context"
	"fmt"
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
