package e2e

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v3/app/params"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	bsctypes "github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
)

type IbcCallbackBsnAddRewards struct {
	BaseBtcRewardsDistribution

	// consumer
	bsn0 *bsctypes.ConsumerRegister

	// 3 fps
	// babylon => fp1
	// consumer0 => fp2, fp3
	// consumer4 => fp4
	fp1bbnBTCSK   *btcec.PrivateKey
	fp2cons0BTCSK *btcec.PrivateKey
	fp3cons0BTCSK *btcec.PrivateKey

	del1BTCSK *btcec.PrivateKey
	del2BTCSK *btcec.PrivateKey

	fp1bbn   *bstypes.FinalityProvider
	fp2cons0 *bstypes.FinalityProvider
	fp3cons0 *bstypes.FinalityProvider

	// 3 BTC Delegations will be made at the beginning
	// (fp1bbn,fp2cons0 del1), (fp1bbn,fp3cons0 del1), (fp1bbn, fp2cons0 del2)

	// (fp1bbn,fp2cons0 del1) fp2fp4Del1StkAmt => 2_00000000
	// (fp1bbn,fp3cons0 del1) fp3fp4Del1StkAmt => 2_00000000
	// (fp1bbn,fp2cons0 del2) fp2Del2StkAmt => 4_00000000
	fp2Del1StkAmt int64
	fp3Del1StkAmt int64
	fp2Del2StkAmt int64

	// bech32 address of the delegators
	del1Addr string
	del2Addr string

	// bech32 address of the finality providers
	fp1bbnAddr   string
	fp2cons0Addr string
	fp3cons0Addr string

	configurer                 configurer.Configurer
	bbnIbcCallbackReceiverAddr string
	bsnSenderAddr              string
	bsnCustomTokenDenom        string
}

const (
	bsnRewardDistributionMemo = "bsn_reward_distribution"
	transferAmount            = int64(100_000000) // 100k custom tokens
	customDenomName           = "testcoin"        // Custom token name
)

// getTestDistributionAddress returns the same deterministic test address used in the keeper
func getTestDistributionAddress() sdk.AccAddress {
	hash := sha256.Sum256([]byte("test_distribution_account"))
	return sdk.AccAddress(hash[:20])
}

func (s *IbcCallbackBsnAddRewards) SetupSuite() {
	s.T().Log("setting up BSN fee collection test suite...")
	var err error

	s.r = rand.New(rand.NewSource(time.Now().Unix()))
	s.net = &chaincfg.SimNetParams
	s.fp1bbnBTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.fp2cons0BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.fp3cons0BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.del1BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.del2BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)

	s.fp2Del1StkAmt = int64(2 * 10e8)
	s.fp3Del1StkAmt = int64(2 * 10e8)
	s.fp2Del2StkAmt = int64(4 * 10e8)

	covenantSKs, _, _ := bstypes.DefaultCovenantCommittee()
	s.covenantSKs = covenantSKs

	// chain config 0 is babylon
	// chain config 1 is BSN
	s.configurer, err = configurer.NewIBCTransferConfigurer(s.T(), true)
	s.Require().NoError(err)

	err = s.configurer.ConfigureChains()
	s.Require().NoError(err)

	err = s.configurer.RunSetup()
	s.Require().NoError(err)
}

func (s *IbcCallbackBsnAddRewards) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

