package keeper_test

import (
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	testhelper "github.com/babylonlabs-io/babylon/v3/testutil/helper"
	"github.com/babylonlabs-io/babylon/v3/x/epoching"
	"github.com/babylonlabs-io/babylon/v3/x/epoching/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
)

// Helper function to setup basic test environment
func setupTestEnvironment(t *testing.T) (*testhelper.Helper, sdk.Context) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	helper := testhelper.NewHelper(t)
	ctx, err := helper.ApplyEmptyBlockWithVoteExtension(r)
	require.NoError(t, err)
	return helper, ctx
}

// Helper function to get existing validator from helper
func getExistingValidator(t *testing.T, helper *testhelper.Helper, ctx sdk.Context) (sdk.AccAddress, sdk.ValAddress) {
	vals, err := helper.App.StakingKeeper.GetValidators(ctx, 1)
	require.NoError(t, err)
	require.Len(t, vals, 1)

	delegatorAddr := helper.GenAccs[0].GetAddress()
	validatorAddr, err := sdk.ValAddressFromBech32(vals[0].OperatorAddress)
	require.NoError(t, err)

	return delegatorAddr, validatorAddr
}

// Helper function to get initial balances and delegation
func getInitialState(helper *testhelper.Helper, ctx sdk.Context, delegatorAddr sdk.AccAddress, validatorAddr sdk.ValAddress, amount sdk.Coin) (sdk.Coin, sdk.Coin, sdkmath.LegacyDec) {
	// Get delegate pool address for balance checks
	delegatePoolAddr := helper.App.AccountKeeper.GetModuleAddress("epoching_delegate_pool")

	// Record initial balances and delegation
	initialUserBalance := helper.App.BankKeeper.GetBalance(ctx, delegatorAddr, amount.Denom)
	initialPoolBalance := helper.App.BankKeeper.GetBalance(ctx, delegatePoolAddr, amount.Denom)

	// Get initial delegation shares
	var initialDelegationShares sdkmath.LegacyDec
	delegation, err := helper.App.StakingKeeper.GetDelegation(ctx, delegatorAddr, validatorAddr)
	if err == nil {
		initialDelegationShares = delegation.Shares
	} else {
		initialDelegationShares = sdkmath.LegacyZeroDec()
	}

	return initialUserBalance, initialPoolBalance, initialDelegationShares
}

// Helper function to submit wrapped delegate message
func submitWrappedDelegate(t *testing.T, helper *testhelper.Helper, ctx sdk.Context, delegatorAddr sdk.AccAddress, validatorAddr sdk.ValAddress, amount sdk.Coin, shouldLockFunds bool) {
	wrappedMsg := types.NewMsgWrappedDelegate(
		stakingtypes.NewMsgDelegate(
			delegatorAddr.String(),
			validatorAddr.String(),
			amount,
		),
	)

	if shouldLockFunds {
		_, err := helper.MsgSrvr.WrappedDelegate(ctx, wrappedMsg)
		require.NoError(t, err, "WrappedDelegate should succeed when funds should be locked")
	} else {
		// Only enqueue message without locking funds
		txId := []byte("test-tx-id-no-lock")
		queuedMsg, err := types.NewQueuedMessage(uint64(ctx.BlockHeight()), ctx.BlockTime(), txId, wrappedMsg)
		require.NoError(t, err)
		helper.App.EpochingKeeper.EnqueueMsg(ctx, queuedMsg)
	}

	// Verify message was enqueued
	epochMsgs := helper.App.EpochingKeeper.GetCurrentEpochMsgs(ctx)
	require.Len(t, epochMsgs, 1, "One message should be enqueued")
}

