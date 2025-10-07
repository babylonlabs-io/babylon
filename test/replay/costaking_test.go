package replay

import (
	"bytes"
	"encoding/hex"
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/math"
	sdkmath "cosmossdk.io/math"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	costktypes "github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	distkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	disttypes "github.com/cosmos/cosmos-sdk/x/distribution/types"

	"github.com/stretchr/testify/require"
)

var zeroInt = sdkmath.ZeroInt()

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

	// check that baby delegation reached costaking
	zero := sdkmath.ZeroInt()
	d.CheckCostakerRewards(del1.Address(), del1BabyDelegatedAmt, zero, zero, rwd.Period)

	fps := d.CreateNFinalityProviderAccounts(2)
	fp1 := fps[0]
	for _, fp := range fps {
		fp.RegisterFinalityProvider()
	}
	d.GenerateNewBlockAssertExecutionSuccess()

	p := costkK.GetParams(d.Ctx())
	// costaking ratio of btc by baby is 200, so for every sat staked it needs to
	// have 200 baby staked to take full account of the btcs in the score.
	del1BtcStakedAmt := del1BabyDelegatedAmt.Quo(p.ScoreRatioBtcByBaby)
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

// TestCostakingFpSlasheAndBtcUnbondSameBlockPreventsDoubleSatsRemoval tests the specific case where
// an FP becomes slashed and a BTC delegation is unbonded in the same block, ensuring satoshis are not removed twice.
func TestCostakingFpSlashedAndBtcUnbondSameBlockPreventsDoubleSatsRemoval(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlockAssertExecutionSuccess()

	stkK, costkK := d.App.StakingKeeper, d.App.CostakingKeeper
	covSender := d.CreateCovenantSender()

	// Get the validator
	validators, err := stkK.GetAllValidators(d.Ctx())
	require.NoError(t, err)
	val := validators[0]
	valAddr := sdk.MustValAddressFromBech32(val.OperatorAddress)

	// Create a delegator and delegate baby tokens
	delegators := d.CreateNStakerAccounts(1)
	del1 := delegators[0]
	del1BabyDelegatedAmt := sdkmath.NewInt(20_000000)

	d.MintNativeTo(del1.Address(), 100_000000)
	d.TxWrappedDelegate(del1.SenderInfo, valAddr.String(), del1BabyDelegatedAmt)

	d.GenerateNewBlockAssertExecutionSuccess()
	d.ProgressTillFirstBlockTheNextEpoch()

	// Create and register an FP
	fps := d.CreateNFinalityProviderAccounts(1)
	fp1 := fps[0]
	fp1.RegisterFinalityProvider()
	d.GenerateNewBlockAssertExecutionSuccess()

	p := costkK.GetParams(d.Ctx())

	// Create BTC delegation
	del1BtcStakedAmt := del1BabyDelegatedAmt.Quo(p.ScoreRatioBtcByBaby)
	del1.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
		defaultStakingTime,
		del1BtcStakedAmt.Int64(),
	)
	d.GenerateNewBlockAssertExecutionSuccess()

	// Activate the BTC delegation
	covSender.SendCovenantSignatures()
	d.GenerateNewBlockAssertExecutionSuccess()

	d.ActivateVerifiedDelegations(1)
	activeDelegations := d.GetActiveBTCDelegations(t)
	require.Len(t, activeDelegations, 1)

	// Activate the FP by committing randomness and finalizing checkpoints
	fp1.CommitRandomness()
	d.GenerateNewBlockAssertExecutionSuccess()

	currentEpoch := d.GetEpoch().EpochNumber
	d.ProgressTillFirstBlockTheNextEpoch()
	d.FinalizeCkptForEpoch(currentEpoch - 1)
	d.FinalizeCkptForEpoch(currentEpoch)

	d.GenerateNewBlockAssertExecutionSuccess()

	// Verify FP is now active
	activeFps := d.GetActiveFpsAtCurrentHeight(t)
	require.Len(t, activeFps, 1)

	// Check that costaker rewards are properly set with active sats
	currRwd, err := costkK.GetCurrentRewards(d.Ctx())
	require.NoError(t, err)
	d.CheckCostakerRewards(del1.Address(), del1BabyDelegatedAmt, del1BtcStakedAmt, del1BtcStakedAmt, currRwd.Period-1)

	// Now create the critical scenario:
	// 1. FP becomes slashed
	// 2. BTC delegation is unbonded
	// In the same babylon block and check if the sats were not removed twice

	// Get the active delegation details for unbonding
	activation := activeDelegations[0]
	stakingTx := &wire.MsgTx{}
	txBytes, err := hex.DecodeString(activation.StakingTxHex)
	require.NoError(t, err)
	err = stakingTx.Deserialize(bytes.NewReader(txBytes))
	require.NoError(t, err)
	stakingTxHash := stakingTx.TxHash()

	// Prepare both operations to happen in the same block:
	// 1. FP sends slashing evidence (makes FP slashed/inactive)
	fp1.SendSelectiveSlashingEvidence()

	// 2. Delegator unbonds their delegation (removes delegation)
	del1.UnbondDelegation(&stakingTxHash, stakingTx, covSender)

	// Process voting power distribution and finality hooks
	d.GenerateNewBlockAssertExecutionSuccess()

	slashedFp := d.GetFp(*fp1.BTCPublicKey())
	require.True(t, slashedFp.IsSlashed(), "FP should be slashed")

	unbondedDels := d.GetUnbondedBTCDelegations(t)
	require.Len(t, unbondedDels, 1, "Delegation should be unbonded")

	// Check the critical part: costaker active satoshis should be zero (removed once)
	// and NOT negative (which would indicate double removal)
	zero := sdkmath.ZeroInt()
	d.CheckCostakerRewards(del1.Address(), del1BabyDelegatedAmt, zero, zero, currRwd.Period)
}

