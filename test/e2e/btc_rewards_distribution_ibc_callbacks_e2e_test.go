package e2e

import (
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
	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v3/testutil/coins"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	bsctypes "github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
	itypes "github.com/babylonlabs-io/babylon/v3/x/incentive/types"
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
	chainB := s.configurer.GetChainConfig(1)
	chainA.WaitUntilHeight(2)
	chainB.WaitUntilHeight(2)

	bbnNode := s.BbnNode()
	bbnNode.WaitForNextBlock()

	s.fp1bbnAddr = bbnNode.KeysAdd(wFp1)
	s.fp2cons0Addr = bbnNode.KeysAdd(wFp2)
	s.fp3cons0Addr = bbnNode.KeysAdd(wFp3)

	bbnNode.BankMultiSendFromNode([]string{s.fp1bbnAddr, s.fp2cons0Addr, s.fp3cons0Addr}, "1000000ubbn")

	bbnNode.WaitForNextBlock()

	clientStatesResp, err := bbnNode.QueryClientStates()
	require.NoError(s.T(), err)
	require.Equal(s.T(), clientStatesResp.ClientStates.Len(), 1)
	clientState := clientStatesResp.ClientStates[0]

	bsn0 := bsctypes.NewCosmosConsumerRegister(
		clientState.ClientId,
		datagen.GenRandomHexStr(s.r, 5),
		"Chain description: "+datagen.GenRandomHexStr(s.r, 15),
		datagen.GenBabylonRewardsCommission(s.r),
	)
	bbnNode.RegisterConsumerChain(bbnNode.WalletName, bsn0.ConsumerId, bsn0.ConsumerName, bsn0.ConsumerDescription, bsn0.BabylonRewardsCommission.String())
	s.bsn0 = bsn0

	bbnNode.WaitForNextBlock()

	consumers := bbnNode.QueryBTCStkConsumerConsumers()
	require.Len(s.T(), consumers, 1)
	s.T().Log("All Consumers created")

	s.fp1bbn = CreateNodeFP(
		s.T(),
		s.r,
		s.fp1bbnBTCSK,
		bbnNode,
		s.fp1bbnAddr,
		bbnNode.ChainID(),
	)
	require.NotNil(s.T(), s.fp1bbn)

	s.fp2cons0 = CreateNodeFP(
		s.T(),
		s.r,
		s.fp2cons0BTCSK,
		bbnNode,
		s.fp2cons0Addr,
		bsn0.ConsumerId,
	)
	s.NotNil(s.fp2cons0)

	s.fp3cons0 = CreateNodeFP(
		s.T(),
		s.r,
		s.fp3cons0BTCSK,
		bbnNode,
		s.fp3cons0Addr,
		bsn0.ConsumerId,
	)
	s.NotNil(s.fp3cons0)

	babylonFps := bbnNode.QueryFinalityProviders(bbnNode.ChainID())
	cons0Fps := bbnNode.QueryFinalityProviders(bsn0.ConsumerId)

	require.Len(s.T(), append(babylonFps, cons0Fps...), 3, "should have created all the FPs to start the test")
	s.T().Log("All Fps created")

	bbnNode.WaitForNextBlock()
}

// Test2CreateBtcDelegations creates 3 btc delegations
func (s *IbcCallbackBsnAddRewards) Test2CreateBtcDelegations() {
	bbnNode := s.BbnNode()

	s.del1Addr = bbnNode.KeysAdd(wDel1)
	s.del2Addr = bbnNode.KeysAdd(wDel2)

	bbnNode.BankMultiSendFromNode([]string{s.del1Addr, s.del2Addr}, "1000000ubbn")

	bbnNode.WaitForNextBlock()

	s.CreateBTCDelegationMultipleFPsAndCheck(bbnNode, wDel1, s.del1BTCSK, s.del1Addr, s.fp2Del1StkAmt, s.fp1bbn, s.fp2cons0)
	s.CreateBTCDelegationMultipleFPsAndCheck(bbnNode, wDel1, s.del1BTCSK, s.del1Addr, s.fp3Del1StkAmt, s.fp1bbn, s.fp3cons0)
	s.CreateBTCDelegationMultipleFPsAndCheck(bbnNode, wDel2, s.del2BTCSK, s.del2Addr, s.fp2Del2StkAmt, s.fp1bbn, s.fp2cons0)

	resp := bbnNode.QueryBtcDelegations(bstypes.BTCDelegationStatus_ANY)
	require.Len(s.T(), resp.BtcDelegations, 3, "should have all 3 delegations")

	s.CreateCovenantsAndSubmitSignaturesToPendDels(bbnNode, s.fp1bbn)

	s.bbnIbcCallbackReceiverAddr = bbnNode.KeysAdd("bsn-receiver")
	s.T().Log("All BTC delegations created")
}

