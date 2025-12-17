package replay

import (
	"bytes"
	"encoding/hex"
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	costktypes "github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	"github.com/btcsuite/btcd/wire"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	distkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	disttypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"

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
	existingFees := sdk.NewCoins(sdk.NewCoin("ubbn", sdkmath.NewInt(50000000))) // 50 BBN
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

// TestCostakingBabyBondUnbondAllBondAgain creates one baby delegation it unbonds in the same block
// and bond it again with an different value all in the same block
func TestCostakingBabyBondUnbondAllBondAgain(t *testing.T) {
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

	delegators := d.CreateNStakerAccounts(1)
	del1 := delegators[0]
	del1BabyDelegatedAmt := sdkmath.NewInt(20_000000)

	d.MintNativeTo(del1.Address(), 100_000000)
	d.TxWrappedDelegate(del1.SenderInfo, valAddr.String(), del1BabyDelegatedAmt)
	d.GenerateNewBlockAssertExecutionSuccess()

	// gets the current rewards prior to the end of epoch as it will be starting point
	rwd, err := costkK.GetCurrentRewards(d.Ctx())
	require.NoError(t, err)

	// goes until end of epoch
	d.ProgressTillFirstBlockTheNextEpoch()

	// confirms that baby delegation was done properly
	del, err := stkK.GetDelegation(d.Ctx(), del1.Address(), valAddr)
	require.NoError(t, err)
	require.Equal(t, del.DelegatorAddress, del1.Address().String())

	// check that baby delegation reached costaking
	zero := sdkmath.ZeroInt()
	d.CheckCostakerRewards(del1.Address(), del1BabyDelegatedAmt, zero, zero, rwd.Period)

	d.TxWrappedUndelegate(del1.SenderInfo, valAddr.String(), del1BabyDelegatedAmt)

	del1BabyDelegatedAmtAgain := sdkmath.NewInt(35_000000)
	d.TxWrappedDelegate(del1.SenderInfo, valAddr.String(), del1BabyDelegatedAmtAgain)

	d.ProgressTillFirstBlockTheNextEpoch()

	// confirms that baby delegation is still there
	del, err = stkK.GetDelegation(d.Ctx(), del1.Address(), valAddr)
	require.NoError(t, err)
	require.Equal(t, del.DelegatorAddress, del1.Address().String())
	require.Equal(t, del.Shares.TruncateInt().String(), del1BabyDelegatedAmtAgain.String())

	// verify that the amount of active baby is the second amount staked
	d.CheckCostakerRewards(del1.Address(), del1BabyDelegatedAmtAgain, zero, zero, rwd.Period)
	// period doesn't change as the delegator has zero score
}

// TestBabyCoStaking creates 2 validators and jails one
// Performs delegations to the jailed validator and makes corresponding checks
func TestBabyCoStaking(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlockAssertExecutionSuccess()

	stkK, costkK, slashK, epochK := d.App.StakingKeeper, d.App.CostakingKeeper, d.App.SlashingKeeper, d.App.EpochingKeeper

	stkParams, err := stkK.GetParams(d.Ctx())
	require.NoError(d.t, err)
	maxVals := 3
	stkParams.MaxValidators = uint32(maxVals)

	err = stkK.SetParams(d.Ctx(), stkParams)
	require.NoError(d.t, err)

	d.GenerateNewBlockAssertExecutionSuccess()
	stkParams, err = stkK.GetParams(d.Ctx())
	require.NoError(d.t, err)
	require.Equal(d.t, maxVals, int(stkParams.MaxValidators))

	// Get all validators to check their commissions
	validators, err := stkK.GetAllValidators(d.Ctx())
	require.NoError(d.t, err)
	require.Len(d.t, validators, 1, "There should be exactly one validator in the test setup")
	val1 := validators[0]
	var val1ConsPubKey cryptotypes.PubKey
	err = util.Cdc.UnpackAny(val1.ConsensusPubkey, &val1ConsPubKey)
	require.NoError(d.t, err)

	// get val1 self delegation
	val1ValAddr := sdk.MustValAddressFromBech32(val1.OperatorAddress)
	val1AccAddr := sdk.AccAddress(val1ValAddr)
	val1Dels, err := stkK.GetValidatorDelegations(d.Ctx(), val1ValAddr)
	require.NoError(d.t, err)
	require.Len(d.t, val1Dels, 1, "There should be exactly one delegation for the initial validator")
	val1SelfDel := val1Dels[0]
	require.Equal(d.t, val1SelfDel.DelegatorAddress, val1AccAddr.String(), "The delegation should be the self-delegation of the initial validator")
	val1SelfDelAmt := val1SelfDel.Shares.TruncateInt()

	val1Tracker, err := costkK.GetCostakerRewards(d.Ctx(), val1AccAddr)
	require.NoError(t, err)
	require.NotNil(t, val1Tracker)
	require.Equal(t, val1Tracker.ActiveBaby, val1SelfDelAmt, "active baby should be self delegation amount", val1Tracker.ActiveBaby.String())
	require.True(t, val1Tracker.ActiveSatoshis.IsZero(), "Active sats should be zero")
	require.True(t, val1Tracker.TotalScore.IsZero())

	delegators := d.CreateNStakerAccounts(11)
	val2Oper := delegators[0]
	del2 := delegators[1]
	del3 := delegators[2]
	del4 := delegators[3]
	del5 := delegators[4]
	del6 := delegators[5]
	del7 := delegators[6]
	val3Oper := delegators[7]
	val4Oper := delegators[8]
	val5Oper := delegators[9]
	val6Oper := delegators[10]

	d.MintNativeTo(val2Oper.Address(), 1000_000000)
	d.MintNativeTo(val6Oper.Address(), 1000_000000)

	// Create a new validator
	newValSelfDelegatedAmt := sdkmath.NewInt(10_000000)
	d.TxCreateValidator(val2Oper.SenderInfo, newValSelfDelegatedAmt)

	otherValSelfDelegatedAmt := sdkmath.NewInt(1_000000)
	for i, val := range []*Staker{val3Oper, val4Oper, val5Oper} {
		d.MintNativeTo(val.Address(), 1000_000000)
		d.TxCreateValidator(val.SenderInfo, otherValSelfDelegatedAmt.AddRaw(int64(i)*1_000000))
	}

	d.GenerateNewBlockAssertExecutionSuccess()
	d.ProgressTillFirstBlockTheNextEpoch()

	// Check if new validator is in the list
	validators, err = stkK.GetAllValidators(d.Ctx())
	require.NoError(d.t, err)
	require.Len(d.t, validators, 5, "There should be exactly five validators in the test setup")
	var val2, val4, val5 stktypes.Validator
	var val2ConsPubKey cryptotypes.PubKey
	for _, v := range validators {
		valAddrBz := sdk.MustValAddressFromBech32(v.OperatorAddress)
		if bytes.Equal(valAddrBz.Bytes(), val2Oper.Address().Bytes()) {
			val2 = v
			err := util.Cdc.UnpackAny(v.ConsensusPubkey, &val2ConsPubKey)
			require.NoError(t, err)
		}
		if bytes.Equal(valAddrBz.Bytes(), val4Oper.Address().Bytes()) {
			val4 = v
		}
		if bytes.Equal(valAddrBz.Bytes(), val5Oper.Address().Bytes()) {
			val5 = v
		}
	}
	require.Equal(t, val2.Status, stktypes.Bonded, "New validator should be in Bonded status")

	epoch := epochK.GetEpoch(d.Ctx())
	valset := epochK.GetValidatorSet(d.Ctx(), epoch.EpochNumber)
	require.Len(t, valset, maxVals)
	require.True(t, isValidatorInValset(valset, sdk.MustValAddressFromBech32(val2.OperatorAddress)), "New validator should be in the validator set")
	require.True(t, isValidatorInValset(valset, sdk.MustValAddressFromBech32(val5.OperatorAddress)), "New validator should be in the validator set")

	// new validators should have a costaker tracker created with the self delegation
	val2Tracker, err := costkK.GetCostakerRewards(d.Ctx(), val2Oper.Address())
	require.NoError(t, err)
	require.NotNil(t, val2Tracker)
	require.Equal(t, val2Tracker.ActiveBaby, newValSelfDelegatedAmt, "active baby should be self delegation amount", val2Tracker.ActiveBaby.String())
	require.True(t, val2Tracker.ActiveSatoshis.IsZero(), "Active sats should be zero")
	require.True(t, val2Tracker.TotalScore.IsZero())

	val5Tracker, err := costkK.GetCostakerRewards(d.Ctx(), val5Oper.Address())
	require.NoError(t, err)
	require.NotNil(t, val5Tracker)
	require.Equal(t, val5Tracker.ActiveBaby, otherValSelfDelegatedAmt.AddRaw(2*1_000000), "active baby should be self delegation amount", val5Tracker.ActiveBaby.String())
	require.True(t, val5Tracker.ActiveSatoshis.IsZero(), "Active sats should be zero")
	require.True(t, val5Tracker.TotalScore.IsZero())

	// Others validators outside the active set should not have a costaker tracker created
	_, err = costkK.GetCostakerRewards(d.Ctx(), val3Oper.Address())
	require.ErrorContains(t, err, "not found")
	_, err = costkK.GetCostakerRewards(d.Ctx(), val4Oper.Address())
	require.ErrorContains(t, err, "not found")

	// delegate to new validator (val2)
	del3BabyDelegatedAmtBeforeJailing := sdkmath.NewInt(1_000000)
	d.TxWrappedDelegate(del3.SenderInfo, val2.OperatorAddress, del3BabyDelegatedAmtBeforeJailing)

	del4BabyDelegatedAmt := sdkmath.NewInt(2_000000)
	d.TxWrappedDelegate(del4.SenderInfo, val2.OperatorAddress, del4BabyDelegatedAmt)

	del5BabyDelegatedAmt := sdkmath.NewInt(1_000000)
	d.TxWrappedDelegate(del5.SenderInfo, val2.OperatorAddress, del5BabyDelegatedAmt)

	del6BabyDelegatedAmt := sdkmath.NewInt(1_000000)
	d.TxWrappedDelegate(del6.SenderInfo, val2.OperatorAddress, del6BabyDelegatedAmt)

	del7BabyDelegatedAmt := sdkmath.NewInt(1_000000)
	d.TxWrappedDelegate(del7.SenderInfo, val2.OperatorAddress, del7BabyDelegatedAmt)

	// partial undelegate from val5 (currently in active set) to make it inactive
	// val4 should take its place in the active set
	partialUnbondVal5 := sdkmath.NewInt(3 * 1_000000)
	d.TxWrappedUndelegate(val5Oper.SenderInfo, val5.OperatorAddress, partialUnbondVal5)

	// make a delegation to val4 (currently not in active set but will become active after val5 partial unbonding)
	val4DelAmt := sdkmath.NewInt(1_000000)
	d.TxWrappedDelegate(val4Oper.SenderInfo, val4.OperatorAddress, val4DelAmt)

	d.GenerateNewBlockAssertExecutionSuccess()
	d.ProgressTillFirstBlockTheNextEpoch()

	// check costaking trackers are created accordingly
	del3Tracker, err := costkK.GetCostakerRewards(d.Ctx(), del3.Address())
	require.NoError(t, err)
	require.NotNil(t, del3Tracker)
	require.Equal(t, del3Tracker.ActiveBaby, del3BabyDelegatedAmtBeforeJailing, "active baby should be delegation amount", del3Tracker.ActiveBaby.String())
	require.True(t, del3Tracker.ActiveSatoshis.IsZero())
	require.True(t, del3Tracker.TotalScore.IsZero())

	del4Tracker, err := costkK.GetCostakerRewards(d.Ctx(), del4.Address())
	require.NoError(t, err)
	require.NotNil(t, del4Tracker)
	require.Equal(t, del4Tracker.ActiveBaby, del4BabyDelegatedAmt, "active baby should be delegation amount", del4Tracker.ActiveBaby.String())
	require.True(t, del4Tracker.ActiveSatoshis.IsZero())
	require.True(t, del4Tracker.TotalScore.IsZero())

	del5Tracker, err := costkK.GetCostakerRewards(d.Ctx(), del5.Address())
	require.NoError(t, err)
	require.NotNil(t, del5Tracker)
	require.Equal(t, del5Tracker.ActiveBaby, del5BabyDelegatedAmt, "active baby should be delegation amount", del5Tracker.ActiveBaby.String())
	require.True(t, del5Tracker.ActiveSatoshis.IsZero())
	require.True(t, del5Tracker.TotalScore.IsZero())

	del6Tracker, err := costkK.GetCostakerRewards(d.Ctx(), del6.Address())
	require.NoError(t, err)
	require.NotNil(t, del6Tracker)
	require.Equal(t, del6Tracker.ActiveBaby, del6BabyDelegatedAmt, del6Tracker.ActiveBaby.String())
	require.True(t, del6Tracker.ActiveSatoshis.IsZero())
	require.True(t, del6Tracker.TotalScore.IsZero())

	// Check that val5 dropped from the active set and val4 entered
	epoch = epochK.GetEpoch(d.Ctx())
	valset = epochK.GetValidatorSet(d.Ctx(), epoch.EpochNumber)
	require.Len(t, valset, maxVals)
	require.True(t, isValidatorInValset(valset, sdk.MustValAddressFromBech32(val2.OperatorAddress)), "Validator 2 should be in the validator set")
	require.True(t, isValidatorInValset(valset, sdk.MustValAddressFromBech32(val4.OperatorAddress)), "Validator 4 should be in the validator set")
	require.False(t, isValidatorInValset(valset, sdk.MustValAddressFromBech32(val5.OperatorAddress)), "Validator 5 should not be in the validator set")

	// Check that val5 co-staker tracker is zeroed
	assertZeroCostkTracker(d.t, d.Ctx(), costkK, val5Oper.Address())

	// Check that val4 co-staker tracker is created with self delegation amount
	val4Tracker, err := costkK.GetCostakerRewards(d.Ctx(), val4Oper.Address())
	require.NoError(t, err)
	require.NotNil(t, val4Tracker)
	require.Equal(t, val4Tracker.ActiveBaby, otherValSelfDelegatedAmt.AddRaw(1_000000).Add(val4DelAmt), "active baby should be self delegation amount", val4Tracker.ActiveBaby.String())
	require.True(t, val4Tracker.ActiveSatoshis.IsZero())
	require.True(t, val4Tracker.TotalScore.IsZero())

	// validator 6 is the one that will receive a slashed redelegation and drop active set
	val6SelfDelegatedAmt := sdkmath.NewInt(20_000000)
	val6ValAddr := sdk.ValAddress(val6Oper.SenderInfo.Address())

	val2ValAddr := sdk.MustValAddressFromBech32(val2.OperatorAddress)
	// Produce new blocks till new validator gets jailed for missing blocks
	var height int64
	jailedHeight := int64(0) // validator is jailed at height 111
	for jailedHeight == 0 {
		// begin a redelgation from val2 to val1 one block before jailing

		// create a validator that enters the active set (destination of redelegation to be slashed)
		// create validator epoch before so it is active on the redelegation epoch
		if height == 80 {
			d.TxCreateValidator(val6Oper.SenderInfo, val6SelfDelegatedAmt)
		}

		if height == 100 {
			// On slashing due to downtime, the SlashRedelegation func is called
			// Redelegate to a validator that will remain active
			d.TxWrappedBeginRedelegate(del6.SenderInfo, val2.OperatorAddress, val1.OperatorAddress, del6BabyDelegatedAmt)
			// redelegate to a validator that will drop active set (due to unbonding) on same epoch that jailing is processed
			// Check that the val6 is in active set
			epoch = epochK.GetEpoch(d.Ctx())
			valset = epochK.GetValidatorSet(d.Ctx(), epoch.EpochNumber)
			require.Len(t, valset, maxVals)
			val6, err := stkK.GetValidator(d.Ctx(), val6ValAddr)
			require.NoError(t, err)
			require.Equal(t, stktypes.Bonded, val6.Status, "Validator 6 should be in Bonded status")
			require.True(t, isValidatorInValset(valset, val6ValAddr), "Validator 6 should be in the validator set")
			// redelegate to val6
			d.TxWrappedBeginRedelegate(del7.SenderInfo, val2.OperatorAddress, val6ValAddr.String(), del7BabyDelegatedAmt)
		}
		d.GenerateNewBlockAssertExecutionSuccess()
		height = d.Ctx().BlockHeight()
		val, err := stkK.GetValidator(d.Ctx(), val2ValAddr)
		require.NoError(t, err)
		if val.Jailed {
			jailedHeight = height
			epoch := epochK.GetEpoch(d.Ctx())
			valset := epochK.GetValidatorSet(d.Ctx(), epoch.EpochNumber)
			// check that jailed validator is still on epoch validator set
			require.Len(t, valset, maxVals, "Jailed validator should still be in the validator set")
			require.True(t, isValidatorInValset(valset, val2ValAddr), "Jailed validator should not be in the validator set")
		}
	}

	// Redelegation msg made it to the last epoch block, so it is processed
	// And the redelegation is slashed, so the tracker should be updated accordingly
	_, err = stkK.GetDelegation(d.Ctx(), del6.Address(), val2ValAddr)
	require.Error(d.t, err)
	require.ErrorContains(d.t, err, "no delegation")

	del6Delegation, err := stkK.GetDelegation(d.Ctx(), del6.Address(), val1ValAddr)
	require.NoError(d.t, err)

	val1, err = stkK.GetValidator(d.Ctx(), val1ValAddr)
	require.NoError(d.t, err)
	del6AmtAfterSlashing := val1.TokensFromShares(del6Delegation.Shares).TruncateInt()

	del6Tracker, err = costkK.GetCostakerRewards(d.Ctx(), del6.Address())
	require.NoError(t, err)
	require.NotNil(t, del6Tracker)
	assertActiveBabyWithinRange(t, del6AmtAfterSlashing, del6Tracker.ActiveBaby, 1, "del6 active baby after redelegation slashing (during jailing)")
	require.True(t, del6Tracker.ActiveSatoshis.IsZero())
	require.True(t, del6Tracker.TotalScore.IsZero())

	_, err = stkK.GetDelegation(d.Ctx(), del7.Address(), val2ValAddr)
	require.Error(d.t, err)
	require.ErrorContains(d.t, err, "no delegation")

	del7Delegation, err := stkK.GetDelegation(d.Ctx(), del7.Address(), val6ValAddr)
	require.NoError(d.t, err)

	val6, err := stkK.GetValidator(d.Ctx(), val6ValAddr)
	require.NoError(d.t, err)
	del7AmtAfterSlashing := val6.TokensFromShares(del7Delegation.Shares).TruncateInt()

	del7Tracker, err := costkK.GetCostakerRewards(d.Ctx(), del7.Address())
	require.NoError(t, err)
	require.NotNil(t, del7Tracker)
	assertActiveBabyWithinRange(t, del7AmtAfterSlashing, del7Tracker.ActiveBaby, 1, "del7 active baby after redelegation slashing")
	require.True(t, del7Tracker.ActiveSatoshis.IsZero())
	require.True(t, del7Tracker.TotalScore.IsZero())

	// =================================================
	// OPERATIONS ON SAME EPOCH THAT VALIDATOR IS JAILED
	// =================================================
	val2, err = stkK.GetValidator(d.Ctx(), val2ValAddr)
	require.NoError(d.t, err)

	// Make a NEW delegation to validator
	del2BabyDelegatedAmt := sdkmath.NewInt(1000000)
	d.TxWrappedDelegate(del2.SenderInfo, val2.OperatorAddress, del2BabyDelegatedAmt)

	// Extend the existing del3 delegation
	del3BabyDelegatedAmtAfterJailing := sdkmath.NewInt(500000)
	d.TxWrappedDelegate(del3.SenderInfo, val2.OperatorAddress, del3BabyDelegatedAmtAfterJailing)

	// The first delegation of del3 was slashed. Get the new delegation amount
	del3Delegation, err := stkK.GetDelegation(d.Ctx(), del3.Address(), val2ValAddr)
	require.NoError(d.t, err)
	del3FirstDelAmtAfterSlashing := val2.TokensFromShares(del3Delegation.Shares).TruncateInt()

	// Totally unbond a delegation
	// Tokens are already slashed, so for total unbonding need to get the new tokens per shares
	del4Delegation, err := stkK.GetDelegation(d.Ctx(), del4.Address(), val2ValAddr)
	require.NoError(d.t, err)
	del4TotalUnbondAmt := val2.TokensFromShares(del4Delegation.Shares).TruncateInt()
	d.TxWrappedUndelegate(del4.SenderInfo, val2.OperatorAddress, del4TotalUnbondAmt)

	// get updated delegation amount for del5 after slashing
	del5Delegation, err := stkK.GetDelegation(d.Ctx(), del5.Address(), val2ValAddr)
	require.NoError(d.t, err)
	del5TotalAmtAfterSlashing := val2.TokensFromShares(del5Delegation.Shares).TruncateInt()

	// Partially unbond a delegation with many msgs and re-delegate
	del5BabyUnstakeAmt := sdkmath.NewInt(7)
	d.TxWrappedUndelegate(del5.SenderInfo, val2.OperatorAddress, del5BabyUnstakeAmt)
	d.TxWrappedUndelegate(del5.SenderInfo, val2.OperatorAddress, del5BabyUnstakeAmt)
	d.TxWrappedDelegate(del5.SenderInfo, val2.OperatorAddress, del5BabyUnstakeAmt)

	// new validator should drop active set on same epoch that jailing is processed
	// undelegate here enough tokens to drop active set
	d.TxWrappedUndelegate(val6Oper.SenderInfo, val6ValAddr.String(), val6SelfDelegatedAmt.Sub(sdkmath.NewInt(5000)))

	d.GenerateNewBlockAssertExecutionSuccess()
	// progress to next epoch to ensure delegation and jailing are processed
	d.ProgressTillFirstBlockTheNextEpoch()

	// check active set stored in epoching module removed the jailed validator
	epoch = epochK.GetEpoch(d.Ctx())
	valset = epochK.GetValidatorSet(d.Ctx(), epoch.EpochNumber)
	// check that jailed validator is still on epoch validator set
	require.Len(t, valset, maxVals, "Jailed validator should not be in the validator set")
	require.False(t, isValidatorInValset(valset, val2ValAddr), "Jailed validator should not be in the validator set")
	require.False(t, isValidatorInValset(valset, val6ValAddr), "validator 6 should not be in the validator set")

	// check delegation was created
	del, err := stkK.GetDelegation(d.Ctx(), del2.Address(), val2ValAddr)
	require.NoError(t, err)
	require.Equal(t, del.DelegatorAddress, del2.Address().String())

	// Check costaker trackers are correct
	// del2 created a delegation at same epoch that the validator got jailed, so the tracker was not even created (skipped due to jailing)
	_, err = costkK.GetCostakerRewards(d.Ctx(), del2.Address())
	require.Error(t, err)
	require.ErrorContains(t, err, "not found")

	// Trackers for val2, del3, del4 y del5 should be zeroed
	for _, addr := range []sdk.AccAddress{val2Oper.Address(), del3.Address(), del4.Address(), del5.Address()} {
		assertZeroCostkTracker(d.t, d.Ctx(), costkK, addr)
	}

	// Trackers for val 1 delegators should be: self delegation unaffected, redelegation slashed amt
	val1Tracker, err = costkK.GetCostakerRewards(d.Ctx(), val1AccAddr)
	require.NoError(t, err)
	require.NotNil(t, val1Tracker)
	require.Equal(t, val1Tracker.ActiveBaby, val1SelfDelAmt, "active baby should be self delegation amount", val1Tracker.ActiveBaby.String())
	require.True(t, val1Tracker.ActiveSatoshis.IsZero(), "Active sats should be zero")
	require.True(t, val1Tracker.TotalScore.IsZero())

	// del6 redelegated, so should not have delegation to val2
	// and should have the slashed delegation to val1
	del6Tracker, err = costkK.GetCostakerRewards(d.Ctx(), del6.Address())
	require.NoError(t, err)
	require.NotNil(t, del6Tracker)
	assertActiveBabyWithinRange(t, del6AmtAfterSlashing, del6Tracker.ActiveBaby, 1, "del6 active baby after redelegation slashing")
	require.True(t, del6Tracker.ActiveSatoshis.IsZero())
	require.True(t, del6Tracker.TotalScore.IsZero())

	// del7 redelegated to val6, which got kicked out of active set, so co-staker tracker should be zeroed
	assertZeroCostkTracker(d.t, d.Ctx(), costkK, del7.Address())
	// val6 dropped out of active set, so co-staker tracker should be zeroed
	assertZeroCostkTracker(d.t, d.Ctx(), costkK, val6Oper.Address())

	// =================================================
	// OPERATIONS AFTER VALIDATOR IS JAILED
	// =================================================

	// New delegation to already jailed validator (should continue as zero active baby)
	del3DelegatedAmtAfterJailing := sdkmath.NewInt(100000)
	d.TxWrappedDelegate(del3.SenderInfo, val2.OperatorAddress, del3DelegatedAmtAfterJailing)

	d.GenerateNewBlockAssertExecutionSuccess()
	d.ProgressTillFirstBlockTheNextEpoch()

	epoch = epochK.GetEpoch(d.Ctx())
	valset = epochK.GetValidatorSet(d.Ctx(), epoch.EpochNumber)
	require.Len(t, valset, 3)
	require.False(t, isValidatorInValset(valset, val2ValAddr), "jailed validator should not be in the validator set")

	// check costk tracker is still zero
	assertZeroCostkTracker(d.t, d.Ctx(), costkK, del3.Address())

	// Unjail the jail validator
	// make sure block time is after the jail timeout
	var valConsPubKey cryptotypes.PubKey
	err = util.Cdc.UnpackAny(val2.ConsensusPubkey, &valConsPubKey)
	require.NoError(d.t, err)
	// check unjailing time
	val2ConsAddr := sdk.ConsAddress(valConsPubKey.Address())
	info, err := slashK.GetValidatorSigningInfo(d.Ctx(), val2ConsAddr)
	require.NoError(d.t, err)
	currBlckTime := d.Ctx().BlockTime()
	timeToUnjail := info.JailedUntil.Sub(currBlckTime)
	require.True(d.t, timeToUnjail > 0)

	// produce blocks till after unjail time
	for currBlckTime.Before(info.JailedUntil.Add(1 * time.Second)) {
		currBlckTime = d.Ctx().BlockTime()
		d.GenerateNewBlockAssertExecutionSuccess()
	}

	d.TxUnjailValidator(val2Oper.SenderInfo, val2.OperatorAddress)
	d.GenerateNewBlockAssertExecutionSuccess()

	// Wait for an epoch
	d.ProgressTillFirstBlockTheNextEpoch()
	// check unjailed validator is back in active set
	epoch = epochK.GetEpoch(d.Ctx())
	valset = epochK.GetValidatorSet(d.Ctx(), epoch.EpochNumber)
	require.Len(t, valset, 2, "Unjailed validator should be in the validator set")
	require.True(t, isValidatorInValset(valset, val2ValAddr), "Unjailed validator should be in the validator set")

	// Check the active baby is properly set back for delegations to this validator
	// NOTE: Consider that the ones that were slashed will be less than the original staking amount

	// val2 self delegation was slashed
	selfDel, err := stkK.GetDelegation(d.Ctx(), val2Oper.Address(), val2ValAddr)
	require.NoError(t, err)
	expSelfDelAmt := val2.TokensFromShares(selfDel.Shares).TruncateInt()
	require.True(t, expSelfDelAmt.LT(newValSelfDelegatedAmt), "self delegation should be less than original amount due to slashing", expSelfDelAmt.String())
	// active baby should be less than self delegation amount due to slashing
	val2Tracker, err = costkK.GetCostakerRewards(d.Ctx(), val2Oper.Address())
	require.NoError(t, err)
	require.NotNil(t, val2Tracker)
	assertActiveBabyWithinRange(t, expSelfDelAmt, val2Tracker.ActiveBaby, 1, "val2 active baby after slashing")
	require.True(t, val2Tracker.ActiveSatoshis.IsZero(), "Active sats should be zero")
	require.True(t, val2Tracker.TotalScore.IsZero(), "Active score should be zero as validator was jailed entire epoch")

	del3Tracker, err = costkK.GetCostakerRewards(d.Ctx(), del3.Address())
	require.NoError(t, err)
	require.NotNil(t, del3Tracker)

	expectedDel3ActiveBaby := del3FirstDelAmtAfterSlashing.Add(del3BabyDelegatedAmtAfterJailing).Add(del3DelegatedAmtAfterJailing)
	assertActiveBabyWithinRange(t, expectedDel3ActiveBaby, del3Tracker.ActiveBaby, 1, "del3 active baby after slashing")
	require.True(t, del3Tracker.ActiveSatoshis.IsZero(), "Active sats should be zero")
	require.True(t, del3Tracker.TotalScore.IsZero(), "Active score should be zero as validator was jailed entire epoch")

	// del4 fully unbonded so tracker should still be zero
	assertZeroCostkTracker(d.t, d.Ctx(), costkK, del4.Address())

	// del5 got slashed first and then partially unbonded with 2 msgs
	del5Tracker, err = costkK.GetCostakerRewards(d.Ctx(), del5.Address())
	require.NoError(t, err)
	require.NotNil(t, del5Tracker)
	// expected active baby is total delegation after slashing minus the unstake amount
	// There're 2 undelegate msgs of 7 ubbn each, but after the second one, there's a re-delegation for same amount
	expectedDel5ActiveBaby := del5TotalAmtAfterSlashing.Sub(del5BabyUnstakeAmt)
	assertActiveBabyWithinRange(t, expectedDel5ActiveBaby, del5Tracker.ActiveBaby, 1, "del5 active baby after slashing and unstaking")
	require.True(t, del5Tracker.ActiveSatoshis.IsZero(), "Active sats should be zero")
	require.True(t, del5Tracker.TotalScore.IsZero(), "Active score should be zero as validator was jailed entire epoch")
}

func TestCostakingFpRemovalAndBtcUnbondSameBlockClearsActiveSats(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlockAssertExecutionSuccess()

	stkK, costkK, finalityK := d.App.StakingKeeper, d.App.CostakingKeeper, d.App.FinalityKeeper
	covSender := d.CreateCovenantSender()

	// Only one active FP allowed so someone must be evicted
	fParams := finalityK.GetParams(d.Ctx())
	fParams.MaxActiveFinalityProviders = 1
	err := finalityK.SetParams(d.Ctx(), fParams)
	require.NoError(t, err)

	// Validator / baby staking setup
	validators, err := stkK.GetAllValidators(d.Ctx())
	require.NoError(t, err)
	val := validators[0]
	valAddr := sdk.MustValAddressFromBech32(val.OperatorAddress)

	delegators := d.CreateNStakerAccounts(2)
	del1, del2 := delegators[0], delegators[1]

	del1BabyDelegatedAmt := sdkmath.NewInt(20_000000)
	del2BabyDelegatedAmt := sdkmath.NewInt(20_000000)

	d.MintNativeTo(del1.Address(), 100_000000)
	d.MintNativeTo(del2.Address(), 100_000000)

	d.TxWrappedDelegate(del1.SenderInfo, valAddr.String(), del1BabyDelegatedAmt)
	d.TxWrappedDelegate(del2.SenderInfo, valAddr.String(), del2BabyDelegatedAmt)

	d.GenerateNewBlockAssertExecutionSuccess()
	d.ProgressTillFirstBlockTheNextEpoch()

	// Two finality providers
	fps := d.CreateNFinalityProviderAccounts(2)
	fp1, fp2 := fps[0], fps[1]
	fp1.RegisterFinalityProvider()
	fp2.RegisterFinalityProvider()
	d.GenerateNewBlockAssertExecutionSuccess()

	// BTC delegations: fp1 has 2x power of fp2
	p := costkK.GetParams(d.Ctx())
	del1BtcStakedAmtFp1 := del1BabyDelegatedAmt.Quo(p.ScoreRatioBtcByBaby) // e.g. 100k sats
	del2BtcStakedAmtFp2 := del1BtcStakedAmtFp1.QuoRaw(2)                   // e.g. 50k sats

	del1MsgCreateFp1 := del1.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
		defaultStakingTime,
		del1BtcStakedAmtFp1.Int64(),
	)

	del2.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{fp2.BTCPublicKey()},
		defaultStakingTime,
		del2BtcStakedAmtFp2.Int64(),
	)

	d.GenerateNewBlockAssertExecutionSuccess()

	// Activate both BTC delegations
	covSender.SendCovenantSignatures()
	d.GenerateNewBlockAssertExecutionSuccess()
	d.ActivateVerifiedDelegations(2)

	// Make both FPs eligible and then finalize
	fp1.CommitRandomness()
	fp2.CommitRandomness()
	d.GenerateNewBlockAssertExecutionSuccess()

	currentEpoch := d.GetEpoch().EpochNumber
	d.ProgressTillFirstBlockTheNextEpoch()
	d.FinalizeCkptForEpoch(currentEpoch - 1)
	d.FinalizeCkptForEpoch(currentEpoch)
	d.GenerateNewBlockAssertExecutionSuccess()

	// At this point fp1 should be the only active FP (more power than fp2)
	activeFps := d.GetActiveFpsAtCurrentHeight(t)
	require.Len(t, activeFps, 1, "expected exactly 1 active FP before unbond")
	require.True(t, activeFps[0].BtcPkHex.Equals(fp1.BTCPublicKey()), "fp1 should be active before unbond")

	// Precondition: del1 has some active sats in costaking
	trkBefore, err := costkK.GetCostakerRewards(d.Ctx(), del1.Address())
	require.NoError(t, err)
	require.Equal(t, trkBefore.ActiveSatoshis.Uint64(), del1BtcStakedAmtFp1.Uint64())
	require.Equal(t, trkBefore.ActiveBaby.Uint64(), del1BabyDelegatedAmt.Uint64())

	// Unbond the *entire* BTC delegation to fp1.
	stakingTx := &wire.MsgTx{}
	err = stakingTx.Deserialize(bytes.NewReader(del1MsgCreateFp1.StakingTx))
	require.NoError(t, err)
	stakingTxHash := stakingTx.TxHash()

	del1.UnbondDelegation(&stakingTxHash, stakingTx, covSender)

	// Process the blocks with the unbond
	d.GenerateNewBlockAssertExecutionSuccess()
	d.GenerateNewBlockAssertExecutionSuccess()

	unbonded := d.GetUnbondedBTCDelegations(t)
	require.Len(t, unbonded, 1, "delegation should be unbonded")

	ub := unbonded[0]
	require.Equal(t, del1.AddressString(), ub.StakerAddr,
		"unbonded delegation should belong to del1")

	foundFp1 := false
	for _, pk := range ub.FpBtcPkList {
		if pk.Equals(fp1.BTCPublicKey()) {
			foundFp1 = true
			break
		}
	}
	require.True(t, foundFp1, "unbonded delegation should target fp1")

	activeDelegations := d.GetActiveBTCDelegations(t)
	for _, ad := range activeDelegations {
		for _, pk := range ad.FpBtcPkList {
			require.False(t, pk.Equals(fp1.BTCPublicKey()),
				"expected no active BTC delegation to fp1 after unbond",
			)
		}
	}

	// fp1 should no longer be active
	activeFps = d.GetActiveFpsAtCurrentHeight(t)
	require.Len(t, activeFps, 1, "expected exactly 1 active FP before unbond")
	require.True(t, activeFps[0].BtcPkHex.Equals(fp2.BTCPublicKey()), "fp1 should be inactive after unbond, and fp2 active")

	unbondedLater := d.GetUnbondedBTCDelegations(t)
	require.Len(t, unbondedLater, 1, "delegation should remain unbonded after extra blocks")
	require.Equal(t, del1.AddressString(), unbondedLater[0].StakerAddr)

	trkAfter, err := costkK.GetCostakerRewards(d.Ctx(), del1.Address())
	require.NoError(t, err)

	require.True(t, trkAfter.ActiveSatoshis.IsZero(),
		"costaker ActiveSatoshis must be zero after unbonding last delegation to fp1")
	require.Equal(t, trkBefore.ActiveBaby.Uint64(), del1BabyDelegatedAmt.Uint64())
}