// Helper function to move to epoch boundary and execute EndBlocker
func executeEndBlocker(t *testing.T, helper *testhelper.Helper, ctx sdk.Context, corruptMessage bool) sdk.Context {
	// Move to epoch boundary context
	epoch := helper.App.EpochingKeeper.GetEpoch(ctx)
	blkHeader := ctx.BlockHeader()
	blkHeader.Time = ctx.BlockTime().Add(time.Hour * 25)
	ctx = ctx.WithBlockHeader(blkHeader)

	info := ctx.HeaderInfo()
	info.Height = int64(epoch.GetLastBlockHeight())
	info.Time = blkHeader.Time
	ctx = ctx.WithHeaderInfo(info)

	ctx = sdk.NewContext(ctx.MultiStore(), ctx.BlockHeader(), ctx.IsCheckTx(), ctx.Logger()).WithHeaderInfo(info)

	if corruptMessage {
		// Get current messages and corrupt them
		currentMsgs := helper.App.EpochingKeeper.GetCurrentEpochMsgs(ctx)
		require.Len(t, currentMsgs, 1, "Should have exactly one queued message")

		// Clear queue and add corrupted message
		helper.App.EpochingKeeper.InitMsgQueue(ctx)

		// Create corrupted message (non-existent validator for execution failure)
		corruptedMsg := *currentMsgs[0]
		if delegateMsg := corruptedMsg.GetMsgDelegate(); delegateMsg != nil {
			// Create validator address that will pass unlock but fail execution
			nonExistentValAddr := sdk.ValAddress(make([]byte, 20))
			delegateMsg.ValidatorAddress = nonExistentValAddr.String()
			corruptedMsg.Msg = &types.QueuedMessage_MsgDelegate{MsgDelegate: delegateMsg}
		}
		helper.App.EpochingKeeper.EnqueueMsg(ctx, corruptedMsg)
	}

	// Use actual EndBlocker to verify continue logic
	_, err := epoching.EndBlocker(ctx, helper.App.EpochingKeeper)
	require.NoError(t, err, "EndBlocker should not return error even if individual messages fail")

	return ctx
}

// Helper function to verify final state for unlock/execution scenarios
func verifyFinalState(t *testing.T, helper *testhelper.Helper, ctx sdk.Context, delegatorAddr sdk.AccAddress, validatorAddr sdk.ValAddress, amount sdk.Coin, initialUserBalance, initialPoolBalance sdk.Coin, initialDelegationShares sdkmath.LegacyDec, expectUnlockErr, expectMessageExecErr bool) {
	// Get delegate pool address
	delegatePoolAddr := helper.App.AccountKeeper.GetModuleAddress("epoching_delegate_pool")

	// Check final balances
	finalUserBalance := helper.App.BankKeeper.GetBalance(ctx, delegatorAddr, amount.Denom)
	finalPoolBalance := helper.App.BankKeeper.GetBalance(ctx, delegatePoolAddr, amount.Denom)

	switch {
	case expectUnlockErr:
		// If unlock failed, pool balance should remain the same (no funds were locked)
		require.Equal(t, initialPoolBalance.Amount, finalPoolBalance.Amount,
			"Pool balance should remain unchanged when unlock fails")
		// User balance should remain unchanged too
		require.Equal(t, initialUserBalance.Amount, finalUserBalance.Amount,
			"User balance should remain unchanged when unlock fails")
	case expectMessageExecErr:
		// If unlock succeeded but execution failed, funds should be returned (automatic refund)
		require.Equal(t, finalUserBalance.Amount, initialUserBalance.Amount,
			"User balance should return to initial level due to automatic refund after execution failure")
		require.Equal(t, finalPoolBalance.Amount, initialPoolBalance.Amount,
			"Pool balance should return to initial level due to automatic refund")
	default:
		// If both unlock and execution succeeded, funds should be used for delegation
		// User balance should remain at locked level (funds transferred to staking module)
		lockedBalance := initialUserBalance.Amount.Sub(amount.Amount)
		require.Equal(t, finalUserBalance.Amount, lockedBalance,
			"User balance should remain at locked level after successful delegation (funds used for staking)")
	}

	// Check delegation changes to verify execution behavior
	var finalDelegationShares sdkmath.LegacyDec
	delegation, err := helper.App.StakingKeeper.GetDelegation(ctx, delegatorAddr, validatorAddr)
	if err == nil {
		finalDelegationShares = delegation.Shares
	} else {
		finalDelegationShares = sdkmath.LegacyZeroDec()
	}

	// Verify execution behavior based on unlock success/failure
	switch {
	case expectUnlockErr:
		// Case 2: Unlock failed → EndBlocker continue → execution skipped
		require.True(t, initialDelegationShares.Equal(finalDelegationShares),
			"Delegation shares should remain unchanged when unlock fails (execution skipped by EndBlocker continue)")
	case expectMessageExecErr:
		// Case 3: Unlock succeeded but execution failed → no delegation created
		require.True(t, initialDelegationShares.Equal(finalDelegationShares),
			"Delegation shares should remain unchanged when execution fails (despite unlock success)")
	default:
		// Case 1: Both unlock and execution succeeded → delegation created
		require.True(t, finalDelegationShares.GT(initialDelegationShares),
			"Delegation shares should increase when both unlock and execution succeed")
	}
}

