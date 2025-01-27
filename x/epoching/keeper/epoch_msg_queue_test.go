package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	testhelper "github.com/babylonlabs-io/babylon/testutil/helper"
	"github.com/babylonlabs-io/babylon/x/epoching/types"

	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/app/params"
)

var (
	coinWithOnePower = sdk.NewInt64Coin(appparams.DefaultBondDenom, sdk.DefaultPowerReduction.Int64())
)

// FuzzEnqueueMsg tests EnqueueMsg. It enqueues some wrapped msgs, and check if the message queue includes the enqueued msgs or not
func FuzzEnqueueMsg(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		helper := testhelper.NewHelper(t)
		ctx, keeper := helper.Ctx, helper.App.EpochingKeeper
		// ensure that the epoch msg queue is correct at the genesis
		require.Empty(t, keeper.GetCurrentEpochMsgs(ctx))
		require.Equal(t, uint64(0), keeper.GetCurrentQueueLength(ctx))

		// at epoch 1 right now
		epoch := keeper.GetEpoch(ctx)
		require.Equal(t, uint64(1), epoch.EpochNumber)
		// ensure that the epoch msg queue is correct at epoch 1
		require.Empty(t, keeper.GetCurrentEpochMsgs(ctx))
		require.Equal(t, uint64(0), keeper.GetCurrentQueueLength(ctx))

		// Enqueue a random number of msgs
		numQueuedMsgs := datagen.RandomInt(r, 100)
		for i := uint64(0); i < numQueuedMsgs; i++ {
			msg := types.QueuedMessage{
				TxId:  sdk.Uint64ToBigEndian(i),
				MsgId: sdk.Uint64ToBigEndian(i),
				Msg:   &types.QueuedMessage_MsgDelegate{MsgDelegate: &stakingtypes.MsgDelegate{}},
			}
			keeper.EnqueueMsg(ctx, msg)
		}

		// ensure that each msg in the queue is correct
		epochMsgs := keeper.GetCurrentEpochMsgs(ctx)
		for i, msg := range epochMsgs {
			require.Equal(t, sdk.Uint64ToBigEndian(uint64(i)), msg.TxId)
			require.Equal(t, sdk.Uint64ToBigEndian(uint64(i)), msg.MsgId)
			require.NotNil(t, msg)
		}
	})
}

// FuzzHandleQueuedMsg_MsgWrappedDelegate tests HandleQueueMsg over MsgWrappedDelegate.
// It enqueues some MsgWrappedDelegate, enters a new epoch (which triggers HandleQueueMsg), and check if the newly delegated tokens take effect or not
func FuzzHandleQueuedMsg_MsgWrappedDelegate(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// generate the validator set with 10 validators as genesis
		genesisValSet, privSigner, err := datagen.GenesisValidatorSetWithPrivSigner(10)
		require.NoError(t, err)
		helper := testhelper.NewHelperWithValSet(t, genesisValSet, privSigner)
		ctx, keeper, genAccs := helper.Ctx, helper.App.EpochingKeeper, helper.GenAccs
		params := keeper.GetParams(ctx)

		// get genesis account's address, whose holder will be the delegator
		require.NotNil(t, genAccs)
		require.NotEmpty(t, genAccs)
		genAddr := genAccs[0].GetAddress()

		// at epoch 1 right now
		epoch := keeper.GetEpoch(ctx)
		require.Equal(t, uint64(1), epoch.EpochNumber)

		// get validator to be undelegated
		valSet := keeper.GetCurrentValidatorSet(ctx)
		val := valSet[0].Addr
		valPower, err := keeper.GetCurrentValidatorVotingPower(ctx, val)
		require.NoError(t, err)

		// ensure the validator's lifecycle data is generated
		lc := keeper.GetValLifecycle(ctx, val)
		require.NotNil(t, lc)
		require.Equal(t, 1, len(lc.ValLife))
		require.Equal(t, types.BondState_CREATED, lc.ValLife[0].State)
		require.Equal(t, uint64(0), lc.ValLife[0].BlockHeight)

		// delegate a random amount of tokens to the validator
		numNewDels := r.Int63n(15) + 1
		for i := int64(0); i < numNewDels; i++ {
			helper.WrappedDelegate(genAddr, val, coinWithOnePower.Amount)
		}
		// ensure the msgs are queued
		epochMsgs := keeper.GetCurrentEpochMsgs(ctx)
		require.Equal(t, numNewDels, int64(len(epochMsgs)))

		// go to BeginBlock of block 11, and thus entering epoch 2
		for i := uint64(0); i < params.EpochInterval; i++ {
			ctx, err = helper.ApplyEmptyBlockWithVoteExtension(r)
			require.NoError(t, err)
		}
		epoch = keeper.GetEpoch(ctx)
		require.Equal(t, uint64(2), epoch.EpochNumber)

		// ensure epoch 2 has initialised an empty msg queue
		require.Empty(t, keeper.GetCurrentEpochMsgs(ctx))

		// ensure the voting power has been added w.r.t. the newly delegated tokens
		valPower2, err := keeper.GetCurrentValidatorVotingPower(ctx, val)
		require.NoError(t, err)
		addedPower := helper.App.StakingKeeper.TokensToConsensusPower(ctx, coinWithOnePower.Amount.MulRaw(numNewDels))
		require.Equal(t, valPower+addedPower, valPower2)
	})
}

