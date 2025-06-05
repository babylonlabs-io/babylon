package keeper_test

import (
	"encoding/hex"
	"math"
	"math/rand"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	"github.com/btcsuite/btcd/btcec/v2"
	dbm "github.com/cosmos/cosmos-db"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	v1 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v1"
	testnetdata "github.com/babylonlabs-io/babylon/v3/app/upgrades/v1/testnet"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v3/testutil/helper"
	testutilk "github.com/babylonlabs-io/babylon/v3/testutil/keeper"
	btclightclientt "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
)

func TestInitGenesisWithSetParams(t *testing.T) {
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())
	k, ctx := testutilk.BTCStakingKeeperWithStore(t, db, stateStore, nil, nil, nil, nil)

	err := k.InitGenesis(ctx, *types.DefaultGenesis())
	require.NoError(t, err)

	params, err := v1.LoadBtcStakingParamsFromData(testnetdata.BtcStakingParamsStr)
	require.NoError(t, err)

	for _, p := range params {
		err = k.SetParams(ctx, p)
		require.NoError(t, err)
	}
}

func TestInitGenesis(t *testing.T) {
	ctx, h, gs := setupTest(t)
	k := h.App.BTCStakingKeeper

	// params already exist when setting up the test suite
	// so we'll pass the params as nil in the InitGenesis
	// to avoid having an error when trying to set the same params
	gs.Params = nil

	err := k.InitGenesis(ctx, *gs)
	require.NoError(t, err)

	// restore the params for checking equality
	dp := types.DefaultParams()
	gs.Params = []*types.Params{&dp}

	exportedGs, err := k.ExportGenesis(ctx)
	require.NoError(t, err)

	types.SortData(exportedGs)
	types.SortData(gs)

	require.Equal(t, gs, exportedGs)
}

func TestExportGenesis(t *testing.T) {
	ctx, h, gs := setupTest(t)
	k, btclcK := h.App.BTCStakingKeeper, h.App.BTCLightClientKeeper

	require.NoError(t, k.SetLargestBtcReorg(ctx, *gs.LargestBtcReorg))
	fps, delegations, chainsHeight := gs.FinalityProviders, gs.BtcDelegations, gs.BlockHeightChains

	for i := range gs.FinalityProviders {
		// set finality
		h.AddFinalityProvider(fps[i])
		// on creating the finality providers, the commission UpdateTime
		// is set to be the current block time. To check equality afterwards,
		// we update the randomly generated fps (with UpdateTime = 0) to have UpdateTime = block time
		fps[i].CommissionInfo.UpdateTime = ctx.BlockHeader().Time

		// make delegations per fp so event indexes are the same as the
		// generated data in the setupTest func
		delegateToFP(h, delegations, fps[i].BtcPk.MustToBTCPK())
	}

	// index blocks heights
	for _, ch := range chainsHeight {
		btcHead := btclcK.GetTipInfo(ctx)
		btcHead.Height = ch.BlockHeightBtc
		btclcK.InsertHeaderInfos(ctx, []*btclightclientt.BTCHeaderInfo{
			btcHead,
		})

		header := ctx.HeaderInfo()
		header.Height = int64(ch.BlockHeightBbn)
		ctx = ctx.WithHeaderInfo(header)
		h.Ctx = ctx

		k.IndexBTCHeight(ctx)
	}

	exportedGs, err := k.ExportGenesis(ctx)
	h.NoError(err)

	// sort expected and exported data to have deterministic order in the results
	types.SortData(gs)
	types.SortData(exportedGs)

	require.Equal(t, gs, exportedGs)

	// TODO: vp dst cache
}