// Test1CreateFinalityProviders creates all finality providers
func (s *IbcCallbackBsnAddRewards) Test1CreateFinalityProviders() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(2)

	n1, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)
	n2, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	n1.WaitForNextBlock()
	n2.WaitForNextBlock()

	s.fp1bbnAddr = n1.KeysAdd(wFp1)
	s.fp2cons0Addr = n2.KeysAdd(wFp2)
	s.fp3cons0Addr = n2.KeysAdd(wFp3)

	n2.BankMultiSendFromNode([]string{s.fp1bbnAddr, s.fp2cons0Addr, s.fp3cons0Addr}, "1000000ubbn")

	n1.WaitForNextBlock()
	n2.WaitForNextBlock()

	clientStatesResp, err := n1.QueryClientStates()
	require.NoError(s.T(), err)
	require.Equal(s.T(), clientStatesResp.ClientStates.Len(), 1)
	clientState := clientStatesResp.ClientStates[0]

	bsn0 := bsctypes.NewCosmosConsumerRegister(
		clientState.ClientId,
		datagen.GenRandomHexStr(s.r, 5),
		"Chain description: "+datagen.GenRandomHexStr(s.r, 15),
		datagen.GenBabylonRewardsCommission(s.r),
	)
	n1.RegisterConsumerChain(n1.WalletName, bsn0.ConsumerId, bsn0.ConsumerName, bsn0.ConsumerDescription, bsn0.BabylonRewardsCommission.String())
	s.bsn0 = bsn0

	n1.WaitForNextBlock()
	n2.WaitForNextBlock()

	consumers := n2.QueryBTCStkConsumerConsumers()
	require.Len(s.T(), consumers, 1)
	s.T().Log("All Consumers created")

	s.fp1bbn = CreateNodeFP(
		s.T(),
		s.r,
		s.fp1bbnBTCSK,
		n1,
		s.fp1bbnAddr,
		n1.ChainID(),
	)
	require.NotNil(s.T(), s.fp1bbn)

	s.fp2cons0 = CreateNodeFP(
		s.T(),
		s.r,
		s.fp2cons0BTCSK,
		n2,
		s.fp2cons0Addr,
		bsn0.ConsumerId,
	)
	s.NotNil(s.fp2cons0)

	s.fp3cons0 = CreateNodeFP(
		s.T(),
		s.r,
		s.fp3cons0BTCSK,
		n2,
		s.fp3cons0Addr,
		bsn0.ConsumerId,
	)
	s.NotNil(s.fp3cons0)

	n1.WaitForNextBlock()
	n2.WaitForNextBlock()

	babylonFps := n2.QueryFinalityProviders(n1.ChainID())
	cons0Fps := n2.QueryFinalityProviders(bsn0.ConsumerId)

	require.Len(s.T(), append(babylonFps, cons0Fps...), 3, "should have created all the FPs to start the test")
	s.T().Log("All Fps created")
}

// Test2CreateBtcDelegations creates 3 btc delegations
func (s *IbcCallbackBsnAddRewards) Test2CreateBtcDelegations() {
	n2, err := s.configurer.GetChainConfig(0).GetNodeAtIndex(2)
	s.NoError(err)

	s.del1Addr = n2.KeysAdd(wDel1)
	s.del2Addr = n2.KeysAdd(wDel2)

	n2.BankMultiSendFromNode([]string{s.del1Addr, s.del2Addr}, "1000000ubbn")

	n2.WaitForNextBlock()

	s.CreateBTCDelegationMultipleFPsAndCheck(n2, wDel1, s.del1BTCSK, s.del1Addr, s.fp2Del1StkAmt, s.fp1bbn, s.fp2cons0)
	s.CreateBTCDelegationMultipleFPsAndCheck(n2, wDel1, s.del1BTCSK, s.del1Addr, s.fp3Del1StkAmt, s.fp1bbn, s.fp3cons0)
	s.CreateBTCDelegationMultipleFPsAndCheck(n2, wDel2, s.del2BTCSK, s.del2Addr, s.fp2Del2StkAmt, s.fp1bbn, s.fp2cons0)

	resp := n2.QueryBtcDelegations(bstypes.BTCDelegationStatus_ANY)
	require.Len(s.T(), resp.BtcDelegations, 3)

	s.CreateCovenantsAndSubmitSignaturesToPendDels(n2, s.fp1bbn)
}

