package replay

import (
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/math"
	sdkmath "cosmossdk.io/math"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	costktypes "github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	distkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	disttypes "github.com/cosmos/cosmos-sdk/x/distribution/types"

	"github.com/stretchr/testify/require"
)

// TestCostakingValidatorDirectRewards tests the intercept_fee_collector logic
// by generating blocks and verifying that validators receive direct rewards from both
// minted tokens and existing fee collector balance
func TestCostakingValidatorDirectRewards(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlockAssertExecutionSuccess()

	// Get necessary keepers
	costakingK := d.App.CostakingKeeper
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

	// Get initial costaking module balance
	costakingModuleAddr := d.App.AccountKeeper.GetModuleAddress("costaking")
	initialCostakingBalance := bankK.GetAllBalances(ctx, costakingModuleAddr)

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
	// 3. Distribution of fees according to ValidatorsPortion and CostakingPortion
	d.GenerateNewBlockAssertExecutionSuccess()

	// Get new context after block generation
	ctx = d.Ctx()

	// Check final balances and rewards
	finalFeeCollectorBalance := bankK.GetAllBalances(ctx, feeCollectorAddr)
	finalCostakingBalance := bankK.GetAllBalances(ctx, costakingModuleAddr)

	// all fee collector balance is distributed
	require.True(t, finalFeeCollectorBalance.IsZero(), "Expected all fee collector balance to be distributed, but got: %s", finalFeeCollectorBalance.String())

	distQuerier := distkeeper.NewQuerier(distributionK)
	// Get costaking parameters
	params := costakingK.GetParams(ctx)
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

	// Verify that costaking module balance increased
	costakingIncrease := finalCostakingBalance.Sub(initialCostakingBalance...)
	require.True(t, costakingIncrease.IsAllPositive())
}

