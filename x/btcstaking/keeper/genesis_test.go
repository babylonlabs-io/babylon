package keeper_test

import (
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v3/testutil/helper"
	btclightclientt "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	bsctypes "github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
)

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
	fps, delegations, chainsHeight, consumerEvents := gs.FinalityProviders, gs.BtcDelegations, gs.BlockHeightChains, gs.ConsumerEvents

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

	// register consumers as cosmos consumers
	for _, ev := range consumerEvents {
		consumerRegister := &bsctypes.ConsumerRegister{
			ConsumerId: ev.ConsumerId,
			ConsumerMetadata: &bsctypes.ConsumerRegister_CosmosConsumerMetadata{
				CosmosConsumerMetadata: &bsctypes.CosmosConsumerMetadata{
					ChannelId: ev.ConsumerId,
				},
			},
		}
		err := h.App.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, consumerRegister)
		require.NoError(t, err)
	}

	// store consumer events
	for _, e := range consumerEvents {
		event := &types.BTCStakingConsumerEvent{
			Event: &types.BTCStakingConsumerEvent_ActiveDel{
				ActiveDel: e.Events.ActiveDel[0],
			},
		}
		k.AddBTCStakingConsumerEvent(ctx, e.ConsumerId, event)
	}

	exportedGs, err := k.ExportGenesis(ctx)
	h.NoError(err)

	// sort expected and exported data to have deterministic order in the results
	types.SortData(gs)
	types.SortData(exportedGs)

	require.Equal(t, gs, exportedGs)

	// TODO: vp dst cache
}

func TestConsumerEventsDeterministicOrder(t *testing.T) {
	ctx, h, _ := setupTest(t)
	k := h.App.BTCStakingKeeper

	unsortedBsnIDs := []string{"bsn-z", "bsn-a", "bsn-m", "bsn-b"}

	for _, bsnID := range unsortedBsnIDs {
		event := &types.BTCStakingConsumerEvent{Event: &types.BTCStakingConsumerEvent_NewFp{
			NewFp: &types.NewFinalityProvider{
				BtcPkHex: hex.EncodeToString([]byte("test-pk-" + bsnID)),
				BsnId:    bsnID,
			},
		},
		}

		err := h.App.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, &bsctypes.ConsumerRegister{
			ConsumerId: bsnID,
			ConsumerMetadata: &bsctypes.ConsumerRegister_CosmosConsumerMetadata{
				CosmosConsumerMetadata: &bsctypes.CosmosConsumerMetadata{
					ChannelId: bsnID,
				},
			},
		})
		require.NoError(t, err)

		err = k.AddBTCStakingConsumerEvent(ctx, bsnID, event)
		require.NoError(t, err)
	}

	var results []*types.GenesisState
	for i := 0; i < 5; i++ {
		gs, err := k.ExportGenesis(ctx)
		require.NoError(t, err)
		results = append(results, gs)
	}

	for i := 1; i < len(results); i++ {
		require.Equal(t, results[0].ConsumerEvents, results[i].ConsumerEvents, "ExportGenesis should return deterministic consumer events order")
	}

	events := results[0].ConsumerEvents
	require.Len(t, events, len(unsortedBsnIDs))

	expectedSortedIDs := []string{"bsn-a", "bsn-b", "bsn-m", "bsn-z"}
	for i, event := range events {
		require.Equal(t, expectedSortedIDs[i], event.ConsumerId, "Consumer events should be sorted by consumer ID")
	}
}

func setupTest(t *testing.T) (sdk.Context, *helper.Helper, *types.GenesisState) {
	r, h := rand.New(rand.NewSource(11)), helper.NewHelper(t)
	k, ctx := h.App.BTCStakingKeeper, h.Ctx
	numFps := 3

	fps := datagen.CreateNFinalityProviders(r, t, h.FpPopContext(), "", numFps)
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
	consumerEvents := make([]*types.ConsumerEvent, 0)

	blkHeight := uint64(r.Int63n(1000)) + math.MaxUint16
	totalDelegations := 0

	latestBtcReOrg := &types.LargestBtcReOrg{RollbackFrom: datagen.GenRandomBTCHeaderInfoWithHeight(r, 150), RollbackTo: datagen.GenRandomBTCHeaderInfoWithHeight(r, 100), BlockDiff: 50}

	for i := range fps {
		stakingValue := r.Int31n(200000) + 10000
		numDelegations := r.Int31n(10)
		delegations := createNDelegationsForFinalityProvider(
			r,
			t,
			h.StakerPopContext(),
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

			consumerEvents = append(consumerEvents, &types.ConsumerEvent{
				ConsumerId: fmt.Sprintf("consumer%d", totalDelegations+1),
				Events: &types.BTCStakingIBCPacket{
					ActiveDel: []*types.ActiveBTCDelegation{{}},
				},
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
		Params:            []*types.Params{&params},
		FinalityProviders: fps,
		BtcDelegations:    btcDelegations,
		BlockHeightChains: chainsHeight,
		BtcDelegators:     btcDelegators,
		Events:            events,
		LargestBtcReorg:   latestBtcReOrg,
		ConsumerEvents:    consumerEvents,
	}
	require.NoError(t, gs.Validate())
	return ctx, h, gs
}

func delegateToFP(h *helper.Helper, delegations []*types.BTCDelegation, fpBtcPk *btcec.PublicKey) {
	for _, del := range delegations {
		if !del.FpBtcPkList[0].MustToBTCPK().IsEqual(fpBtcPk) {
			continue
		}
		// sets delegations
		h.AddDelegation(del)
	}
}