// FuzzHandleQueuedMsg_EventWrappedEditValidator tests HandleQueueMsg over EventWrappedEditValidator.
// It enqueues some EventWrappedEditValidator, enters a new epoch (which triggers HandleQueueMsg), and check if the validator data was modified
func FuzzHandleQueuedMsg_EventWrappedEditValidator(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		genesisValSet, privSigner, err := datagen.GenesisValidatorSetWithPrivSigner(1)
		require.NoError(t, err)
		h := testhelper.NewHelperWithValSet(t, genesisValSet, privSigner)
		ctx, k, stkK, genAccs := h.Ctx, h.App.EpochingKeeper, h.App.StakingKeeper, h.GenAccs
		params := k.GetParams(ctx)

		// get genesis account's address, whose holder will be the delegator
		require.NotNil(t, genAccs)
		require.NotEmpty(t, genAccs)

		// at epoch 1 right now
		epoch := k.GetEpoch(ctx)
		require.Equal(t, uint64(1), epoch.EpochNumber)

		// get validator to be undelegated
		valSet := k.GetCurrentValidatorSet(ctx)
		val := sdk.ValAddress(valSet[0].Addr)

		// ensure the validator's lifecycle data is generated
		lc := k.GetValLifecycle(ctx, val)
		require.NotNil(t, lc)
		require.Equal(t, 1, len(lc.ValLife))
		require.Equal(t, types.BondState_CREATED, lc.ValLife[0].State)
		require.Equal(t, uint64(0), lc.ValLife[0].BlockHeight)

		// stkP, err := stkK.GetParams(ctx)
		// require.NoError(t, err)

		valBeforeChange, err := stkK.GetValidator(ctx, val)
		require.NoError(t, err)

		newDescription := datagen.GenRandomDescription(r)

		newCommissionRate := sdkmath.LegacyMustNewDecFromStr(fmt.Sprintf("0.%d", r.Int31n(5)+1))
		newMinSelfDel := valBeforeChange.MinSelfDelegation.AddRaw(int64(datagen.RandomInt(r, 10) + 1))

		// commission rate can only change once per day '-'
		blkHeader := h.Ctx.BlockHeader()
		blkHeader.Time = valBeforeChange.Commission.UpdateTime.Add(time.Hour * 25)
		h.Ctx = h.Ctx.WithBlockHeader(blkHeader)

		h.WrappedEditValidator(val, *newDescription, &newCommissionRate, &newMinSelfDel)

		// ensure the msgs are queued
		epochMsgs := k.GetCurrentEpochMsgs(ctx)
		require.Len(t, epochMsgs, 1)

		// go to BeginBlock of block 11, and thus entering epoch 2
		for i := uint64(0); i < params.EpochInterval; i++ {
			ctx, err = h.ApplyEmptyBlockWithVoteExtension(r)
			require.NoError(t, err)
		}
		epoch = k.GetEpoch(ctx)
		require.Equal(t, uint64(2), epoch.EpochNumber)

		// ensure epoch 2 has initialised an empty msg queue
		require.Empty(t, k.GetCurrentEpochMsgs(ctx))

		valAfterChange, err := stkK.GetValidator(ctx, val)
		require.NoError(t, err)
		require.Equal(t, newDescription.String(), valAfterChange.Description.String())
		require.Equal(t, newCommissionRate.String(), valAfterChange.Commission.String())
		require.Equal(t, newMinSelfDel.String(), valAfterChange.MinSelfDelegation.String())
	})
}