// Helper function to verify WrappedDelegate result and balances
func verifyWrappedDelegateResult(t *testing.T, helper *testhelper.Helper, ctx sdk.Context,
	delegatorAddr sdk.AccAddress, amount sdk.Coin, wrappedMsg *types.MsgWrappedDelegate, expectError bool, description string) {
	// Get initial balances
	initialBalance := helper.App.BankKeeper.GetBalance(ctx, delegatorAddr, "ubbn")
	delegatePoolAddr := helper.App.AccountKeeper.GetModuleAddress(types.DelegatePoolModuleName)
	initialPoolBalance := helper.App.BankKeeper.GetBalance(ctx, delegatePoolAddr, "ubbn")

	// Execute WrappedDelegate (validation → enqueue → lock)
	_, err := helper.MsgSrvr.WrappedDelegate(ctx, wrappedMsg)

	if expectError {
		require.Error(t, err, description+" - WrappedDelegate should fail")

		// Verify balances are unchanged due to atomic failure
		finalBalance := helper.App.BankKeeper.GetBalance(ctx, delegatorAddr, "ubbn")
		finalPoolBalance := helper.App.BankKeeper.GetBalance(ctx, delegatePoolAddr, "ubbn")
		require.Equal(t, initialBalance.Amount, finalBalance.Amount, "Balance should not change when WrappedDelegate fails")
		require.Equal(t, initialPoolBalance.Amount, finalPoolBalance.Amount, "Pool balance should not change when WrappedDelegate fails")

		// Verify message queue status
		epochMsgs := helper.App.EpochingKeeper.GetCurrentEpochMsgs(ctx)
		require.Len(t, epochMsgs, 0, "No messages should be enqueued when WrappedDelegate fails")
	} else {
		// WrappedDelegate should succeed
		require.NoError(t, err, description+" - WrappedDelegate should succeed")

		// Verify funds were locked
		balanceAfterLock := helper.App.BankKeeper.GetBalance(ctx, delegatorAddr, "ubbn")
		expectedBalanceAfterLock := initialBalance.Amount.Sub(amount.Amount)
		require.Equal(t, expectedBalanceAfterLock, balanceAfterLock.Amount, "Funds should be locked after WrappedDelegate")

		// Verify pool received the funds
		poolBalanceAfterLock := helper.App.BankKeeper.GetBalance(ctx, delegatePoolAddr, "ubbn")
		expectedPoolBalanceAfterLock := initialPoolBalance.Amount.Add(amount.Amount)
		require.Equal(t, expectedPoolBalanceAfterLock, poolBalanceAfterLock.Amount, "Pool should have received the locked funds")

		// Verify message was enqueued
		epochMsgs := helper.App.EpochingKeeper.GetCurrentEpochMsgs(ctx)
		require.Len(t, epochMsgs, 1, "One message should be enqueued")
	}
}