// TestCostakingRewardsHappyCase creates 2 fps and 3 btc delegations and a few baby delegations
// checking all the expected rewards are available in the coostaker reward tracker.
func TestCostakingRewardsHappyCase(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlockAssertExecutionSuccess()

	stkK, costkK := d.App.StakingKeeper, d.App.CostakingKeeper

	params := costkK.GetParams(d.Ctx())
	require.Equal(t, params, costktypes.DefaultParams())

	// Get all validators to check their commissions
	validators, err := stkK.GetAllValidators(d.Ctx())
	require.NoError(d.t, err)
	val := validators[0]
	valAddr := sdk.MustValAddressFromBech32(val.OperatorAddress)
	covSender := d.CreateCovenantSender()

	delegators := d.CreateNStakerAccounts(3)
	del1 := delegators[0]
	del1BabyDelegatedAmt := sdkmath.NewInt(20_000000)

	d.MintNativeTo(del1.Address(), 100_000000)
	d.TxWrappedDelegate(del1.SenderInfo, valAddr.String(), del1BabyDelegatedAmt)

	// gets the current rewards prior to the end of epoch as it will be starting point
	rwd, err := costkK.GetCurrentRewards(d.Ctx())
	require.NoError(t, err)

	d.GenerateNewBlockAssertExecutionSuccess()
	d.ProgressTillFirstBlockTheNextEpoch()

	// confirms that baby delegation was done properly
	del, err := stkK.GetDelegation(d.Ctx(), del1.Address(), valAddr)
	require.NoError(t, err)
	require.Equal(t, del.DelegatorAddress, del1.Address().String())

	// check that baby delegation reached coostaking
	zero := sdkmath.ZeroInt()
	d.CheckCostakerRewards(del1.Address(), del1BabyDelegatedAmt, zero, zero, rwd.Period)

	fps := d.CreateNFinalityProviderAccounts(2)
	fp1 := fps[0]
	for _, fp := range fps {
		fp.RegisterFinalityProvider("")
	}
	d.GenerateNewBlockAssertExecutionSuccess()

	// costaking ratio of btc by baby is 50, so for every sat staked it needs to
	// have 50 baby staked to take full account of the btcs in the score.
	del1BtcStakedAmt := del1BabyDelegatedAmt.QuoRaw(50)
	del1.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
		defaultStakingTime,
		del1BtcStakedAmt.Int64(),
	)
	d.GenerateNewBlockAssertExecutionSuccess()

	covSender.SendCovenantSignatures()
	d.GenerateNewBlockAssertExecutionSuccess()

	verifiedDels := d.GetVerifiedBTCDelegations(t)
	require.Len(t, verifiedDels, 1)

	d.ActivateVerifiedDelegations(1)
	activeDelegations := d.GetActiveBTCDelegations(t)
	require.Len(t, activeDelegations, 1)

	activeFps := d.GetActiveFpsAtCurrentHeight(d.t)
	require.Len(t, activeFps, 0)

	// zero active sats and score, because fp is not active
	d.CheckCostakerRewards(del1.Address(), del1BabyDelegatedAmt, zero, zero, rwd.Period)

	// activate fp
	fp1.CommitRandomness()
	d.GenerateNewBlockAssertExecutionSuccess()

	// Randomness timestamped
	currentEpoch := d.GetEpoch().EpochNumber
	d.ProgressTillFirstBlockTheNextEpoch()
	d.FinalizeCkptForEpoch(currentEpoch - 1) // previous unfinalized epoch
	d.FinalizeCkptForEpoch(currentEpoch)

	rwd, err = costkK.GetCurrentRewards(d.Ctx())
	require.NoError(t, err)

	// produce block to activate fp
	d.GenerateNewBlockAssertExecutionSuccess()

	// fp should be activated
	activeFps = d.GetActiveFpsAtCurrentHeight(d.t)
	require.Len(t, activeFps, 1)

	// score is the same as btc staked as del1 have 50 ubbn to each sat
	del1StartCumulativeRewardPeriod := rwd.Period
	d.CheckCostakerRewards(del1.Address(), del1BabyDelegatedAmt, del1BtcStakedAmt, del1BtcStakedAmt, del1StartCumulativeRewardPeriod)

	// new period without rewards is created
	d.CheckCostakingCurrentRewards(sdk.NewCoins(), rwd.Period+1, del1BtcStakedAmt)
	// historical will not have any rewards, because costaker didn't participated until the fp become active and no other block
	// was produced to add rewards.
	d.CheckCostakingCurrentHistoricalRewards(del1StartCumulativeRewardPeriod, sdk.NewCoins())

	// produce 2 blocks to add rewards to coostaker
	costakerRewadsTwoBlocks := sdk.NewCoins()
	costakerRewadsTwoBlocks = costakerRewadsTwoBlocks.Add(d.GenerateNewBlockAssertExecutionSuccessWithCostakerRewards()...)
	costakerRewadsTwoBlocks = costakerRewadsTwoBlocks.Add(d.GenerateNewBlockAssertExecutionSuccessWithCostakerRewards()...)
	currentRwdPeriod := rwd.Period + 1
	d.CheckCostakingCurrentRewards(costakerRewadsTwoBlocks, currentRwdPeriod, del1BtcStakedAmt)

	del1BalancesBeforeRewardWithdraw := d.App.BankKeeper.GetAllBalances(d.Ctx(), del1.Address())

	resp, err := d.MsgServerIncentive().WithdrawReward(d.Ctx(), &ictvtypes.MsgWithdrawReward{
		Type:    ictvtypes.COSTAKER.String(),
		Address: del1.AddressString(),
	})
	require.NoError(t, err)
	require.Equal(t, resp.Coins.String(), costakerRewadsTwoBlocks.String())

	del1BalancesAfterRewardWithdraw := d.App.BankKeeper.GetAllBalances(d.Ctx(), del1.Address())
	diff := del1BalancesAfterRewardWithdraw.Sub(del1BalancesBeforeRewardWithdraw...).String()
	require.Equal(t, diff, costakerRewadsTwoBlocks.String())

	// after withdraw of rewards the period must increase
	del1StartCumulativeRewardPeriod++
	currentRwdPeriod++
	d.CheckCostakerRewards(del1.Address(), del1BabyDelegatedAmt, del1BtcStakedAmt, del1BtcStakedAmt, del1StartCumulativeRewardPeriod)
	d.CheckCostakingCurrentRewards(sdk.NewCoins(), currentRwdPeriod, del1BtcStakedAmt)

	fp2 := fps[1]
	del2, del3 := delegators[1], delegators[2]
	// del1 (400000sats, 20_000000ubbn) = 400000score
	// del2 (300000sats, 20_000000ubbn) = 300000score
	// del3 (300000sats, 10_000000ubbn) = 200000score
	del2BtcStakedAmt := sdkmath.NewInt(300_000)
	del3BtcStakedAmt := del2BtcStakedAmt

	del2.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
		defaultStakingTime,
		del2BtcStakedAmt.Int64(),
	)

	del3.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
		defaultStakingTime,
		del3BtcStakedAmt.Int64(),
	)

	// nothing should change yet in the rewards and score
	costakerCumulativeRewads := sdk.NewCoins()
	d.CheckCostakingCurrentRewards(costakerCumulativeRewads, currentRwdPeriod, del1BtcStakedAmt)

	// activate btc delegations del2 and del3
	costakerCumulativeRewads = costakerCumulativeRewads.Add(d.GenerateNewBlockAssertExecutionSuccessWithCostakerRewards()...)
	covSender.SendCovenantSignatures()
	costakerCumulativeRewads = costakerCumulativeRewads.Add(d.GenerateNewBlockAssertExecutionSuccessWithCostakerRewards()...)

	blocksResults := d.ActivateVerifiedDelegations(2)
	activeDelegations = d.GetActiveBTCDelegations(t)
	require.Len(t, activeDelegations, 3)
	costakerCumulativeRewads = costakerCumulativeRewads.Add(EventCostakerRewardsFromBlocks(d.t, blocksResults)...)

	d.CheckCostakingCurrentRewards(costakerCumulativeRewads, currentRwdPeriod, del1BtcStakedAmt)

	// activate fp 2
	fp2.CommitRandomness()
	d.GenerateNewBlockAssertExecutionSuccess()
}
