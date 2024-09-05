package e2e

import (
	"encoding/hex"
	"fmt"
	"math"
	"time"

	sdkmath "cosmossdk.io/math"
	wasmparams "github.com/CosmWasm/wasmd/app/params"
	bcdapp "github.com/babylonlabs-io/babylon-sdk/demo/app"
	bcdparams "github.com/babylonlabs-io/babylon-sdk/demo/app/params"
	bbnparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/client/config"
	"github.com/babylonlabs-io/babylon/test/e2e/clientcontroller/babylon"
	cwconfig "github.com/babylonlabs-io/babylon/test/e2e/clientcontroller/config"
	"github.com/babylonlabs-io/babylon/test/e2e/clientcontroller/cosmwasm"
	cwcc "github.com/babylonlabs-io/babylon/test/e2e/clientcontroller/cosmwasm"
	"github.com/babylonlabs-io/babylon/test/e2e/initialization"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	"github.com/babylonlabs-io/babylon/types"
	bbn "github.com/babylonlabs-io/babylon/types"
	bbntypes "github.com/babylonlabs-io/babylon/types"
	btcctypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	bsctypes "github.com/babylonlabs-io/babylon/x/btcstkconsumer/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

var MinCommissionRate = sdkmath.LegacyNewDecWithPrec(5, 2) // 5%

// DefaultSingleCovenantKey returns a single, constant private key and its corresponding public key
func DefaultSingleCovenantKey() (*btcec.PrivateKey, *btcec.PublicKey, string, error) {
	// This is a constant private key for testing purposes only
	const constantPrivateKeyHex = "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	privateKeyBytes, err := hex.DecodeString(constantPrivateKeyHex)
	if err != nil {
		return nil, nil, "", err
	}

	privateKey, publicKey := btcec.PrivKeyFromBytes(privateKeyBytes)

	// Convert to BIP340 public key
	bip340PubKey := types.NewBIP340PubKeyFromBTCPK(publicKey)

	// Get the hex representation of the BIP340 public key
	publicKeyHex := bip340PubKey.MarshalHex()

	return privateKey, publicKey, publicKeyHex, nil
}

type BTCStakingIntegration2TestSuite struct {
	suite.Suite

	babylonRPC1      string
	babylonRPC2      string
	consumerChainRPC string

	babylonController  *babylon.BabylonController
	cosmwasmController *cosmwasm.CosmwasmConsumerController
}

func (s *BTCStakingIntegration2TestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite...")

	// Set the RPC URLs for the Babylon nodes and consumer chain
	s.babylonRPC1 = "http://localhost:26657"
	s.babylonRPC2 = "http://localhost:26667"
	s.consumerChainRPC = "http://localhost:26677"

	err := s.initBabylonController()
	s.Require().NoError(err, "Failed to initialize BabylonController")

	err = s.initCosmwasmController()
	s.Require().NoError(err, "Failed to initialize CosmwasmConsumerController")

	//time.Sleep(1 * time.Minute)
}

func (s *BTCStakingIntegration2TestSuite) TearDownSuite() {
	s.T().Log("tearing down e2e integration test suite...")

	// Run the stop-integration-test make target
	// cmd := exec.Command("make", "-C", "../consumer", "stop-integration-test")
	// output, err := cmd.CombinedOutput()
	// if err != nil {
	// 	s.T().Logf("Failed to run stop-integration-test: %s", output)
	// }
}

func (s *BTCStakingIntegration2TestSuite) Test1ChainStartup() {
	//s.T().Skip()
	var (
		babylonStatus  *coretypes.ResultStatus
		consumerStatus *coretypes.ResultStatus
		err            error
	)

	// Use Babylon controller
	s.Eventually(func() bool {
		babylonStatus, err = s.babylonController.QueryNodeStatus()
		return err == nil && babylonStatus.SyncInfo.LatestBlockHeight >= 1
	}, time.Minute, time.Second, "Failed to query Babylon node status", err)
	s.T().Logf("Babylon node status: %v", babylonStatus.SyncInfo.LatestBlockHeight)

	// Use Cosmwasm controller
	s.Eventually(func() bool {
		consumerStatus, err = s.cosmwasmController.GetCometNodeStatus()
		return err == nil && consumerStatus.SyncInfo.LatestBlockHeight >= 1
	}, time.Minute, time.Second, "Failed to query Consumer node status", err)
	s.T().Logf("Consumer node status: %v", consumerStatus.SyncInfo.LatestBlockHeight)
	// Add your test assertions here
	// ...
}

func (s *BTCStakingIntegration2TestSuite) Test2AutoRegisterAndVerifyNewConsumer() {
	//s.T().Skip()
	// TODO: try to fix the error otherwise hardcode consumer id for now
	consumerID := "07-tendermint-0" //  s.getIBCClientID()
	s.verifyConsumerRegistration(consumerID)
}

func (s *BTCStakingIntegration2TestSuite) Test3CreateConsumerFinalityProvider() {
	//s.T().Skip()
	consumerID := "07-tendermint-0"

	// generate a random number of finality providers from 1 to 5
	numConsumerFPs := datagen.RandomInt(r, 5) + 1
	var consumerFps []*bstypes.FinalityProvider
	for i := 0; i < int(numConsumerFPs); i++ {
		consumerFp := s.createVerifyConsumerFP(consumerID)
		consumerFps = append(consumerFps, consumerFp)
	}

	dataFromContract, err := s.cosmwasmController.QueryFinalityProviders()
	s.Require().NoError(err)

	// create a map of expected finality providers for verification
	fpMap := make(map[string]*bstypes.FinalityProvider)
	for _, czFp := range consumerFps {
		fpMap[czFp.BtcPk.MarshalHex()] = czFp
	}

	// validate that all finality providers match with the consumer list
	for _, czFp := range dataFromContract.Fps {
		fpFromMap, ok := fpMap[czFp.BtcPkHex]
		s.True(ok)
		s.Equal(fpFromMap.BtcPk.MarshalHex(), czFp.BtcPkHex)
		s.Equal(fpFromMap.SlashedBabylonHeight, czFp.SlashedBabylonHeight)
		s.Equal(fpFromMap.SlashedBtcHeight, czFp.SlashedBtcHeight)
		s.Equal(fpFromMap.ConsumerId, czFp.ConsumerId)
	}
}

func (s *BTCStakingIntegration2TestSuite) Test4RestakeDelegationToMultipleFPs() {
	//s.T().Skip()
	consumerID := "07-tendermint-0"

	consumerFps, err := s.babylonController.QueryConsumerFinalityProviders(consumerID)
	s.Require().NoError(err)
	consumerFp := consumerFps[0]

	// register a babylon finality provider
	babylonFp := s.createVerifyBabylonFP()

	// create a delegation and restake to both Babylon and consumer finality providers
	// NOTE: this will create delegation in pending state as covenant sigs are not provided
	delBtcPk, stakingTxHash := s.createBabylonDelegation(babylonFp, consumerFp)

	// check delegation
	delegation, err := s.babylonController.QueryBTCDelegation(stakingTxHash)
	s.Require().NoError(err)
	s.NotNil(delegation)

	// check consumer finality provider delegation
	czPendingDelSet, err := s.babylonController.QueryFinalityProviderDelegations(consumerFp.BtcPk.MarshalHex(), 1)
	s.Require().NoError(err)
	s.Len(czPendingDelSet, 1)
	czPendingDels := czPendingDelSet[0]
	s.Len(czPendingDels.Dels, 1)
	s.Equal(delBtcPk.SerializeCompressed()[1:], czPendingDels.Dels[0].BtcPk.MustToBTCPK().SerializeCompressed()[1:])
	s.Len(czPendingDels.Dels[0].CovenantSigs, 0)

	// check Babylon finality provider delegation
	pendingDelSet, err := s.babylonController.QueryFinalityProviderDelegations(babylonFp.BtcPk.MarshalHex(), 1)
	s.Require().NoError(err)
	s.Len(pendingDelSet, 1)
	pendingDels := pendingDelSet[0]
	s.Len(pendingDels.Dels, 1)
	s.Equal(delBtcPk.SerializeCompressed()[1:], pendingDels.Dels[0].BtcPk.MustToBTCPK().SerializeCompressed()[1:])
	s.Len(pendingDels.Dels[0].CovenantSigs, 0)
}

func (s *BTCStakingIntegration2TestSuite) Test5ActivateDelegation() {
	cvSK, cvPK, cvPKHex, err := DefaultSingleCovenantKey()
	// 02bb50e2d89a4ed70663d080659fe0ad4b9bc3e06c17a227433966cb59ceee020d
	s.Require().NoError(err)
	fmt.Println("covenantPKHex", cvPKHex)
	fmt.Println("cvSK", hex.EncodeToString(cvSK.Serialize()))
	fmt.Println("cvPK", hex.EncodeToString(cvPK.SerializeCompressed()))
	consumerId := "07-tendermint-0"

	// Query consumer finality providers
	consumerFps, err := s.babylonController.QueryConsumerFinalityProviders(consumerId)
	s.Require().NoError(err)
	s.Require().NotEmpty(consumerFps)
	consumerFp := consumerFps[0]

	// Activate the delegation by submitting covenant sigs
	s.submitCovenantSigs(consumerFp)

	// ensure the BTC delegation has covenant sigs now
	activeDelsSet, err := s.babylonController.QueryFinalityProviderDelegations(consumerFp.BtcPk.MarshalHex(), 1)
	s.NoError(err)
	s.Len(activeDelsSet, 1)

	activeDels, err := ParseRespsBTCDelToBTCDel(activeDelsSet[0])
	s.NoError(err)
	s.NotNil(activeDels)
	s.Len(activeDels.Dels, 1)

	activeDel := activeDels.Dels[0]
	s.True(activeDel.HasCovenantQuorums(covenantQuorum))

	// Query the staking contract for delegations on the consumer chain
	var dataFromContract *cwcc.ConsumerDelegationsResponse
	s.Eventually(func() bool {
		dataFromContract, err = s.cosmwasmController.QueryDelegations()
		return err == nil && dataFromContract != nil && len(dataFromContract.Delegations) == 1
	}, time.Second*20, time.Second)

	// Assert delegation details
	s.Empty(dataFromContract.Delegations[0].UndelegationInfo.DelegatorUnbondingSig)
	s.Equal(activeDel.BtcPk.MarshalHex(), dataFromContract.Delegations[0].BtcPkHex)
	s.Len(dataFromContract.Delegations[0].FpBtcPkList, 2)
	s.Equal(activeDel.FpBtcPkList[0].MarshalHex(), dataFromContract.Delegations[0].FpBtcPkList[0])
	s.Equal(activeDel.FpBtcPkList[1].MarshalHex(), dataFromContract.Delegations[0].FpBtcPkList[1])
	s.Equal(activeDel.StartHeight, dataFromContract.Delegations[0].StartHeight)
	s.Equal(activeDel.EndHeight, dataFromContract.Delegations[0].EndHeight)
	s.Equal(activeDel.TotalSat, dataFromContract.Delegations[0].TotalSat)
	s.Equal(hex.EncodeToString(activeDel.StakingTx), hex.EncodeToString(dataFromContract.Delegations[0].StakingTx))
	s.Equal(activeDel.SlashingTx.ToHexStr(), hex.EncodeToString(dataFromContract.Delegations[0].SlashingTx))

	// Query and assert finality provider voting power
	var fpsByPower *cwcc.ConsumerFpsByPowerResponse
	s.Eventually(func() bool {
		fpsByPower, err = s.cosmwasmController.QueryFinalityProvidersByPower()
		return err == nil && len(fpsByPower.Fps) > 0
	}, time.Second*20, time.Second)

	s.Require().NotNil(fpsByPower)

	totalPower := uint64(0)
	for _, fp := range fpsByPower.Fps {
		totalPower += fp.Power
	}

	//// Get FP total power from Babylon node
	//babylonPower, err := s.getFpTotalPowerFromBabylonNode(consumerFp)
	//s.Require().NoError(err)
	//s.Equal(babylonPower, totalPower)
}

func (s *BTCStakingIntegration2TestSuite) submitCovenantSigs(consumerFp *bsctypes.FinalityProviderResponse) {
	cvSK, _, _, err := DefaultSingleCovenantKey()
	s.NoError(err)

	// check consumer finality provider delegation
	pendingDelsSet, err := s.babylonController.QueryFinalityProviderDelegations(consumerFp.BtcPk.MarshalHex(), 1)
	s.Require().NoError(err)
	s.Len(pendingDelsSet, 1)
	pendingDels := pendingDelsSet[0]
	s.Len(pendingDels.Dels, 1)
	pendingDelResp := pendingDels.Dels[0]
	pendingDel, err := ParseRespBTCDelToBTCDel(pendingDelResp)
	s.NoError(err)
	s.Len(pendingDel.CovenantSigs, 0)

	slashingTx := pendingDel.SlashingTx
	stakingTx := pendingDel.StakingTx

	stakingMsgTx, err := bbn.NewBTCTxFromBytes(stakingTx)
	s.NoError(err)
	stakingTxHash := stakingMsgTx.TxHash().String()

	params, err := s.babylonController.QueryBTCStakingParams()
	s.NoError(err)

	fpBTCPKs, err := bbn.NewBTCPKsFromBIP340PKs(pendingDel.FpBtcPkList)
	s.NoError(err)

	stakingInfo, err := pendingDel.GetStakingInfo(params, net)
	s.NoError(err)

	stakingSlashingPathInfo, err := stakingInfo.SlashingPathSpendInfo()
	s.NoError(err)

	/*
		generate and insert new covenant signature, in order to activate the BTC delegation
	*/
	// covenant signatures on slashing tx
	covenantSlashingSigs, err := datagen.GenCovenantAdaptorSigs(
		[]*btcec.PrivateKey{cvSK},
		fpBTCPKs,
		stakingMsgTx,
		stakingSlashingPathInfo.GetPkScriptPath(),
		slashingTx,
	)
	s.NoError(err)

	// cov Schnorr sigs on unbonding signature
	unbondingPathInfo, err := stakingInfo.UnbondingPathSpendInfo()
	s.NoError(err)
	unbondingTx, err := bbn.NewBTCTxFromBytes(pendingDel.BtcUndelegation.UnbondingTx)
	s.NoError(err)

	covUnbondingSigs, err := datagen.GenCovenantUnbondingSigs(
		[]*btcec.PrivateKey{cvSK},
		stakingMsgTx,
		pendingDel.StakingOutputIdx,
		unbondingPathInfo.GetPkScriptPath(),
		unbondingTx,
	)
	s.NoError(err)

	unbondingInfo, err := pendingDel.GetUnbondingInfo(params, net)
	s.NoError(err)
	unbondingSlashingPathInfo, err := unbondingInfo.SlashingPathSpendInfo()
	s.NoError(err)
	covenantUnbondingSlashingSigs, err := datagen.GenCovenantAdaptorSigs(
		[]*btcec.PrivateKey{cvSK},
		fpBTCPKs,
		unbondingTx,
		unbondingSlashingPathInfo.GetPkScriptPath(),
		pendingDel.BtcUndelegation.SlashingTx,
	)
	s.NoError(err)

	covPk, err := covenantSlashingSigs[0].CovPk.ToBTCPK()
	s.NoError(err)

	for i := 0; i < int(params.CovenantQuorum); i++ {
		tx, err := s.babylonController.SubmitCovenantSigs(
			covPk,
			stakingTxHash,
			covenantSlashingSigs[i].AdaptorSigs,
			covUnbondingSigs[i],
			covenantUnbondingSlashingSigs[i].AdaptorSigs,
		)
		s.Require().NoError(err)
		s.Require().NotNil(tx)
		// // wait for a block so that above txs take effect
		// nonValidatorNode.WaitForNextBlock()
	}

	// // wait for a block so that above txs take effect
	// nonValidatorNode.WaitForNextBlock()
	// nonValidatorNode.WaitForNextBlock()

	// ensure the BTC delegation has covenant sigs now
	activeDelsSet, err := s.babylonController.QueryFinalityProviderDelegations(consumerFp.BtcPk.MarshalHex(), 1)
	s.NoError(err)
	s.Len(activeDelsSet, 1)

	activeDels, err := ParseRespsBTCDelToBTCDel(activeDelsSet[0])
	s.NoError(err)
	s.NotNil(activeDels)
	s.Len(activeDels.Dels, 1)

	activeDel := activeDels.Dels[0]
	s.True(activeDel.HasCovenantQuorums(covenantQuorum))

	// wait for a block so that above txs take effect and the voting power table
	// is updated in the next block's BeginBlock
	// s.babylonController.WaitForNextBlock()

	// ensure BTC staking is activated
	activatedHeight, err := s.babylonController.QueryActivatedHeight()
	s.NoError(err)
	s.Positive(activatedHeight)
	// ensure finality provider has voting power at activated height
	// currentBtcTip, err := s.babylonController.QueryBtcLightClientTip()
	// s.NoError(err)
	// activeFps := nonValidatorNode.QueryActiveFinalityProvidersAtHeight(activatedHeight)
	// s.Len(activeFps, 1)
	// s.Equal(activeFps[0].VotingPower, activeDels.VotingPower(currentBtcTip.Height, initialization.BabylonBtcFinalizationPeriod, params.CovenantQuorum))
}

func (s *BTCStakingIntegration2TestSuite) getFpTotalPowerFromBabylonNode(fp *bsctypes.FinalityProviderResponse) (uint64, error) {
	// Implement logic to get FP total power from Babylon node
	// This might involve querying the Babylon node for the FP's power
	// You'll need to implement this based on your specific requirements
	return 0, nil
}

func (s *BTCStakingIntegration2TestSuite) createBabylonDelegation(babylonFp *bstypes.FinalityProviderResponse, consumerFp *bsctypes.FinalityProviderResponse) (*btcec.PublicKey, string) {
	/*
		create a random BTC delegation restaking to Babylon and consumer finality providers
	*/

	delBabylonAddr, err := sdk.AccAddressFromBech32(s.babylonController.MustGetTxSigner())
	s.NoError(err)
	// BTC staking params, BTC delegation key pairs and PoP
	params, err := s.babylonController.QueryStakingParams()
	s.Require().NoError(err)

	// minimal required unbonding time
	unbondingTime := uint16(initialization.BabylonBtcFinalizationPeriod) + 1

	// get covenant BTC PKs
	//covenantBTCPKs := []*btcec.PublicKey{}
	//for _, covenantPK := range params.CovenantPks {
	//	covenantBTCPKs = append(covenantBTCPKs, covenantPK.MustToBTCPK())
	//}
	// NOTE: we use the node's secret key as Babylon secret key for the BTC delegation
	pop, err := bstypes.NewPoPBTC(delBabylonAddr, czDelBtcSk)
	s.NoError(err)
	// generate staking tx and slashing tx
	stakingTimeBlocks := uint16(math.MaxUint16)
	testStakingInfo := datagen.GenBTCStakingSlashingInfo(
		r,
		s.T(),
		&chaincfg.RegressionNetParams,
		czDelBtcSk,
		[]*btcec.PublicKey{babylonFp.BtcPk.MustToBTCPK(), consumerFp.BtcPk.MustToBTCPK()},
		params.CovenantPks,
		params.CovenantQuorum,
		stakingTimeBlocks,
		stakingValue,
		params.SlashingAddress.String(),
		params.SlashingRate,
		unbondingTime,
	)

	stakingMsgTx := testStakingInfo.StakingTx
	stakingTxHash := stakingMsgTx.TxHash().String()
	stakingSlashingPathInfo, err := testStakingInfo.StakingInfo.SlashingPathSpendInfo()
	s.NoError(err)

	// generate proper delegator sig
	delegatorSig, err := testStakingInfo.SlashingTx.Sign(
		stakingMsgTx,
		datagen.StakingOutIdx,
		stakingSlashingPathInfo.GetPkScriptPath(),
		czDelBtcSk,
	)
	s.NoError(err)

	//// submit staking tx to Bitcoin and get inclusion proof
	//currentBtcTipResp, err := s.babylonController.QueryBtcLightClientTip()
	//s.NoError(err)
	//currentBtcTip, err := chain.ParseBTCHeaderInfoResponseToInfo(currentBtcTipResp)
	//s.NoError(err)
	//
	//blockWithStakingTx := datagen.CreateBlockWithTransaction(r, currentBtcTip.Header.ToBlockHeader(), stakingMsgTx)
	//s.babylonController.InsertBtcBlockHeaders(&blockWithStakingTx.HeaderBytes)
	//// make block k-deep
	//for i := 0; i < initialization.BabylonBtcConfirmationPeriod; i++ {
	//	nonValidatorNode.InsertNewEmptyBtcHeader(r)
	//}
	//stakingTxInfo := btcctypes.NewTransactionInfoFromSpvProof(blockWithStakingTx.SpvProof)

	// create and insert BTC headers which include the staking tx to get staking tx info
	btcTipHeaderResp, err := s.babylonController.QueryBtcLightClientTip()
	s.NoError(err)
	tipHeader, err := bbntypes.NewBTCHeaderBytesFromHex(btcTipHeaderResp.HeaderHex)
	s.NoError(err)
	blockWithStakingTx := datagen.CreateBlockWithTransaction(r, tipHeader.ToBlockHeader(), testStakingInfo.StakingTx)
	accumulatedWork := btclctypes.CalcWork(&blockWithStakingTx.HeaderBytes)
	accumulatedWork = btclctypes.CumulativeWork(accumulatedWork, btcTipHeaderResp.Work)
	parentBlockHeaderInfo := &btclctypes.BTCHeaderInfo{
		Header: &blockWithStakingTx.HeaderBytes,
		Hash:   blockWithStakingTx.HeaderBytes.Hash(),
		Height: btcTipHeaderResp.Height + 1,
		Work:   &accumulatedWork,
	}
	headers := make([]bbntypes.BTCHeaderBytes, 0)
	headers = append(headers, blockWithStakingTx.HeaderBytes)
	for i := 0; i < int(params.ComfirmationTimeBlocks); i++ {
		headerInfo := datagen.GenRandomValidBTCHeaderInfoWithParent(r, *parentBlockHeaderInfo)
		headers = append(headers, *headerInfo.Header)
		parentBlockHeaderInfo = headerInfo
	}
	_, err = s.babylonController.InsertBtcBlockHeaders(headers)
	s.NoError(err)
	btcHeader := blockWithStakingTx.HeaderBytes
	serializedStakingTx, err := bbntypes.SerializeBTCTx(testStakingInfo.StakingTx)
	s.NoError(err)
	stakingTxInfo := btcctypes.NewTransactionInfo(&btcctypes.TransactionKey{Index: 1, Hash: btcHeader.Hash()}, serializedStakingTx, blockWithStakingTx.SpvProof.MerkleNodes)

	// generate BTC undelegation stuff
	stkTxHash := testStakingInfo.StakingTx.TxHash()
	unbondingValue := stakingValue - datagen.UnbondingTxFee // TODO: parameterise fee
	testUnbondingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r,
		s.T(),
		&chaincfg.RegressionNetParams,
		czDelBtcSk,
		[]*btcec.PublicKey{babylonFp.BtcPk.MustToBTCPK(), consumerFp.BtcPk.MustToBTCPK()},
		params.CovenantPks,
		params.CovenantQuorum,
		wire.NewOutPoint(&stkTxHash, datagen.StakingOutIdx),
		stakingTimeBlocks,
		unbondingValue,
		params.SlashingAddress.String(),
		params.SlashingRate,
		unbondingTime,
	)
	delUnbondingSlashingSig, err := testUnbondingInfo.GenDelSlashingTxSig(czDelBtcSk)
	s.NoError(err)

	// submit the message for creating BTC delegation
	delBTCPKs := []bbn.BIP340PubKey{*bbn.NewBIP340PubKeyFromBTCPK(czDelBtcPk)}
	//s.babylonController.CreateBTCDelegation(
	//	delBTCPKs,
	//	pop,
	//	stakingTxInfo,
	//	[]*bbn.BIP340PubKey{babylonFp.BtcPk, consumerFp.BtcPk},
	//	stakingTimeBlocks,
	//	btcutil.Amount(stakingValue),
	//	testStakingInfo.SlashingTx,
	//	delegatorSig,
	//	testUnbondingInfo.UnbondingTx,
	//	testUnbondingInfo.SlashingTx,
	//	unbondingTime,
	//	btcutil.Amount(unbondingValue),
	//	delUnbondingSlashingSig,
	//	"val",
	//	false,
	//)

	serializedUnbondingTx, err := bbn.SerializeBTCTx(testUnbondingInfo.UnbondingTx)
	s.NoError(err)

	// submit the BTC delegation to Babylon
	_, err = s.babylonController.CreateBTCDelegation(
		&delBTCPKs[0],
		[]*btcec.PublicKey{babylonFp.BtcPk.MustToBTCPK(), consumerFp.BtcPk.MustToBTCPK()},
		pop,
		uint32(stakingTimeBlocks),
		stakingValue,
		stakingTxInfo,
		testStakingInfo.SlashingTx,
		delegatorSig,
		serializedUnbondingTx,
		uint32(unbondingTime),
		unbondingValue,
		testUnbondingInfo.SlashingTx,
		delUnbondingSlashingSig)
	s.NoError(err)

	// wait for a block so that above txs take effect
	//nonValidatorNode.WaitForNextBlock()
	//nonValidatorNode.WaitForNextBlock()

	return czDelBtcPk, stakingTxHash
}