// TestCostakingFpVotingPowerLossAndBtcUnbondSameBlockPreventsDoubleSatsRemoval tests the specific case where
// an FP becomes inactive due to losing voting power (being pushed out of the active set) and a BTC delegation
// is unbonded, ensuring satoshis are not removed twice. Also it creates an btc delegation to an already active
// finality provider
func TestCostakingFpVotingPowerLossAndBtcUnbondSameBlockPreventsDoubleSatsRemoval(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlockAssertExecutionSuccess()

	stkK, costkK, finalityK := d.App.StakingKeeper, d.App.CostakingKeeper, d.App.FinalityKeeper
	covSender := d.CreateCovenantSender()

	// Set finality parameters to allow only 1 active FP to test voting power competition
	fParams := finalityK.GetParams(d.Ctx())
	fParams.MaxActiveFinalityProviders = 1
	err := finalityK.SetParams(d.Ctx(), fParams)
	require.NoError(t, err)

	validators, err := stkK.GetAllValidators(d.Ctx())
	require.NoError(t, err)
	val := validators[0]
	valAddr := sdk.MustValAddressFromBech32(val.OperatorAddress)

	delegators := d.CreateNStakerAccounts(3)
	del1 := delegators[0]
	del1BabyDelegatedAmt := sdkmath.NewInt(20_000000)

	d.MintNativeTo(del1.Address(), 100_000000)
	d.TxWrappedDelegate(del1.SenderInfo, valAddr.String(), del1BabyDelegatedAmt)

	d.GenerateNewBlockAssertExecutionSuccess()
	d.ProgressTillFirstBlockTheNextEpoch()

	fps := d.CreateNFinalityProviderAccounts(2)
	fp1, fp2 := fps[0], fps[1]
	fp1.RegisterFinalityProvider()
	fp2.RegisterFinalityProvider()
	d.GenerateNewBlockAssertExecutionSuccess()

	// Create BTC delegations for both FPs
	// FP1 gets a smaller delegation initially
	p := costkK.GetParams(d.Ctx())
	del1BtcStakedAmtFp1 := del1BabyDelegatedAmt.Quo(p.ScoreRatioBtcByBaby)
	del1MsgCreate := del1.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
		defaultStakingTime,
		del1BtcStakedAmtFp1.Int64(),
	)
	d.GenerateNewBlockAssertExecutionSuccess()

	// Activate the BTC delegation for FP1
	covSender.SendCovenantSignatures()
	d.GenerateNewBlockAssertExecutionSuccess()

	d.ActivateVerifiedDelegations(1)
	activeDelegations := d.GetActiveBTCDelegations(t)
	require.Len(t, activeDelegations, 1)

	// Activate FP1 by committing randomness and finalizing checkpoints
	fp1.CommitRandomness()
	d.GenerateNewBlockAssertExecutionSuccess()

	currentEpoch := d.GetEpoch().EpochNumber
	d.ProgressTillFirstBlockTheNextEpoch()
	d.FinalizeCkptForEpoch(currentEpoch - 1)
	d.FinalizeCkptForEpoch(currentEpoch)

	d.GenerateNewBlockAssertExecutionSuccess()

	// Verify FP1 is now active (since it's the only one with voting power)
	activeFps := d.GetActiveFpsAtCurrentHeight(t)
	require.Len(t, activeFps, 1)
	require.True(t, activeFps[0].BtcPkHex.Equals(fp1.BTCPublicKey()), "FP1 should be the active FP")

	// Check that costaker rewards are properly set with active sats for FP1
	currRwd, err := costkK.GetCurrentRewards(d.Ctx())
	require.NoError(t, err)
	d.CheckCostakerRewards(del1.Address(), del1BabyDelegatedAmt, del1BtcStakedAmtFp1, del1BtcStakedAmtFp1, currRwd.Period-1)

	// Create another btc delegation with half of del1 voting power to fp1
	del2 := delegators[1]
	del2BtcStakedAmtFp1 := del1BtcStakedAmtFp1.QuoRaw(2) // 200k sats
	del2.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
		defaultStakingTime,
		del2BtcStakedAmtFp1.Int64(),
	)
	d.GenerateNewBlockAssertExecutionSuccess()
	covSender.SendCovenantSignatures()
	d.GenerateNewBlockAssertExecutionSuccess()

	d.ActivateVerifiedDelegations(1)
	activeDelegations = d.GetActiveBTCDelegations(t)
	require.Len(t, activeDelegations, 2)
	d.GenerateNewBlockAssertExecutionSuccess()

	d.CheckCostakerRewards(del2.Address(), zeroInt, del2BtcStakedAmtFp1, zeroInt, currRwd.Period)

	// Now create a larger delegation for fp2 that will push FP1 out of the active set
	// Create a new delegator with more voting power
	del3 := delegators[2]
	del3BabyDelegatedAmt := del1BabyDelegatedAmt.MulRaw(2)
	d.MintNativeTo(del3.Address(), 100_000000)
	d.TxWrappedDelegate(del3.SenderInfo, valAddr.String(), del3BabyDelegatedAmt)
	d.GenerateNewBlockAssertExecutionSuccess()

	// Create larger BTC delegation for fp2 to active fp2 and inactive fp1
	del3BtcStakedAmtFp2 := del3BabyDelegatedAmt.Quo(p.ScoreRatioBtcByBaby)
	del3.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{fp2.BTCPublicKey()},
		defaultStakingTime,
		del3BtcStakedAmtFp2.Int64(),
	)
	d.GenerateNewBlockAssertExecutionSuccess()

	// Activate fp2's delegation
	covSender.SendCovenantSignatures()
	d.GenerateNewBlockAssertExecutionSuccess()

	d.ActivateVerifiedDelegations(1)
	activeDelegations = d.GetActiveBTCDelegations(t)
	require.Len(t, activeDelegations, 3)

	fp2.CommitRandomness()
	d.GenerateNewBlockAssertExecutionSuccess()

	currentEpoch = d.GetEpoch().EpochNumber
	d.ProgressTillFirstBlockTheNextEpoch()
	d.FinalizeCkptForEpoch(currentEpoch - 1)
	d.FinalizeCkptForEpoch(currentEpoch)

	d.GenerateNewBlockAssertExecutionSuccess()

	// Verify the active FP state after finalization
	// fp2 should now be the only active FP since it has more voting power (800k > 600k)
	// and MaxActiveFinalityProviders = 1, so FP1 should become inactive
	activeFpsAfterFp2 := d.GetActiveFpsAtCurrentHeight(t)
	require.Len(t, activeFpsAfterFp2, 1, "Should have exactly 1 active FP due to MaxActiveFinalityProviders = 1")
	currentlyActiveFp := activeFpsAfterFp2[0].BtcPkHex.MarshalHex()
	expectedActiveFp := fp2.BTCPublicKey().MarshalHex()
	require.Equal(t, expectedActiveFp, currentlyActiveFp, "fp2 should be active due to higher voting power, FP1 should be inactive")

	// Now test the critical scenario: unbond FP1's delegation while FP1 is inactive
	// This tests the double removal prevention logic for inactive FPs
	stakingTx := &wire.MsgTx{}
	err = stakingTx.Deserialize(bytes.NewReader(del1MsgCreate.StakingTx))
	require.NoError(t, err)
	stakingTxHash := stakingTx.TxHash()

	// Unbond FP1's delegation while FP1 is inactive (due to fp2 taking over)
	del1.UnbondDelegation(&stakingTxHash, stakingTx, covSender)
	d.GenerateNewBlockAssertExecutionSuccess()
	unbondedDels := d.GetUnbondedBTCDelegations(t)
	require.Len(t, unbondedDels, 1, "FP1's delegation should be unbonded")

	// fp2 should still be the only active FP
	finalActiveFps := d.GetActiveFpsAtCurrentHeight(t)
	require.Len(t, finalActiveFps, 1, "Should still have 1 active FP")
	require.Equal(t, expectedActiveFp, finalActiveFps[0].BtcPkHex.MarshalHex(), "fp2 should remain active")

	// Check the critical part: FP1's (del1, del2) should have zero active satoshis
	// and NOT negative (which would indicate double removal)
	d.CheckCostakerRewards(del1.Address(), del1BabyDelegatedAmt, zeroInt, zeroInt, currRwd.Period+1)
	d.CheckCostakerRewards(del2.Address(), zeroInt, zeroInt, zeroInt, currRwd.Period) // period is not updated as this guy never had score

	// fp2's delegator should still have their correct active satoshis (unaffected)
	d.CheckCostakerRewards(del3.Address(), del3BabyDelegatedAmt, del3BtcStakedAmtFp2, del3BtcStakedAmtFp2, currRwd.Period)
}

