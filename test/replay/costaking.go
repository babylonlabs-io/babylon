package replay

import (
	"encoding/json"
	"testing"

	sdkmath "cosmossdk.io/math"
	abci "github.com/cometbft/cometbft/abci/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	costktypes "github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

const (
	evtAttrAddRewards = "add_rewards"
)

func (d *BabylonAppDriver) CheckCostakerRewards(
	addr sdk.AccAddress,
	expActiveBaby, expActiveSats, expTotalScore sdkmath.Int,
	expStartPeriod uint64,
) {
	costkK := d.App.CostakingKeeper

	del, err := costkK.GetCostakerRewards(d.Ctx(), addr)
	require.NoError(d.t, err)
	require.Equal(d.t, expActiveBaby.String(), del.ActiveBaby.String(), "active baby")
	require.Equal(d.t, expActiveSats.String(), del.ActiveSatoshis.String(), "active sats")
	require.Equal(d.t, expStartPeriod, del.StartPeriodCumulativeReward, "start period cumulative rewards exp %d != %d act", expStartPeriod, del.StartPeriodCumulativeReward)
	require.Equal(d.t, expTotalScore.String(), del.TotalScore.String(), "total score")
}

func (d *BabylonAppDriver) CheckCostakingCurrentRewards(
	expRewardsWithoutDecimals sdk.Coins,
	expPeriod uint64,
	expTotalScore sdkmath.Int,
) {
	costkK := d.App.CostakingKeeper

	rwdWithDecimals := expRewardsWithoutDecimals.MulInt(ictvtypes.DecimalRewards)
	currRwd, err := costkK.GetCurrentRewards(d.Ctx())
	require.NoError(d.t, err)
	require.Equal(d.t, rwdWithDecimals.String(), currRwd.Rewards.String(), "current rewards decimals doesn't match")
	require.Equal(d.t, expPeriod, currRwd.Period, "current rewards period doesn't match")
	require.Equal(d.t, expTotalScore.String(), currRwd.TotalScore.String(), "current rewards total score doesn't match")
}

func (d *BabylonAppDriver) CheckCostakingCurrentHistoricalRewards(
	period uint64,
	expCumulativeRewardsPerScore sdk.Coins,
) {
	costkK := d.App.CostakingKeeper

	hist, err := costkK.GetHistoricalRewards(d.Ctx(), period)
	require.NoError(d.t, err)
	require.Equal(d.t, expCumulativeRewardsPerScore.String(), hist.CumulativeRewardsPerScore.String(), "cumulative rewards per score")
}

func (d *BabylonAppDriver) GenerateNewBlockAssertExecutionSuccessWithCostakerRewards() sdk.Coins {
	response := d.GenerateNewBlock()

	for _, tx := range response.TxResults {
		// ignore checkpoint txs
		if tx.GasWanted == 0 {
			continue
		}

		require.Equal(d.t, tx.Code, uint32(0), tx.Log)
	}

	return FindEventCostakerRewards(d.t, response.Events)
}

func EventCostakerRewardsFromBlocks(t *testing.T, blocks []*abci.ResponseFinalizeBlock) sdk.Coins {
	totalRewardsAdded := sdk.NewCoins()

	for _, block := range blocks {
		totalRewardsAdded = totalRewardsAdded.Add(FindEventCostakerRewards(t, block.Events)...)
	}
	return totalRewardsAdded
}

func FindEventCostakerRewards(t *testing.T, evts []abcitypes.Event) sdk.Coins {
	// "babylon.costaking.v1.EventCostakersAddRewards"
	evtTypeCostAddRwd := sdk.MsgTypeURL(&costktypes.EventCostakersAddRewards{})[1:]

	totalRewardsAdded := sdk.NewCoins()
	for _, evt := range evts {
		if evt.Type != evtTypeCostAddRwd {
			continue
		}

		for _, attr := range evt.Attributes {
			if attr.Key != evtAttrAddRewards {
				continue
			}

			var addRewards sdk.Coins
			err := json.Unmarshal([]byte(attr.Value), &addRewards)
			require.NoError(t, err)

			totalRewardsAdded = totalRewardsAdded.Add(addRewards...)
			break
		}
	}

	return totalRewardsAdded
}
