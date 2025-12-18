package replay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	sdkmath "cosmossdk.io/math"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	costkkeeper "github.com/babylonlabs-io/babylon/v4/x/costaking/keeper"
	costktypes "github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
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

type EventCoinsExtractor func(attr abcitypes.EventAttribute) (sdk.Coins, error)

func FindEventCoinsT(t *testing.T, evts []abcitypes.Event, eventType, attrKey string, extractor EventCoinsExtractor) sdk.Coins {
	total, err := FindEventCoins(evts, eventType, attrKey, extractor)
	require.NoError(t, err)
	return total
}

func FindEventCoins(evts []abcitypes.Event, eventType, attrKey string, extractor EventCoinsExtractor) (sdk.Coins, error) {
	totalCoins := sdk.NewCoins()
	for _, evt := range evts {
		if evt.Type != eventType {
			continue
		}

		for _, attr := range evt.Attributes {
			if attr.Key != attrKey {
				continue
			}

			coins, err := extractor(attr)
			if err != nil {
				return sdk.NewCoins(), err
			}
			totalCoins = totalCoins.Add(coins...)
			break
		}
	}
	return totalCoins, nil
}

func ExtractCoinsFromJSON(attr abcitypes.EventAttribute) (sdk.Coins, error) {
	var coins sdk.Coins
	err := json.Unmarshal([]byte(attr.Value), &coins)
	return coins, err
}

func ExtractCoinsFromInt(attr abcitypes.EventAttribute) (sdk.Coins, error) {
	amt, ok := sdkmath.NewIntFromString(attr.Value)
	if !ok {
		return sdk.NewCoins(), fmt.Errorf("failed to parse int from %s", attr.Value)
	}
	return sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, amt)), nil
}

func ExtractCoinsFromDecCoins(attr abcitypes.EventAttribute) (sdk.Coins, error) {
	decCoins, err := sdk.ParseDecCoins(attr.Value)
	if err != nil {
		return nil, err
	}
	coins, _ := decCoins.TruncateDecimal()
	return coins, nil
}

func FindEventCostakerRewards(t *testing.T, evts []abcitypes.Event) sdk.Coins {
	evtTypeCostAddRwd := sdk.MsgTypeURL(&costktypes.EventCostakersAddRewards{})[1:]
	return FindEventCoinsT(t, evts, evtTypeCostAddRwd, evtAttrAddRewards, ExtractCoinsFromJSON)
}

func FindEventMint(t *testing.T, evts []abcitypes.Event) sdk.Coins {
	return FindEventCoinsT(t, evts, minttypes.EventTypeMint, sdk.AttributeKeyAmount, ExtractCoinsFromInt)
}

func FindEventBtcStakers(t *testing.T, evts []abcitypes.Event) sdk.Coins {
	return FindEventCoinsT(t, evts, ictvtypes.EventTypeBTCStakingReward, sdk.AttributeKeyAmount, ExtractCoinsFromDecCoins)
}

func FindEventTypeFPDirectRewards(t *testing.T, evts []abcitypes.Event) sdk.Coins {
	return FindEventCoinsT(t, evts, ictvtypes.EventTypeFPDirectRewards, sdk.AttributeKeyAmount, ExtractCoinsFromDecCoins)
}

func FindEventTypeValidatorDirectRewards(t *testing.T, evts []abcitypes.Event) sdk.Coins {
	return FindEventCoinsT(t, evts, costktypes.EventTypeValidatorDirectRewards, sdk.AttributeKeyAmount, ExtractCoinsFromDecCoins)
}

func assertZeroCostkTracker(t *testing.T, ctx context.Context, costkK costkkeeper.Keeper, addr sdk.AccAddress) {
	trk, err := costkK.GetCostakerRewards(ctx, addr)
	require.NoError(t, err)
	require.NotNil(t, trk)
	require.Truef(t, trk.ActiveBaby.IsZero(), "active baby should be zero %s", trk.ActiveBaby.String())
	require.Truef(t, trk.ActiveSatoshis.IsZero(), "Active sats should be zero %s", trk.ActiveSatoshis.String())
	require.Truef(t, trk.TotalScore.IsZero(), "Active score should be zero %s", trk.TotalScore.String())
}

func isValidatorInValset(valset []epochingtypes.Validator, valAddr sdk.ValAddress) bool {
	return FindValInValset(valset, valAddr) != nil
}

func FindValInValset(valset []epochingtypes.Validator, valAddr sdk.ValAddress) *epochingtypes.Validator {
	for _, v := range valset {
		if bytes.Equal(v.GetValAddress().Bytes(), valAddr.Bytes()) {
			return &v
		}
	}
	return nil
}

func FindValInValidators(validators []stktypes.Validator, valAddr sdk.ValAddress) *stktypes.Validator {
	for _, v := range validators {
		if strings.EqualFold(v.OperatorAddress, valAddr.String()) {
			return &v
		}
	}
	return nil
}

// assertActiveBabyWithinRange checks if actual is within expected ± tolerance (for rounding differences)
func assertActiveBabyWithinRange(t *testing.T, expected, actual sdkmath.Int, tolerance int64, msgAndArgs ...interface{}) { //nolint:unparam
	diff := actual.Sub(expected).Abs()
	maxDiff := sdkmath.NewInt(tolerance)
	require.True(t, diff.LTE(maxDiff), "ActiveBaby difference exceeds tolerance: expected %s ± %d, got %s (diff: %s). %v",
		expected.String(), tolerance, actual.String(), diff.String(), msgAndArgs)
}

func (d *BabylonAppDriver) IsValsInCurrActiveValset(expLenValset int, valAddrs ...sdk.ValAddress) epochingtypes.ValidatorSet {
	epochK := d.App.EpochingKeeper
	epoch := epochK.GetEpoch(d.Ctx())
	valset := epochK.GetValidatorSet(d.Ctx(), epoch.EpochNumber)
	require.Lenf(d.t, valset, expLenValset, "expected %d validators in active set", expLenValset)

	for _, valAddr := range valAddrs {
		val := FindValInValset(valset, valAddr)
		require.NotNil(d.t, val)
	}
	return valset
}

// JailValidatorForDowntime returns the validator if the validator indeed got jailed
func (d *BabylonAppDriver) JailValidatorForDowntime(val sdk.ValAddress) *stktypes.Validator {
	stkK := d.App.StakingKeeper
	for i := 0; i < 1000; i++ { // it should get jailed before that
		d.GenerateNewBlockAssertExecutionSuccess()
		val, err := stkK.GetValidator(d.Ctx(), val)
		require.NoError(d.t, err)

		if val.IsJailed() {
			return &val
		}
	}
	return nil
}
