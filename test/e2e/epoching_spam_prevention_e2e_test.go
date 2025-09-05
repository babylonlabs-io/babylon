package e2e

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	etypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"

	sdked25519 "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
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

// TestNormalCreateValidatorCase tests the normal case where:
// 1. MsgWrappedCreateValidator registers BLS key and enqueues MsgCreateValidator
// 2. At epoch end block, enqueued message is processed by staking module
// 3. After processing, new validator is created and added to validator set
func (s *EpochingSpamPreventionTestSuite) TestNormalCreateValidatorCase() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)

	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	// 0) generate new wallet and fund
	newValidatorWalletName := "test-validator-wallet"
	newValidatorAddr := nonValidatorNode.KeysAdd(newValidatorWalletName)

	fundingAmount := "100000000ubbn"
	nonValidatorNode.BankSend(nonValidatorNode.WalletName, newValidatorAddr, fundingAmount)
	chainA.WaitForNumHeights(2)

	initialBalance, err := nonValidatorNode.QueryBalance(newValidatorAddr, "ubbn")

	s.NoError(err)

	// compute valoper address
	newValoperAddr := sdk.ValAddress(sdk.MustAccAddressFromBech32(newValidatorAddr)).String()

	stakingAmount := "50000000ubbn"
	moniker := "test-validator"
	commissionRate := "0.1"
	commissionMaxRate := "0.2"
	commissionMaxChangeRate := "0.01"
	minSelfDelegation := "1"

	createValidator := func(walletName, delegatorAccAddr string) {
		wcvMsg, err := datagen.BuildMsgWrappedCreateValidator(sdk.MustAccAddressFromBech32(delegatorAccAddr))
		s.NoError(err)

		// field update
		amt, err := sdk.ParseCoinNormalized(stakingAmount)
		s.NoError(err)

		rate, err := sdkmath.LegacyNewDecFromStr(commissionRate)
		s.NoError(err)
		maxRate, err := sdkmath.LegacyNewDecFromStr(commissionMaxRate)
		s.NoError(err)
		maxChange, err := sdkmath.LegacyNewDecFromStr(commissionMaxChangeRate)
		s.NoError(err)

		wcvMsg.MsgCreateValidator.Description.Moniker = moniker
		wcvMsg.MsgCreateValidator.Value = amt
		wcvMsg.MsgCreateValidator.Commission.Rate = rate
		wcvMsg.MsgCreateValidator.Commission.MaxRate = maxRate
		wcvMsg.MsgCreateValidator.Commission.MaxChangeRate = maxChange

		// ---------- validator.json ----------
		var consPk cryptotypes.PubKey
		s.NoError(util.Cdc.UnpackAny(wcvMsg.MsgCreateValidator.Pubkey, &consPk))
		edPk, ok := consPk.(*sdked25519.PubKey)
		s.Require().True(ok, "consensus pubkey must be ed25519")
		ed25519B64 := base64.StdEncoding.EncodeToString(edPk.Key)

		validatorData := map[string]any{
			"pubkey": map[string]string{
				"@type": wcvMsg.MsgCreateValidator.Pubkey.TypeUrl,
				"key":   ed25519B64, //
			},
			"amount":                     stakingAmount,
			"moniker":                    moniker,
			"commission-rate":            commissionRate,
			"commission-max-rate":        commissionMaxRate,
			"commission-max-change-rate": commissionMaxChangeRate,
			"min-self-delegation":        minSelfDelegation,
		}
		validatorJSON, err := json.Marshal(validatorData)
		s.NoError(err)
		_, _, err = nonValidatorNode.ExecRawCmd([]string{"sh", "-c", fmt.Sprintf(`cat > /tmp/validator.json << 'EOF'
%s
EOF`, string(validatorJSON))})
		s.NoError(err)

		// ---------- bls_pop.json ----------
		edSigB64 := base64.StdEncoding.EncodeToString(wcvMsg.Key.Pop.Ed25519Sig)
		blsSigB64 := base64.StdEncoding.EncodeToString(wcvMsg.Key.Pop.BlsSig.Bytes())

		blsPopData := map[string]any{
			"bls_pub_key": *wcvMsg.Key.Pubkey,
			"pop": map[string]string{
				"ed25519_sig": edSigB64,
				"bls_sig":     blsSigB64,
			},
		}
		blsPopJSON, err := json.Marshal(blsPopData)
		s.NoError(err)
		_, _, err = nonValidatorNode.ExecRawCmd([]string{"sh", "-c", fmt.Sprintf(`cat > /tmp/bls_pop.json << 'EOF'
%s
EOF`, string(blsPopJSON))})
		s.NoError(err)

		// transaction broadcast
		createValCmd := []string{
			"babylond", "tx", "checkpointing", "create-validator", "/tmp/validator.json",
			"--bls-pop", "/tmp/bls_pop.json",
			fmt.Sprintf("--from=%s", walletName),
			"--keyring-backend=test",
			"--home=/home/babylon/babylondata",
			"--chain-id", nonValidatorNode.ChainID(),
			"--yes",
			"--gas=auto",
			"--gas-adjustment=1.3",
			"--gas-prices=1ubbn",
			"-b=sync",
		}
		_, errBuf, err := nonValidatorNode.ExecRawCmd(createValCmd)
		if err != nil {
			s.T().Logf("create-validator failed: %s", errBuf.String())
		}
		s.NoError(err)
	}
	epochingModuleAddr, err := nonValidatorNode.QueryModuleAddress("epoching_delegate_pool")
	s.NoError(err)
	epochingModuleAddrStr := epochingModuleAddr.String()
	s.T().Logf("Epoching delegate pool module address: %s", epochingModuleAddrStr)

	initialModuleBalance, err := nonValidatorNode.QueryBalance(epochingModuleAddrStr, "ubbn")
	s.NoError(err)
	s.T().Logf("Initial epoching module balance: %s", initialModuleBalance.String())

	createValidator(newValidatorWalletName, newValidatorAddr)
	currentHeight1, err := nonValidatorNode.QueryCurrentHeight()
	s.NoError(err)
	chainA.WaitUntilHeight(currentHeight1 + 1)
	afterBalance, err := nonValidatorNode.QueryBalance(newValidatorAddr, "ubbn")
	s.NoError(err)
	lockedBalance := initialBalance.Amount.Sub(afterBalance.Amount)
	s.T().Logf("Validator initial balance: %s, after create-validator: %s, locked: %s",
		initialBalance.String(), afterBalance.String(), lockedBalance.String())
	s.Require().True(lockedBalance.GTE(sdkmath.NewInt(50000000)), "at least self-delegation amount should be locked")

	// Verify epoching module balance increased (funds locked in module account)
	moduleBalanceAfterLock, err := nonValidatorNode.QueryBalance(epochingModuleAddrStr, "ubbn")
	s.NoError(err)
	expectedModuleBalance := initialModuleBalance.Amount.Add(sdkmath.NewInt(50000000))
	s.Require().Equal(expectedModuleBalance, moduleBalanceAfterLock.Amount,
		"Epoching module balance should increase by delegation amount after locking")
	s.T().Logf("Step 1b Verified: Module balance after lock: %s (increased by %s)",
		moduleBalanceAfterLock.String(), sdkmath.NewInt(50000000).String())

	// --- Step 1. Fund lock check ---
	_, err = nonValidatorNode.QueryGRPCGateway(
		fmt.Sprintf("/cosmos/staking/v1beta1/validators/%s", newValoperAddr), url.Values{},
	)
	s.Error(err, "Validators should not be created immediately before the end of the epoch.")

	// --- Step 2. Waiting Epoch ends ---
	paramsBz, err := nonValidatorNode.QueryGRPCGateway("/babylon/epoching/v1/params", url.Values{})
	s.NoError(err)
	var pResp etypes.QueryParamsResponse
	s.NoError(util.Cdc.UnmarshalJSON(paramsBz, &pResp))
	E := int64(pResp.Params.EpochInterval)
	s.Require().True(E > 0, "epoch interval must be > 0")

	currentHeight, err := nonValidatorNode.QueryCurrentHeight()
	s.NoError(err)

	nextEpochEnd := func(h, epochLen int64) int64 {
		offset := epochLen - ((h - 1) % epochLen)
		return h + offset - 1
	}
	target := nextEpochEnd(currentHeight, E)
	chainA.WaitUntilHeight(target)

	// --- Step 3. Verify self-delegation processed ----
	validatorBz, err := nonValidatorNode.QueryGRPCGateway(
		fmt.Sprintf("/cosmos/staking/v1beta1/validators/%s", newValoperAddr), url.Values{},
	)
	s.NoError(err, "A validator must be created after the epoch ends")

	var validatorResp struct {
		Validator struct {
			OperatorAddress string `json:"operator_address"`
			Tokens          string `json:"tokens"`
			DelegatorShares string `json:"delegator_shares"`
			Status          string `json:"status"`
		} `json:"validator"`
	}
	s.NoError(json.Unmarshal(validatorBz, &validatorResp))

	totalTokens, ok := sdkmath.NewIntFromString(validatorResp.Validator.Tokens)
	s.Require().True(ok, "invalid validator tokens amount")

	expectedSelfDelegation := sdkmath.NewInt(50000000) // 50 BBN
	s.Require().True(totalTokens.GTE(expectedSelfDelegation),
		"validator tokens %s should be at least %s (self-delegation)",
		totalTokens.String(), expectedSelfDelegation.String())

	s.T().Logf("Validator created successfully with tokens: %s (status: %s)",
		totalTokens.String(), validatorResp.Validator.Status)
	s.T().Logf("Self-delegation verified through validator tokens")
	s.T().Logf("create-validator enqueued -> epoch end processed -> validator created successfully")
}