func TestLockFunds_WrappedDelegate(t *testing.T) {
	helper, ctx := setupTestEnvironment(t)

	delegatorAddr := helper.GenAccs[0].GetAddress()
	validatorAddr := sdk.ValAddress(delegatorAddr)
	amount := sdk.NewCoin("ubbn", sdkmath.NewInt(100000))

	// Create delegate message
	delegateMsg := stakingtypes.NewMsgDelegate(
		delegatorAddr.String(),
		validatorAddr.String(),
		amount,
	)

	// Create QueuedMessage for delegate
	msgId := []byte("test-msg-id")
	queuedMsg := &types.QueuedMessage{
		MsgId:       msgId,
		TxId:        []byte("test-tx-id"),
		BlockHeight: uint64(ctx.BlockHeight()),
		BlockTime:   &[]time.Time{ctx.BlockTime()}[0],
		Msg: &types.QueuedMessage_MsgDelegate{
			MsgDelegate: delegateMsg,
		},
	}

	// Get initial balance
	initialBalance := helper.App.BankKeeper.GetBalance(ctx, delegatorAddr, "ubbn")

	// Get delegate pool balance before
	delegatePoolAddr := helper.App.AccountKeeper.GetModuleAddress(types.DelegatePoolModuleName)
	poolBalanceBefore := helper.App.BankKeeper.GetBalance(ctx, delegatePoolAddr, "ubbn")

	// Test LockFundsForDelegateMsgs
	err := helper.App.EpochingKeeper.LockFundsForDelegateMsgs(ctx, queuedMsg)
	require.NoError(t, err)

	// Verify user balance decreased
	userBalanceAfter := helper.App.BankKeeper.GetBalance(ctx, delegatorAddr, "ubbn")
	expectedUserBalance := initialBalance.Amount.Sub(amount.Amount)
	require.Equal(t, expectedUserBalance, userBalanceAfter.Amount, "User balance should decrease by locked amount")

	// Verify pool balance increased
	poolBalanceAfter := helper.App.BankKeeper.GetBalance(ctx, delegatePoolAddr, "ubbn")
	expectedPoolBalance := poolBalanceBefore.Amount.Add(amount.Amount)
	require.Equal(t, expectedPoolBalance, poolBalanceAfter.Amount, "Pool balance should increase by locked amount")

	// Test UnlockFundsForDelegateMsgs
	err = helper.App.EpochingKeeper.UnlockFundsForDelegateMsgs(ctx, queuedMsg)
	require.NoError(t, err)

	// Verify user balance restored
	userBalanceFinal := helper.App.BankKeeper.GetBalance(ctx, delegatorAddr, "ubbn")
	require.Equal(t, initialBalance.Amount, userBalanceFinal.Amount, "User balance should be restored after unlock")

	// Verify pool balance restored
	poolBalanceFinal := helper.App.BankKeeper.GetBalance(ctx, delegatePoolAddr, "ubbn")
	require.Equal(t, poolBalanceBefore.Amount, poolBalanceFinal.Amount, "Pool balance should be restored after unlock")
}