func (s *IbcCallbackBsnAddRewards) Test3CreateFactoryToken() {
	bbnChain0 := s.configurer.GetChainConfig(0)
	bsnChain1 := s.configurer.GetChainConfig(1)

	bbnNode, err := bbnChain0.GetNodeAtIndex(2)
	s.NoError(err)
	bsnNode, err := bsnChain1.GetNodeAtIndex(2)
	s.NoError(err)

	s.bbnIbcCallbackReceiverAddr = bbnNode.KeysAdd("bsn-receiver")

	// Create and fund sender account
	s.bsnSenderAddr = bsnNode.KeysAdd("bsn-sender")
	bsnNode.BankSendFromNode(s.bsnSenderAddr, "15000000ubbn") // Give enough ubbn for tokenfactory creation fee (10M) + tx fees
	bsnNode.WaitForNextBlock()

	// Create custom denom using tokenfactory
	customDenomName := datagen.GenRandomHexStr(s.r, 10)
	s.bsnCustomTokenDenom = fmt.Sprintf("factory/%s/%s", s.bsnSenderAddr, customDenomName)
	s.T().Logf("Creating custom denom: %s", s.bsnCustomTokenDenom)

	bsnNode.CreateDenom(s.bsnSenderAddr, customDenomName)
	bsnNode.WaitForNextBlock()

	mintAmt := s.r.Int63n(10_000000) + 10_000000
	mintInt := math.NewInt(mintAmt)

	bsnNode.MintDenom(s.bsnSenderAddr, mintInt.String(), s.bsnCustomTokenDenom)
	bsnNode.WaitForNextBlock()

	bsnSenderBalances, err := bsnNode.QueryBalances(s.bsnSenderAddr)
	s.Require().NoError(err)

	// Check custom denom balance specifically
	customBalance := bsnSenderBalances.AmountOf(s.bsnCustomTokenDenom)
	require.Equal(s.T(), customBalance.String(), mintInt.String(), "Should have custom tokens after minting")
}

func (s *IbcCallbackBsnAddRewards) Test4FailSendBsnRewardsCallback() {
	bsnChain1 := s.configurer.GetChainConfig(1)

	bsnNode, err := bsnChain1.GetNodeAtIndex(2)
	s.NoError(err)

	// Create transfer coin using custom denom
	transferAmt := s.r.Int63n(2_000000) + 1_000000
	tranferInt := math.NewInt(transferAmt)
	transferCoin := sdk.NewCoin(s.bsnCustomTokenDenom, tranferInt)

	// Create bad JSON callback memo
	callbackMemo := bstypes.CallbackMemo{
		Action: bstypes.CallbackActionAddBsnRewardsMemo,
		AddBsnRewards: &bstypes.CallbackAddBsnRewards{
			BsnConsumerID: "x",
		},
	}

	callbackMemoJSON, err := json.Marshal(callbackMemo)
	s.Require().NoError(err)
	callbackMemoString := string(callbackMemoJSON)

	bsnSenderBefore, err := bsnNode.QueryBalances(s.bsnSenderAddr)
	s.Require().NoError(err)

	ibcTransferTxHash := bsnNode.SendIBCTransfer(s.bsnSenderAddr, s.bbnIbcCallbackReceiverAddr, callbackMemoString, transferCoin)
	bsnNode.WaitForNextBlocks(5)

	// Query transaction to get fees
	ibcTxRes, ibcTx, err := bsnNode.QueryTxWithError(ibcTransferTxHash)
	s.Require().NoError(err)
	s.Require().Zero(ibcTxRes.Code, fmt.Sprintf("Transaction failed with code %d: %s", ibcTxRes.Code, ibcTxRes.RawLog))

	s.Eventually(func() bool {
		bsnSenderAfter, err := bsnNode.QueryBalances(s.bsnSenderAddr)
		s.Require().NoError(err)

		bsnSenderAfterFee := bsnSenderAfter.Sub(ibcTx.GetFee()...)
		return bsnSenderAfterFee.Equal(bsnSenderBefore)
	}, time.Minute*4, time.Second, "balance is not equal to %s", bsnSenderBefore.String())
}

