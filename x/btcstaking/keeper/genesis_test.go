package keeper_test

import (
	"math"
	"math/rand"
	"strings"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	v1 "github.com/babylonlabs-io/babylon/app/upgrades/v1"
	testnetdata "github.com/babylonlabs-io/babylon/app/upgrades/v1/testnet"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	"github.com/babylonlabs-io/babylon/testutil/helper"
	testutilk "github.com/babylonlabs-io/babylon/testutil/keeper"
	btclightclientt "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

func TestInitGenesisWithSetParams(t *testing.T) {
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())
	k, ctx := testutilk.BTCStakingKeeperWithStore(t, db, stateStore, nil, nil, nil)

	err := k.InitGenesis(ctx, *types.DefaultGenesis())
	require.NoError(t, err)

	params, err := v1.LoadBtcStakingParamsFromData(testnetdata.BtcStakingParamsStr)
	require.NoError(t, err)

	for _, p := range params {
		err = k.SetParams(ctx, p)
		require.NoError(t, err)
	}
}

func TestExportGenesis(t *testing.T) {
	r, h := rand.New(rand.NewSource(11)), helper.NewHelper(t)
	k, btclcK, ctx := h.App.BTCStakingKeeper, h.App.BTCLightClientKeeper, h.Ctx
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
	eventsIdx := make(map[uint64]*types.EventIndex, 0)
	btcDelegatorIndex := make(map[string]*types.BTCDelegator, 0)

	blkHeight := uint64(r.Int63n(1000)) + math.MaxUint16
	totalDelegations := 0

	for _, fp := range fps {
		btcHead := btclcK.GetTipInfo(ctx)
		btcHead.Height = uint32(blkHeight + 100)
		btclcK.InsertHeaderInfos(ctx, []*btclightclientt.BTCHeaderInfo{
			btcHead,
		})

		// set finality
		h.AddFinalityProvider(fp)

		stakingValue := r.Int31n(200000) + 10000
		numDelegations := r.Int31n(10)
		delegations := createNDelegationsForFinalityProvider(
			r,
			t,
			fp.BtcPk.MustToBTCPK(),
			int64(stakingValue),
			int(numDelegations),
			params.CovenantQuorum,
		)

		for _, del := range delegations {
			totalDelegations++

			// sets delegations
			h.AddDelegation(del)
			btcDelegations = append(btcDelegations, del)

			// BTC delegators idx
			stakingTxHash, err := del.GetStakingTxHash()
			h.NoError(err)

			idxDelegatorStk := types.NewBTCDelegatorDelegationIndex()
			err = idxDelegatorStk.Add(stakingTxHash)
			h.NoError(err)

			btcDelegatorIndex[del.BtcPk.MarshalHex()] = &types.BTCDelegator{
				Idx: &types.BTCDelegatorDelegationIndex{
					StakingTxHashList: idxDelegatorStk.StakingTxHashList,
				},
				FpBtcPk:  fp.BtcPk,
				DelBtcPk: del.BtcPk,
			}

			// record event that the BTC delegation will become expired (unbonded) at EndHeight-w
			unbondedEvent := types.NewEventPowerDistUpdateWithBTCDel(&types.EventBTCDelegationStateUpdate{
				StakingTxHash: stakingTxHash.String(),
				NewState:      types.BTCDelegationStatus_EXPIRED,
			})

			// events
			idxEvent := uint64(totalDelegations - 1)
			eventsIdx[idxEvent] = &types.EventIndex{
				Idx:            idxEvent,
				BlockHeightBtc: del.EndHeight - del.UnbondingTime,
				Event:          unbondedEvent,
			}
		}

		// sets chain heights
		header := ctx.HeaderInfo()
		header.Height = int64(blkHeight)
		ctx = ctx.WithHeaderInfo(header)
		h.Ctx = ctx

		k.IndexBTCHeight(ctx)
		chainsHeight = append(chainsHeight, &types.BlockHeightBbnToBtc{
			BlockHeightBbn: blkHeight,
			BlockHeightBtc: btcHead.Height,
		})

		blkHeight++ // each fp increase blk height to modify data in state.
	}

	gs, err := k.ExportGenesis(ctx)
	h.NoError(err)
	require.Equal(t, k.GetParams(ctx), *gs.Params[0])

	// finality providers
	correctFps := 0
	for _, fp := range fps {
		for _, gsfp := range gs.FinalityProviders {
			if !strings.EqualFold(fp.Addr, gsfp.Addr) {
				continue
			}
			require.EqualValues(t, fp, gsfp)
			correctFps++
		}
	}
	require.Equal(t, correctFps, numFps)

	// btc delegations
	correctDels := 0
	for _, del := range btcDelegations {
		for _, gsdel := range gs.BtcDelegations {
			if !strings.EqualFold(del.StakerAddr, gsdel.StakerAddr) {
				continue
			}
			correctDels++
			require.Equal(t, del, gsdel)
		}
	}
	require.Equal(t, correctDels, len(btcDelegations))

	// chains height
	require.Equal(t, chainsHeight, gs.BlockHeightChains)

	// btc delegators
	require.Equal(t, totalDelegations, len(gs.BtcDelegators))
	for _, btcDel := range gs.BtcDelegators {
		idxBtcDel := btcDelegatorIndex[btcDel.DelBtcPk.MarshalHex()]
		require.Equal(t, btcDel, idxBtcDel)
	}

	// events
	require.Equal(t, totalDelegations, len(gs.Events))
	for _, evt := range gs.Events {
		evtIdx := eventsIdx[evt.Idx]
		require.Equal(t, evt, evtIdx)
	}

	// TODO: vp dst cache
}