func TestLockFunds_WrappedCreateValidator(t *testing.T) {
	helper, ctx := setupTestEnvironment(t)

	validatorAddr := sdk.ValAddress(helper.GenAccs[0].GetAddress())
	amount := sdk.NewCoin("ubbn", sdkmath.NewInt(1000000))

	// Generate validator data
	valPubKey := ed25519.GenPrivKey().PubKey()
	description := stakingtypes.NewDescription("test-validator", "", "", "", "")
	commission := stakingtypes.NewCommissionRates(
		sdkmath.LegacyMustNewDecFromStr("0.1"),
		sdkmath.LegacyMustNewDecFromStr("1.0"),
		sdkmath.LegacyMustNewDecFromStr("0.01"),
	)

	// Create validator message
	createValidatorMsg, err := stakingtypes.NewMsgCreateValidator(
		validatorAddr.String(),
		valPubKey,
		amount,
		description,
		commission,
		sdkmath.OneInt(),
	)
	require.NoError(t, err)

	// Create QueuedMessage for create validator
	msgId := []byte("test-validator-msg-id")
	queuedMsg := &types.QueuedMessage{
		MsgId:       msgId,
		TxId:        []byte("test-validator-tx-id"),
		BlockHeight: uint64(ctx.BlockHeight()),
		BlockTime:   &[]time.Time{ctx.BlockTime()}[0],
		Msg: &types.QueuedMessage_MsgCreateValidator{
			MsgCreateValidator: createValidatorMsg,
		},
	}

	// Get initial balance
	validatorAccAddr := sdk.AccAddress(validatorAddr)
	initialBalance := helper.App.BankKeeper.GetBalance(ctx, validatorAccAddr, "ubbn")

	// Get delegate pool balance before
	delegatePoolAddr := helper.App.AccountKeeper.GetModuleAddress(types.DelegatePoolModuleName)
	poolBalanceBefore := helper.App.BankKeeper.GetBalance(ctx, delegatePoolAddr, "ubbn")

	// Test LockFunds
	err = helper.App.EpochingKeeper.LockFundsForDelegateMsgs(ctx, queuedMsg)
	require.NoError(t, err)

	// Verify validator balance decreased
	validatorBalanceAfter := helper.App.BankKeeper.GetBalance(ctx, validatorAccAddr, "ubbn")
	expectedValidatorBalance := initialBalance.Amount.Sub(amount.Amount)
	require.Equal(t, expectedValidatorBalance, validatorBalanceAfter.Amount, "Validator balance should decrease by locked amount")

	// Verify pool balance increased
	poolBalanceAfter := helper.App.BankKeeper.GetBalance(ctx, delegatePoolAddr, "ubbn")
	expectedPoolBalance := poolBalanceBefore.Amount.Add(amount.Amount)
	require.Equal(t, expectedPoolBalance, poolBalanceAfter.Amount, "Pool balance should increase by locked amount")

	// Test UnlockFundsForDelegateMsgs
	err = helper.App.EpochingKeeper.UnlockFundsForDelegateMsgs(ctx, queuedMsg)
	require.NoError(t, err)

	// Verify validator balance restored
	validatorBalanceFinal := helper.App.BankKeeper.GetBalance(ctx, validatorAccAddr, "ubbn")
	require.Equal(t, initialBalance.Amount, validatorBalanceFinal.Amount, "Validator balance should be restored after unlock")

	// Verify pool balance restored
	poolBalanceFinal := helper.App.BankKeeper.GetBalance(ctx, delegatePoolAddr, "ubbn")
	require.Equal(t, poolBalanceBefore.Amount, poolBalanceFinal.Amount, "Pool balance should be restored after unlock")
}

func TestLockFunds_UnsupportedMessageType(t *testing.T) {
	helper, ctx := setupTestEnvironment(t)

	delegatorAddr := helper.GenAccs[0].GetAddress()
	validatorAddr := sdk.ValAddress(delegatorAddr)
	amount := sdk.NewCoin("ubbn", sdkmath.NewInt(100000))

	// Create undelegate message (should not require fund locking)
	undelegateMsg := stakingtypes.NewMsgUndelegate(
		delegatorAddr.String(),
		validatorAddr.String(),
		amount,
	)

	// Create QueuedMessage for undelegate
	msgId := []byte("test-undelegate-msg-id")
	queuedMsg := &types.QueuedMessage{
		MsgId:       msgId,
		TxId:        []byte("test-undelegate-tx-id"),
		BlockHeight: uint64(ctx.BlockHeight()),
		BlockTime:   &[]time.Time{ctx.BlockTime()}[0],
		Msg: &types.QueuedMessage_MsgUndelegate{
			MsgUndelegate: undelegateMsg,
		},
	}

	// Get initial balances
	initialUserBalance := helper.App.BankKeeper.GetBalance(ctx, delegatorAddr, "ubbn")
	delegatePoolAddr := helper.App.AccountKeeper.GetModuleAddress(types.DelegatePoolModuleName)
	initialPoolBalance := helper.App.BankKeeper.GetBalance(ctx, delegatePoolAddr, "ubbn")

	// Test LockFundsForDelegateMsgs - should not lock funds for unsupported message types
	err := helper.App.EpochingKeeper.LockFundsForDelegateMsgs(ctx, queuedMsg)
	require.NoError(t, err) // Should not error, but should do nothing

	// Verify no balance changes
	userBalanceAfter := helper.App.BankKeeper.GetBalance(ctx, delegatorAddr, "ubbn")
	poolBalanceAfter := helper.App.BankKeeper.GetBalance(ctx, delegatePoolAddr, "ubbn")

	require.Equal(t, initialUserBalance.Amount, userBalanceAfter.Amount, "User balance should not change for unsupported message")
	require.Equal(t, initialPoolBalance.Amount, poolBalanceAfter.Amount, "Pool balance should not change for unsupported message")

	// Test UnlockFundsForDelegateMsgs - should also not error
	err = helper.App.EpochingKeeper.UnlockFundsForDelegateMsgs(ctx, queuedMsg)
	require.NoError(t, err) // Should not error, but should do nothing
}

