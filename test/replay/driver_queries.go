package replay

import (
	goMath "math"
	"testing"

	btckckpttypes "github.com/babylonlabs-io/babylon/v3/x/btccheckpoint/types"
	"github.com/btcsuite/btcd/wire"
	govk "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govv1types "github.com/cosmos/cosmos-sdk/x/gov/types/v1"

	ftypes "github.com/babylonlabs-io/babylon/v3/x/finality/types"

	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	ckpttypes "github.com/babylonlabs-io/babylon/v3/x/checkpointing/types"
	et "github.com/babylonlabs-io/babylon/v3/x/epoching/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/require"
)

func (d *BabylonAppDriver) getDelegationWithStatus(t *testing.T, status bstypes.BTCDelegationStatus) []*bstypes.BTCDelegationResponse {
	pagination := &query.PageRequest{}
	pagination.Limit = goMath.MaxUint32

	ctx, err := d.App.CreateQueryContext(0, false)
	require.NoError(t, err)
	delegations, err := d.App.BTCStakingKeeper.BTCDelegations(ctx, &bstypes.QueryBTCDelegationsRequest{
		Status:     status,
		Pagination: pagination,
	})
	require.NoError(t, err)
	return delegations.BtcDelegations
}

func (d *BabylonAppDriver) GetAllBTCDelegations(t *testing.T) []*bstypes.BTCDelegationResponse {
	return d.getDelegationWithStatus(t, bstypes.BTCDelegationStatus_ANY)
}

func (d *BabylonAppDriver) GetVerifiedBTCDelegations(t *testing.T) []*bstypes.BTCDelegationResponse {
	return d.getDelegationWithStatus(t, bstypes.BTCDelegationStatus_VERIFIED)
}

func (d *BabylonAppDriver) GetActiveBTCDelegations(t *testing.T) []*bstypes.BTCDelegationResponse {
	return d.getDelegationWithStatus(t, bstypes.BTCDelegationStatus_ACTIVE)
}

func (d *BabylonAppDriver) GetPendingBTCDelegations(t *testing.T) []*bstypes.BTCDelegationResponse {
	return d.getDelegationWithStatus(t, bstypes.BTCDelegationStatus_PENDING)
}

func (d *BabylonAppDriver) GetUnbondedBTCDelegations(t *testing.T) []*bstypes.BTCDelegationResponse {
	return d.getDelegationWithStatus(t, bstypes.BTCDelegationStatus_UNBONDED)
}

func (d *BabylonAppDriver) GetBTCDelegation(t *testing.T, stakingTxHex string) *bstypes.BTCDelegationResponse {
	ctx, err := d.App.CreateQueryContext(0, false)
	require.NoError(t, err)
	res, err := d.App.BTCStakingKeeper.BTCDelegation(ctx, &bstypes.QueryBTCDelegationRequest{
		StakingTxHashHex: stakingTxHex,
	})
	require.NoError(t, err)
	return res.BtcDelegation
}

func (d *BabylonAppDriver) GetBTCStakingParams(t *testing.T) *bstypes.Params {
	params := d.App.BTCStakingKeeper.GetParams(d.GetContextForLastFinalizedBlock())
	return &params
}

func (d *BabylonAppDriver) GetEpochingParams() et.Params {
	return d.App.EpochingKeeper.GetParams(d.GetContextForLastFinalizedBlock())
}

func (d *BabylonAppDriver) GetEpoch() *et.Epoch {
	return d.App.EpochingKeeper.GetEpoch(d.GetContextForLastFinalizedBlock())
}

func (d *BabylonAppDriver) GetCheckpoint(
	t *testing.T,
	epochNumber uint64,
) *ckpttypes.RawCheckpointWithMeta {
	checkpoint, err := d.App.CheckpointingKeeper.GetRawCheckpoint(d.GetContextForLastFinalizedBlock(), epochNumber)
	require.NoError(t, err)
	return checkpoint
}

