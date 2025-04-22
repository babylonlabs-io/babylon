package replay

import (
	goMath "math"
	"testing"

	btckckpttypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	"github.com/btcsuite/btcd/wire"

	ftypes "github.com/babylonlabs-io/babylon/x/finality/types"

	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	ckpttypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
	et "github.com/babylonlabs-io/babylon/x/epoching/types"
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