// helper function: create a random Babylon finality provider and verify it
func (s *BTCStakingIntegration2TestSuite) createVerifyBabylonFP() *bstypes.FinalityProviderResponse {

	/*
		create a random finality provider on Babylon
	*/
	// NOTE: we use the node's secret key as Babylon secret key for the finality provider
	babylonFpBTCSK, _, _ := datagen.GenRandomBTCKeyPair(r)
	sdk.SetAddrCacheEnabled(false)
	bbnparams.SetAddressPrefixes()
	fpBabylonAddr, err := sdk.AccAddressFromBech32(s.babylonController.MustGetTxSigner())
	s.NoError(err)
	babylonFp, err := datagen.GenCustomFinalityProvider(r, babylonFpBTCSK, fpBabylonAddr, "")
	s.NoError(err)
	babylonFp.Commission = &MinCommissionRate
	bbnFpPop, err := babylonFp.Pop.Marshal()
	s.NoError(err)
	bbnDescription, err := babylonFp.Description.Marshal()
	s.NoError(err)

	_, err = s.babylonController.RegisterFinalityProvider(
		"",
		babylonFp.BtcPk,
		bbnFpPop,
		babylonFp.Commission,
		bbnDescription,
	)
	s.NoError(err)

	// query the existence of finality provider and assert equivalence
	actualFps, err := s.babylonController.QueryFinalityProviders()
	s.Require().NoError(err)
	//s.Len(actualFps, 1) //TODO: fix this back
	//s.Equal(babylonFp.Description, actualFps[0].Description)
	//s.Equal(babylonFp.Commission, actualFps[0].Commission)
	//s.Equal(babylonFp.BtcPk, actualFps[0].BtcPk)
	//s.Equal(babylonFp.Pop, actualFps[0].Pop)
	//s.Equal(babylonFp.SlashedBabylonHeight, actualFps[0].SlashedBabylonHeight)
	//s.Equal(babylonFp.SlashedBtcHeight, actualFps[0].SlashedBtcHeight)

	return actualFps[0]
}

