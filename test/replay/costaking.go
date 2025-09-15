package replay

import (
	"encoding/json"

	sdkmath "cosmossdk.io/math"
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
	require.Equal(d.t, del.ActiveBaby.String(), expActiveBaby.String())
	require.Equal(d.t, del.ActiveSatoshis.String(), expActiveSats.String())
	require.Equal(d.t, del.StartPeriodCumulativeReward, expStartPeriod)
	require.Equal(d.t, del.TotalScore.String(), expTotalScore.String())
}

func (d *BabylonAppDriver) CheckCostakingCurrentRewards(
	expRewardsWithoutDecimals sdk.Coins,
	expPeriod uint64,
	expTotalScore sdkmath.Int,
) {
	costkK := d.App.CostakingKeeper

	rwdWithDecimals := expRewardsWithoutDecimals.MulInt(ictvtypes.DecimalRewards)
	del, err := costkK.GetCurrentRewards(d.Ctx())
	require.NoError(d.t, err)
	require.Equal(d.t, del.Rewards.String(), rwdWithDecimals.String())
	require.Equal(d.t, del.Period, expPeriod)
	require.Equal(d.t, del.TotalScore.String(), expTotalScore.String())
}

func (d *BabylonAppDriver) CheckCostakingCurrentHistoricalRewards(
	period uint64,
	expCumulativeRewardsPerScore sdk.Coins,
) {
	costkK := d.App.CostakingKeeper

	hist, err := costkK.GetHistoricalRewards(d.Ctx(), period)
	require.NoError(d.t, err)
	require.Equal(d.t, hist.CumulativeRewardsPerScore.String(), expCumulativeRewardsPerScore.String())
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

	// "babylon.costaking.v1.EventCostakersAddRewards"
	evtTypeCostAddRwd := sdk.MsgTypeURL(&costktypes.EventCostakersAddRewards{})[1:]

	totalRewardsAdded := sdk.NewCoins()
	for _, evt := range response.Events {
		if evt.Type != evtTypeCostAddRwd {
			continue
		}

		for _, attr := range evt.Attributes {
			if attr.Key != evtAttrAddRewards {
				continue
			}

			var addRewards sdk.Coins
			err := json.Unmarshal([]byte(attr.Value), &addRewards)
			require.NoError(d.t, err)

			totalRewardsAdded = totalRewardsAdded.Add(addRewards...)
			break
		}
	}

	return totalRewardsAdded
}