// TestCostakingFpBecomesActiveAndBtcUnbondSameBlockKeepsActiveSatsZero tests that when:
// - Block X: FP is inactive, BTC staker has BTC stake with ActiveSatoshis = 0
// - Block X+1: FP becomes active, and the BTC staker unbonds its BTC stake
// The BTC staker's ActiveSatoshis remains 0
func TestCostakingFpBecomesActiveAndBtcUnbondSameBlockKeepsActiveSatsZero(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlockAssertExecutionSuccess()

	stkK, costkK, finalityK := d.App.StakingKeeper, d.App.CostakingKeeper, d.App.FinalityKeeper
	covSender := d.CreateCovenantSender()

	// Only one active FP allowed so fp2 will be inactive initially
	fParams := finalityK.GetParams(d.Ctx())
	fParams.MaxActiveFinalityProviders = 1
	err := finalityK.SetParams(d.Ctx(), fParams)
	require.NoError(t, err)

	// Validator / baby staking setup
	validators, err := stkK.GetAllValidators(d.Ctx())
	require.NoError(t, err)
	val := validators[0]
	valAddr := sdk.MustValAddressFromBech32(val.OperatorAddress)

	delegators := d.CreateNStakerAccounts(3)
	del1, del2, del3 := delegators[0], delegators[1], delegators[2]

	del1BabyDelegatedAmt := sdkmath.NewInt(20_000000)
	del2BabyDelegatedAmt := sdkmath.NewInt(20_000000)
	del3BabyDelegatedAmt := sdkmath.NewInt(20_000000)

	d.MintNativeTo(del1.Address(), 100_000000)
	d.MintNativeTo(del2.Address(), 100_000000)
	d.MintNativeTo(del3.Address(), 100_000000)

	d.TxWrappedDelegate(del1.SenderInfo, valAddr.String(), del1BabyDelegatedAmt)
	d.TxWrappedDelegate(del2.SenderInfo, valAddr.String(), del2BabyDelegatedAmt)
	d.TxWrappedDelegate(del3.SenderInfo, valAddr.String(), del3BabyDelegatedAmt)

	d.GenerateNewBlockAssertExecutionSuccess()
	d.ProgressTillFirstBlockTheNextEpoch()

	// Two finality providers
	fps := d.CreateNFinalityProviderAccounts(2)
	fp1, fp2 := fps[0], fps[1]
	fp1.RegisterFinalityProvider()
	fp2.RegisterFinalityProvider()
	d.GenerateNewBlockAssertExecutionSuccess()

	// BTC delegations: fp1 has 1.5x power of fp2
	// fp1 will be active (more power), fp2 will be inactive
	p := costkK.GetParams(d.Ctx())
	del1BtcStakedAmtFp1 := del1BabyDelegatedAmt.Quo(p.ScoreRatioBtcByBaby) // e.g. 100k sats
	del2BtcStakedAmtFp2 := del1BtcStakedAmtFp1.QuoRaw(2)                   // e.g. 50k sats
	del3BtcStakedAmtFp2 := del2BtcStakedAmtFp2.QuoRaw(2)                   // e.g. 25K sats

	del1MsgCreateFp1 := del1.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
		defaultStakingTime,
		del1BtcStakedAmtFp1.Int64(),
	)

	del2MsgCreateFp2 := del2.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{fp2.BTCPublicKey()},
		defaultStakingTime,
		del2BtcStakedAmtFp2.Int64(),
	)

	del3.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{fp2.BTCPublicKey()},
		defaultStakingTime,
		del3BtcStakedAmtFp2.Int64(),
	)

	d.GenerateNewBlockAssertExecutionSuccess()

	// Activate both BTC delegations
	covSender.SendCovenantSignatures()
	d.GenerateNewBlockAssertExecutionSuccess()
	d.ActivateVerifiedDelegations(3)

	// Make both FPs eligible and then finalize
	fp1.CommitRandomness()
	fp2.CommitRandomness()
	d.GenerateNewBlockAssertExecutionSuccess()

	currentEpoch := d.GetEpoch().EpochNumber
	d.ProgressTillFirstBlockTheNextEpoch()
	d.FinalizeCkptForEpoch(currentEpoch - 1)
	d.FinalizeCkptForEpoch(currentEpoch)
	d.GenerateNewBlockAssertExecutionSuccess()

	// At this point fp1 should be the only active FP (more power than fp2)
	// del2 and del3 have BTC stake to fp2 (inactive), so del2's and del3's ActiveSatoshis should be 0
	activeFps := d.GetActiveFpsAtCurrentHeight(t)
	require.Len(t, activeFps, 1, "expected exactly 1 active FP")
	require.True(t, activeFps[0].BtcPkHex.Equals(fp1.BTCPublicKey()), "fp1 should be active")

	// Precondition: del2 and del3 have zero active sats (fp2 is inactive)
	trkBefore, err := costkK.GetCostakerRewards(d.Ctx(), del2.Address())
	require.NoError(t, err)
	require.True(t, trkBefore.ActiveSatoshis.IsZero(),
		"del2 ActiveSatoshis must be zero because fp2 is inactive")
	require.Equal(t, trkBefore.ActiveBaby.Uint64(), del2BabyDelegatedAmt.Uint64())

	// Precondition: del3 has zero active sats (fp2 is inactive)
	trkBefore3, err := costkK.GetCostakerRewards(d.Ctx(), del3.Address())
	require.NoError(t, err)
	require.True(t, trkBefore3.ActiveSatoshis.IsZero(),
		"del3 ActiveSatoshis must be zero because fp2 is inactive")
	require.Equal(t, trkBefore3.ActiveBaby.Uint64(), del3BabyDelegatedAmt.Uint64())

	// Now unbond del1's BTC delegation to fp1, which will make fp1 inactive
	// and fp2 will become active
	stakingTx1 := &wire.MsgTx{}
	err = stakingTx1.Deserialize(bytes.NewReader(del1MsgCreateFp1.StakingTx))
	require.NoError(t, err)
	stakingTxHash1 := stakingTx1.TxHash()

	// del2 delegation is also unbonded below
	stakingTx2 := &wire.MsgTx{}
	err = stakingTx2.Deserialize(bytes.NewReader(del2MsgCreateFp2.StakingTx))
	require.NoError(t, err)
	stakingTxHash2 := stakingTx2.TxHash()

	// Unbond del1's delegation to fp1, making fp2 become active
	// At the same time, del2 unbonds from fp2
	unbonding1 := del1.PrepareUnbonding(&stakingTxHash1, stakingTx1, covSender)
	unbonding2 := del2.PrepareUnbonding(&stakingTxHash2, stakingTx2, covSender)
	d.BatchUnbondDelegations([]*UnbondingInfo{unbonding1, unbonding2})
	d.GenerateNewBlockAssertExecutionSuccess()

	// Verify del2's unbond happened
	unbonded := d.GetUnbondedBTCDelegations(t)
	require.Len(t, unbonded, 2, "both delegations should be unbonded")

	var del2Unbonded bool
	for _, ub := range unbonded {
		if ub.StakerAddr == del2.AddressString() {
			for _, pk := range ub.FpBtcPkList {
				if pk.Equals(fp2.BTCPublicKey()) {
					del2Unbonded = true
					break
				}
			}
		}
	}
	require.True(t, del2Unbonded, "del2's delegation to fp2 should be unbonded")

	// fp2 should now be active
	activeFps = d.GetActiveFpsAtCurrentHeight(t)
	require.Len(t, activeFps, 1, "expected exactly 1 active FP")
	require.True(t, activeFps[0].BtcPkHex.Equals(fp2.BTCPublicKey()), "fp2 should be active")

	// Key assertion: del2's ActiveSatoshis should still be 0
	// Even though fp2 became active in the same block as del2's unbond,
	// del2's ActiveSatoshis should remain 0
	del2TrkAfter, err := costkK.GetCostakerRewards(d.Ctx(), del2.Address())
	require.NoError(t, err)

	require.True(t, del2TrkAfter.ActiveSatoshis.IsZero(),
		"costaker ActiveSatoshis must remain zero - fp2 became active but del2 unbonded in same block")
	require.Equal(t, del2TrkAfter.ActiveBaby.Uint64(), del2BabyDelegatedAmt.Uint64())

	// del3's ActiveSatoshis should now become > 0
	// because fp2 is now active
	del3TrkAfter, err := costkK.GetCostakerRewards(d.Ctx(), del3.Address())
	require.NoError(t, err)

	require.Equal(t, del3TrkAfter.ActiveSatoshis, del3BtcStakedAmtFp2,
		"costaker ActiveSatoshis must be greater than zero - fp2 is now active")
	require.Equal(t, del3TrkAfter.ActiveBaby.Uint64(), del3BabyDelegatedAmt.Uint64())

	// del1 active sats should be zero as it unbonded its only delegation
	del1TrkAfter, err := costkK.GetCostakerRewards(d.Ctx(), del1.Address())
	require.NoError(t, err)

	require.True(t, del1TrkAfter.ActiveSatoshis.IsZero(),
		"costaker ActiveSatoshis must be zero after unbonding last delegation to fp1")
	require.Equal(t, del1TrkAfter.ActiveBaby.Uint64(), del1BabyDelegatedAmt.Uint64())
}