func TestLockFundsError_InsufficientBalance(t *testing.T) {
	helper, ctx := setupTestEnvironment(t)

	delegatorAddr := helper.GenAccs[0].GetAddress()
	validatorAddr := sdk.ValAddress(delegatorAddr)

	// Get current balance and try to delegate more than available
	currentBalance := helper.App.BankKeeper.GetBalance(ctx, delegatorAddr, "ubbn")
	excessiveAmount := sdk.NewCoin("ubbn", currentBalance.Amount.Add(sdkmath.NewInt(1000000)))

	// Create delegate message with excessive amount
	delegateMsg := stakingtypes.NewMsgDelegate(
		delegatorAddr.String(),
		validatorAddr.String(),
		excessiveAmount,
	)

	// Create QueuedMessage
	msgId := []byte("test-excessive-msg-id")
	queuedMsg := &types.QueuedMessage{
		MsgId:       msgId,
		TxId:        []byte("test-excessive-tx-id"),
		BlockHeight: uint64(ctx.BlockHeight()),
		BlockTime:   &[]time.Time{ctx.BlockTime()}[0],
		Msg: &types.QueuedMessage_MsgDelegate{
			MsgDelegate: delegateMsg,
		},
	}

	// Test LockFundsForDelegateMsgs - should fail due to insufficient balance
	err := helper.App.EpochingKeeper.LockFundsForDelegateMsgs(ctx, queuedMsg)
	require.Error(t, err, "LockFunds should fail when user has insufficient balance")
	require.Contains(t, err.Error(), "failed to lock funds for msg", "Error should mention fund locking failure")
}

func TestLockUnlockFunds_InvalidAddress(t *testing.T) {
	helper, ctx := setupTestEnvironment(t)

	amount := sdk.NewCoin("ubbn", sdkmath.NewInt(100000))

	// Create delegate message with invalid address
	delegateMsg := &stakingtypes.MsgDelegate{
		DelegatorAddress: "invalid-address",
		ValidatorAddress: "invalid-validator",
		Amount:           amount,
	}

	// Create QueuedMessage
	msgId := []byte("test-invalid-addr-msg-id")
	queuedMsg := &types.QueuedMessage{
		MsgId:       msgId,
		TxId:        []byte("test-invalid-addr-tx-id"),
		BlockHeight: uint64(ctx.BlockHeight()),
		BlockTime:   &[]time.Time{ctx.BlockTime()}[0],
		Msg: &types.QueuedMessage_MsgDelegate{
			MsgDelegate: delegateMsg,
		},
	}

	// Test LockFundsForDelegateMsgs - should fail due to invalid address
	err := helper.App.EpochingKeeper.LockFundsForDelegateMsgs(ctx, queuedMsg)
	require.Error(t, err, "LockFunds should fail with invalid delegator address")

	// Test UnlockFundsForDelegateMsgs - should also fail due to invalid address
	err = helper.App.EpochingKeeper.UnlockFundsForDelegateMsgs(ctx, queuedMsg)
	require.Error(t, err, "UnLockFunds should fail with invalid delegator address")
}