func setupTest(t *testing.T) (sdk.Context, *helper.Helper, *types.GenesisState) {
	r, h := rand.New(rand.NewSource(11)), helper.NewHelper(t)
	k, ctx := h.App.BTCStakingKeeper, h.Ctx
	numFps := 3

	fps := datagen.CreateNFinalityProviders(r, t, numFps)
	params := k.GetParams(ctx)

	chainsHeight := make([]*types.BlockHeightBbnToBtc, 0)
	// creates the first as it starts already with an chain height from the helper.
	chainsHeight = append(chainsHeight, &types.BlockHeightBbnToBtc{
		BlockHeightBbn: 1,
		BlockHeightBtc: 0,
	})
	btcDelegations := make([]*types.BTCDelegation, 0)
	events := make([]*types.EventIndex, 0)
	btcDelegators := make([]*types.BTCDelegator, 0)
	allowedStkTxHashes := make([]string, 0)

	blkHeight := uint64(r.Int63n(1000)) + math.MaxUint16
	totalDelegations := 0

	latestBtcReOrg := &types.LargestBtcReOrg{RollbackFrom: datagen.GenRandomBTCHeaderInfoWithHeight(r, 150), RollbackTo: datagen.GenRandomBTCHeaderInfoWithHeight(r, 100), BlockDiff: 50}

	for i := range fps {
		stakingValue := r.Int31n(200000) + 10000
		numDelegations := r.Int31n(10)
		delegations := createNDelegationsForFinalityProvider(
			r,
			t,
			fps[i].BtcPk.MustToBTCPK(),
			int64(stakingValue),
			int(numDelegations),
			params.CovenantQuorum,
		)

		for _, del := range delegations {
			totalDelegations++

			btcDelegations = append(btcDelegations, del)

			// BTC delegators idx
			stakingTxHash, err := del.GetStakingTxHash()
			h.NoError(err)

			idxDelegatorStk := types.NewBTCDelegatorDelegationIndex()
			err = idxDelegatorStk.Add(stakingTxHash)
			h.NoError(err)

			btcDelegators = append(btcDelegators, &types.BTCDelegator{
				Idx: &types.BTCDelegatorDelegationIndex{
					StakingTxHashList: idxDelegatorStk.StakingTxHashList,
				},
				FpBtcPk:  fps[i].BtcPk,
				DelBtcPk: del.BtcPk,
			})

			allowedStkTxHashes = append(allowedStkTxHashes, hex.EncodeToString(stakingTxHash[:]))

			// record event that the BTC delegation will become expired (unbonded) at EndHeight-w
			unbondedEvent := types.NewEventPowerDistUpdateWithBTCDel(&types.EventBTCDelegationStateUpdate{
				StakingTxHash: stakingTxHash.String(),
				NewState:      types.BTCDelegationStatus_EXPIRED,
			})

			// events
			idxEvent := uint64(totalDelegations - 1)
			events = append(events, &types.EventIndex{
				Idx:            idxEvent,
				BlockHeightBtc: del.EndHeight - del.UnbondingTime,
				Event:          unbondedEvent,
			})
		}

		// chain heights
		btcHeight := uint32(blkHeight + 100)
		chainsHeight = append(chainsHeight, &types.BlockHeightBbnToBtc{
			BlockHeightBbn: blkHeight,
			BlockHeightBtc: btcHeight,
		})

		blkHeight++ // each fp increase blk height to modify data in state.
	}

	gs := &types.GenesisState{
		Params:                 []*types.Params{&params},
		FinalityProviders:      fps,
		BtcDelegations:         btcDelegations,
		BlockHeightChains:      chainsHeight,
		BtcDelegators:          btcDelegators,
		Events:                 events,
		AllowedStakingTxHashes: allowedStkTxHashes,
		LargestBtcReorg:        latestBtcReOrg,
	}
	require.NoError(t, gs.Validate())
	return ctx, h, gs
}

func delegateToFP(h *helper.Helper, delegations []*types.BTCDelegation, fpBtcPk *btcec.PublicKey) {
	ctx, k := h.Ctx, h.App.BTCStakingKeeper
	for _, del := range delegations {
		if !del.FpBtcPkList[0].MustToBTCPK().IsEqual(fpBtcPk) {
			continue
		}
		// sets delegations
		h.AddDelegation(del)

		stakingTxHash, err := del.GetStakingTxHash()
		h.NoError(err)

		// store the staking tx hashes as allowed staking tx
		k.IndexAllowedStakingTransaction(ctx, &stakingTxHash)
	}
}