func TestMainnetInflationDistributionAmount(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlockAssertExecutionSuccess()
	d.GenerateNewBlockAssertExecutionSuccess()

	// example with 100 ubbn to be distributed
	// 1. incentives 2. costaking 3. distribution

	// (1 / 5.5) ≈ 0.181818182 of total inflation goes to btc stakers
	// percentageBtcStakers * 100 ubbn ≈ 18.181818 ubbn

	// (0.075 / 5.5) ≈ 0.013636364 of total inflation goes to fp directly
	// percentageFpDirect * 100 ubbn ≈ 1.3636364 ubbn

	// 2. costaking
	// 100 ubbn - (btc stakers + fp direct) ≈ 81.4545456 ubbn

	// (2.35 / 5.5) ≈ 0.427272727 of total inflation to costakers
	// percentageCostakers * remaining ubbn ≈ 43.016949153 ubbn

	// (0.075 / 5.5) ≈ 0.013636364 of remaining inflation goes to baby validators
	// percentageBabyValidators * remaining ubbn ≈ 1.372881356 ubbn

	// rest goes to baby stakers and validators ≈ 36.619414447 ubbn
	inflation := sdkmath.LegacyMustNewDecFromStr("5.5")

	percentageBtcStakers := sdkmath.LegacyMustNewDecFromStr("1").Quo(inflation)
	require.Equal(t, "0.181818181818181818", percentageBtcStakers.String())

	percentageFpDirect := sdkmath.LegacyMustNewDecFromStr("0.075").Quo(inflation)
	require.Equal(t, "0.013636363636363636", percentageFpDirect.String())

	percentageCostakers := sdkmath.LegacyMustNewDecFromStr("2.35").Quo(inflation)
	require.Equal(t, "0.427272727272727273", percentageCostakers.String())

	percentageBabyValDirect := sdkmath.LegacyMustNewDecFromStr("0.075").Quo(inflation)
	require.Equal(t, "0.013636363636363636", percentageBabyValDirect.String())

	dstrModAcc := authtypes.NewModuleAddress(disttypes.ModuleName)
	dstrModBalancesBefore := d.App.BankKeeper.GetAllBalances(d.Ctx(), dstrModAcc)

	block := d.GenerateNewBlock()
	amountMinted := FindEventMint(t, block.Events)

	dstrModBalancesAfter := d.App.BankKeeper.GetAllBalances(d.Ctx(), dstrModAcc)

	expectedBtcStaker, _ := sdk.NewDecCoinsFromCoins(amountMinted...).MulDecTruncate(percentageBtcStakers).TruncateDecimal()
	actualBtcStakers := FindEventBtcStakers(t, block.Events)
	require.False(t, actualBtcStakers.IsZero())
	require.Equal(t, expectedBtcStaker.String(), actualBtcStakers.String())

	expectedFpDirect, _ := sdk.NewDecCoinsFromCoins(amountMinted...).MulDecTruncate(percentageFpDirect).TruncateDecimal()
	actualFpDirect := FindEventTypeFPDirectRewards(t, block.Events)
	require.False(t, actualFpDirect.IsZero())
	require.Equal(t, expectedFpDirect.String(), actualFpDirect.String())

	expectedCostakers, _ := sdk.NewDecCoinsFromCoins(amountMinted...).MulDecTruncate(percentageCostakers).TruncateDecimal()
	actualCostakers := FindEventCostakerRewards(t, block.Events)
	require.False(t, actualCostakers.IsZero())
	require.Equal(t, expectedCostakers.String(), actualCostakers.String())

	expectedBabyVal, _ := sdk.NewDecCoinsFromCoins(amountMinted...).MulDecTruncate(percentageBabyValDirect).TruncateDecimal()
	actualBabyVals := FindEventTypeValidatorDirectRewards(t, block.Events)
	require.False(t, actualBabyVals.IsZero())
	require.Equal(t, expectedBabyVal.String(), actualBabyVals.String())

	// baby vals are not subtracted here, as the amount are transferred to the distribution module account as well
	expectedDistributionModule := amountMinted.Sub(expectedBtcStaker...).Sub(expectedFpDirect...).Sub(expectedCostakers...)
	actualDistributionModule := dstrModBalancesAfter.Sub(dstrModBalancesBefore...)
	require.False(t, actualDistributionModule.IsZero())
	require.Equal(t, expectedDistributionModule.String(), actualDistributionModule.String())
}