func (s *BTCStakingIntegration2TestSuite) createVerifyConsumerFP(consumerId string) *bstypes.FinalityProvider {
	/*
		create a random consumer finality provider on Babylon
	*/
	// NOTE: we use the node's secret key as Babylon secret key for the finality provider
	czFpBTCSK, _, _ := datagen.GenRandomBTCKeyPair(r)
	sdk.SetAddrCacheEnabled(false)
	bbnparams.SetAddressPrefixes()
	sdkCfg := sdk.GetConfig()
	fmt.Printf("Current - Account Prefix: %s\n", sdkCfg.GetBech32AccountAddrPrefix())
	fpBabylonAddr, err := sdk.AccAddressFromBech32(s.babylonController.MustGetTxSigner())
	s.NoError(err)
	fmt.Println("fpbabylonaddr", s.babylonController.MustGetTxSigner())
	czFp, err := datagen.GenCustomFinalityProvider(r, czFpBTCSK, fpBabylonAddr, consumerId)
	s.NoError(err)
	czFp.Commission = &MinCommissionRate
	czFpPop, err := czFp.Pop.Marshal()
	s.NoError(err)
	czDescription, err := czFp.Description.Marshal()
	s.NoError(err)

	_, err = s.babylonController.RegisterFinalityProvider(
		consumerId,
		czFp.BtcPk,
		czFpPop,
		czFp.Commission,
		czDescription,
	)
	s.NoError(err)

	// query the existence of finality provider and assert equivalence
	actualFp, err := s.babylonController.QueryConsumerFinalityProvider(consumerId, czFp.BtcPk.MarshalHex())
	s.NoError(err)
	s.Equal(czFp.Description, actualFp.Description)
	s.Equal(czFp.Commission.String(), actualFp.Commission.String())
	s.Equal(czFp.BtcPk, actualFp.BtcPk)
	s.Equal(czFp.Pop, actualFp.Pop)
	s.Equal(czFp.SlashedBabylonHeight, actualFp.SlashedBabylonHeight)
	s.Equal(czFp.SlashedBtcHeight, actualFp.SlashedBtcHeight)
	s.Equal(consumerId, actualFp.ConsumerId)
	return czFp
}

