package chain

import (
	"encoding/hex"
	"fmt"
	"net/url"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

func (n *NodeConfig) QueryBTCStakingParams() *bstypes.Params {
	bz, err := n.QueryGRPCGateway("/babylon/btcstaking/v1/params", url.Values{})
	require.NoError(n.t, err)

	var resp bstypes.QueryParamsResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return &resp.Params
}

func (n *NodeConfig) QueryBTCStakingParamsByVersion(version uint32) *bstypes.Params {
	path := fmt.Sprintf("/babylon/btcstaking/v1/params/%d", version)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp bstypes.QueryParamsByVersionResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return &resp.Params
}

func (n *NodeConfig) QueryFinalityParams() *ftypes.Params {
	bz, err := n.QueryGRPCGateway("/babylon/finality/v1/params", url.Values{})
	require.NoError(n.t, err)

	var resp ftypes.QueryParamsResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return &resp.Params
}

func (n *NodeConfig) QueryFinalityProviders() []*bstypes.FinalityProviderResponse {
	bz, err := n.QueryGRPCGateway("/babylon/btcstaking/v1/finality_providers", url.Values{})
	require.NoError(n.t, err)

	var resp bstypes.QueryFinalityProvidersResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp.FinalityProviders
}

func (n *NodeConfig) QueryFinalityProvider(btcPkHex string) *bstypes.FinalityProviderResponse {
	path := fmt.Sprintf("/babylon/btcstaking/v1/finality_providers/%s/finality_provider", btcPkHex)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp bstypes.QueryFinalityProviderResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp.FinalityProvider
}

func (n *NodeConfig) QueryActiveFinalityProvidersAtHeight(height uint64) []*ftypes.ActiveFinalityProvidersAtHeightResponse {
	path := fmt.Sprintf("/babylon/finality/v1/finality_providers/%d", height)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp ftypes.QueryActiveFinalityProvidersAtHeightResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp.FinalityProviders
}

func (n *NodeConfig) QueryFinalityProviderDelegations(fpBTCPK string) []*bstypes.BTCDelegatorDelegationsResponse {
	path := fmt.Sprintf("/babylon/btcstaking/v1/finality_providers/%s/delegations", fpBTCPK)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp bstypes.QueryFinalityProviderDelegationsResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp.BtcDelegatorDelegations
}

func (n *NodeConfig) QueryBtcDelegation(stakingTxHash string) *bstypes.QueryBTCDelegationResponse {
	path := fmt.Sprintf("/babylon/btcstaking/v1/btc_delegation/%s", stakingTxHash)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp bstypes.QueryBTCDelegationResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return &resp
}

func (n *NodeConfig) QueryBtcDelegations(status bstypes.BTCDelegationStatus) *bstypes.QueryBTCDelegationsResponse {
	path := fmt.Sprintf("/babylon/btcstaking/v1/btc_delegations/%d", status)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp bstypes.QueryBTCDelegationsResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return &resp
}

func (n *NodeConfig) QueryUnbondedDelegations() []*bstypes.BTCDelegationResponse {
	return n.QueryBtcDelegations(bstypes.BTCDelegationStatus_UNBONDED).BtcDelegations
}

func (n *NodeConfig) QueryVerifiedDelegations() []*bstypes.BTCDelegationResponse {
	return n.QueryBtcDelegations(bstypes.BTCDelegationStatus_VERIFIED).BtcDelegations
}

func (n *NodeConfig) QueryActiveDelegations() []*bstypes.BTCDelegationResponse {
	return n.QueryBtcDelegations(bstypes.BTCDelegationStatus_ACTIVE).BtcDelegations
}

func (n *NodeConfig) QueryActivatedHeight() (uint64, error) {
	bz, err := n.QueryGRPCGateway("/babylon/finality/v1/activated_height", url.Values{})
	if err != nil {
		return 0, err
	}

	var resp ftypes.QueryActivatedHeightResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	if err != nil {
		return 0, err
	}

	return resp.Height, nil
}

// TODO: pagination support
func (n *NodeConfig) QueryListPublicRandomness(fpBTCPK *bbn.BIP340PubKey) map[uint64]*bbn.SchnorrPubRand {
	path := fmt.Sprintf("/babylon/finality/v1/finality_providers/%s/public_randomness_list", fpBTCPK.MarshalHex())
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp ftypes.QueryListPublicRandomnessResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp.PubRandMap
}

// TODO: pagination support
func (n *NodeConfig) QueryListPubRandCommit(fpBTCPK *bbn.BIP340PubKey) map[uint64]*ftypes.PubRandCommitResponse {
	path := fmt.Sprintf("/babylon/finality/v1/finality_providers/%s/pub_rand_commit_list", fpBTCPK.MarshalHex())
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp ftypes.QueryListPubRandCommitResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp.PubRandCommitMap
}

func (n *NodeConfig) QueryVotesAtHeight(height uint64) []bbn.BIP340PubKey {
	path := fmt.Sprintf("/babylon/finality/v1/votes/%d", height)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp ftypes.QueryVotesAtHeightResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp.BtcPks
}

// TODO: pagination support
func (n *NodeConfig) QueryListBlocks(status ftypes.QueriedBlockStatus) []*ftypes.IndexedBlock {
	values := url.Values{}
	values.Set("status", fmt.Sprintf("%d", status))
	bz, err := n.QueryGRPCGateway("/babylon/finality/v1/blocks", values)
	require.NoError(n.t, err)

	var resp ftypes.QueryListBlocksResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp.Blocks
}

func (n *NodeConfig) QueryIndexedBlock(height uint64) *ftypes.IndexedBlock {
	path := fmt.Sprintf("/babylon/finality/v1/blocks/%d", height)
	bz, err := n.QueryGRPCGateway(path, url.Values{})
	require.NoError(n.t, err)

	var resp ftypes.QueryBlockResponse
	err = util.Cdc.UnmarshalJSON(bz, &resp)
	require.NoError(n.t, err)

	return resp.Block
}

func (n *NodeConfig) QueryFinalityProvidersDelegations(fpsBTCPK ...string) []*bstypes.BTCDelegationResponse {
	pendingDelsResp := make([]*bstypes.BTCDelegationResponse, 0)
	for _, fpBTCPK := range fpsBTCPK {
		fpDelsResp := n.QueryFinalityProviderDelegations(fpBTCPK)
		for _, fpDel := range fpDelsResp {
			pendingDelsResp = append(pendingDelsResp, fpDel.Dels...)
		}
	}
	return pendingDelsResp
}

// ParseRespBTCDelToBTCDel parses an BTC delegation response to BTC Delegation
func ParseRespBTCDelToBTCDel(resp *bstypes.BTCDelegationResponse) (btcDel *bstypes.BTCDelegation, err error) {
	stakingTx, err := hex.DecodeString(resp.StakingTxHex)
	if err != nil {
		return nil, err
	}

	delSig, err := bbn.NewBIP340SignatureFromHex(resp.DelegatorSlashSigHex)
	if err != nil {
		return nil, err
	}

	slashingTx, err := bstypes.NewBTCSlashingTxFromHex(resp.SlashingTxHex)
	if err != nil {
		return nil, err
	}

	btcDel = &bstypes.BTCDelegation{
		StakerAddr:       resp.StakerAddr,
		BtcPk:            resp.BtcPk,
		FpBtcPkList:      resp.FpBtcPkList,
		StartHeight:      resp.StartHeight,
		StakingTime:      resp.StakingTime,
		EndHeight:        resp.EndHeight,
		TotalSat:         resp.TotalSat,
		StakingTx:        stakingTx,
		DelegatorSig:     delSig,
		StakingOutputIdx: resp.StakingOutputIdx,
		CovenantSigs:     resp.CovenantSigs,
		UnbondingTime:    resp.UnbondingTime,
		SlashingTx:       slashingTx,
	}

	if resp.UndelegationResponse != nil {
		ud := resp.UndelegationResponse
		unbondTx, err := hex.DecodeString(ud.UnbondingTxHex)
		if err != nil {
			return nil, err
		}

		slashTx, err := bstypes.NewBTCSlashingTxFromHex(ud.SlashingTxHex)
		if err != nil {
			return nil, err
		}

		delSlashingSig, err := bbn.NewBIP340SignatureFromHex(ud.DelegatorSlashingSigHex)
		if err != nil {
			return nil, err
		}

		btcDel.BtcUndelegation = &bstypes.BTCUndelegation{
			UnbondingTx:              unbondTx,
			CovenantUnbondingSigList: ud.CovenantUnbondingSigList,
			CovenantSlashingSigs:     ud.CovenantSlashingSigs,
			SlashingTx:               slashTx,
			DelegatorSlashingSig:     delSlashingSig,
		}

		if ud.DelegatorUnbondingInfoResponse != nil {
			var spendStakeTx []byte = make([]byte, 0)
			if ud.DelegatorUnbondingInfoResponse.SpendStakeTxHex != "" {
				spendStakeTx, err = hex.DecodeString(ud.DelegatorUnbondingInfoResponse.SpendStakeTxHex)
				if err != nil {
					return nil, err
				}
			}

			btcDel.BtcUndelegation.DelegatorUnbondingInfo = &bstypes.DelegatorUnbondingInfo{
				SpendStakeTx: spendStakeTx,
			}
		}
	}

	return btcDel, nil
}

// ParseRespsBTCDelToBTCDel parses an BTC delegation response to BTC Delegation
func ParseRespsBTCDelToBTCDel(resp *bstypes.BTCDelegatorDelegationsResponse) (btcDels *bstypes.BTCDelegatorDelegations, err error) {
	if resp == nil {
		return nil, nil
	}
	btcDels = &bstypes.BTCDelegatorDelegations{
		Dels: make([]*bstypes.BTCDelegation, len(resp.Dels)),
	}

	for i, delResp := range resp.Dels {
		del, err := ParseRespBTCDelToBTCDel(delResp)
		if err != nil {
			return nil, err
		}
		btcDels.Dels[i] = del
	}
	return btcDels, nil
}
