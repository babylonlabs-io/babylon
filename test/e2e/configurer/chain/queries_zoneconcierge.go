package chain

import (
	"fmt"
	"net/url"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v3/test/e2e/util"
	zoneconciergetype "github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
)

// QueryZoneConciergeParams retrieves the current parameters for the ZoneConcierge module
func (n *NodeConfig) QueryZoneConciergeParams() *zoneconciergetype.Params {
	bz, err := n.QueryGRPCGateway("/babylon/zoneconcierge/v1/params", url.Values{})
	require.NoError(n.t, err)

	var resp zoneconciergetype.QueryParamsResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return &resp.Params
}

// QueryFinalizedBSNsInfo retrieves finalized BSN (Babylon Secured Network) information
// for the specified consumer IDs, optionally including proofs if prove is true
func (n *NodeConfig) QueryFinalizedBSNsInfo(consumerIds []string, prove bool) *zoneconciergetype.QueryFinalizedBSNsInfoResponse {
	params := url.Values{}
	for _, id := range consumerIds {
		params.Add("consumer_ids", id)
	}
	if prove {
		params.Set("prove", "true")
	}

	bz, err := n.QueryGRPCGateway("/babylon/zoneconcierge/v1/finalized_bsns_info", params)
	require.NoError(n.t, err)

	var resp zoneconciergetype.QueryFinalizedBSNsInfoResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return &resp
}

// QueryLatestEpochHeader retrieves the latest epoch header for the specified consumer ID
func (n *NodeConfig) QueryLatestEpochHeader(consumerID string) *zoneconciergetype.QueryLatestEpochHeaderResponse {
	path := fmt.Sprintf("/babylon/zoneconcierge/v1/latest_epoch_header/%s", consumerID)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp zoneconciergetype.QueryLatestEpochHeaderResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return &resp
}

// QueryBSNLastSentSegment retrieves the last sent segment information for the specified consumer ID
func (n *NodeConfig) QueryBSNLastSentSegment(consumerID string) *zoneconciergetype.QueryBSNLastSentSegmentResponse {
	path := fmt.Sprintf("/babylon/zoneconcierge/v1/bsn_last_sent_segment/%s", consumerID)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp zoneconciergetype.QueryBSNLastSentSegmentResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return &resp
}

// QueryGetSealedEpochProof retrieves the sealed epoch proof for the specified epoch number
func (n *NodeConfig) QueryGetSealedEpochProof(epochNum uint64) *zoneconciergetype.QueryGetSealedEpochProofResponse {
	path := fmt.Sprintf("/babylon/zoneconcierge/v1/sealed_epoch_proof/%d", epochNum)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp zoneconciergetype.QueryGetSealedEpochProofResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return &resp
}