func TestLockUnlockFunds_CreateValidatorInvalidAddress(t *testing.T) {
	helper, ctx := setupTestEnvironment(t)

	amount := sdk.NewCoin("ubbn", sdkmath.NewInt(1000000))
	valPubKey := ed25519.GenPrivKey().PubKey()
	description := stakingtypes.NewDescription("test", "", "", "", "")
	commission := stakingtypes.NewCommissionRates(
		sdkmath.LegacyMustNewDecFromStr("0.1"),
		sdkmath.LegacyMustNewDecFromStr("1.0"),
		sdkmath.LegacyMustNewDecFromStr("0.01"),
	)

	// Create validator message with invalid address
	createValidatorMsg := &stakingtypes.MsgCreateValidator{
		Description:       description,
		Commission:        commission,
		MinSelfDelegation: sdkmath.OneInt(),
		ValidatorAddress:  "invalid-validator-address",
		Pubkey:            nil,
		Value:             amount,
	}
	var err error
	createValidatorMsg.Pubkey, err = codectypes.NewAnyWithValue(valPubKey)
	require.NoError(t, err)

	// Create QueuedMessage
	msgId := []byte("test-invalid-val-msg-id")
	queuedMsg := &types.QueuedMessage{
		MsgId:       msgId,
		TxId:        []byte("test-invalid-val-tx-id"),
		BlockHeight: uint64(ctx.BlockHeight()),
		BlockTime:   &[]time.Time{ctx.BlockTime()}[0],
		Msg: &types.QueuedMessage_MsgCreateValidator{
			MsgCreateValidator: createValidatorMsg,
		},
	}

	// Test LockFundsForDelegateMsgs - should fail due to invalid validator address
	err = helper.App.EpochingKeeper.LockFundsForDelegateMsgs(ctx, queuedMsg)
	require.Error(t, err, "LockFunds should fail with invalid validator address")

	// Test UnlockFundsForDelegateMsgs - should also fail due to invalid validator address
	err = helper.App.EpochingKeeper.UnlockFundsForDelegateMsgs(ctx, queuedMsg)
	require.Error(t, err, "UnLockFunds should fail with invalid validator address")
}

func TestIntegrationUnlockMessageExecution_UnlockSuccessExecutionSuccess(t *testing.T) {
	helper, ctx := setupTestEnvironment(t)
	delegatorAddr, validatorAddr := getExistingValidator(t, helper, ctx)
	amount := sdk.NewCoin("ubbn", sdkmath.NewInt(100000))

	// Get initial state
	initialUserBalance, initialPoolBalance, initialDelegationShares := getInitialState(helper, ctx, delegatorAddr, validatorAddr, amount)

	// Submit wrapped delegate message with fund locking
	submitWrappedDelegate(t, helper, ctx, delegatorAddr, validatorAddr, amount, true)

	// Execute EndBlocker (no message corruption)
	ctx = executeEndBlocker(t, helper, ctx, false)

	// Verify final state - both unlock and execution should succeed
	verifyFinalState(t, helper, ctx, delegatorAddr, validatorAddr, amount,
		initialUserBalance, initialPoolBalance, initialDelegationShares, false, false)
}

func TestIntegrationUnlockMessageExecution_UnlockFailExecutionSkip(t *testing.T) {
	helper, ctx := setupTestEnvironment(t)
	delegatorAddr, validatorAddr := getExistingValidator(t, helper, ctx)
	amount := sdk.NewCoin("ubbn", sdkmath.NewInt(100000))

	// Get initial state
	initialUserBalance, initialPoolBalance, initialDelegationShares := getInitialState(helper, ctx, delegatorAddr, validatorAddr, amount)

	// Submit wrapped delegate message without fund locking
	submitWrappedDelegate(t, helper, ctx, delegatorAddr, validatorAddr, amount, false)

	// Execute EndBlocker (no message corruption)
	ctx = executeEndBlocker(t, helper, ctx, false)

	// Verify final state - unlock should fail, execution should be skipped
	verifyFinalState(t, helper, ctx, delegatorAddr, validatorAddr, amount,
		initialUserBalance, initialPoolBalance, initialDelegationShares, true, false)
}

