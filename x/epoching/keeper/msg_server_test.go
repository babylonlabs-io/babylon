package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	testhelper "github.com/babylonlabs-io/babylon/v4/testutil/helper"
	"github.com/babylonlabs-io/babylon/v4/x/epoching"
	"github.com/babylonlabs-io/babylon/v4/x/epoching/types"
)

// TODO (fuzz tests): replace the following tests with fuzz ones
func TestMsgWrappedDelegate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	helper := testhelper.NewHelper(t)
	msgSrvr := helper.MsgSrvr
	// enter 1st epoch, in which BBN starts handling validator-related msgs
	ctx, err := helper.ApplyEmptyBlockWithVoteExtension(r)
	require.NoError(t, err)

	testCases := []struct {
		name      string
		req       *stakingtypes.MsgDelegate
		expectErr bool
	}{
		{
			"empty wrapped msg",
			&stakingtypes.MsgDelegate{},
			true,
		},
	}
	for _, tc := range testCases {
		wrappedMsg := types.NewMsgWrappedDelegate(tc.req)
		_, err := msgSrvr.WrappedDelegate(ctx, wrappedMsg)
		if tc.expectErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}
}

func TestMsgWrappedUndelegate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	helper := testhelper.NewHelper(t)
	msgSrvr := helper.MsgSrvr
	// enter 1st epoch, in which BBN starts handling validator-related msgs
	ctx, err := helper.ApplyEmptyBlockWithVoteExtension(r)
	require.NoError(t, err)

	testCases := []struct {
		name      string
		req       *stakingtypes.MsgUndelegate
		expectErr bool
	}{
		{
			"empty wrapped msg",
			&stakingtypes.MsgUndelegate{},
			true,
		},
	}
	for _, tc := range testCases {
		wrappedMsg := types.NewMsgWrappedUndelegate(tc.req)
		_, err := msgSrvr.WrappedUndelegate(ctx, wrappedMsg)
		if tc.expectErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}
}

func TestMsgWrappedBeginRedelegate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	helper := testhelper.NewHelper(t)
	msgSrvr := helper.MsgSrvr
	// enter 1st epoch, in which BBN starts handling validator-related msgs
	ctx, err := helper.ApplyEmptyBlockWithVoteExtension(r)
	require.NoError(t, err)

	testCases := []struct {
		name      string
		req       *stakingtypes.MsgBeginRedelegate
		expectErr bool
	}{
		{
			"empty wrapped msg",
			&stakingtypes.MsgBeginRedelegate{},
			true,
		},
	}
	for _, tc := range testCases {
		wrappedMsg := types.NewMsgWrappedBeginRedelegate(tc.req)

		_, err := msgSrvr.WrappedBeginRedelegate(ctx, wrappedMsg)
		if tc.expectErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}
}

func TestMsgWrappedCancelUnbondingDelegation(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	helper := testhelper.NewHelper(t)
	msgSrvr := helper.MsgSrvr
	// enter 1st epoch, in which BBN starts handling validator-related msgs
	ctx, err := helper.ApplyEmptyBlockWithVoteExtension(r)
	require.NoError(t, err)

	testCases := []struct {
		name      string
		req       *stakingtypes.MsgCancelUnbondingDelegation
		expectErr bool
	}{
		{
			"empty wrapped msg",
			&stakingtypes.MsgCancelUnbondingDelegation{},
			true,
		},
	}
	for _, tc := range testCases {
		wrappedMsg := types.NewMsgWrappedCancelUnbondingDelegation(tc.req)

		_, err := msgSrvr.WrappedCancelUnbondingDelegation(ctx, wrappedMsg)
		if tc.expectErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}
}