// TestCostakingRewardsUnbondAllBaby creates 1 fp and 1 btc delegation and a one baby delegations
// getting rewards and later unbonding all this baby delegation, it will call the staking hook
// BeforeDelegationRemoved.
func TestCostakingRewardsUnbondAllBaby(t *testing.T) {
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

	delegators := d.CreateNStakerAccounts(1)
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

	// check that baby delegation reached costaking
	zero := sdkmath.ZeroInt()
	d.CheckCostakerRewards(del1.Address(), del1BabyDelegatedAmt, zero, zero, rwd.Period)

	fps := d.CreateNFinalityProviderAccounts(1)
	fp1 := fps[0]
	fp1.RegisterFinalityProvider()
	d.GenerateNewBlockAssertExecutionSuccess()

	p := costkK.GetParams(d.Ctx())
	// costaking ratio of btc by baby is 200, so for every sat staked it needs to
	// have 200 baby staked to take full account of the btcs in the score.
	del1BtcStakedAmt := del1BabyDelegatedAmt.Quo(p.ScoreRatioBtcByBaby)
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

	// reduces unbonding time
	stkP, err := stkK.GetParams(d.Ctx())
	require.NoError(t, err)
	stkP.UnbondingTime = time.Second * 20
	err = stkK.SetParams(d.Ctx(), stkP)
	require.NoError(t, err)

	d.TxWrappedUndelegate(del1.SenderInfo, valAddr.String(), del1BabyDelegatedAmt)
	d.ProgressTillFirstBlockTheNextEpoch()

	for i := 0; i < 10; i++ {
		d.GenerateNewBlockAssertExecutionSuccess()
	}

	d.CheckCostakerRewards(del1.Address(), zero, del1BtcStakedAmt, zero, currentRwdPeriod+1)
}