func TestCostakingSlashedSteal(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlockAssertExecutionSuccess()

	stkK, costkK, epochK := d.App.StakingKeeper, d.App.CostakingKeeper, d.App.EpochingKeeper

	// Allow 2 validators in the active set so the new validator (B) can enter.
	d.StakingUpdateParams(2)

	// Validator A: the initial validator
	validators, err := stkK.GetAllValidators(d.Ctx())
	require.NoError(t, err)
	require.Len(t, validators, 1)
	valA := validators[0]
	valAAddr := sdk.MustValAddressFromBech32(valA.OperatorAddress)

	// Delegator with an existing X delegation to validator A
	delegators := d.CreateNStakerAccounts(2)
	delegator := delegators[0]
	selfDelValB := delegators[1]

	delAmtValA := sdkmath.NewInt(20_000000)
	d.TxWrappedDelegate(delegator.SenderInfo, valAAddr.String(), delAmtValA)
	d.GenerateNewBlockAssertExecutionSuccess()
	d.ProgressTillFirstBlockTheNextEpoch() // executes the delegation at epoch end

	// Sanity: costaking tracks del's A delegation as ActiveBaby = X (score is zero)
	currRwd, err := costkK.GetCurrentRewards(d.Ctx())
	require.NoError(t, err)
	d.CheckCostakerRewards(delegator.Address(), delAmtValA, zeroInt, zeroInt, currRwd.Period)

	// Validator B: create and ensure it is in the active set
	d.MintNativeTo(selfDelValB.Address(), 1000_000000)
	d.TxCreateValidator(selfDelValB.SenderInfo, sdkmath.NewInt(3_000000))
	d.GenerateNewBlockAssertExecutionSuccess()
	d.ProgressTillFirstBlockTheNextEpoch()

	valBAddr := sdk.ValAddress(selfDelValB.SenderInfo.Address())
	valB, err := stkK.GetValidator(d.Ctx(), valBAddr)
	require.NoError(t, err)
	require.Equal(t, stktypes.Bonded, valB.Status, "validator B should be bonded")

	epoch := epochK.GetEpoch(d.Ctx())
	valset := epochK.GetValidatorSet(d.Ctx(), epoch.EpochNumber)
	require.Len(t, valset, 2, "expected 2 validators in active set")
	require.True(t, isValidatorInValset(valset, valAAddr), "validator A should be in active set")
	require.True(t, isValidatorInValset(valset, valBAddr), "validator B should be in active set")

	// Slash validator B.
	// This changes the token/share ratio (costaking marks IsSlashed=true) while keeping B in the active set.
	var valBConsPubKey cryptotypes.PubKey
	err = util.Cdc.UnpackAny(valB.ConsensusPubkey, &valBConsPubKey)
	require.NoError(t, err)
	valBConsAddr := sdk.ConsAddress(valBConsPubKey.Address())

	valBFromValset := FindValInValset(valset, valBAddr)
	valBPower := valBFromValset.Power
	require.True(t, valBPower > 0, "validator B should have positive voting power")

	blkHeightSlashed := d.Ctx().BlockHeader().Height
	_, err = stkK.Slash(d.Ctx(), valBConsAddr, blkHeightSlashed, valBPower, sdkmath.LegacyMustNewDecFromStr("0.16"))
	require.NoError(t, err)

	// Snapshot A delegation tokens (this is what ActiveBaby should equal if B roundtrip is neutral)
	valADel, err := stkK.GetDelegation(d.Ctx(), delegator.Address(), valAAddr)
	require.NoError(t, err)
	valAState, err := stkK.GetValidator(d.Ctx(), valAAddr)
	require.NoError(t, err)
	expActiveBabyFromA := valAState.TokensFromShares(valADel.Shares).TruncateInt()
	require.True(t, expActiveBabyFromA.GTE(delAmtValA), "expected A delegation tokens to be at least X")

	// Choose Y small so X >= Y and we get the "silent steal" (non-negative) case.
	amtValB := sdkmath.NewInt(1_500000)
	require.True(t, expActiveBabyFromA.GTE(amtValB), "precondition X >= Y should hold")

	// Record the costaker tracker before interacting with B.
	trkBefore, err := costkK.GetCostakerRewards(d.Ctx(), delegator.Address())
	require.NoError(t, err)
	activeBabyBefore := trkBefore.ActiveBaby

	// queue and execute a delegation to the slashed validator B.
	// This creates a real staking delegation at epoch end, but costaking won't add ActiveBaby due to IsSlashed=true.
	d.TxWrappedDelegate(delegator.SenderInfo, valBAddr.String(), amtValB)
	d.GenerateNewBlockAssertExecutionSuccess()
	d.ProgressTillFirstBlockTheNextEpoch()

	// Sanity: delegation to B exists now (created at epoch end above).
	delBDelegation, err := stkK.GetDelegation(d.Ctx(), delegator.Address(), valBAddr)
	require.NoError(t, err)

	// queue and execute a *full* undelegation from B (using current tokens-from-shares),
	// which should remove the delegation and (incorrectly) subtract from the delegator's global ActiveBaby.
	valBState, err := stkK.GetValidator(d.Ctx(), valBAddr)
	require.NoError(t, err)
	unbondAmt := valBState.TokensFromShares(delBDelegation.Shares).TruncateInt()
	require.True(t, unbondAmt.IsPositive(), "expected positive unbond amount for B delegation")

	d.TxWrappedUndelegate(delegator.SenderInfo, valBAddr.String(), unbondAmt)
	d.GenerateNewBlockAssertExecutionSuccess()
	d.ProgressTillFirstBlockTheNextEpoch()

	// Sanity: delegator should not have any delegation to B after undelegation.
	_, err = stkK.GetDelegation(d.Ctx(), delegator.Address(), valBAddr)
	require.Error(t, err)

	d.GenerateNewBlockAssertExecutionSuccess()
	d.ProgressTillFirstBlockTheNextEpoch()
	d.GenerateNewBlockAssertExecutionSuccess()
	d.ProgressTillFirstBlockTheNextEpoch()

	// BUG REPRODUCER: ActiveBaby should equal A-only delegation, but is reduced (steals from A).
	trk, err := costkK.GetCostakerRewards(d.Ctx(), delegator.Address())
	require.NoError(t, err)
	activeBabyAfter := trk.ActiveBaby

	// Output the before/after values for the PoC. Use `go test -v` to always show t.Logf output.
	t.Logf("PoC ActiveBaby before B delegate/undelegate: %s", activeBabyBefore.String())
	t.Logf("PoC ActiveBaby after B delegate/undelegate: %s", activeBabyAfter.String())

	require.False(t, trk.ActiveBaby.IsNegative(), "ActiveBaby should be non-negative in the X>=Y case")
	require.True(t, trk.ActiveBaby.LT(expActiveBabyFromA),
		"expected ActiveBaby to be incorrectly reduced below A-only amount (silent steal). got=%s want<%s",
		trk.ActiveBaby.String(), expActiveBabyFromA.String(),
	)
	d.CheckCostakerRewards(delegator.Address(), delAmtValA, zeroInt, zeroInt, currRwd.Period)
}