func FuzzMsgWrappedEditValidator(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		h := testhelper.NewHelper(t)

		ctx, k, stkK := h.Ctx, h.App.EpochingKeeper, h.App.StakingKeeper

		vals, err := stkK.GetValidators(ctx, 1)
		h.NoError(err)
		require.Len(t, vals, 1)

		valBeforeChange := vals[0]
		valAddr, err := sdk.ValAddressFromBech32(valBeforeChange.OperatorAddress)
		h.NoError(err)

		newDescription := datagen.GenRandomDescription(r)
		newCommissionRate := sdkmath.LegacyMustNewDecFromStr(fmt.Sprintf("0.%d", r.Int31n(5)+1))
		newMinSelfDel := valBeforeChange.MinSelfDelegation.AddRaw(r.Int63n(valBeforeChange.Tokens.Sub(valBeforeChange.MinSelfDelegation).Int64()))

		msg := stakingtypes.NewMsgEditValidator(valAddr.String(), *newDescription, &newCommissionRate, &newMinSelfDel)
		wMsg := types.NewMsgWrappedEditValidator(msg)

		res, err := h.MsgSrvr.WrappedEditValidator(ctx, wMsg)
		h.NoError(err)
		require.NotNil(t, res)

		epochMsgs := k.GetCurrentEpochMsgs(ctx)
		require.Len(t, epochMsgs, 1)

		blkHeader := ctx.BlockHeader()
		blkHeader.Time = valBeforeChange.Commission.UpdateTime.Add(time.Hour * 25)
		ctx = ctx.WithBlockHeader(blkHeader)

		epoch := k.GetEpoch(ctx)
		info := ctx.HeaderInfo()
		info.Height = int64(epoch.GetLastBlockHeight())
		info.Time = blkHeader.Time

		ctx = ctx.WithHeaderInfo(info)

		valsetUpdate, err := epoching.EndBlocker(ctx, k)
		h.NoError(err)
		require.Len(t, valsetUpdate, 0)

		valAfterChange, err := stkK.GetValidator(ctx, valAddr)
		require.NoError(t, err)
		require.Equal(t, newDescription.String(), valAfterChange.Description.String())
		require.Equal(t, newCommissionRate.String(), valAfterChange.Commission.Rate.String())
		require.Equal(t, newMinSelfDel.String(), valAfterChange.MinSelfDelegation.String())
	})
}

func FuzzMsgWrappedUpdateStakingParams(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		h := testhelper.NewHelper(t)

		ctx, k, stkK := h.Ctx, h.App.EpochingKeeper, h.App.StakingKeeper

		newUnbondingTime := time.Hour * time.Duration(r.Intn(100)+1)
		newMaxValidators := r.Uint32() + 1
		newMaxEntries := r.Uint32() + 1
		newHistoricalEntries := r.Uint32() + 1
		newBondDenom := datagen.GenRandomDenom(r)
		newMinCommissionRate := datagen.GenRandomCommission(r)

		newParams := stakingtypes.NewParams(newUnbondingTime, newMaxValidators, newMaxEntries, newHistoricalEntries, newBondDenom, newMinCommissionRate)

		msg := &stakingtypes.MsgUpdateParams{
			Authority: appparams.AccGov.String(),
			Params:    newParams,
		}
		wMsg := types.NewMsgWrappedStakingUpdateParams(msg)

		res, err := h.MsgSrvr.WrappedStakingUpdateParams(ctx, wMsg)
		h.NoError(err)
		require.NotNil(t, res)

		epochMsgs := k.GetCurrentEpochMsgs(ctx)
		require.Len(t, epochMsgs, 1)

		epoch := k.GetEpoch(ctx)
		info := ctx.HeaderInfo()
		info.Height = int64(epoch.GetLastBlockHeight())

		ctx = ctx.WithHeaderInfo(info)

		valsetUpdate, err := epoching.EndBlocker(ctx, k)
		h.NoError(err)
		require.Len(t, valsetUpdate, 0)

		stakingParamsAfterChange, err := stkK.GetParams(ctx)
		h.NoError(err)
		require.Equal(t, newUnbondingTime, stakingParamsAfterChange.UnbondingTime)
		require.Equal(t, newMaxValidators, stakingParamsAfterChange.MaxValidators)
		require.Equal(t, newMaxEntries, stakingParamsAfterChange.MaxEntries)
		require.Equal(t, newHistoricalEntries, stakingParamsAfterChange.HistoricalEntries)
		require.Equal(t, newBondDenom, stakingParamsAfterChange.BondDenom)
		require.Equal(t, newMinCommissionRate.String(), stakingParamsAfterChange.MinCommissionRate.String())
	})
}

