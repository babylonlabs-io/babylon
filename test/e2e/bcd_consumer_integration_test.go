package e2e

import (
	"encoding/hex"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	sdkmath "cosmossdk.io/math"
	wasmparams "github.com/CosmWasm/wasmd/app/params"
	bcdapp "github.com/babylonlabs-io/babylon-sdk/demo/app"
	bcdparams "github.com/babylonlabs-io/babylon-sdk/demo/app/params"
	bbnparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/client/config"
	"github.com/babylonlabs-io/babylon/test/e2e/bcd_consumer_integration/clientcontroller/babylon"
	cwconfig "github.com/babylonlabs-io/babylon/test/e2e/bcd_consumer_integration/clientcontroller/config"
	"github.com/babylonlabs-io/babylon/test/e2e/bcd_consumer_integration/clientcontroller/cosmwasm"
	"github.com/babylonlabs-io/babylon/test/e2e/initialization"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
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

type BCDConsumerIntegrationTestSuite struct {
	suite.Suite

	babylonController  *babylon.BabylonController
	cosmwasmController *cosmwasm.CosmwasmConsumerController
}

func (s *BCDConsumerIntegrationTestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite...")

	err := s.initBabylonController()
	s.Require().NoError(err, "Failed to initialize BabylonController")

	err = s.initCosmwasmController()
	s.Require().NoError(err, "Failed to initialize CosmwasmConsumerController")
}

func (s *BCDConsumerIntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down e2e integration test suite...")

	// Get the current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		s.T().Errorf("Failed to get current working directory: %v", err)
		return
	}

	// Construct the path to the Makefile directory
	makefileDir := filepath.Join(currentDir, "../../contrib/images")

	// Run the stop-bcd-consumer-integration make target
	cmd := exec.Command("make", "-C", makefileDir, "stop-bcd-consumer-integration")
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.T().Errorf("Failed to run stop-bcd-consumer-integration: %v\nOutput: %s", err, output)
	} else {
		s.T().Log("Successfully stopped integration test")
	}
}

func (s *BCDConsumerIntegrationTestSuite) Test1ChainStartup() {
	var (
		babylonStatus  *coretypes.ResultStatus
		consumerStatus *coretypes.ResultStatus
		err            error
	)

	// Use Babylon controller
	s.Eventually(func() bool {
		babylonStatus, err = s.babylonController.QueryNodeStatus()
		return err == nil && babylonStatus != nil && babylonStatus.SyncInfo.LatestBlockHeight >= 1
	}, time.Minute, time.Second, "Failed to query Babylon node status", err)
	s.T().Logf("Babylon node status: %v", babylonStatus.SyncInfo.LatestBlockHeight)

	// Use Cosmwasm controller
	s.Eventually(func() bool {
		consumerStatus, err = s.cosmwasmController.GetCometNodeStatus()
		return err == nil && consumerStatus != nil && consumerStatus.SyncInfo.LatestBlockHeight >= 1
	}, time.Minute, time.Second, "Failed to query Consumer node status", err)
	s.T().Logf("Consumer node status: %v", consumerStatus.SyncInfo.LatestBlockHeight)

	// Wait till IBC connection/channel b/w babylon<->bcd is established
	s.waitForIBCConnection()
}

func (s *BCDConsumerIntegrationTestSuite) Test2AutoRegisterAndVerifyNewConsumer() {
	// TODO: getting some error in ibc client-state, hardcode consumer id for now
	consumerID := "07-tendermint-0" //  s.getIBCClientID()
	s.verifyConsumerRegistration(consumerID)
}