// FuzzHandleQueuedMsg_MsgWrappedUndelegate tests HandleQueueMsg over MsgWrappedUndelegate.
// It enqueues some MsgWrappedUndelegate, enters a new epoch (which triggers HandleQueueMsg), and check if the tokens become unbonding or not
func FuzzHandleQueuedMsg_MsgWrappedUndelegate(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// generate the validator set with 10 validators as genesis
		genesisValSet, privSigner, err := datagen.GenesisValidatorSetWithPrivSigner(10)
		require.NoError(t, err)
		helper := testhelper.NewHelperWithValSet(t, genesisValSet, privSigner)
		ctx, keeper, genAccs := helper.Ctx, helper.App.EpochingKeeper, helper.GenAccs

		// get genesis account's address, whose holder will be the delegator
		require.NotNil(t, genAccs)
		require.NotEmpty(t, genAccs)
		genAddr := genAccs[0].GetAddress()

		// at epoch 1 right now
		epoch := keeper.GetEpoch(ctx)
		require.Equal(t, uint64(1), epoch.EpochNumber)

		valSet1 := helper.App.EpochingKeeper.GetCurrentValidatorSet(helper.Ctx)
		val := valSet1[0].Addr // validator to be undelegated
		valPower, err := helper.App.EpochingKeeper.GetCurrentValidatorVotingPower(ctx, val)
		require.NoError(t, err)

		// ensure the validator's lifecycle data is generated
		lc := keeper.GetValLifecycle(ctx, val)
		require.NotNil(t, lc)
		require.Equal(t, 1, len(lc.ValLife))
		require.Equal(t, types.BondState_CREATED, lc.ValLife[0].State)
		require.Equal(t, uint64(0), lc.ValLife[0].BlockHeight)

		// unbond a random amount of tokens from the validator
		// Note that for any pair of delegator and validator, there can be `<=DefaultMaxEntries=7` concurrent undelegations at any time slot
		// Otherwise, only `DefaultMaxEntries` undelegations will be processed at this height and the rest will be rejected
		// See https://github.com/cosmos/cosmos-sdk/blob/v0.45.5/x/staking/keeper/delegation.go#L814-L816
		numNewUndels := r.Int63n(7) + 1
		for i := int64(0); i < numNewUndels; i++ {
			helper.WrappedUndelegate(genAddr, val, coinWithOnePower.Amount)
		}
		// ensure the msgs are queued
		epochMsgs := keeper.GetCurrentEpochMsgs(ctx)
		require.Equal(t, numNewUndels, int64(len(epochMsgs)))

		// enter epoch 2
		for i := uint64(0); i < keeper.GetParams(ctx).EpochInterval; i++ {
			ctx, err = helper.ApplyEmptyBlockWithVoteExtension(r)
			require.NoError(t, err)
		}
		epoch = keeper.GetEpoch(ctx)
		require.Equal(t, uint64(2), epoch.EpochNumber)

		// ensure epoch 2 has initialised an empty msg queue
		require.Empty(t, keeper.GetCurrentEpochMsgs(ctx))

		// ensure the voting power has been reduced w.r.t. the unbonding tokens
		valPower2, err := helper.App.EpochingKeeper.GetCurrentValidatorVotingPower(ctx, val)
		require.NoError(t, err)
		reducedPower := helper.App.StakingKeeper.TokensToConsensusPower(ctx, coinWithOnePower.Amount.MulRaw(numNewUndels))
		require.Equal(t, valPower-reducedPower, valPower2)

		// ensure the genesis account has these unbonding tokens
		unbondingDels, err := helper.App.StakingKeeper.GetAllUnbondingDelegations(ctx, genAddr)
		require.NoError(t, err)
		require.Equal(t, 1, len(unbondingDels)) // there is only 1 type of tokens

		// from cosmos v47, all undelegations made at the same height are represented
		// by one entry see: https://github.com/cosmos/cosmos-sdk/pull/12967
		require.Equal(t, 1, len(unbondingDels[0].Entries))
		require.Equal(t, unbondingDels[0].Entries[0].Balance, coinWithOnePower.Amount.MulRaw(numNewUndels))
	})
}