func (s *IbcCallbackBsnAddRewards) Test5SendBsnRewardsCallback() {
	bbnChain0 := s.configurer.GetChainConfig(0)
	bsnChain1 := s.configurer.GetChainConfig(1)

	bbnNode, err := bbnChain0.GetNodeAtIndex(2)
	s.NoError(err)
	bsnNode, err := bsnChain1.GetNodeAtIndex(2)
	s.NoError(err)

	transferAmt := s.r.Int63n(2_000000) + 1_000000
	tranferInt := math.NewInt(transferAmt)
	transferCoin := sdk.NewCoin(s.bsnCustomTokenDenom, tranferInt)

	fp2Ratio, fp3Ratio := math.LegacyMustNewDecFromStr("0.7"), math.LegacyMustNewDecFromStr("0.3")

	// Create JSON callback memo for IBC callback middleware with BSN action
	callbackMemo := bstypes.CallbackMemo{
		Action: bstypes.CallbackActionAddBsnRewardsMemo,
		AddBsnRewards: &bstypes.CallbackAddBsnRewards{
			BsnConsumerID: s.bsn0.ConsumerId,
			FpRatios: []bstypes.FpRatio{
				{
					BtcPk: s.fp2cons0.BtcPk,
					Ratio: fp2Ratio,
				},
				{
					BtcPk: s.fp3cons0.BtcPk,
					Ratio: fp3Ratio,
				},
			},
		},
	}

	// Convert struct to JSON string
	callbackMemoJSON, err := json.Marshal(callbackMemo)
	s.Require().NoError(err)
	callbackMemoString := string(callbackMemoJSON)

	bbnAccCommAddr := params.AccBbnComissionCollectorBsn.String()
	var ibcTransferTxHash string
	balancesDiff := bbnNode.BalancesDiff(func() {
		ibcTransferTxHash = bsnNode.SendIBCTransfer(s.bsnSenderAddr, s.bbnIbcCallbackReceiverAddr, callbackMemoString, transferCoin)
		bsnNode.WaitForNextBlocks(5)
	}, s.bbnIbcCallbackReceiverAddr, bbnAccCommAddr)

	// Query transaction to ensure it was successful
	txRes, _, err := bsnNode.QueryTxWithError(ibcTransferTxHash)
	s.Require().NoError(err)
	s.Require().Zero(txRes.Code, fmt.Sprintf("Transaction failed with code %d: %s", txRes.Code, txRes.RawLog))

	bbnCommissionDiff := balancesDiff[bbnAccCommAddr]
	bsnRewardsDenomInBbn := getFirstIBCDenom(bbnCommissionDiff)
	s.Require().Equal("x", bsnRewardsDenomInBbn)
}

// TestBSNFeeCollectionWithCorrectMemo tests BSN fee collection with the correct memo
// func (s *IbcCallbackBsnAddRewardsTestSuite) TestBSNFeeCollectionWithCorrectMemo() {
// 	bbnChainA := s.configurer.GetChainConfig(0)
// 	bbnChainB := s.configurer.GetChainConfig(1)

// 	nA, err := bbnChainA.GetNodeAtIndex(2)
// 	s.NoError(err)
// 	nB, err := bbnChainB.GetNodeAtIndex(2)
// 	s.NoError(err)

// 	// Create and fund sender account
// 	s.bsnSenderAddr = nA.KeysAdd("bsn-sender")
// 	nA.BankSendFromNode(s.bsnSenderAddr, "15000000ubbn") // Give enough ubbn for tokenfactory creation fee (10M) + tx fees
// 	nA.WaitForNextBlock()

// 	// Create custom denom using tokenfactory
// 	customDenom := fmt.Sprintf("factory/%s/%s", s.bsnSenderAddr, customDenomName)
// 	s.T().Logf("Creating custom denom: %s", customDenom)

// 	// Create the denom
// 	nA.CreateDenom(s.bsnSenderAddr, customDenomName)
// 	nA.WaitForNextBlock()

// 	// Mint custom tokens to sender
// 	mintAmount := fmt.Sprintf("%d", transferAmount*10) // Mint 10x what we need
// 	nA.MintDenom(s.bsnSenderAddr, mintAmount, customDenom)
// 	nA.WaitForNextBlock()

// 	// Verify sender has the custom tokens
// 	balanceBeforeSend, err := nA.QueryBalances(s.bsnSenderAddr)
// 	s.Require().NoError(err)

// 	// Check custom denom balance specifically
// 	customBalance := balanceBeforeSend.AmountOf(customDenom)
// 	s.Require().True(customBalance.GT(sdkmath.ZeroInt()), "Should have custom tokens after minting")

// 	// // Get BSN fee collector module account address on chain B
// 	// bsnFeeCollectorAddr, err := nB.QueryModuleAddress(ictvtypes.BSNFeeCollectorName)
// 	// s.Require().NoError(err)

// 	// Get test distribution account address (instead of distribution module which can't receive custom tokens)
// 	testDistributionAddr := getTestDistributionAddress()

// 	// Get initial balances
// 	initialBSNBalance, err := nB.QueryBalances(bsnFeeCollectorAddr.String())
// 	s.Require().NoError(err)