func (s *BCDConsumerIntegrationTestSuite) Test3CreateConsumerFinalityProvider() {
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

func (s *BCDConsumerIntegrationTestSuite) Test4RestakeDelegationToMultipleFPs() {
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

func (s *BCDConsumerIntegrationTestSuite) Test5ActivateDelegation() {
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
	s.True(activeDel.HasCovenantQuorums(1))

	// Query the staking contract for delegations on the consumer chain
	var dataFromContract *cosmwasm.ConsumerDelegationsResponse
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
	var fpsByPower *cosmwasm.ConsumerFpsByPowerResponse
	s.Eventually(func() bool {
		fpsByPower, err = s.cosmwasmController.QueryFinalityProvidersByPower()
		return err == nil && len(fpsByPower.Fps) > 0
	}, time.Second*20, time.Second)

	s.Require().NotNil(fpsByPower)
}

func (s *BCDConsumerIntegrationTestSuite) submitCovenantSigs(consumerFp *bsctypes.FinalityProviderResponse) {
	cvSK, _, _, err := getDeterministicCovenantKey()
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

	stakingMsgTx, err := bbntypes.NewBTCTxFromBytes(stakingTx)
	s.NoError(err)
	stakingTxHash := stakingMsgTx.TxHash().String()

	params, err := s.babylonController.QueryBTCStakingParams()
	s.NoError(err)

	fpBTCPKs, err := bbntypes.NewBTCPKsFromBIP340PKs(pendingDel.FpBtcPkList)
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
	unbondingTx, err := bbntypes.NewBTCTxFromBytes(pendingDel.BtcUndelegation.UnbondingTx)
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
	}

	// ensure the BTC delegation has covenant sigs now
	activeDelsSet, err := s.babylonController.QueryFinalityProviderDelegations(consumerFp.BtcPk.MarshalHex(), 1)
	s.NoError(err)
	s.Len(activeDelsSet, 1)

	activeDels, err := ParseRespsBTCDelToBTCDel(activeDelsSet[0])
	s.NoError(err)
	s.NotNil(activeDels)
	s.Len(activeDels.Dels, 1)
	activeDel := activeDels.Dels[0]
	s.True(activeDel.HasCovenantQuorums(1))

	// ensure BTC staking is activated
	s.Eventually(func() bool {
		activatedHeight, err := s.babylonController.QueryActivatedHeight()
		if err != nil {
			s.T().Logf("Error querying activated height: %v", err)
			return false
		}
		if activatedHeight == nil {
			s.T().Log("Activated height is nil")
			return false
		}
		return activatedHeight.Height > 0
	}, time.Minute, time.Second*15, "BTC staking was not activated within the expected time")
}

func (s *BCDConsumerIntegrationTestSuite) createBabylonDelegation(babylonFp *bstypes.FinalityProviderResponse, consumerFp *bsctypes.FinalityProviderResponse) (*btcec.PublicKey, string) {
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
	delBTCPKs := []bbntypes.BIP340PubKey{*bbntypes.NewBIP340PubKeyFromBTCPK(czDelBtcPk)}

	serializedUnbondingTx, err := bbntypes.SerializeBTCTx(testUnbondingInfo.UnbondingTx)
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

	return czDelBtcPk, stakingTxHash
}

// helper function: create a random Babylon finality provider and verify it
func (s *BCDConsumerIntegrationTestSuite) createVerifyBabylonFP() *bstypes.FinalityProviderResponse {

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
	s.Len(actualFps, 1)
	s.Equal(babylonFp.Description, actualFps[0].Description)
	s.Equal(babylonFp.Commission, actualFps[0].Commission)
	s.Equal(babylonFp.BtcPk, actualFps[0].BtcPk)
	s.Equal(babylonFp.Pop, actualFps[0].Pop)
	s.Equal(babylonFp.SlashedBabylonHeight, actualFps[0].SlashedBabylonHeight)
	s.Equal(babylonFp.SlashedBtcHeight, actualFps[0].SlashedBtcHeight)

	return actualFps[0]
}

func (s *BCDConsumerIntegrationTestSuite) createVerifyConsumerFP(consumerId string) *bstypes.FinalityProvider {
	/*
		create a random consumer finality provider on Babylon
	*/
	// NOTE: we use the node's secret key as Babylon secret key for the finality provider
	czFpBTCSK, _, _ := datagen.GenRandomBTCKeyPair(r)
	sdk.SetAddrCacheEnabled(false)
	bbnparams.SetAddressPrefixes()
	fpBabylonAddr, err := sdk.AccAddressFromBech32(s.babylonController.MustGetTxSigner())
	s.NoError(err)
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

func (s *BCDConsumerIntegrationTestSuite) initBabylonController() error {
	cfg := config.DefaultBabylonConfig()

	btcParams := &chaincfg.RegressionNetParams // or whichever network you're using

	logger, _ := zap.NewDevelopment()

	// Get the current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		s.T().Fatalf("Failed to get current working directory: %v", err)
	}

	// Construct the path to the Makefile directory
	cfg.KeyDirectory = filepath.Join(currentDir, "../../contrib/images/bcd/.testnets/node0/babylond")
	cfg.GasPrices = "0.02ubbn"
	cfg.GasAdjustment = 20

	sdk.SetAddrCacheEnabled(false)
	bbnparams.SetAddressPrefixes()
	controller, err := babylon.NewBabylonController(&cfg, btcParams, logger)
	if err != nil {
		return err
	}

	s.babylonController = controller
	return nil
}

func (s *BCDConsumerIntegrationTestSuite) initCosmwasmController() error {
	cfg := cwconfig.DefaultCosmwasmConfig()
	cfg.BtcStakingContractAddress = "bbnc1nc5tatafv6eyq7llkr2gv50ff9e22mnf70qgjlv737ktmt4eswrqgn0kq0"

	// Create a logger
	logger, _ := zap.NewDevelopment()

	sdk.SetAddrCacheEnabled(false)
	bcdparams.SetAddressPrefixes()
	tempApp := bcdapp.NewTmpApp()
	encodingCfg := wasmparams.EncodingConfig{
		InterfaceRegistry: tempApp.InterfaceRegistry(),
		Codec:             tempApp.AppCodec(),
		TxConfig:          tempApp.TxConfig(),
		Amino:             tempApp.LegacyAmino(),
	}

	wcc, err := cosmwasm.NewCosmwasmConsumerController(cfg, encodingCfg, logger)
	require.NoError(s.T(), err)

	s.cosmwasmController = wcc
	return nil
}

func (s *BCDConsumerIntegrationTestSuite) waitForIBCConnection() {
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
	//babylonChannelState, err := s.babylonController.QueryChannelClientState(babylonChannel.ChannelId, babylonChannel.PortId)
	//s.Require().NoError(err, "Failed to query Babylon channel client state")
	//
	//return babylonChannelState.IdentifiedClientState.ClientId
}

func (s *BCDConsumerIntegrationTestSuite) verifyConsumerRegistration(consumerID string) *bsctypes.ConsumerRegister {
	var consumerRegistryResp *bsctypes.QueryConsumersRegistryResponse

	s.Eventually(func() bool {
		var err error
		consumerRegistryResp, err = s.babylonController.QueryConsumerRegistry(consumerID)
		if err != nil {
			s.T().Logf("Error querying consumer registry: %v", err)
			return false
		}
		return consumerRegistryResp != nil && len(consumerRegistryResp.GetConsumersRegister()) == 1
	}, time.Minute, 5*time.Second, "Consumer was not registered within the expected time")

	registeredConsumer := consumerRegistryResp.GetConsumersRegister()[0]

	s.T().Logf("Consumer registered: ID=%s, Name=%s, Description=%s",
		registeredConsumer.ConsumerId,
		registeredConsumer.ConsumerName,
		registeredConsumer.ConsumerDescription)

	return registeredConsumer
}

// getDeterministicCovenantKey returns a single, constant private key and its corresponding public key.
// This function is for testing purposes only and should never be used in production environments.
func getDeterministicCovenantKey() (*btcec.PrivateKey, *btcec.PublicKey, string, error) {
	// This is a constant private key for testing purposes only
	const constantPrivateKeyHex = "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	privateKeyBytes, err := hex.DecodeString(constantPrivateKeyHex)
	if err != nil {
		return nil, nil, "", err
	}

	privateKey, publicKey := btcec.PrivKeyFromBytes(privateKeyBytes)

	// Convert to BIP340 public key
	bip340PubKey := bbntypes.NewBIP340PubKeyFromBTCPK(publicKey)

	// Get the hex representation of the BIP340 public key
	publicKeyHex := bip340PubKey.MarshalHex()

	return privateKey, publicKey, publicKeyHex, nil
}
