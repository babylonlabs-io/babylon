package replay

import (
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	distkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	disttypes "github.com/cosmos/cosmos-sdk/x/distribution/types"

	"github.com/stretchr/testify/require"
)

// TestCoostakingValidatorDirectRewards tests the intercept_fee_collector logic
// by generating blocks and verifying that validators receive direct rewards from both
// minted tokens and existing fee collector balance
func TestCoostakingValidatorDirectRewards(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlockAssertExecutionSuccess()

	// Get necessary keepers
	coostakingK := d.App.CoostakingKeeper
	distributionK := d.App.DistrKeeper
	stakingK := d.App.StakingKeeper
	bankK := d.App.BankKeeper

	ctx := d.Ctx()

	// Get all validators to check their commissions
	validators, err := stakingK.GetAllValidators(ctx)
	require.NoError(t, err)
	require.Len(t, validators, 1, "should have one validator")

	// First, withdraw all existing validator commission and rewards to start with clean state
	for _, validator := range validators {
		valAddr, err := sdk.ValAddressFromBech32(validator.OperatorAddress)
		require.NoError(t, err)

		// Withdraw validator commission
		_, err = distributionK.WithdrawValidatorCommission(ctx, valAddr)
		if err != nil {
			require.ErrorContains(t, err, disttypes.ErrNoValidatorCommission.Error())
		}

		// Withdraw delegator rewards (if any self-delegation exists)
		delAddr := sdk.AccAddress(valAddr)
		_, err = distributionK.WithdrawDelegationRewards(ctx, delAddr, valAddr)
		require.NoError(t, err)
	}

	// Verify validators have zero outstanding rewards and commission after withdrawal
	for _, validator := range validators {
		valAddr, err := sdk.ValAddressFromBech32(validator.OperatorAddress)
		require.NoError(t, err)

		// Check outstanding rewards are zero or minimal
		rewards, err := distributionK.GetValidatorOutstandingRewards(ctx, valAddr)
		require.NoError(t, err)
		require.Empty(t, rewards.Rewards)

		// Check commission is zero or minimal
		commission, err := distributionK.GetValidatorAccumulatedCommission(ctx, valAddr)
		require.NoError(t, err)
		require.Empty(t, commission.Commission)
	}

	feeCollectorAddr := d.App.AccountKeeper.GetModuleAddress("fee_collector")
	distrModuleAddr := d.App.AccountKeeper.GetModuleAddress(disttypes.ModuleName)

	// Get initial coostaking module balance
	coostakingModuleAddr := d.App.AccountKeeper.GetModuleAddress("coostaking")
	initialCoostakingBalance := bankK.GetAllBalances(ctx, coostakingModuleAddr)

	// Add some existing fees to fee collector (simulating accumulated transaction fees)
	existingFees := sdk.NewCoins(sdk.NewCoin("ubbn", math.NewInt(50000000))) // 50 BBN
	err = bankK.MintCoins(ctx, "mint", existingFees)
	require.NoError(t, err)
	err = bankK.SendCoinsFromModuleToModule(ctx, "mint", "fee_collector", existingFees)
	require.NoError(t, err)

	// Record fee collector balance before block generation
	preBlockFeeCollectorBalance := bankK.GetAllBalances(ctx, feeCollectorAddr)

	// Generate a new block - this will trigger:
	// 1. Minting new tokens (added to fee collector)
	// 2. BeginBlock -> HandleCoinsInFeeCollector
	// 3. Distribution of fees according to ValidatorsPortion and CoostakingPortion
	d.GenerateNewBlockAssertExecutionSuccess()

	// Get new context after block generation
	ctx = d.Ctx()

	// Check final balances and rewards
	finalFeeCollectorBalance := bankK.GetAllBalances(ctx, feeCollectorAddr)
	finalCoostakingBalance := bankK.GetAllBalances(ctx, coostakingModuleAddr)

	// all fee collector balance is distributed
	require.True(t, finalFeeCollectorBalance.IsZero(), "Expected all fee collector balance to be distributed, but got: %s", finalFeeCollectorBalance.String())

	distQuerier := distkeeper.NewQuerier(distributionK)
	// Get coostaking parameters
	params := coostakingK.GetParams(ctx)
	// calculate expected validator commission based on params
	preBlockFCBal := sdk.NewDecCoinsFromCoins(preBlockFeeCollectorBalance...)
	// There's only one validator, so the extra commission goes to that one
	expValComm := preBlockFCBal.MulDecTruncate(params.ValidatorsPortion)
	require.True(t, expValComm.IsAllPositive(), "Expected positive validator commission, got: %s", expValComm.String())
	// Check that validators received commission after block generation
	for _, validator := range validators {
		valAddr, err := sdk.ValAddressFromBech32(validator.OperatorAddress)
		require.NoError(t, err)

		// Withdraw commission after block generation
		commissionRewards, err := distributionK.WithdrawValidatorCommission(ctx, valAddr)
		if err != nil {
			t.Logf("No commission to withdraw for validator %s after block: %v", validator.OperatorAddress, err)
			continue
		}

		// Check that withdrawn commission is at least the expected amount
		diff := sdk.NewDecCoinsFromCoins(commissionRewards...).Sub(expValComm)
		require.True(t, diff.IsAllPositive(), diff.String())

		// Check that there're some outstanding rewards for delegator
		delAddr := sdk.AccAddress(valAddr)
		rewards, err := distQuerier.DelegationRewards(ctx, &disttypes.QueryDelegationRewardsRequest{
			DelegatorAddress: delAddr.String(),
			ValidatorAddress: valAddr.String(),
		})
		require.NoError(t, err)
		require.NotNil(t, rewards)
		require.True(t, rewards.Rewards.IsAllPositive(), "Expected some delegator rewards, got: %s", rewards.Rewards.String())

		// distribution module should only have the remaining rewards
		// and community pool funds
		feePool, err := distributionK.FeePool.Get(ctx)
		require.NoError(t, err)
		distModBalance := bankK.GetAllBalances(ctx, distrModuleAddr)
		distModDecCoins := sdk.NewDecCoinsFromCoins(distModBalance...)
		diffCoins, _ := distModDecCoins.Sub(rewards.Rewards).Sub(feePool.CommunityPool).TruncateDecimal()
		require.True(t, diffCoins.IsZero(), diffCoins.String())
	}

	// Verify that coostaking module balance increased
	coostakingIncrease := finalCoostakingBalance.Sub(initialCoostakingBalance...)
	require.True(t, coostakingIncrease.IsAllPositive())
}