func (s *BTCStakingIntegration2TestSuite) initBabylonController() error {
	cfg := config.DefaultBabylonConfig()

	btcParams := &chaincfg.RegressionNetParams // or whichever network you're using

	logger, _ := zap.NewDevelopment()
	cfg.KeyDirectory = "/Users/gusin/Github/labs/cursor-bcd-babylon/babylon-private/test/e2e/consumer/.testnets/node0/babylond"
	cfg.GasPrices = "0.02ubbn"
	cfg.GasAdjustment = 20

	sdkCfg := sdk.GetConfig()
	fmt.Printf("CURRENT - SDK Account Prefix babylon init: %s\n", sdkCfg.GetBech32AccountAddrPrefix())
	sdk.SetAddrCacheEnabled(false)
	bbnparams.SetAddressPrefixes()
	sdkCfg = sdk.GetConfig()
	fmt.Printf("AFTER - SDK Account Prefix babylon init: %s\n", sdkCfg.GetBech32AccountAddrPrefix())

	controller, err := babylon.NewBabylonController(&cfg, btcParams, logger)
	if err != nil {
		return err
	}

	s.babylonController = controller
	return nil
}

func (s *BTCStakingIntegration2TestSuite) initCosmwasmController() error {
	cfg := cwconfig.DefaultCosmwasmConfig()

	// TODO: should not hardcode
	cfg.BtcStakingContractAddress = "bbnc1nc5tatafv6eyq7llkr2gv50ff9e22mnf70qgjlv737ktmt4eswrqgn0kq0"
	// Override the RPC address with the one from your test suite
	//cfg.RPCAddr = s.consumerChainRPC

	// You might need to adjust other config values as needed for your test environment

	// Create a logger
	logger, _ := zap.NewDevelopment()

	// // You'll need to provide the correct encoding config
	// // This is typically available from your app's setup
	// encodingConfig := wasmparams.MakeEncodingConfig()

	sdkCfg := sdk.GetConfig()
	fmt.Printf("CURRENT - SDK Account Prefix BCD init: %s\n", sdkCfg.GetBech32AccountAddrPrefix())
	sdk.SetAddrCacheEnabled(false)
	bcdparams.SetAddressPrefixes()
	sdkCfg = sdk.GetConfig()
	fmt.Printf("AFTER - SDK Account Prefix BCD init: %s\n", sdkCfg.GetBech32AccountAddrPrefix())
	tempApp := bcdapp.NewTmpApp()
	//tempApp := wasmapp.NewWasmApp(sdklogs.NewNopLogger(), dbm.NewMemDB(), nil, false, simtestutil.NewAppOptionsWithFlagHome(s.T().TempDir()), []wasmkeeper.Option{})
	encodingCfg := wasmparams.EncodingConfig{
		InterfaceRegistry: tempApp.InterfaceRegistry(),
		Codec:             tempApp.AppCodec(),
		TxConfig:          tempApp.TxConfig(),
		Amino:             tempApp.LegacyAmino(),
	}

	interfaces := encodingCfg.InterfaceRegistry.ListAllInterfaces()
	s.T().Logf("Interfaces: %v", interfaces)

	// Log implementations of ClientState
	impls := encodingCfg.InterfaceRegistry.ListImplementations("ibc.core.client.v1.ClientState")
	s.T().Logf("ClientState implementations: %v", impls)

	// encodingCfg.InterfaceRegistry.RegisterImplementations()

	// // Ensure that IBC types are registered
	// clienttypes.RegisterInterfaces(encodingCfg.InterfaceRegistry)
	// channeltypes.RegisterInterfaces(encodingCfg.InterfaceRegistry)
	// connectiontypes.RegisterInterfaces(encodingCfg.InterfaceRegistry)

	wcc, err := cwcc.NewCosmwasmConsumerController(cfg, encodingCfg, logger)
	require.NoError(s.T(), err)

	s.cosmwasmController = wcc
	return nil
}