func TestIntegrationUnlockMessageExecution_UnlockSuccessExecutionFailAutomaticRefund(t *testing.T) {
	helper, ctx := setupTestEnvironment(t)
	delegatorAddr, validatorAddr := getExistingValidator(t, helper, ctx)
	amount := sdk.NewCoin("ubbn", sdkmath.NewInt(100000))

	// Get initial state
	initialUserBalance, initialPoolBalance, initialDelegationShares := getInitialState(helper, ctx, delegatorAddr, validatorAddr, amount)

	// Submit wrapped delegate message with fund locking
	submitWrappedDelegate(t, helper, ctx, delegatorAddr, validatorAddr, amount, true)

	// Execute EndBlocker with message corruption to cause execution failure
	ctx = executeEndBlocker(t, helper, ctx, true)

	// Verify final state - unlock should succeed, but execution should fail with automatic refund
	verifyFinalState(t, helper, ctx, delegatorAddr, validatorAddr, amount,
		initialUserBalance, initialPoolBalance, initialDelegationShares, false, true)
}

func TestIntegrationLockUnlock_ValidationEnqueueLockAllSuccess(t *testing.T) {
	helper, ctx := setupTestEnvironment(t)
	delegatorAddr := helper.GenAccs[0].GetAddress()
	validatorAddr := sdk.ValAddress(delegatorAddr)
	amount := sdk.NewCoin("ubbn", sdkmath.NewInt(50000000))

	wrappedMsg := types.NewMsgWrappedDelegate(
		stakingtypes.NewMsgDelegate(
			delegatorAddr.String(),
			validatorAddr.String(),
			amount,
		),
	)

	verifyWrappedDelegateResult(t, helper, ctx, delegatorAddr, amount, wrappedMsg, false,
		"Case 1: validation → enqueue → lock all success")
}

func TestIntegrationLockUnlock_ValidationFailMinimumAmount(t *testing.T) {
	helper, ctx := setupTestEnvironment(t)
	delegatorAddr := helper.GenAccs[0].GetAddress()
	validatorAddr := sdk.ValAddress(delegatorAddr)
	zeroAmount := sdk.NewCoin("ubbn", sdkmath.NewInt(0))

	wrappedMsg := types.NewMsgWrappedDelegate(
		stakingtypes.NewMsgDelegate(
			delegatorAddr.String(),
			validatorAddr.String(),
			zeroAmount,
		),
	)

	verifyWrappedDelegateResult(t, helper, ctx, delegatorAddr, zeroAmount, wrappedMsg, true,
		"Case 2: validation fail (minimum amount) → no enqueue → no lock")
}

func TestIntegrationLockUnlock_InsufficientBalanceLockFail(t *testing.T) {
	helper, ctx := setupTestEnvironment(t)
	delegatorAddr := helper.GenAccs[0].GetAddress()
	validatorAddr := sdk.ValAddress(delegatorAddr)

	// Get current balance and try to delegate more than available
	currentBalance := helper.App.BankKeeper.GetBalance(ctx, delegatorAddr, "ubbn")
	excessiveAmount := sdk.NewCoin("ubbn", currentBalance.Amount.Add(sdkmath.NewInt(1000000)))

	wrappedMsg := types.NewMsgWrappedDelegate(
		stakingtypes.NewMsgDelegate(
			delegatorAddr.String(),
			validatorAddr.String(),
			excessiveAmount,
		),
	)

	verifyWrappedDelegateResult(t, helper, ctx, delegatorAddr, excessiveAmount, wrappedMsg, true,
		"Case 3: validation pass → lock fail → no enqueue (atomic failure)")
}

func TestIntegrationLockUnlock_InvalidDenomValidationFail(t *testing.T) {
	helper, ctx := setupTestEnvironment(t)
	delegatorAddr := helper.GenAccs[0].GetAddress()
	validatorAddr := sdk.ValAddress(delegatorAddr)
	invalidDenomAmount := sdk.NewCoin("invalid-denom", sdkmath.NewInt(50000000))

	wrappedMsg := types.NewMsgWrappedDelegate(
		stakingtypes.NewMsgDelegate(
			delegatorAddr.String(),
			validatorAddr.String(),
			invalidDenomAmount,
		),
	)

	verifyWrappedDelegateResult(t, helper, ctx, delegatorAddr, invalidDenomAmount, wrappedMsg, true,
		"Case 4: invalid denomination → validation fail → no enqueue → no lock")
}