// TestCostakingRewardsWithdraw creates 1 fp and 1 btc delegation and a one baby delegation
// getting rewards and later stop voting in btc staking so it doesn't earn rewards
// and continue to earn rewards in costaking
func TestCostakingRewardsWithdraw(t *testing.T) {
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

	delegators := d.CreateNStakerAccounts(1)
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

	// check that baby delegation reached costaking
	zero := sdkmath.ZeroInt()
	d.CheckCostakerRewards(del1.Address(), del1BabyDelegatedAmt, zero, zero, rwd.Period)

	fps := d.CreateNFinalityProviderAccounts(1)
	fp1 := fps[0]
	fp1.RegisterFinalityProvider()
	d.GenerateNewBlockAssertExecutionSuccess()

	p := costkK.GetParams(d.Ctx())
	// costaking ratio of btc by baby is 200, so for every sat staked it needs to
	// have 200 baby staked to take full account of the btcs in the score.
	del1BtcStakedAmt := del1BabyDelegatedAmt.Quo(p.ScoreRatioBtcByBaby)
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
		Type:    ictvtypes.BTC_STAKER.String(),
		Address: del1.AddressString(),
	})
	require.NoError(t, err)
	require.Equal(t, resp.Coins.String(), costakerRewadsTwoBlocks.String())

	del1BalancesAfterRewardWithdraw := d.App.BankKeeper.GetAllBalances(d.Ctx(), del1.Address())
	diff := del1BalancesAfterRewardWithdraw.Sub(del1BalancesBeforeRewardWithdraw...).String()
	require.Equal(t, diff, costakerRewadsTwoBlocks.String())

	// checks with query that there is no reward for btc staker, only costaking
	costakerRewadsOneBlock := d.GenerateNewBlockAssertExecutionSuccessWithCostakerRewards()
	rwds, err := d.App.IncentiveKeeper.RewardGauges(d.Ctx(), &ictvtypes.QueryRewardGaugesRequest{
		Address: del1.AddressString(),
	})
	require.NoError(t, err)
	costk := rwds.RewardGauges[ictvtypes.COSTAKER.String()]
	_, existBtcRewards := rwds.RewardGauges[ictvtypes.BTC_STAKER.String()]
	require.False(t, existBtcRewards)
	require.Equal(t, costk.Coins.Sub(costk.WithdrawnCoins...).String(), costakerRewadsOneBlock.String())

	// withdraws the costaker rewards without btc staking rewards
	resp, err = d.MsgServerIncentive().WithdrawReward(d.Ctx(), &ictvtypes.MsgWithdrawReward{
		Type:    ictvtypes.BTC_STAKER.String(),
		Address: del1.AddressString(),
	})
	require.NoError(t, err)
	require.Equal(t, resp.Coins.String(), costakerRewadsOneBlock.String())
}
