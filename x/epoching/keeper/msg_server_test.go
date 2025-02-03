package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	testhelper "github.com/babylonlabs-io/babylon/testutil/helper"
	"github.com/babylonlabs-io/babylon/x/epoching"
	"github.com/babylonlabs-io/babylon/x/epoching/types"
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