// 	initialTestDistributionBalance, err := nB.QueryBalances(testDistributionAddr.String())
// 	s.Require().NoError(err)

// 	// Create transfer coin using custom denom
// 	transferCoin := sdk.NewInt64Coin(customDenom, transferAmount)

// 	// Create JSON callback memo for IBC callback middleware with BSN action
// 	callbackMemo := bstypes.CallbackMemo{
// 		Action: bstypes.CallbackActionAddBsnRewardsMemo,
// 		AddBsnRewards: &bstypes.CallbackAddBsnRewards{
// 			BsnConsumerID: "x",
// 		},
// 	}
// 	// Convert struct to JSON string
// 	callbackMemoJSON, err := json.Marshal(callbackMemo)
// 	s.Require().NoError(err)
// 	callbackMemoString := string(callbackMemoJSON)

// 	txHash := nA.SendIBCTransfer(s.bsnSenderAddr, bsnFeeCollectorAddr.String(), callbackMemoString, transferCoin)
// 	nA.WaitForNextBlock()

// 	// Query transaction to ensure it was successful
// 	txRes, _, err := nA.QueryTxWithError(txHash)
// 	s.Require().NoError(err)
// 	s.Require().Zero(txRes.Code, fmt.Sprintf("Transaction failed with code %d: %s", txRes.Code, txRes.RawLog))

// 	// Calculate expected amounts after 50% distribution
// 	expectedDistributionAmount := transferAmount / 2 // 50% goes to distribution
// 	expectedBSNAmount := transferAmount / 2          // 50% remains in BSN fee collector

// 	// Check BSN fee collector balance - should have received 50% of transfer
// 	s.Require().Eventually(func() bool {
// 		finalBSNBalance, err := nB.QueryBalances(bsnFeeCollectorAddr.String())
// 		if err != nil {
// 			s.T().Logf("Failed to query BSN fee collector balance: %s", err.Error())
// 			return false
// 		}

// 		// Look for the IBC denom
// 		ibcDenom := getFirstIBCDenom(finalBSNBalance)
// 		if ibcDenom == "" {
// 			s.T().Logf("No IBC denom found in BSN fee collector balance: %s", finalBSNBalance.String())
// 			return false
// 		}
// 		s.T().Logf("Found IBC denom: %s", ibcDenom)

// 		// Calculate received amount (final - initial)
// 		initialIBCAmount := initialBSNBalance.AmountOf(ibcDenom).Int64()
// 		finalIBCAmount := finalBSNBalance.AmountOf(ibcDenom).Int64()
// 		actualBSNAmount := finalIBCAmount - initialIBCAmount

// 		return actualBSNAmount == expectedBSNAmount
// 	}, 2*time.Minute, 5*time.Second, "BSN fee collector did not receive expected amount")

// 	// Check test distribution account balance - should have received 50% of transfer
// 	s.Require().Eventually(func() bool {
// 		finalTestDistributionBalance, err := nB.QueryBalances(testDistributionAddr.String())
// 		if err != nil {
// 			s.T().Logf("Failed to query test distribution balance: %s", err.Error())
// 			return false
// 		}

// 		// Look for the IBC denom
// 		ibcDenom := getFirstIBCDenom(finalTestDistributionBalance)
// 		if ibcDenom == "" {
// 			s.T().Logf("No IBC denom found in test distribution balance: %s", finalTestDistributionBalance.String())
// 			return false
// 		}
// 		s.T().Logf("Found IBC denom in test distribution: %s", ibcDenom)

// 		// Calculate received amount (final - initial)
// 		initialIBCAmount := initialTestDistributionBalance.AmountOf(ibcDenom).Int64()
// 		finalIBCAmount := finalTestDistributionBalance.AmountOf(ibcDenom).Int64()
// 		actualDistributionAmount := finalIBCAmount - initialIBCAmount

// 		return actualDistributionAmount == expectedDistributionAmount
// 	}, 2*time.Minute, 5*time.Second, "Test distribution account did not receive expected amount")

// 	s.T().Logf("BSN fee collector received: %d custom tokens (50%% of transfer)", expectedBSNAmount)
// 	s.T().Logf("Test distribution account received: %d custom tokens (50%% of transfer)", expectedDistributionAmount)
// }
