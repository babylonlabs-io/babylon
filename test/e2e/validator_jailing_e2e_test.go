//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"math"
	"time"

	sdkmath "cosmossdk.io/math"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
)

type ValidatorJailingTestSuite struct {
	suite.Suite

	configurer configurer.Configurer
}

func (s *ValidatorJailingTestSuite) SetupSuite() {
	s.T().Log("setting up validator jailing e2e integration test suite...")
	var err error

	// Configure 1 chain with 2 validator nodes
	s.configurer, err = configurer.NewBabylonConfigurer(s.T(), true)
	s.NoError(err)
	err = s.configurer.ConfigureChains()
	s.NoError(err)
	err = s.configurer.RunSetup()
	s.NoError(err)
}

func (s *ValidatorJailingTestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

// TestValidatorJailingWithExtraDelegation tests the scenario where:
// 1. Two validators are running with delegations
// 2. We delegate to validator 1 to give it >66% of voting power
// 3. The second validator stops signing blocks
// 4. After missing enough blocks, the second validator gets jailed according to x/slashing logic
// 5. The chain continues operating because validator 1 has >66% voting power
func (s *ValidatorJailingTestSuite) TestValidatorJailingWithExtraDelegation() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)

	// Get both validator nodes
	validatorNode1, err := chainA.GetNodeAtIndex(0)
	s.NoError(err)
	validatorNode2, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)

	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	// Get consensus pubkeys from each validator node to map them to on-chain validators
	s.T().Log("Mapping validator nodes to on-chain validators...")
	node0ConsPubKey := validatorNode1.ValidatorConsPubKey()
	node1ConsPubKey := validatorNode2.ValidatorConsPubKey()
	s.T().Logf("Node 0 consensus pubkey: %s", node0ConsPubKey)
	s.T().Logf("Node 1 consensus pubkey: %s", node1ConsPubKey)

	// Query validators from the chain
	validators, err := nonValidatorNode.QueryValidators()
	s.NoError(err)
	s.Require().Len(validators, 2, "Need exactly 2 validators")
	s.T().Logf("Initial validator count: %d", len(validators))

	// Map nodes to validators by matching consensus pubkey
	var val1, val2 *stakingtypes.Validator
	var val2ConsAddr string
	for i := range validators {
		// Unmarshal the consensus pubkey using the codec
		var valConsPubKey cryptotypes.PubKey
		err := util.Cdc.UnpackAny(validators[i].ConsensusPubkey, &valConsPubKey)
		s.Require().NoError(err, "failed to unmarshal consensus pubkey")

		if bytes.Equal(valConsPubKey.Bytes(), node0ConsPubKey.Bytes()) {
			val1 = &validators[i]
			s.T().Logf("Node 0 maps to validator: %s (tokens: %s)", validators[i].OperatorAddress, validators[i].Tokens.String())
		}
		if bytes.Equal(valConsPubKey.Bytes(), node1ConsPubKey.Bytes()) {
			val2 = &validators[i]
			val2ConsAddr = sdk.ConsAddress(valConsPubKey.Address()).String()
			s.T().Logf("Node 1 maps to validator: %s (tokens: %s)", validators[i].OperatorAddress, validators[i].Tokens.String())
		}
	}

	s.Require().NotNil(val1, "Could not map node 0 to validator")
	s.Require().NotNil(val2, "Could not map node 1 to validator")
	s.T().Logf("Validator 2 consensus address: %s", val2ConsAddr)

	s.T().Logf("Will delegate to validator 1: %s", val1.OperatorAddress)
	s.T().Logf("Will stop validator 2 node: %s (validator: %s)", validatorNode2.Name, val2.OperatorAddress)

	// Calculate total voting power
	totalVotingPower := val1.Tokens.Add(val2.Tokens)
	val1Percentage := val1.Tokens.ToLegacyDec().Quo(totalVotingPower.ToLegacyDec()).MulInt64(100)
	val2Percentage := val2.Tokens.ToLegacyDec().Quo(totalVotingPower.ToLegacyDec()).MulInt64(100)

	s.T().Logf("Total voting power: %s", totalVotingPower.String())
	s.T().Logf("Validator 1 percentage: %s%%", val1Percentage.String())
	s.T().Logf("Validator 2 percentage: %s%%", val2Percentage.String())

	// Delegate to validator 1 to give it >66% voting power
	s.T().Log("Delegating to validator 1 to achieve >66% voting power...")

	// Calculate how much to delegate to get validator 1 to 70% of total
	targetPercentage := sdkmath.LegacyMustNewDecFromStr("0.70")
	// If val1 needs 70% of total, then: (val1 + x) / (total + x) = 0.70
	// Solving for x: x = (0.70 * total - val1) / 0.30
	targetTotalTokens := totalVotingPower.ToLegacyDec().Mul(targetPercentage)
	currentVal1Tokens := val1.Tokens.ToLegacyDec()
	numerator := targetTotalTokens.Sub(currentVal1Tokens)
	denominator := sdkmath.LegacyOneDec().Sub(targetPercentage)
	additionalDelegation := numerator.Quo(denominator).TruncateInt()

	// Add 10% buffer to ensure we're well above 66%
	additionalDelegation = additionalDelegation.MulRaw(11).QuoRaw(10)

	s.T().Logf("Need to delegate %s ubbn to validator 1", additionalDelegation.String())

	// Delegate from non-validator node to validator 1
	delegationAmount := additionalDelegation.String() + "ubbn"
	txHash := nonValidatorNode.Delegate(nonValidatorNode.WalletName, val1.OperatorAddress, delegationAmount, "--gas=500000")

	chainA.WaitForNumHeights(2)
	res, _ := nonValidatorNode.QueryTx(txHash)
	s.Equal(res.Code, uint32(0), res.RawLog)

	// delegate 1 ubbn to validator 2 as well
	txHash = nonValidatorNode.Delegate(nonValidatorNode.WalletName, val2.OperatorAddress, "1ubbn", "--gas=500000")

	chainA.WaitForNumHeights(2)
	res, _ = nonValidatorNode.QueryTx(txHash)
	s.Equal(res.Code, uint32(0), res.RawLog)

	// Wait for delegation to take effect - need to wait for epoch end in Babylon
	s.T().Log("Waiting for epoch to end so delegation takes effect...")
	_, err = nonValidatorNode.WaitForNextEpoch()
	s.NoError(err)
	s.T().Log("Epoch ended, delegation should now be active")

	// Wait a few more blocks for validator set update
	chainA.WaitForNumHeights(3)

	// Verify the delegation took effect
	validators, err = nonValidatorNode.QueryValidators()
	s.NoError(err)
	for i := range validators {
		if validators[i].OperatorAddress == val1.OperatorAddress {
			val1 = &validators[i]
		}
		if validators[i].OperatorAddress == val2.OperatorAddress {
			val2 = &validators[i]
		}
	}
	totalVotingPower = val1.Tokens.Add(val2.Tokens)
	val1Percentage = val1.Tokens.ToLegacyDec().Quo(totalVotingPower.ToLegacyDec()).MulInt64(100)
	s.T().Logf("After delegation - Validator 1 percentage: %s%%", val1Percentage.String())
	s.T().Logf("After delegation - Validator 1 tokens: %s, operator: %s", val1.Tokens.String(), val1.OperatorAddress)
	s.T().Logf("After delegation - Validator 2 tokens: %s, operator: %s", val2.Tokens.String(), val2.OperatorAddress)
	s.Require().True(val1Percentage.GT(sdkmath.LegacyMustNewDecFromStr("66")),
		"Validator 1 should have >66%% voting power after delegation")

	// Query initial signing info for validator 2
	val2SigningInfo, err := nonValidatorNode.QuerySigningInfo(val2ConsAddr)
	s.NoError(err)
	s.T().Logf("Initial signing info for validator 2:")
	s.T().Logf("  - Missed blocks counter: %d", val2SigningInfo.MissedBlocksCounter)
	s.T().Logf("  - Jailed until: %s", val2SigningInfo.JailedUntil)

	// Get both validators' delegators and their delegation amounts
	s.T().Log("Checking co-staking trackers before jailing validator 2...")

	val1Delegators := s.getValidatorDelegators(nonValidatorNode, val1.OperatorAddress)
	s.Require().NotEmpty(val1Delegators, "Validator 1 should have at least one delegator")

	val2Delegators := s.getValidatorDelegators(nonValidatorNode, val2.OperatorAddress)
	s.Require().NotEmpty(val2Delegators, "Validator 2 should have at least one delegator")

	// Get all unique delegators from both validators
	allDelegators := make(map[string]bool)
	for d := range val1Delegators {
		allDelegators[d] = true
	}
	for d := range val2Delegators {
		allDelegators[d] = true
	}

	var delegatorsList []string
	for d := range allDelegators {
		delegatorsList = append(delegatorsList, d)
	}

	// Query co-staking trackers before jailing
	s.T().Log("Co-staking trackers BEFORE jailing:")
	trackersBefore := s.getCostakingTrackers(nonValidatorNode, delegatorsList)

	// Verify that all delegators have the expected active baby before jailing
	for delegator, activeBaby := range trackersBefore {
		// Calculate expected active baby (sum of delegations to both validators)
		expectedBaby := sdkmath.ZeroInt()
		if amt, ok := val1Delegators[delegator]; ok {
			expectedBaby = expectedBaby.Add(amt)
		}
		if amt, ok := val2Delegators[delegator]; ok {
			expectedBaby = expectedBaby.Add(amt)
		}

		s.T().Logf("Delegator %s expected active baby: %s", delegator, expectedBaby.String())
		s.Require().Equal(expectedBaby, activeBaby,
			"Delegator %s should have active baby equal to total delegations before jailing", delegator)
	}

	// Query slashing parameters to know how many blocks need to be missed
	slashingParams, err := nonValidatorNode.QuerySlashingParams()
	s.NoError(err)
	s.T().Logf("Slashing parameters:")
	s.T().Logf("  - Signed blocks window: %d", slashingParams.SignedBlocksWindow)
	s.T().Logf("  - Min signed per window: %s", slashingParams.MinSignedPerWindow.String())
	s.T().Logf("  - Downtime jail duration: %s", slashingParams.DowntimeJailDuration)

	// Calculate how many blocks need to be missed
	minSignedPerWindow := slashingParams.MinSignedPerWindow
	signedBlocksWindow := slashingParams.SignedBlocksWindow
	maxMissedBlocks := signedBlocksWindow - minSignedPerWindow.MulInt64(signedBlocksWindow).TruncateInt64()
	s.T().Logf("Max blocks that can be missed before jailing: %d", maxMissedBlocks)
	s.T().Logf("Need to miss at least %d blocks to trigger jailing", maxMissedBlocks+1)

	// Get current height before stopping validator
	currentHeight, err := nonValidatorNode.QueryCurrentHeight()
	s.NoError(err)
	s.T().Logf("Current height before stopping validator 2: %d", currentHeight)

	// Stop validator 2 node to make it miss blocks
	// Note: We're stopping the node at index 1, which should correspond to one of the validators
	s.T().Logf("Stopping validator 2 node (%s) to simulate downtime...", validatorNode2.Name)
	err = validatorNode2.Stop()
	s.NoError(err)
	s.T().Logf("Validator 2 node stopped successfully")

	// Wait for enough blocks to be produced for validator 2 to be jailed
	// We need to wait for signed_blocks_window + buffer blocks to ensure jailing
	blocksToWait := int64(math.Ceil(float64(signedBlocksWindow)*1.2)) + 5
	s.T().Logf("Waiting for %d blocks to ensure validator 2 gets jailed...", blocksToWait)

	// Wait by polling from non-validator node only (since validator 2 is stopped)
	targetHeight := currentHeight + blocksToWait
	s.waitForHeight(nonValidatorNode, targetHeight)

	// Query signing info again to see missed blocks
	afterStopSigningInfo, err := nonValidatorNode.QuerySigningInfo(val2ConsAddr)
	s.NoError(err)
	s.T().Logf("Signing info after stopping validator 2:")
	s.T().Logf("  - Missed blocks counter: %d", afterStopSigningInfo.MissedBlocksCounter)
	s.T().Logf("  - Index offset: %d", afterStopSigningInfo.IndexOffset)

	// Verify validator 2 is jailed
	s.T().Log("Verifying validator 2 is jailed...")
	validators, err = nonValidatorNode.QueryValidators()
	s.NoError(err)

	var jailedVal *stakingtypes.Validator
	for i := range validators {
		if validators[i].OperatorAddress == val2.OperatorAddress {
			jailedVal = &validators[i]
			break
		}
	}

	s.Require().NotNil(jailedVal, "Validator 2 should still be in validator set")
	s.Require().True(jailedVal.Jailed, "Validator 2 should be jailed after missing blocks")
	s.T().Logf("✓ Validator 2 successfully jailed!")
	s.T().Logf("  - Jailed status: %v", jailedVal.Jailed)
	s.T().Logf("  - Validator status: %s", jailedVal.Status)

	// Verify the signing info shows jailing
	finalSigningInfo, err := nonValidatorNode.QuerySigningInfo(val2ConsAddr)
	s.NoError(err)
	s.T().Logf("Final signing info:")
	s.T().Logf("  - Missed blocks counter: %d", finalSigningInfo.MissedBlocksCounter)
	s.T().Logf("  - Jailed until: %s", finalSigningInfo.JailedUntil)
	s.Require().True(finalSigningInfo.JailedUntil.After(time.Now()),
		"JailedUntil timestamp should be in the future")

	// Need to wait for epoch to end so that co-staking tracker updates
	s.T().Log("Waiting for epoch to end so co-staking trackers update after jailing...")
	_, err = nonValidatorNode.WaitForNextEpoch()
	s.NoError(err)
	s.T().Log("Epoch ended, co-staking trackers should now be updated")

	// Check co-staking trackers after jailing
	s.T().Log("Checking co-staking trackers AFTER jailing validator 2...")
	trackersAfter := s.getCostakingTrackers(nonValidatorNode, delegatorsList)

	// Verify the co-staking tracker changes based on delegation pattern
	for delegator, activeBabyAfter := range trackersAfter {
		activeBabyBefore := trackersBefore[delegator]
		val1Amount, hasVal1Delegation := val1Delegators[delegator]
		val2Amount, hasVal2Delegation := val2Delegators[delegator]

		// Calculate expected active baby after jailing (only val1 delegations should remain active)
		expectedBabyAfter := sdkmath.ZeroInt()
		if hasVal1Delegation {
			expectedBabyAfter = expectedBabyAfter.Add(val1Amount)
		}

		s.T().Logf("Delegator %s:", delegator)
		s.T().Logf("  - Val1 delegation: %s (active: %v)", val1Amount.String(), hasVal1Delegation)
		s.T().Logf("  - Val2 delegation: %s (active: %v, now jailed)", val2Amount.String(), hasVal2Delegation)
		s.T().Logf("  - Active baby before jailing: %s", activeBabyBefore.String())
		s.T().Logf("  - Active baby after jailing: %s", activeBabyAfter.String())
		s.T().Logf("  - Expected active baby after: %s", expectedBabyAfter.String())

		// Verify exact expected amount
		s.Require().Equal(expectedBabyAfter, activeBabyAfter,
			"Delegator %s active baby after jailing should equal only val1 delegation amount", delegator)

		// Additional specific checks based on delegation pattern
		if hasVal2Delegation && !hasVal1Delegation {
			s.T().Logf("  ✓ Delegator only to val2: active baby correctly decreased to zero")
		} else if hasVal1Delegation && !hasVal2Delegation {
			s.T().Logf("  ✓ Delegator only to val1: active baby correctly remained unchanged")
		} else if hasVal1Delegation && hasVal2Delegation {
			s.T().Logf("  ✓ Delegator to both: active baby correctly decreased by val2 amount")
		}
	}

	// Verify chain is still producing blocks (validator 1 has >66% so consensus continues)
	finalHeight, err := nonValidatorNode.QueryCurrentHeight()
	s.NoError(err)
	s.T().Logf("Final height: %d", finalHeight)
	s.Require().Greater(finalHeight, currentHeight+blocksToWait-5,
		"Chain should continue producing blocks with validator 1's >66%% voting power")

	s.T().Log("✓ Test completed successfully!")
	s.T().Log("Summary:")
	s.T().Logf("  - Validator 1 had >66%% voting power and continued producing blocks")
	s.T().Logf("  - Validator 2 stopped signing and missed %d blocks", afterStopSigningInfo.MissedBlocksCounter)
	s.T().Logf("  - Validator 2 was automatically jailed by the slashing module")
	s.T().Logf("  - Co-staking trackers verified for %d unique delegators", len(delegatorsList))
	s.T().Logf("  - Delegators only to val2 (jailed): active baby decreased to zero")
	s.T().Logf("  - Delegators only to val1 (not jailed): active baby remained unchanged")
	s.T().Logf("  - Delegators to both validators: active baby decreased proportionally")
	s.T().Logf("  - Chain continued operating with single validator (>66%% threshold)")
}

