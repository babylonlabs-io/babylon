package e2e

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
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

func (s *IbcCallbackBsnAddRewards) TestAll() {
	s.CreateFinalityProviders()
	s.CreateBtcDelegations()
	s.CreateFactoryToken()
	s.SendBsnRewardsCallback()
	s.IbcSendBadBsnRewardsCallbackReturnFunds()
	s.SendBsnRewardsCallbackWithNativeToken()
}

// CreateFinalityProviders creates all finality providers
func (s *IbcCallbackBsnAddRewards) CreateFinalityProviders() {
	chainA := s.configurer.GetChainConfig(0)
	chainB := s.configurer.GetChainConfig(1)
	chainA.WaitUntilHeight(2)
	chainB.WaitUntilHeight(2)

	bsnNode := s.BsnNode()
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
	// Register consumer chain on BBN
	bbnNode.RegisterConsumerChain(bbnNode.WalletName, bsn0.ConsumerId, bsn0.ConsumerName, bsn0.ConsumerDescription, bsn0.BabylonRewardsCommission.String())
	// The other chain is BBN too, so need to register the consumer chain
	// to be able to open the zoneconcierge channel
	bsnNode.RegisterConsumerChain(bsnNode.WalletName, bsn0.ConsumerId, bsn0.ConsumerName, bsn0.ConsumerDescription, bsn0.BabylonRewardsCommission.String())

	s.bsn0 = bsn0

	bbnNode.WaitForNextBlock()
	bsnNode.WaitForNextBlock()

	consumers := bbnNode.QueryBTCStkConsumerConsumers()
	require.Len(s.T(), consumers, 1)
	s.T().Log("All Consumers created")
	require.Equal(s.T(), consumers[0].ConsumerId, clientState.ClientId)

	// Open zoneconcierge channel
	// Need to use same connection ID as the one used in the consumer registration
	connResp, err := bbnNode.QueryConnections()
	require.NoError(s.T(), err)
	require.Len(s.T(), connResp.Connections, 2)
	connID := connResp.Connections[0].Id
	err = s.configurer.OpenZoneConciergeChannel(chainA, chainB, connID)
	require.NoError(s.T(), err, "failed to create zoneconcierge channel between Babylon and BSN")
	bbnNode.WaitForNextBlock()
	s.T().Log("Opened zoneconcierge channel")

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

// CreateBtcDelegations creates 3 btc delegations
func (s *IbcCallbackBsnAddRewards) CreateBtcDelegations() {
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

func (s *IbcCallbackBsnAddRewards) CreateFactoryToken() {
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

func (s *IbcCallbackBsnAddRewards) SendBsnRewardsCallback() {
	bbnNode := s.BbnNode()
	bsnNode := s.BsnNode()

	transferAmt := s.r.Int63n(2_000000) + 1_000000
	transferInt := math.NewInt(transferAmt)
	rewardCoin := sdk.NewCoin(s.bsnCustomTokenDenom, transferInt)

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
		bsnSenderBalances, err := bsnNode.QueryBalances(s.bsnSenderAddr)
		s.Require().NoError(err)

		ibcTransferTxHash := bsnNode.SendIBCTransfer(s.bsnSenderAddr, s.bbnIbcCallbackReceiverAddr, callbackMemoString, rewardCoin)
		bsnNode.WaitForNextBlocks(5)

		// Query transaction to ensure it was successful
		ibcTxRes, ibcTx, err := bsnNode.QueryTxWithError(ibcTransferTxHash)
		s.Require().NoError(err)
		s.Require().Zero(ibcTxRes.Code, fmt.Sprintf("Transaction failed with code %d: %s", ibcTxRes.Code, ibcTxRes.RawLog))

		// check sender balances
		bsnSenderAfter, err := bsnNode.QueryBalances(s.bsnSenderAddr)
		s.Require().NoError(err)

		feesPlusRewards := ibcTx.GetFee().Add(rewardCoin)
		require.Equal(s.T(), bsnSenderBalances.Sub(feesPlusRewards...).String(), bsnSenderAfter.String(), "bsn sender balance check")
	})

	rewardDenomInBbn := getFirstIBCDenom(bbnCommDiff)
	rewardCoins := sdk.NewCoins(sdk.NewCoin(rewardDenomInBbn, rewardCoin.Amount))

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

// IbcSendBadBsnRewardsCallbackReturnFunds it send rewards using the memo field
// and `CallbackAddBsnRewards`, but it specifies an invalid BsnConsumerID which
// errors out in the processing of adding rewards and rejects the ICS20 packet
// returning the funds to the BSN sender.
// Note: The bsn sender of rewards will still pay the fees of the IBC transaction
// but will receive back the rewards sent through ICS20. The IBC tx will respond
// without error and code zero, but the IBC packet will be rejected with Acknowledgement_Error
func (s *IbcCallbackBsnAddRewards) IbcSendBadBsnRewardsCallbackReturnFunds() {
	bbnNode := s.BbnNode()
	bsnNode := s.BsnNode()

	transferAmt := s.r.Int63n(2_000000) + 1_000000
	transferInt := math.NewInt(transferAmt)
	rewardCoin := sdk.NewCoin(s.bsnCustomTokenDenom, transferInt)

	failingCallbackMemo := bstypes.CallbackMemo{
		Action: bstypes.CallbackActionAddBsnRewardsMemo,
		DestCallback: &bstypes.CallbackInfo{
			Address: datagen.GenRandomAccount().Address,
			AddBsnRewards: &bstypes.CallbackAddBsnRewards{
				BsnConsumerID: "x",
			},
		},
	}

	callbackMemoJSON, err := json.Marshal(failingCallbackMemo)
	s.Require().NoError(err)
	callbackMemoString := string(callbackMemoJSON)

	bbnCommDiff, del1Diff, del2Diff, fp1bbnDiff, fp2cons0Diff, fp3cons0Diff := s.SuiteRewardsDiff(bbnNode, func() {
		bsnSenderBalancesBefore, err := bsnNode.QueryBalances(s.bsnSenderAddr)
		s.Require().NoError(err)

		ibcTransferTxHash := bsnNode.SendIBCTransfer(s.bsnSenderAddr, s.bbnIbcCallbackReceiverAddr, callbackMemoString, rewardCoin)
		bsnNode.WaitForNextBlocks(15)

		ibcTxRes, ibcTx, err := bsnNode.QueryTxWithError(ibcTransferTxHash)
		s.Require().NoError(err)
		s.Require().Zero(ibcTxRes.Code, fmt.Sprintf("Transaction failed with code %d: %s", ibcTxRes.Code, ibcTxRes.RawLog))

		bsnSenderBalancesAfter, err := bsnNode.QueryBalances(s.bsnSenderAddr)
		s.Require().NoError(err)

		require.Equal(s.T(), bsnSenderBalancesBefore.Sub(ibcTx.GetFee()...).String(), bsnSenderBalancesAfter.String(), "bsn sender balance check")
	})

	require.True(s.T(), bbnCommDiff.IsZero(), "bbn commission should not be rewarded")
	require.True(s.T(), del1Diff.IsZero(), "del1 was not rewarded")
	require.True(s.T(), del2Diff.IsZero(), "del2 was not rewarded")
	require.True(s.T(), fp1bbnDiff.IsZero(), "fp1 was not rewarded")
	require.True(s.T(), fp2cons0Diff.IsZero(), "fp2 was not rewarded")
	require.True(s.T(), fp3cons0Diff.IsZero(), "fp3 was not rewarded")
}

func (s *IbcCallbackBsnAddRewards) SendBsnRewardsCallbackWithNativeToken() {
	bbnNode := s.BbnNode()
	bsnNode := s.BsnNode()

	transferAmt := s.r.Int63n(500) + 100000
	transferInt := math.NewInt(transferAmt)
	ibcTransferOfNative := sdk.NewCoin(nativeDenom, transferInt)

	bbnBalanceBeforeIbcTransfer, err := bbnNode.QueryBalances(bbnNode.PublicAddress)
	s.Require().NoError(err)

	bsnBalanceBeforeIbcTransfer, err := bsnNode.QueryBalances(s.bsnSenderAddr)
	s.Require().NoError(err)

	// Send ubbn native token transfer to the bsn sender of rewards
	txHashIbcNativeTransfer := bbnNode.SendIBCTransfer(bbnNode.PublicAddress, s.bsnSenderAddr, "transfer", ibcTransferOfNative)
	bbnNode.WaitForNextBlock()

	txResIbcNative, txRespIbcNativeTransfer := bbnNode.QueryTx(txHashIbcNativeTransfer)
	s.Require().Zero(txResIbcNative.Code, fmt.Sprintf("Transaction failed with code %d: %s", txResIbcNative.Code, txResIbcNative.RawLog))
	txFeesPaidIbcNativeTransfer := txRespIbcNativeTransfer.GetFee()

	expectedBbnBalance := bbnBalanceBeforeIbcTransfer.Sub(txFeesPaidIbcNativeTransfer.Add(ibcTransferOfNative)...).String()

	var ibcBabylonNativeTokenTransferInBsn sdk.Coin
	s.Require().Eventually(func() bool {
		bbnBalanceAfterIbcTransfer, err := bbnNode.QueryBalances(bbnNode.PublicAddress)
		s.Require().NoError(err)

		if !strings.EqualFold(expectedBbnBalance, bbnBalanceAfterIbcTransfer.String()) {
			s.T().Logf(
				"bbnBalanceAfterIbcTransfer: %s; \n bbnBalanceBeforeIbcTransfer: %s expectedBbnBalance: %s, txFees: %s, coinTransfer: %s",
				bbnBalanceAfterIbcTransfer.String(), bbnBalanceBeforeIbcTransfer.String(), expectedBbnBalance, txFeesPaidIbcNativeTransfer.String(), ibcTransferOfNative.String(),
			)
			return false
		}

		bsnBalanceAfterIbcTransfer, err := bsnNode.QueryBalances(s.bsnSenderAddr)
		s.Require().NoError(err)

		ibcDenomOfbabylonNativeTokenInBsn := getFirstIBCDenom(bsnBalanceAfterIbcTransfer)
		if len(ibcDenomOfbabylonNativeTokenInBsn) == 0 {
			return false
		}

		ibcBabylonNativeTokenTransferInBsn = sdk.NewCoin(ibcDenomOfbabylonNativeTokenInBsn, ibcTransferOfNative.Amount)
		expectedBsnBalance := bsnBalanceBeforeIbcTransfer.Add(ibcBabylonNativeTokenTransferInBsn).String()

		if !strings.EqualFold(expectedBsnBalance, bsnBalanceAfterIbcTransfer.String()) {
			s.T().Logf(
				"bsnBalanceAfterIbcTransfer: %s; expectedBsnBalance: %s, txFees: %s, coinTransfer: %s",
				bsnBalanceAfterIbcTransfer.String(), expectedBsnBalance, txFeesPaidIbcNativeTransfer.String(), ibcTransferOfNative.String(),
			)
			return false
		}

		return true
	}, 3*time.Minute, 1*time.Second, "Transfer was not successful")

	// send the native token `ubbn` that was bridged to the bsn sender as rewards and check if the
	// ibc callback middleware can correctly parse everything
	fp2Ratio, fp3Ratio := math.LegacyMustNewDecFromStr("0.7"), math.LegacyMustNewDecFromStr("0.3")

	// send the callback without specifying consumer ID to try to load from the ibc channel
	callbackMemo := bstypes.CallbackMemo{
		Action: bstypes.CallbackActionAddBsnRewardsMemo,
		DestCallback: &bstypes.CallbackInfo{
			Address: datagen.GenRandomAccount().Address,
			AddBsnRewards: &bstypes.CallbackAddBsnRewards{
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

	callbackMemoJSON, err := json.Marshal(callbackMemo)
	s.Require().NoError(err)
	callbackMemoString := string(callbackMemoJSON)

	bbnCommDiff, del1Diff, del2Diff, fp1bbnDiff, fp2cons0Diff, fp3cons0Diff := s.SuiteRewardsDiff(bbnNode, func() {
		bsnSenderBalances, err := bsnNode.QueryBalances(s.bsnSenderAddr)
		s.Require().NoError(err)

		ibcTransferTxHash := bsnNode.SendIBCTransfer(s.bsnSenderAddr, s.bbnIbcCallbackReceiverAddr, callbackMemoString, ibcBabylonNativeTokenTransferInBsn)
		bsnNode.WaitForNextBlocks(10)

		ibcTxRes, ibcTx, err := bsnNode.QueryTxWithError(ibcTransferTxHash)
		s.Require().NoError(err)
		s.Require().Zero(ibcTxRes.Code, fmt.Sprintf("Transaction failed with code %d: %s", ibcTxRes.Code, ibcTxRes.RawLog))

		bsnSenderAfter, err := bsnNode.QueryBalances(s.bsnSenderAddr)
		s.Require().NoError(err)

		feesPlusRewards := ibcTx.GetFee().Add(ibcBabylonNativeTokenTransferInBsn)
		require.Equal(s.T(), bsnSenderBalances.Sub(feesPlusRewards...).String(), bsnSenderAfter.String(), "bsn sender balance check")
	})

	rewardCoins := sdk.NewCoins(ibcTransferOfNative)

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
	n.LogActionF(
		"Suite rewards diff before: bbnComm %s, del1 %s, del2 %s, fp1 %s, fp2 %s, fp3 %s",
		bbnCommBefore.String(), del1Before.String(), del2Before.String(), fp1bbnBefore.String(), fp2cons0Before.String(), fp3cons0Before.String(),
	)

	f()
	n.WaitForNextBlock()

	bbnCommAfter, del1After, del2After, fp1bbnAfter, fp2cons0After, fp3cons0After := s.QuerySuiteRewards(n)
	n.LogActionF(
		"Suite rewards diff after: bbnComm %s, del1 %s, del2 %s, fp1 %s, fp2 %s, fp3 %s",
		bbnCommAfter.String(), del1After.String(), del2After.String(), fp1bbnAfter.String(), fp2cons0After.String(), fp3cons0After.String(),
	)

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