func TestCostakingSlashingAndUnbondAll(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlockAssertExecutionSuccess()

	stkK, costkK, epochK, slashK := d.App.StakingKeeper, d.App.CostakingKeeper, d.App.EpochingKeeper, d.App.SlashingKeeper

	validators, err := stkK.GetAllValidators(d.Ctx())
	require.NoError(t, err)
	valA := validators[0]
	valAAddr := sdk.MustValAddressFromBech32(valA.OperatorAddress)

	dels := d.CreateNStakerAccounts(2)
	delegator := dels[0]
	delSelfDelegation := dels[1]

	// Delegates to validator A
	delegateAmtValA := sdkmath.NewInt(20_000000)
	d.TxWrappedDelegate(delegator.SenderInfo, valAAddr.String(), delegateAmtValA)
	d.GenerateNewBlockAssertExecutionSuccess()
	d.ProgressTillFirstBlockTheNextEpoch()

	currRwd, err := costkK.GetCurrentRewards(d.Ctx())
	require.NoError(t, err)
	d.CheckCostakerRewards(delegator.Address(), delegateAmtValA, zeroInt, zeroInt, currRwd.Period)

	// Allow 2 validators in the active set so the new validator (B) can enter.
	d.StakingUpdateParams(2)

	d.TxCreateValidator(delSelfDelegation.SenderInfo, sdkmath.NewInt(10_000000))
	d.GenerateNewBlockAssertExecutionSuccess()
	d.ProgressTillFirstBlockTheNextEpoch()

	validators, err = stkK.GetAllValidators(d.Ctx())
	require.NoError(t, err)
	require.Len(t, validators, 2)

	valBAddr := sdk.ValAddress(delSelfDelegation.Address())
	valB := FindValInValidators(validators, valBAddr)
	require.True(t, valB.IsBonded())

	d.IsValsInCurrActiveValset(2, valAAddr, valBAddr)

	// delegates to new val B
	delegateAmtValB := sdkmath.NewInt(3_000000)
	d.TxWrappedDelegate(delegator.SenderInfo, valBAddr.String(), delegateAmtValB)
	d.GenerateNewBlockAssertExecutionSuccess()
	d.ProgressTillFirstBlockTheNextEpoch()

	amtValAPlusB := delegateAmtValA.Add(delegateAmtValB) // 20 + 3
	d.CheckCostakerRewards(delegator.Address(), amtValAPlusB, zeroInt, zeroInt, currRwd.Period)

	d.JailValidatorForDowntime(valBAddr)

	// B is still in the active set, will be removed at the end of the epoch
	d.IsValsInCurrActiveValset(2, valAAddr, valBAddr)

	// still 20+3
	d.CheckCostakerRewards(delegator.Address(), amtValAPlusB, zeroInt, zeroInt, currRwd.Period)

	delBDelegation, err := stkK.GetDelegation(d.Ctx(), delegator.Address(), valBAddr)
	require.NoError(t, err)

	valBState, err := stkK.GetValidator(d.Ctx(), valBAddr)
	require.NoError(t, err)
	unbondAmt := valBState.TokensFromShares(delBDelegation.Shares).TruncateInt()
	require.True(t, unbondAmt.IsPositive(), "expected positive unbond amount for B delegation")

	// confirm the slashed portion
	slashP, err := slashK.GetParams(d.Ctx())
	require.NoError(t, err)
	decDelegateAmtValB := delegateAmtValB.ToLegacyDec()
	slashedPortion := decDelegateAmtValB.Mul(slashP.SlashFractionDowntime)
	amtValBDel1AfterSlash := decDelegateAmtValB.Sub(slashedPortion).TruncateInt()
	require.Equal(t, unbondAmt.String(), amtValBDel1AfterSlash.String(), "The delegateAmtValB that was delegated prior to jailing slash offence gets slashed")

	// PoC
	// Trigger Hook BeforeDelegationRemoved
	// Reset, no delegation tokens after this
	d.TxWrappedUndelegate(delegator.SenderInfo, valBAddr.String(), unbondAmt)
	d.GenerateNewBlockAssertExecutionSuccess()

	delegateAmtValB2 := sdkmath.NewInt(2_000000)
	// Trigger Hook AfterDelegationModified
	// New delegation, increase delta shares
	d.TxWrappedDelegate(delegator.SenderInfo, valBAddr.String(), delegateAmtValB2)
	d.GenerateNewBlockAssertExecutionSuccess()

	// Trigger Hook BeforeDelegationRemoved
	// Trigger the **misaccounting of the ActiveBaby**, steal from A
	d.TxWrappedUndelegate(delegator.SenderInfo, valBAddr.String(), delegateAmtValB2)
	d.GenerateNewBlockAssertExecutionSuccess()
	// ends the epoch and causes the misscalc
	d.ProgressTillFirstBlockTheNextEpoch()

	// Delegation to B is removed
	delBDelegation, err = stkK.GetDelegation(d.Ctx(), delegator.Address(), valBAddr)
	require.EqualError(t, err, "no delegation for (address, validator) tuple")

	// Validator B is removed from the active set
	epoch := epochK.GetEpoch(d.Ctx())
	valset := epochK.GetValidatorSet(d.Ctx(), epoch.EpochNumber)
	require.Len(t, valset, 1, "expected only A validator in active set")
	require.True(t, isValidatorInValset(valset, valAAddr), "validator A should be in active set")
	require.False(t, isValidatorInValset(valset, valBAddr), "validator B should not be in active set")

	d.GenerateNewBlockAssertExecutionSuccess()
	d.ProgressTillFirstBlockTheNextEpoch()
	d.GenerateNewBlockAssertExecutionSuccess()
	d.ProgressTillFirstBlockTheNextEpoch()

	// verify that the costaker still??? has the amount of the first baby delegation
	// This is all debugging code.
	t.Logf("This part shows the bug where the amount staked from the valA got reduced from 20 -> 17.9")
	t.Logf("Question is how do the amount gets deducted to 17979798 from 20000000")
	t.Logf("The reason for the amount reduced is due to the second unbond: %s", delegateAmtValA.Sub(amtValBDel1AfterSlash).String())
	t.Logf("ValA: %s", valAAddr.String())
	t.Logf("ValB slashed: %s", valBAddr.String())
	t.Logf("delegateAmtValA: %s", delegateAmtValA.String())
	t.Logf("delegateAmtValBBeforeSlash: %s", delegateAmtValB.String())
	t.Logf("delegateAmtValBAfterSlash: %s", amtValBDel1AfterSlash.String())
	t.Logf("delegateAmtValB2: %s", delegateAmtValB2.String())
	t.Logf("slashedPortion: %s", slashedPortion.TruncateInt().String())

	// double checks that the delegation of validator A is still healthy
	delADelegation, err := stkK.GetDelegation(d.Ctx(), delegator.Address(), valAAddr)
	require.NoError(t, err)

	valAState, err := stkK.GetValidator(d.Ctx(), valAAddr)
	require.NoError(t, err)
	delegationAAmt := valAState.TokensFromShares(delADelegation.Shares).TruncateInt()
	require.Equal(t, delegationAAmt.String(), delegateAmtValA.String())

	// This check fails and shows the bug where the second full unbond of an slashed baby validator can cause the issue
	d.CheckCostakerRewards(delegator.Address(), delegateAmtValA, zeroInt, zeroInt, currRwd.Period)
}