// waitForHeight waits for a specific node to reach a target height
func (s *ValidatorJailingTestSuite) waitForHeight(node *chain.NodeConfig, targetHeight int64) {
	maxAttempts := 500
	for i := 0; i < maxAttempts; i++ {
		currentHeight, err := node.QueryCurrentHeight()
		if err == nil && currentHeight >= targetHeight {
			s.T().Logf("Reached target height %d (current: %d)", targetHeight, currentHeight)
			return
		}
		if i%10 == 0 {
			s.T().Logf("Waiting for height %d, current: %d (attempt %d/%d)", targetHeight, currentHeight, i+1, maxAttempts)
		}
		time.Sleep(2 * time.Second)
	}
	s.FailNow("Timeout waiting for height %d", targetHeight)
}

// getCostakingTrackers returns a map of delegator addresses to their co-staking tracker ActiveBaby amounts
// Skips delegators that don't have a co-staking tracker (e.g., validators self-delegating)
func (s *ValidatorJailingTestSuite) getCostakingTrackers(
	node *chain.NodeConfig,
	delegatorAddresses []string,
) map[string]sdkmath.Int {
	trackers := make(map[string]sdkmath.Int)

	for _, delegator := range delegatorAddresses {
		s.T().Logf("Querying co-staking tracker for delegator: %s", delegator)
		tracker, err := node.QueryCostakerRewardsTracker(delegator)
		if err != nil {
			// Co-staking tracker might not exist for some delegators (e.g., validators self-delegating)
			s.T().Logf("  - Co-staking tracker not found (skipping): %v", err)
			continue
		}
		trackers[delegator] = tracker.ActiveBaby
		s.T().Logf("  - Active Baby: %s", tracker.ActiveBaby.String())
		s.T().Logf("  - Active Satoshis: %s", tracker.ActiveSatoshis.String())
		s.T().Logf("  - Total Score: %s", tracker.TotalScore.String())
	}

	return trackers
}

// getValidatorDelegators returns a map of delegator addresses to their delegation amounts for a validator
func (s *ValidatorJailingTestSuite) getValidatorDelegators(
	node *chain.NodeConfig,
	validatorAddr string,
) map[string]sdkmath.Int {
	allDelegations, err := node.QueryValidatorDelegations(validatorAddr)
	s.NoError(err)

	delegators := make(map[string]sdkmath.Int)
	for _, delegation := range allDelegations {
		if delegation.Delegation.ValidatorAddress == validatorAddr {
			delegators[delegation.Delegation.DelegatorAddress] = delegation.Balance.Amount
		}
	}

	s.T().Logf("Validator %s has %d delegators", validatorAddr, len(delegators))
	return delegators
}
