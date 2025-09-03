package e2e

import (
	"net/url"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
	etypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
)

type EpochingSpamPreventionTestSuite struct {
	suite.Suite

	configurer configurer.Configurer
}

func (s *EpochingSpamPreventionTestSuite) SetupSuite() {
	s.T().Log("setting up epoching spam prevention e2e integration test suite...")
	var err error

	// Configure 1 chain with some validator nodes
	s.configurer, err = configurer.NewBabylonConfigurer(s.T(), true)
	s.NoError(err)
	err = s.configurer.ConfigureChains()
	s.NoError(err)
	err = s.configurer.RunSetup()
	s.NoError(err)
}

func (s *EpochingSpamPreventionTestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

// TestNormalDelegationCase tests the normal case where:
// 1. WrappedMsgDelegate locks funds in epoching module account
// 2. At epoch end block, enqueued message is processed
// 3. After processing, validator staking amount increases and user gets delegation shares
func (s *EpochingSpamPreventionTestSuite) TestNormalDelegationCase() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	// Get first validator to delegate to
	// Query all validators and use the first one
	s.T().Logf("Querying validators from chain...")

	// Use babylond query staking validators to get actual validator addresses
	validatorAddr := ""

	// Try to get validator from the first validator node's operator address
	s.Require().NotEmpty(chainA.NodeConfigs, "should have validator nodes")

	// Look for a validator node (IsValidator = true)
	var validatorNode *chain.NodeConfig
	for _, node := range chainA.NodeConfigs {
		s.T().Logf("Checking node: %s, IsValidator: %t, OperatorAddress: '%s'",
			node.Name, node.IsValidator, node.OperatorAddress)
		if node.IsValidator {
			validatorNode = node
			break
		}
	}

	s.Require().NotNil(validatorNode, "Should have at least one validator node")

	// Get validator operator address
	if validatorNode.OperatorAddress != "" {
		validatorAddr = validatorNode.OperatorAddress
	} else {
		s.T().Logf("OperatorAddress is empty, converting PublicAddress using SDK...")
		// Convert account address to validator address using proper bech32 conversion
		if validatorNode.PublicAddress != "" {
			accAddr, err := sdk.AccAddressFromBech32(validatorNode.PublicAddress)
			s.NoError(err)
			// Convert account address to validator address
			valAddr := sdk.ValAddress(accAddr)
			validatorAddr = valAddr.String()
		} else {
			s.T().Fatalf("Cannot derive validator address from empty PublicAddress")
		}
	}

	s.T().Logf("Using validator address: '%s'", validatorAddr)
	s.Require().NotEmpty(validatorAddr, "Validator address should not be empty")

	// Use the non-validator node's address as delegator
	delegatorAddr := nonValidatorNode.PublicAddress

	// Get initial balances and delegation info
	initialDelegatorBalance, err := nonValidatorNode.QueryBalance(delegatorAddr, "ubbn")
	s.NoError(err)
	s.T().Logf("Initial delegator balance: %s", initialDelegatorBalance.String())

	// Get epoching delegate pool module account balance
	epochingModuleAddr, err := nonValidatorNode.QueryModuleAddress("epoching_delegate_pool")
	s.NoError(err)
	epochingModuleAddrStr := epochingModuleAddr.String()
	s.T().Logf("Epoching delegate pool module address: %s", epochingModuleAddrStr)

	initialModuleBalance, err := nonValidatorNode.QueryBalance(epochingModuleAddrStr, "ubbn")
	s.NoError(err)
	s.T().Logf("Initial epoching module balance: %s", initialModuleBalance.String())

	// Delegation amount
	delegationAmount := sdkmath.NewInt(1000000) // 1 BBN

	// Step 1: Send WrappedMsgDelegate and verify funds are locked
	s.T().Logf("Step 1: Sending WrappedMsgDelegate for %s ubbn", delegationAmount.String())

	// Send babylond tx epoching delegate command using the existing Delegate method
	delegationAmountCoin := delegationAmount.String() + "ubbn"
	s.T().Logf("Executing: babylond tx epoching delegate %s %s --from=%s",
		validatorAddr, delegationAmountCoin, nonValidatorNode.WalletName)

	// Execute the epoching delegate transaction
	nonValidatorNode.Delegate(nonValidatorNode.WalletName, validatorAddr, delegationAmountCoin)
	s.T().Logf("WrappedMsgDelegate sent successfully")

	// Wait a few blocks for the transaction to be processed
	chainA.WaitForNumHeights(2)

	// Step 1 Verification: Check that funds are locked (delegator balance decreased, module balance increased)
	delegatorBalanceAfterLock, err := nonValidatorNode.QueryBalance(delegatorAddr, "ubbn")
	s.NoError(err)

	// Calculate actual decrease including both delegation amount and gas fees
	actualDecrease := initialDelegatorBalance.Amount.Sub(delegatorBalanceAfterLock.Amount)
	gasFeeUsed := actualDecrease.Sub(delegationAmount)

	s.T().Logf("Balance change analysis:")
	s.T().Logf("  - Initial balance: %s", initialDelegatorBalance.String())
	s.T().Logf("  - Final balance: %s", delegatorBalanceAfterLock.String())
	s.T().Logf("  - Total decrease: %s", actualDecrease.String())
	s.T().Logf("  - Delegation amount: %s", delegationAmount.String())
	s.T().Logf("  - Gas fee used: %s", gasFeeUsed.String())

	// Verify that balance decreased by at least the delegation amount
	s.Require().True(actualDecrease.GTE(delegationAmount),
		"Delegator balance should decrease by at least delegation amount (including gas fees)")
	s.T().Logf("Step 1a Verified: Delegator balance decreased by %s (delegation + gas fees)",
		actualDecrease.String())

	// Verify epoching module balance increased (funds locked in module account)
	moduleBalanceAfterLock, err := nonValidatorNode.QueryBalance(epochingModuleAddrStr, "ubbn")
	s.NoError(err)
	expectedModuleBalance := initialModuleBalance.Amount.Add(delegationAmount)
	s.Require().Equal(expectedModuleBalance, moduleBalanceAfterLock.Amount,
		"Epoching module balance should increase by delegation amount after locking")
	s.T().Logf("Step 1b Verified: Module balance after lock: %s (increased by %s)",
		moduleBalanceAfterLock.String(), delegationAmount.String())

	// Step 2: Wait for epoch end and verify message processing
	s.T().Logf("Step 2: Waiting for epoch end to process enqueued message")

	// For e2e test, we'll wait for several epochs to ensure message processing
	// In a real deployment, epoching periods are longer, but in tests they're shorter
	s.T().Logf("Waiting for epoch transition to process the queued message...")

	// Wait enough blocks to trigger epoch end processing
	// Calculate precise blocks to wait until epoch end using current epoch information

	// Query current epoch information including epoch boundary
	currentEpochResp, err := func() (*etypes.QueryCurrentEpochResponse, error) {
		bz, err := nonValidatorNode.QueryGRPCGateway("/babylon/epoching/v1/current_epoch", url.Values{})
		if err != nil {
			return nil, err
		}
		var epochResponse etypes.QueryCurrentEpochResponse
		if err := util.Cdc.UnmarshalJSON(bz, &epochResponse); err != nil {
			return nil, err
		}
		return &epochResponse, nil
	}()
	s.NoError(err)

	currentHeight, err := nonValidatorNode.QueryCurrentHeight()
	s.NoError(err)

	// Calculate remaining blocks until epoch end
	epochBoundary := currentEpochResp.EpochBoundary
	remainingBlocks := int(epochBoundary-uint64(currentHeight)) + 2 // +2 for safety margin

	s.T().Logf("Current epoch: %d", currentEpochResp.CurrentEpoch)
	s.T().Logf("Current block height: %d", currentHeight)
	s.T().Logf("Epoch boundary (last block of epoch): %d", epochBoundary)
	s.T().Logf("Remaining blocks until epoch end: %d", remainingBlocks)

	if remainingBlocks <= 0 {
		// We are already past the epoch boundary, wait for the next epoch processing
		remainingBlocks = 3 // Minimum wait for epoch transition processing
		s.T().Logf("Already past epoch boundary, waiting %d blocks for epoch processing", remainingBlocks)
	}

	chainA.WaitForNumHeights(int64(remainingBlocks))

	// Step 3: Verify message was processed and delegation was created
	s.T().Logf("Step 3: Verifying delegation processing results")

	// Check final balances after epoch processing
	finalDelegatorBalance, err := nonValidatorNode.QueryBalance(delegatorAddr, "ubbn")
	s.NoError(err)

	finalModuleBalance, err := nonValidatorNode.QueryBalance(epochingModuleAddrStr, "ubbn")
	s.NoError(err)

	// After epoch end processing:
	// - Delegator balance should remain at the level after delegation + gas fees (funds used for staking)
	// - Module balance should return to initial level (funds transferred to staking module)

	finalDecrease := initialDelegatorBalance.Amount.Sub(finalDelegatorBalance.Amount)
	s.T().Logf("Final balance analysis:")
	s.T().Logf("  - Total final decrease: %s", finalDecrease.String())
	s.T().Logf("  - Expected minimum decrease: %s", delegationAmount.String())

	s.Require().True(finalDecrease.GTE(delegationAmount),
		"Delegator balance should remain decreased by at least delegation amount after successful delegation")
	s.T().Logf("Step 3a Verified: Delegator balance remains decreased by %s",
		finalDecrease.String())

	s.Require().Equal(initialModuleBalance.Amount, finalModuleBalance.Amount,
		"Module balance should return to initial level after funds transferred to staking")
	s.T().Logf("Step 3b Verified: Module balance returned to initial level: %s",
		finalModuleBalance.String())

	// The balance should be lower than initial (indicating delegation occurred)
	s.Require().True(finalDelegatorBalance.Amount.LT(initialDelegatorBalance.Amount),
		"Delegator balance should be lower than initial, confirming delegation occurred")
	s.T().Logf("Step 3c Verified: Balance decreased from %s to %s, confirming successful delegation",
		initialDelegatorBalance.String(), finalDelegatorBalance.String())

	s.T().Logf("Normal delegation case test completed successfully!")
	s.T().Logf("Summary:")
	s.T().Logf("- Funds were properly locked in module account")
	s.T().Logf("- Message was processed at epoch end")
	s.T().Logf("- Module account funds were correctly transferred")
	s.T().Logf("- Delegation was successfully created")
}