func (s *BTCStakingIntegration2TestSuite) getIBCClientID() string {
	var babylonChannel *channeltypes.IdentifiedChannel
	s.Eventually(func() bool {
		babylonChannelsResp, err := s.babylonController.IBCChannels()
		if err != nil {
			s.T().Logf("Error querying Babylon IBC channels: %v", err)
			return false
		}
		if len(babylonChannelsResp.Channels) != 1 {
			s.T().Logf("Expected 1 Babylon IBC channel, got %d", len(babylonChannelsResp.Channels))
			return false
		}
		babylonChannel = babylonChannelsResp.Channels[0]
		if babylonChannel.State != channeltypes.OPEN {
			s.T().Logf("Babylon channel state is not OPEN, got %s", babylonChannel.State)
			return false
		}
		s.Equal(channeltypes.ORDERED, babylonChannel.Ordering)
		s.Contains(babylonChannel.Counterparty.PortId, "wasm.")
		return true
	}, time.Minute*3, time.Second*10, "Failed to get expected Babylon IBC channel")

	var consumerChannel *channeltypes.IdentifiedChannel
	s.Eventually(func() bool {
		consumerChannelsResp, err := s.cosmwasmController.IBCChannels()
		if err != nil {
			s.T().Logf("Error querying Consumer IBC channels: %v", err)
			return false
		}
		if len(consumerChannelsResp.Channels) != 1 {
			return false
		}
		consumerChannel = consumerChannelsResp.Channels[0]
		if consumerChannel.State != channeltypes.OPEN {
			return false
		}
		s.Equal(channeltypes.ORDERED, consumerChannel.Ordering)
		s.Equal(babylonChannel.PortId, consumerChannel.Counterparty.PortId)
		return true
	}, time.Minute, time.Second*2, "Failed to get expected Consumer IBC channel")

	s.T().Logf("IBC channel is established successfully")

	// Query the channel client state
	babylonChannelState, err := s.babylonController.QueryChannelClientState(babylonChannel.ChannelId, babylonChannel.PortId)
	s.Require().NoError(err, "Failed to query Babylon channel client state")

	return babylonChannelState.IdentifiedClientState.ClientId
}

func (s *BTCStakingIntegration2TestSuite) verifyConsumerRegistration(consumerID string) *bsctypes.ConsumerRegister {
	var consumerRegistry []*bsctypes.ConsumerRegister

	s.Eventually(func() bool {
		var err error
		consumerRegistry, err = s.babylonController.QueryConsumerRegistry(consumerID)
		if err != nil {
			s.T().Logf("Error querying consumer registry: %v", err)
			return false
		}
		return len(consumerRegistry) == 1
	}, time.Minute, 5*time.Second, "Consumer was not registered within the expected time")

	s.Require().Len(consumerRegistry, 1)
	registeredConsumer := consumerRegistry[0]

	s.T().Logf("Consumer registered: ID=%s, Name=%s, Description=%s",
		registeredConsumer.ConsumerId,
		registeredConsumer.ConsumerName,
		registeredConsumer.ConsumerDescription)

	return registeredConsumer
}