func (d *BabylonAppDriver) GetLastFinalizedEpoch() uint64 {
	return d.App.CheckpointingKeeper.GetLastFinalizedEpoch(d.GetContextForLastFinalizedBlock())
}

func (d *BabylonAppDriver) GetActiveFpsAtHeight(t *testing.T, height uint64) []*ftypes.ActiveFinalityProvidersAtHeightResponse {
	res, err := d.App.FinalityKeeper.ActiveFinalityProvidersAtHeight(
		d.GetContextForLastFinalizedBlock(),
		&ftypes.QueryActiveFinalityProvidersAtHeightRequest{
			Height:     height,
			Pagination: &query.PageRequest{},
		},
	)
	require.NoError(t, err)
	return res.FinalityProviders
}

func (d *BabylonAppDriver) GetVotingPowerDistCache(height uint64) *ftypes.VotingPowerDistCache {
	return d.App.FinalityKeeper.GetVotingPowerDistCache(d.Ctx(), height)
}

func (d *BabylonAppDriver) GovProposals() []*govv1types.Proposal {
	resp, err := d.GovQuerySvr().Proposals(d.Ctx(), &govv1types.QueryProposalsRequest{
		ProposalStatus: govv1types.ProposalStatus_PROPOSAL_STATUS_VOTING_PERIOD,
	})
	require.NoError(d.t, err)
	return resp.Proposals
}

func (d *BabylonAppDriver) GovProposal(propId uint64) *govv1types.Proposal {
	resp, err := d.GovQuerySvr().Proposal(d.Ctx(), &govv1types.QueryProposalRequest{
		ProposalId: propId,
	})
	require.NoError(d.t, err)
	return resp.Proposal
}

func (d *BabylonAppDriver) GovQuerySvr() govv1types.QueryServer {
	return govk.NewQueryServer(&d.App.GovKeeper)
}

func (d *BabylonAppDriver) GetAllFps(t *testing.T) []*bstypes.FinalityProviderResponse {
	res, err := d.App.BTCStakingKeeper.FinalityProviders(
		d.GetContextForLastFinalizedBlock(),
		&bstypes.QueryFinalityProvidersRequest{},
	)
	require.NoError(t, err)
	return res.FinalityProviders
}

func (d *BabylonAppDriver) GetActiveFpsAtCurrentHeight(t *testing.T) []*ftypes.ActiveFinalityProvidersAtHeightResponse {
	return d.GetActiveFpsAtHeight(t, d.GetLastFinalizedBlock().Height)
}

func (d *BabylonAppDriver) GetFp(fpBTCPK []byte) *bstypes.FinalityProvider {
	fp, err := d.App.BTCStakingKeeper.GetFinalityProvider(d.GetContextForLastFinalizedBlock(), fpBTCPK)
	require.NoError(d.t, err)
	return fp
}

func (d *BabylonAppDriver) GetActivationHeight(t *testing.T) uint64 {
	res, err := d.App.FinalityKeeper.ActivatedHeight(
		d.GetContextForLastFinalizedBlock(),
		&ftypes.QueryActivatedHeightRequest{},
	)
	require.NoError(t, err)
	return res.Height
}

func (d *BabylonAppDriver) GetIndexedBlock(height uint64) *ftypes.IndexedBlock {
	res, err := d.App.FinalityKeeper.Block(
		d.GetContextForLastFinalizedBlock(),
		&ftypes.QueryBlockRequest{Height: height},
	)
	require.NoError(d.t, err)
	return res.Block
}

func (d *BabylonAppDriver) GetBTCCkptParams(t *testing.T) btckckpttypes.Params {
	return d.App.BtcCheckpointKeeper.GetParams(d.GetContextForLastFinalizedBlock())
}

func (d *BabylonAppDriver) GetBTCLCTip() (*wire.BlockHeader, uint32) {
	tipInfo := d.App.BTCLightClientKeeper.GetTipInfo(d.GetContextForLastFinalizedBlock())
	return tipInfo.Header.ToBlockHeader(), tipInfo.Height
}
