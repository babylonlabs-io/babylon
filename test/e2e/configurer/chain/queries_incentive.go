package chain

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
	incentivetypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	grpctypes "github.com/cosmos/cosmos-sdk/types/grpc"
)

func (n *NodeConfig) QueryBTCStakingGauge(height uint64) (*incentivetypes.BTCStakingGaugeResponse, error) {
	path := fmt.Sprintf("/babylon/incentive/btc_staking_gauge/%d", height)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	if err != nil {
		return nil, err
	}

	var resp incentivetypes.QueryBTCStakingGaugeResponse
	if err := util.Cdc.UnmarshalJSON(bz, &resp); err != nil {
		return nil, err
	}

	return resp.Gauge, nil
}

// QueryIncentiveParamsAtHeight returns the incentive module parameters at a specific block height
func (n *NodeConfig) QueryIncentiveParamsAtHeight(height uint64) (*incentivetypes.Params, error) {
	path := "babylon/incentive/params"

	var headers map[string]string
	if height > 0 {
		headers = map[string]string{
			grpctypes.GRPCBlockHeightHeader: strconv.FormatUint(height, 10),
		}
	}

	bz, err := n.QueryGRPCGatewayWithHeaders(path, url.Values{}, headers)
	if err != nil {
		return nil, err
	}

	var resp incentivetypes.QueryParamsResponse
	if err := util.Cdc.UnmarshalJSON(bz, &resp); err != nil {
		return nil, err
	}
	return &resp.Params, nil
}

func (n *NodeConfig) QueryRewardGauge(sAddr sdk.AccAddress) (map[string]*incentivetypes.RewardGaugesResponse, error) {
	path := fmt.Sprintf("/babylon/incentive/address/%s/reward_gauge", sAddr.String())
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	if err != nil {
		return nil, err
	}
	var resp incentivetypes.QueryRewardGaugesResponse
	if err := util.Cdc.UnmarshalJSON(bz, &resp); err != nil {
		return nil, err
	}

	return resp.RewardGauges, nil
}

func (n *NodeConfig) QueryBtcStkGauge(blkHeight uint64) (sdk.Coins, error) {
	path := fmt.Sprintf("/babylon/incentive/btc_staking_gauge/%d", blkHeight)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	if err != nil {
		return nil, err
	}
	var resp incentivetypes.QueryBTCStakingGaugeResponse
	if err := util.Cdc.UnmarshalJSON(bz, &resp); err != nil {
		return nil, err
	}

	return resp.Gauge.Coins, nil
}

func (n *NodeConfig) QueryBtcStkGaugeFromBlocks(blkHeightFrom, blkHeightTo uint64) (sdk.Coins, error) {
	total := sdk.NewCoins()
	for blkHeight := blkHeightFrom; blkHeight <= blkHeightTo; blkHeight++ {
		coinsInBlk, err := n.QueryBtcStkGauge(blkHeight)
		if err != nil {
			return nil, err
		}
		total = total.Add(coinsInBlk...)
	}
	return total, nil
}