func TestExponentiallyEventsEndEpochQueuedMessages(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	h := testhelper.NewHelper(t)
	ctx, k, stkK := h.Ctx, h.App.EpochingKeeper, h.App.StakingKeeper

	vals, err := stkK.GetValidators(ctx, 1)
	h.NoError(err)
	require.Len(t, vals, 1)

	valBeforeChange := vals[0]
	valAddr, err := sdk.ValAddressFromBech32(valBeforeChange.OperatorAddress)
	h.NoError(err)

	newDescription := datagen.GenRandomDescription(r)
	newCommissionRate := sdkmath.LegacyMustNewDecFromStr(fmt.Sprintf("0.%d", r.Int31n(5)+1))
	newMinSelfDel := valBeforeChange.MinSelfDelegation.AddRaw(r.Int63n(valBeforeChange.Tokens.Sub(valBeforeChange.MinSelfDelegation).Int64()))

	msg := stakingtypes.NewMsgEditValidator(valAddr.String(), *newDescription, &newCommissionRate, &newMinSelfDel)
	wMsg := types.NewMsgWrappedEditValidator(msg)
	res, err := h.MsgSrvr.WrappedEditValidator(ctx, wMsg)
	h.NoError(err)
	require.NotNil(t, res)

	epochMsgs := k.GetCurrentEpochMsgs(ctx)
	require.Len(t, epochMsgs, 1)

	var numDelMessages int = datagen.RandomInRange(r, 5, 25)
	userAddr := h.GenAccs[0].GetAddress()

	initialBalance := h.App.BankKeeper.GetBalance(ctx, userAddr, "ubbn")
	totalDiff := sdkmath.ZeroInt()
	for i := 0; i < numDelMessages; i++ {
		balanceBefore := h.App.BankKeeper.GetBalance(ctx, userAddr, "ubbn")
		msgDel := types.NewMsgWrappedDelegate(
			stakingtypes.NewMsgDelegate(
				h.GenAccs[0].GetAddress().String(),
				valAddr.String(),
				sdk.NewCoin("ubbn", sdkmath.NewInt(100000)),
			),
		)

		res, err := h.MsgSrvr.WrappedDelegate(ctx, msgDel)
		h.NoError(err)
		require.NotNil(t, res)
		balanceAfter := h.App.BankKeeper.GetBalance(ctx, userAddr, "ubbn")
		diff := balanceBefore.Amount.Sub(balanceAfter.Amount)
		totalDiff = totalDiff.Add(diff)
	}

	finalBalance := h.App.BankKeeper.GetBalance(ctx, userAddr, "ubbn")
	calculatedInitialBalance := finalBalance.Amount.Add(totalDiff)

	require.Equal(t, initialBalance.Amount, calculatedInitialBalance,
		"Final balance + total diff should equal initial balance")

	epochMsgs = k.GetCurrentEpochMsgs(ctx)
	require.Len(t, epochMsgs, numDelMessages+1)

	blkHeader := ctx.BlockHeader()
	blkHeader.Time = valBeforeChange.Commission.UpdateTime.Add(time.Hour * 25)
	ctx = ctx.WithBlockHeader(blkHeader)

	epoch := k.GetEpoch(ctx)
	info := ctx.HeaderInfo()
	info.Height = int64(epoch.GetLastBlockHeight())
	info.Time = blkHeader.Time

	// cleans out the msg server envents
	// which contained types.EventWrappedDelegate generated
	// by the x/epoching msg server in WrappedDelegate
	ctx = sdk.NewContext(ctx.MultiStore(), ctx.BlockHeader(), ctx.IsCheckTx(), ctx.Logger()).WithHeaderInfo(info)

	delegation, err := stkK.GetDelegation(ctx, h.GenAccs[0].GetAddress(), valAddr)
	h.NoError(err)
	sharesBefore := delegation.Shares

	// with a clean context
	_, err = epoching.EndBlocker(ctx, k)
	h.NoError(err)

	delegation, err = stkK.GetDelegation(ctx, h.GenAccs[0].GetAddress(), valAddr)
	h.NoError(err)
	sharesAfter := delegation.Shares

	expectedTotalAmount := sdkmath.NewInt(int64(numDelMessages * 100000))
	actualSharesIncrease := sharesAfter.Sub(sharesBefore)
	validator, err := stkK.GetValidator(ctx, valAddr)
	h.NoError(err)
	actualTokensFromShares := validator.TokensFromShares(actualSharesIncrease)

	require.True(t, actualTokensFromShares.TruncateInt().Equal(expectedTotalAmount),
		"Delegated tokens should equal expected amount: expected %s, got %s",
		expectedTotalAmount.String(), actualTokensFromShares.TruncateInt().String())

	var events []abci.Event
	if evtMgr := ctx.EventManager(); evtMgr != nil {
		events = evtMgr.ABCIEvents()
	}

	eventsGeneratedByMsgEditValidator := 1
	eventsGeneratedByMsgDelegate := 4
	eventsGeneratedHealthyEpochEndBlocker := 1
	eventsGeneratedByFundUnlock := 4

	expEventsLen := (numDelMessages * (eventsGeneratedByMsgDelegate + eventsGeneratedByFundUnlock)) + eventsGeneratedByMsgEditValidator + eventsGeneratedHealthyEpochEndBlocker
	require.Equal(t, expEventsLen, len(events))
}
