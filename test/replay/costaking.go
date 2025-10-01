package replay

import (
	"encoding/json"
	"testing"

	sdkmath "cosmossdk.io/math"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	costktypes "github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	minttypes "github.com/babylonlabs-io/babylon/v4/x/mint/types"
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

func EventCostakerRewardsFromBlocks(t *testing.T, blocks []*abcitypes.ResponseFinalizeBlock) sdk.Coins {
	totalRewardsAdded := sdk.NewCoins()

	for _, block := range blocks {
		totalRewardsAdded = totalRewardsAdded.Add(FindEventCostakerRewards(t, block.Events)...)
	}
	return totalRewardsAdded
}

type eventCoinsExtractor func(t *testing.T, attr abcitypes.EventAttribute) (sdk.Coins, error)

func findEventCoins(t *testing.T, evts []abcitypes.Event, eventType, attrKey string, extractor eventCoinsExtractor) sdk.Coins {
	totalCoins := sdk.NewCoins()
	for _, evt := range evts {
		if evt.Type != eventType {
			continue
		}

		for _, attr := range evt.Attributes {
			if attr.Key != attrKey {
				continue
			}

			coins, err := extractor(t, attr)
			require.NoError(t, err)
			totalCoins = totalCoins.Add(coins...)
			break
		}
	}
	return totalCoins
}

func extractCoinsFromJSON(t *testing.T, attr abcitypes.EventAttribute) (sdk.Coins, error) {
	var coins sdk.Coins
	err := json.Unmarshal([]byte(attr.Value), &coins)
	return coins, err
}

func extractCoinsFromInt(t *testing.T, attr abcitypes.EventAttribute) (sdk.Coins, error) {
	amt, ok := sdkmath.NewIntFromString(attr.Value)
	require.True(t, ok, "failed to parse int from %s", attr.Value)
	return sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, amt)), nil
}

func extractCoinsFromDecCoins(t *testing.T, attr abcitypes.EventAttribute) (sdk.Coins, error) {
	decCoins, err := sdk.ParseDecCoins(attr.Value)
	if err != nil {
		return nil, err
	}
	coins, _ := decCoins.TruncateDecimal()
	return coins, nil
}

func FindEventCostakerRewards(t *testing.T, evts []abcitypes.Event) sdk.Coins {
	evtTypeCostAddRwd := sdk.MsgTypeURL(&costktypes.EventCostakersAddRewards{})[1:]
	return findEventCoins(t, evts, evtTypeCostAddRwd, evtAttrAddRewards, extractCoinsFromJSON)
}

func FindEventMint(t *testing.T, evts []abcitypes.Event) sdk.Coins {
	return findEventCoins(t, evts, minttypes.EventTypeMint, sdk.AttributeKeyAmount, extractCoinsFromInt)
}

func FindEventBtcStakers(t *testing.T, evts []abcitypes.Event) sdk.Coins {
	return findEventCoins(t, evts, ictvtypes.EventTypeBTCStakingReward, sdk.AttributeKeyAmount, extractCoinsFromDecCoins)
}

func FindEventTypeFPDirectRewards(t *testing.T, evts []abcitypes.Event) sdk.Coins {
	return findEventCoins(t, evts, ictvtypes.EventTypeFPDirectRewards, sdk.AttributeKeyAmount, extractCoinsFromDecCoins)
}

func FindEventTypeValidatorDirectRewards(t *testing.T, evts []abcitypes.Event) sdk.Coins {
	return findEventCoins(t, evts, costktypes.EventTypeValidatorDirectRewards, sdk.AttributeKeyAmount, extractCoinsFromDecCoins)
}