func TestCostakingSlashingAndUnbondSameEpoch(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlockAssertExecutionSuccess()

	// stkK, costkK, epochK, slashK := d.App.StakingKeeper, d.App.CostakingKeeper, d.App.EpochingKeeper, d.App.SlashingKeeper
	stkK, _, _, slashK := d.App.StakingKeeper, d.App.CostakingKeeper, d.App.EpochingKeeper, d.App.SlashingKeeper

	dels := d.CreateNStakerAccounts(2)
	validatorStkAcc := dels[0]
	valAddr := sdk.ValAddress(validatorStkAcc.Address())
	delStkAcc := dels[1]

	// start delegating to the first valid validator
	delegateAmtToActiveVal := sdkmath.NewInt(20_000000)
	d.TxWrappedDelegate(delStkAcc.SenderInfo, d.ValAddress.String(), delegateAmtToActiveVal)
	d.GenerateNewBlockAssertExecutionSuccess()
	d.ProgressTillFirstBlockTheNextEpoch()

	// allows 2 vals
	d.StakingUpdateParams(2)

	// creates validator
	d.TxCreateValidator(validatorStkAcc.SenderInfo, sdkmath.NewInt(10_000000))
	d.GenerateNewBlockAssertExecutionSuccess()
	d.ProgressTillFirstBlockTheNextEpoch()

	_, err := stkK.GetValidator(d.Ctx(), valAddr)
	require.NoError(t, err)

	// Delegates to validator slashed
	delegateAmtToSlashVal := sdkmath.NewInt(10_000000)
	d.TxWrappedDelegate(delStkAcc.SenderInfo, valAddr.String(), delegateAmtToSlashVal)
	d.GenerateNewBlockAssertExecutionSuccess()
	d.ProgressTillFirstBlockTheNextEpoch()

	currExpActiveBaby := delegateAmtToSlashVal.Add(delegateAmtToActiveVal)
	d.CheckCostakerRewards(delStkAcc.Address(), currExpActiveBaby, zeroInt, zeroInt, 1)

	d.JailValidatorForDowntime(valAddr)

	// There is 2 vals and the new added val is in the current epoch valset
	d.IsValsInCurrActiveValset(2, valAddr)
	// jails the validator (slash infraction)
	val := d.JailValidatorForDowntime(valAddr)
	// and continues in the active valset until end of epoch
	d.IsValsInCurrActiveValset(2, valAddr)

	d.CheckCostakerRewards(delStkAcc.Address(), currExpActiveBaby, zeroInt, zeroInt, 1)

	del, err := stkK.GetDelegation(d.Ctx(), delStkAcc.Address(), valAddr)
	require.NoError(t, err)

	slashP, err := slashK.GetParams(d.Ctx())
	require.NoError(t, err)

	// checks the amount after slash is less than it was before
	fullUbdAmt := val.TokensFromShares(del.Shares).TruncateInt()

	legacyDecDelegateAmt := delegateAmtToSlashVal.ToLegacyDec()
	slashedPortion := legacyDecDelegateAmt.Mul(slashP.SlashFractionDowntime)
	delegatAmtAfterSlash := legacyDecDelegateAmt.Sub(slashedPortion).TruncateInt()
	require.Equal(t, fullUbdAmt.String(), delegatAmtAfterSlash.String())
	require.True(t, delegatAmtAfterSlash.LT(delegateAmtToSlashVal))

	// we are still in the same epoch that the val was jailed
	d.TxWrappedUndelegate(delStkAcc.SenderInfo, valAddr.String(), fullUbdAmt)
	d.GenerateNewBlockAssertExecutionSuccess()

	delAmt2 := sdkmath.NewInt(2_000000)
	// Trigger Hook AfterDelegationModified
	// New delegation, increase delta shares
	d.TxWrappedDelegate(delStkAcc.SenderInfo, valAddr.String(), delAmt2)
	d.GenerateNewBlockAssertExecutionSuccess()

	// Trigger Hook BeforeDelegationRemoved
	// Trigger the **misaccounting of the ActiveBaby**, steal from A
	d.TxWrappedUndelegate(delStkAcc.SenderInfo, valAddr.String(), delAmt2)
	d.GenerateNewBlockAssertExecutionSuccess()
	// ends the epoch and causes the misscalc
	d.ProgressTillFirstBlockTheNextEpoch()

	// This show the bug where it reduced the amount from the delegation where the validator was active
	// instead of just removing the assets from the slashed delegation.
	d.CheckCostakerRewards(delStkAcc.Address(), currExpActiveBaby, zeroInt, zeroInt, 1)
}