// FuzzHandleQueuedMsg_MsgWrappedBeginRedelegate tests HandleQueueMsg over MsgWrappedBeginRedelegate.
// It enqueues some MsgWrappedBeginRedelegate, enters a new epoch (which triggers HandleQueueMsg), and check if the tokens become unbonding or not
func FuzzHandleQueuedMsg_MsgWrappedBeginRedelegate(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// generate the validator set with 10 validators as genesis
		genesisValSet, privSigner, err := datagen.GenesisValidatorSetWithPrivSigner(10)
		require.NoError(t, err)
		helper := testhelper.NewHelperWithValSet(t, genesisValSet, privSigner)
		ctx, keeper, genAccs := helper.Ctx, helper.App.EpochingKeeper, helper.GenAccs

		// get genesis account's address, whose holder will be the delegator
		require.NotNil(t, genAccs)
		require.NotEmpty(t, genAccs)
		genAddr := genAccs[0].GetAddress()

		// at epoch 1 right now
		epoch := keeper.GetEpoch(ctx)
		require.Equal(t, uint64(1), epoch.EpochNumber)

		valSet1 := helper.App.EpochingKeeper.GetCurrentValidatorSet(ctx)

		// 2 validators, where the operator will redelegate some token from val1 to val2
		val1 := valSet1[0].Addr
		val1Power, err := helper.App.EpochingKeeper.GetCurrentValidatorVotingPower(ctx, val1)
		require.NoError(t, err)
		val2 := valSet1[1].Addr
		val2Power, err := helper.App.EpochingKeeper.GetCurrentValidatorVotingPower(ctx, val2)
		require.NoError(t, err)
		require.Equal(t, val1Power, val2Power)

		// ensure the validator's lifecycle data is generated
		for _, val := range []sdk.ValAddress{val1, val2} {
			lc := keeper.GetValLifecycle(ctx, val)
			require.NotNil(t, lc)
			require.Equal(t, 1, len(lc.ValLife))
			require.Equal(t, types.BondState_CREATED, lc.ValLife[0].State)
			require.Equal(t, uint64(0), lc.ValLife[0].BlockHeight)
		}

		// redelegate a random amount of tokens from val1 to val2
		// same as undelegation, there can be `<=DefaultMaxEntries=7` concurrent redelegation requests for any tuple (delegatorAddr, srcValidatorAddr, dstValidatorAddr)
		// See https://github.com/cosmos/cosmos-sdk/blob/v0.45.5/x/staking/keeper/delegation.go#L908-L910
		numNewRedels := r.Int63n(7) + 1
		for i := int64(0); i < numNewRedels; i++ {
			helper.WrappedBeginRedelegate(genAddr, val1, val2, coinWithOnePower.Amount)
		}
		// ensure the msgs are queued
		epochMsgs := keeper.GetCurrentEpochMsgs(ctx)
		require.Equal(t, numNewRedels, int64(len(epochMsgs)))

		// enter epoch 2
		for i := uint64(0); i < keeper.GetParams(ctx).EpochInterval; i++ {
			ctx, err = helper.ApplyEmptyBlockWithVoteExtension(r)
			require.NoError(t, err)
		}
		epoch = keeper.GetEpoch(ctx)
		require.Equal(t, uint64(2), epoch.EpochNumber)

		// ensure epoch 2 has initialised an empty msg queue
		require.Empty(t, keeper.GetCurrentEpochMsgs(ctx))

		// ensure the voting power has been redelegated from val1 to val2
		// Note that in Cosmos SDK, redelegation happens upon the next `EndBlock`, rather than waiting for 14 days.
		// This is because redelegation does not affect PoS security: upon redelegation requests, no token is leaving the system.
		// SImilarly, in Babylon, redelegation happens unconditionally upon `EndEpoch`, rather than depending on checkpoint status.
		val1Power2, err := helper.App.EpochingKeeper.GetCurrentValidatorVotingPower(ctx, val1)
		require.NoError(t, err)
		val2Power2, err := helper.App.EpochingKeeper.GetCurrentValidatorVotingPower(ctx, val2)
		require.NoError(t, err)
		redelegatedPower := helper.App.StakingKeeper.TokensToConsensusPower(ctx, coinWithOnePower.Amount.MulRaw(numNewRedels))
		// ensure the voting power of val1 has reduced
		require.Equal(t, val1Power-redelegatedPower, val1Power2)
		// ensure the voting power of val2 has increased
		require.Equal(t, val2Power+redelegatedPower, val2Power2)

		// ensure the genesis account has these redelegating tokens
		redelegations, err := helper.App.StakingKeeper.GetAllRedelegations(ctx, genAddr, val1, val2)
		require.NoError(t, err)
		require.Equal(t, 1, len(redelegations))                              // there is only 1 type of tokens
		require.Equal(t, numNewRedels, int64(len(redelegations[0].Entries))) // there are numNewRedels entries
		for _, entry := range redelegations[0].Entries {
			require.Equal(t, coinWithOnePower.Amount, entry.InitialBalance) // each redelegating entry has tokens of 1 voting power
		}
	})
}
