package e2e

import (
	"crypto/sha256"
	"fmt"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer"
	incentivetypes "github.com/babylonlabs-io/babylon/v3/x/incentive/types"
)

type BSNFeeCollectionTestSuite struct {
	suite.Suite

	configurer configurer.Configurer
	senderAddr string
}

const (
	bsnRewardDistributionMemo = "bsn_reward_distribution"
	transferAmount            = int64(100_000) // 100k custom tokens
	customDenomName           = "testcoin"     // Custom token name
)

// getTestDistributionAddress returns the same deterministic test address used in the keeper
func getTestDistributionAddress() sdk.AccAddress {
	hash := sha256.Sum256([]byte("test_distribution_account"))
	return sdk.AccAddress(hash[:20])
}

func (s *BSNFeeCollectionTestSuite) SetupSuite() {
	s.T().Log("setting up BSN fee collection test suite...")
	var err error

	s.configurer, err = configurer.NewIBCTransferConfigurer(s.T(), true)
	s.Require().NoError(err)

	err = s.configurer.ConfigureChains()
	s.Require().NoError(err)

	err = s.configurer.RunSetup()
	s.Require().NoError(err)
}

func (s *BSNFeeCollectionTestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

// TestBSNFeeCollectionWithCorrectMemo tests BSN fee collection with the correct memo
func (s *BSNFeeCollectionTestSuite) TestBSNFeeCollectionWithCorrectMemo() {
	bbnChainA := s.configurer.GetChainConfig(0)
	bbnChainB := s.configurer.GetChainConfig(1)

	nA, err := bbnChainA.GetNodeAtIndex(2)
	s.NoError(err)
	nB, err := bbnChainB.GetNodeAtIndex(2)
	s.NoError(err)

	// Create and fund sender account
	s.senderAddr = nA.KeysAdd("bsn-sender")
	nA.BankSendFromNode(s.senderAddr, "15000000ubbn") // Give enough ubbn for tokenfactory creation fee (10M) + tx fees
	nA.WaitForNextBlock()

	// Create custom denom using tokenfactory
	customDenom := fmt.Sprintf("factory/%s/%s", s.senderAddr, customDenomName)
	s.T().Logf("Creating custom denom: %s", customDenom)

	// Create the denom
	nA.CreateDenom(s.senderAddr, customDenomName)
	nA.WaitForNextBlock()

	// Mint custom tokens to sender
	mintAmount := fmt.Sprintf("%d", transferAmount*10) // Mint 10x what we need
	nA.MintDenom(s.senderAddr, mintAmount, customDenom)
	nA.WaitForNextBlock()

	// Verify sender has the custom tokens
	balanceBeforeSend, err := nA.QueryBalances(s.senderAddr)
	s.Require().NoError(err)

	// Check custom denom balance specifically
	customBalance := balanceBeforeSend.AmountOf(customDenom)
	s.Require().True(customBalance.GT(sdkmath.ZeroInt()), "Should have custom tokens after minting")

	// Get BSN fee collector module account address on chain B
	bsnFeeCollectorAddr, err := nB.QueryModuleAddress(incentivetypes.BSNFeeCollectorName)
	s.Require().NoError(err)

	// Get test distribution account address (instead of distribution module which can't receive custom tokens)
	testDistributionAddr := getTestDistributionAddress()

	// Get initial balances
	initialBSNBalance, err := nB.QueryBalances(bsnFeeCollectorAddr.String())
	s.Require().NoError(err)

	initialTestDistributionBalance, err := nB.QueryBalances(testDistributionAddr.String())
	s.Require().NoError(err)

	// Create transfer coin using custom denom
	transferCoin := sdk.NewInt64Coin(customDenom, transferAmount)

	// Create JSON callback memo for IBC callback middleware with BSN action
	callbackMemo := fmt.Sprintf(`{
		"dest_callback": {
			"address": "%s"
		},
		"action": "%s"
	}`, bsnFeeCollectorAddr.String(), bsnRewardDistributionMemo)

	txHash := nA.SendIBCTransfer(s.senderAddr, bsnFeeCollectorAddr.String(), callbackMemo, transferCoin)
	nA.WaitForNextBlock()

	// Query transaction to ensure it was successful
	txRes, _, err := nA.QueryTxWithError(txHash)
	s.Require().NoError(err)
	s.Require().Zero(txRes.Code, fmt.Sprintf("Transaction failed with code %d: %s", txRes.Code, txRes.RawLog))

	// Calculate expected amounts after 50% distribution
	expectedDistributionAmount := transferAmount / 2 // 50% goes to distribution
	expectedBSNAmount := transferAmount / 2          // 50% remains in BSN fee collector

	// Check BSN fee collector balance - should have received 50% of transfer
	s.Require().Eventually(func() bool {
		finalBSNBalance, err := nB.QueryBalances(bsnFeeCollectorAddr.String())
		if err != nil {
			s.T().Logf("Failed to query BSN fee collector balance: %s", err.Error())
			return false
		}

		// Look for the IBC denom
		ibcDenom := getFirstIBCDenom(finalBSNBalance)
		if ibcDenom == "" {
			s.T().Logf("No IBC denom found in BSN fee collector balance: %s", finalBSNBalance.String())
			return false
		}
		s.T().Logf("Found IBC denom: %s", ibcDenom)

		// Calculate received amount (final - initial)
		initialIBCAmount := initialBSNBalance.AmountOf(ibcDenom).Int64()
		finalIBCAmount := finalBSNBalance.AmountOf(ibcDenom).Int64()
		actualBSNAmount := finalIBCAmount - initialIBCAmount

		return actualBSNAmount == expectedBSNAmount
	}, 2*time.Minute, 5*time.Second, "BSN fee collector did not receive expected amount")

	// Check test distribution account balance - should have received 50% of transfer
	s.Require().Eventually(func() bool {
		finalTestDistributionBalance, err := nB.QueryBalances(testDistributionAddr.String())
		if err != nil {
			s.T().Logf("Failed to query test distribution balance: %s", err.Error())
			return false
		}

		// Look for the IBC denom
		ibcDenom := getFirstIBCDenom(finalTestDistributionBalance)
		if ibcDenom == "" {
			s.T().Logf("No IBC denom found in test distribution balance: %s", finalTestDistributionBalance.String())
			return false
		}
		s.T().Logf("Found IBC denom in test distribution: %s", ibcDenom)

		// Calculate received amount (final - initial)
		initialIBCAmount := initialTestDistributionBalance.AmountOf(ibcDenom).Int64()
		finalIBCAmount := finalTestDistributionBalance.AmountOf(ibcDenom).Int64()
		actualDistributionAmount := finalIBCAmount - initialIBCAmount

		return actualDistributionAmount == expectedDistributionAmount
	}, 2*time.Minute, 5*time.Second, "Test distribution account did not receive expected amount")

	s.T().Logf("BSN fee collector received: %d custom tokens (50%% of transfer)", expectedBSNAmount)
	s.T().Logf("Test distribution account received: %d custom tokens (50%% of transfer)", expectedDistributionAmount)
}