func (s *IbcCallbackBsnAddRewards) Test3CreateFactoryToken() {
	bsnNode := s.BsnNode()

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

func (s *IbcCallbackBsnAddRewards) Test4SendBsnRewardsCallback() {
	bbnNode := s.BbnNode()
	bsnNode := s.BsnNode()

	transferAmt := s.r.Int63n(2_000000) + 1_000000
	tranferInt := math.NewInt(transferAmt)
	rewardCoin := sdk.NewCoin(s.bsnCustomTokenDenom, tranferInt)

	fp2Ratio, fp3Ratio := math.LegacyMustNewDecFromStr("0.7"), math.LegacyMustNewDecFromStr("0.3")

	// Create JSON callback memo for IBC callback middleware with BSN action
	callbackMemo := bstypes.CallbackMemo{
		Action: bstypes.CallbackActionAddBsnRewardsMemo,
		DestCallback: &bstypes.CallbackInfo{
			// mandatory unused address
			Address: datagen.GenRandomAccount().Address,
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
		},
	}

	// Convert struct to JSON string
	callbackMemoJSON, err := json.Marshal(callbackMemo)
	s.Require().NoError(err)
	callbackMemoString := string(callbackMemoJSON)

	bbnCommDiff, del1Diff, del2Diff, fp1bbnDiff, fp2cons0Diff, fp3cons0Diff := s.SuiteRewardsDiff(bbnNode, func() {
		ibcTransferTxHash := bsnNode.SendIBCTransfer(s.bsnSenderAddr, s.bbnIbcCallbackReceiverAddr, callbackMemoString, rewardCoin)
		bsnNode.WaitForNextBlocks(5)

		// Query transaction to ensure it was successful
		txRes, _, err := bsnNode.QueryTxWithError(ibcTransferTxHash)
		s.Require().NoError(err)
		s.Require().Zero(txRes.Code, fmt.Sprintf("Transaction failed with code %d: %s", txRes.Code, txRes.RawLog))
	})

	rewardCoins := sdk.NewCoins(rewardCoin)

	bbnCommExp := itypes.GetCoinsPortion(rewardCoins, s.bsn0.BabylonRewardsCommission)
	require.Equal(s.T(), bbnCommExp.String(), bbnCommDiff.String(), "babylon commission")

	rewardCoinsAfterBbnComm := rewardCoins.Sub(bbnCommExp...)

	fp2AfterRatio := itypes.GetCoinsPortion(rewardCoinsAfterBbnComm, fp2Ratio)
	fp2CommExp := itypes.GetCoinsPortion(fp2AfterRatio, *s.fp2cons0.Commission)
	require.Equal(s.T(), fp2CommExp.String(), fp2cons0Diff.String(), "fp2 consumer 0 commission")

	fp3AfterRatio := itypes.GetCoinsPortion(rewardCoinsAfterBbnComm, fp3Ratio)
	fp3CommExp := itypes.GetCoinsPortion(fp3AfterRatio, *s.fp3cons0.Commission)
	require.Equal(s.T(), fp3CommExp.String(), fp3cons0Diff.String(), "fp3 consumer 0 commission")

	// Current setup of voting power for consumer 0
	// (fp2cons0, del1) => 2_00000000
	// (fp2cons0, del2) => 4_00000000
	// (fp3cons0, del1) => 2_00000000

	fp2RemainingBtcRewards := fp2AfterRatio.Sub(fp2CommExp...)
	fp3RemainingBtcRewards := fp3AfterRatio.Sub(fp3CommExp...)

	// del1 will receive all the rewards of fp3 and 1/3 of the fp2 rewards
	expectedRewardsDel1 := itypes.GetCoinsPortion(fp2RemainingBtcRewards, math.LegacyMustNewDecFromStr("0.333333333333334")).Add(fp3RemainingBtcRewards...)
	coins.RequireCoinsDiffInPointOnePercentMargin(s.T(), expectedRewardsDel1, del1Diff)
	// del2 will receive 2/3 of the fp2 rewards
	expectedRewardsDel2 := itypes.GetCoinsPortion(fp2RemainingBtcRewards, math.LegacyMustNewDecFromStr("0.666666666666666"))
	coins.RequireCoinsDiffInPointOnePercentMargin(s.T(), expectedRewardsDel2, del2Diff)

	require.True(s.T(), fp1bbnDiff.IsZero(), "fp1 was not rewarded")
}

// func (s *IbcCallbackBsnAddRewards) Test5FailSendBsnRewardsCallback() {
// 	s.T().Skip()
// 	bsnChain1 := s.configurer.GetChainConfig(1)

// 	bsnNode, err := bsnChain1.GetNodeAtIndex(2)
// 	s.NoError(err)

// 	// Create transfer coin using custom denom
// 	transferAmt := s.r.Int63n(2_000000) + 1_000000
// 	tranferInt := math.NewInt(transferAmt)
// 	transferCoin := sdk.NewCoin(s.bsnCustomTokenDenom, tranferInt)

// 	// Create bad JSON callback memo
// 	callbackMemo := bstypes.CallbackMemo{
// 		Action: bstypes.CallbackActionAddBsnRewardsMemo,
// 		DestCallback: &bstypes.CallbackInfo{
// 			Address: datagen.GenRandomAccount().Address,
// 			AddBsnRewards: &bstypes.CallbackAddBsnRewards{
// 				BsnConsumerID: "x",
// 			},
// 		},
// 	}

// 	callbackMemoJSON, err := json.Marshal(callbackMemo)
// 	s.Require().NoError(err)
// 	callbackMemoString := string(callbackMemoJSON)

// 	bsnSenderBefore, err := bsnNode.QueryBalances(s.bsnSenderAddr)
// 	s.Require().NoError(err)

// 	ibcTransferTxHash := bsnNode.SendIBCTransfer(s.bsnSenderAddr, s.bbnIbcCallbackReceiverAddr, callbackMemoString, transferCoin)
// 	bsnNode.WaitForNextBlocks(5)

// 	// Query transaction to get fees
// 	ibcTxRes, ibcTx, err := bsnNode.QueryTxWithError(ibcTransferTxHash)
// 	s.Require().NoError(err)
// 	s.Require().Zero(ibcTxRes.Code, fmt.Sprintf("Transaction failed with code %d: %s", ibcTxRes.Code, ibcTxRes.RawLog))

// 	s.Eventually(func() bool {
// 		bsnSenderAfter, err := bsnNode.QueryBalances(s.bsnSenderAddr)
// 		s.Require().NoError(err)

// 		bsnSenderAfterFee := bsnSenderAfter.Sub(ibcTx.GetFee()...)
// 		return bsnSenderAfterFee.Equal(bsnSenderBefore)
// 	}, time.Minute*4, time.Second, "balance is not equal to %s", bsnSenderBefore.String())
// }

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

// QueryFpRewards returns the rewards available for fp1, fp2, fp3, fp4
func (s *IbcCallbackBsnAddRewards) QueryFpRewards(n *chain.NodeConfig) (
	fp1bbn, fp2cons0, fp3cons0 sdk.Coins,
) {
	rwds := s.BaseBtcRewardsDistribution.QueryFpRewards(n, s.fp1bbnAddr, s.fp2cons0Addr, s.fp3cons0Addr)
	return rwds[s.fp1bbnAddr], rwds[s.fp2cons0Addr], rwds[s.fp3cons0Addr]
}

func (s *IbcCallbackBsnAddRewards) SuiteRewardsDiff(n *chain.NodeConfig, f func()) (
	bbnComm, del1, del2, fp1bbn, fp2cons0, fp3cons0 sdk.Coins,
) {
	bbnCommBefore, del1Before, del2Before, fp1bbnBefore, fp2cons0Before, fp3cons0Before := s.QuerySuiteRewards(n)

	f()
	n.WaitForNextBlock()

	bbnCommAfter, del1After, del2After, fp1bbnAfter, fp2cons0After, fp3cons0After := s.QuerySuiteRewards(n)

	bbnCommDiff := bbnCommAfter.Sub(bbnCommBefore...)
	del1Diff := del1After.Sub(del1Before...)
	del2Diff := del2After.Sub(del2Before...)
	fp1bbnDiff := fp1bbnAfter.Sub(fp1bbnBefore...)
	fp2cons0Diff := fp2cons0After.Sub(fp2cons0Before...)
	fp3cons0Diff := fp3cons0After.Sub(fp3cons0Before...)

	return bbnCommDiff, del1Diff, del2Diff, fp1bbnDiff, fp2cons0Diff, fp3cons0Diff
}

// QuerySuiteRewards returns the babylon commission account balance and fp, dels
// available rewards
func (s *IbcCallbackBsnAddRewards) QuerySuiteRewards(n *chain.NodeConfig) (
	bbnComm, del1, del2, fp1bbn, fp2cons0, fp3cons0 sdk.Coins,
) {
	bbnComm, err := n.QueryBalances(params.AccBbnComissionCollectorBsn.String())
	require.NoError(s.T(), err)

	fp1bbn, fp2cons0, fp3cons0 = s.QueryFpRewards(n)

	delRwd := s.QueryDelRewards(n, s.del1Addr, s.del2Addr)
	del1, del2 = delRwd[s.del1Addr], delRwd[s.del2Addr]

	return bbnComm, del1, del2, fp1bbn, fp2cons0, fp3cons0
}

func (s *IbcCallbackBsnAddRewards) BbnNode() *chain.NodeConfig {
	bbnChain0 := s.configurer.GetChainConfig(0)

	bbnNode, err := bbnChain0.GetNodeAtIndex(2)
	s.NoError(err)

	return bbnNode
}

func (s *IbcCallbackBsnAddRewards) BsnNode() *chain.NodeConfig {
	bsnChain1 := s.configurer.GetChainConfig(1)

	bsnNode, err := bsnChain1.GetNodeAtIndex(2)
	s.NoError(err)

	return bsnNode
}
