package e2e

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	sdkmath "cosmossdk.io/math"
	wasmparams "github.com/CosmWasm/wasmd/app/params"
	bcdapp "github.com/babylonlabs-io/babylon-sdk/demo/app"
	bcdparams "github.com/babylonlabs-io/babylon-sdk/demo/app/params"
	bbnparams "github.com/babylonlabs-io/babylon/app/params"
	txformat "github.com/babylonlabs-io/babylon/btctxformatter"
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
	ckpttypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
	ftypes "github.com/babylonlabs-io/babylon/x/finality/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

var (
	minCommissionRate                   = sdkmath.LegacyNewDecWithPrec(5, 2) // 5%
	babylonFpBTCSK, babylonFpBTCPK, _   = datagen.GenRandomBTCKeyPair(r)
	babylonFpBTCSK2, babylonFpBTCPK2, _ = datagen.GenRandomBTCKeyPair(r)
	randListInfo1                       *datagen.RandListInfo
	// TODO: get consumer id from ibc client-state query
	consumerID = "07-tendermint-0"

	czFpBTCSK                 *btcec.PrivateKey
	czFpBTCPK                 *btcec.PublicKey
	czDelBtcSk, czDelBtcPk, _ = datagen.GenRandomBTCKeyPair(r)
)

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

}

// Test2RegisterAndIntegrateConsumer registers a new consumer and
// 1. Verifies that an IBC connection is established between the consumer chain and Babylon
// 2. Checks that the consumer is registered in Babylon's consumer registry
// 3. Validates the consumer registration details in Babylon
// Then, it waits until the IBC channel between babylon<->bcd is established
func (s *BCDConsumerIntegrationTestSuite) Test2RegisterAndIntegrateConsumer() {
	// register and verify consumer
	s.registerVerifyConsumer()

	// after the consumer is registered, wait till IBC connection/channel
	// between babylon<->bcd is established
	s.waitForIBCConnection()
}

// Test3CreateConsumerFinalityProvider
// 1. Creates and registers a random number of consumer FPs in Babylon.
// 2. Babylon automatically sends IBC packets to the consumer chain to transmit this data.
// 3. Verifies that the registered consumer FPs in Babylon match the data stored in the consumer chain's contract.
func (s *BCDConsumerIntegrationTestSuite) Test3CreateConsumerFinalityProvider() {
	// generate a random number of finality providers from 1 to 5
	numConsumerFPs := datagen.RandomInt(r, 5) + 1
	var consumerFps []*bstypes.FinalityProvider
	for i := 0; i < int(numConsumerFPs); i++ {
		consumerFp, SK, PK := s.createVerifyConsumerFP()
		if i == 0 {
			czFpBTCSK = SK
			czFpBTCPK = PK
		}
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
		s.Equal(fpFromMap.SlashedBabylonHeight, czFp.SlashedHeight)
		s.Equal(fpFromMap.SlashedBtcHeight, czFp.SlashedBtcHeight)
		s.Equal(fpFromMap.ConsumerId, czFp.ConsumerId)
	}
}

// Test4RestakeDelegationToMultipleFPs
// 1. Creates a Babylon finality provider
// 2. Creates a pending state delegation restaking to both Babylon FP and 1 consumer FP
func (s *BCDConsumerIntegrationTestSuite) Test4RestakeDelegationToMultipleFPs() {
	consumerFp, err := s.babylonController.QueryConsumerFinalityProvider(consumerID, bbntypes.NewBIP340PubKeyFromBTCPK(czFpBTCPK).MarshalHex())
	s.Require().NoError(err)
	s.Require().NotNil(consumerFp)

	// register a babylon finality provider
	babylonFp := s.createVerifyBabylonFP(babylonFpBTCSK)
	// commit and finalize pub rand so Babylon FP has voting power
	randList := s.commitAndFinalizePubRand(babylonFpBTCSK, babylonFpBTCPK, uint64(1))
	randListInfo1 = randList

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

// Test5ActivateDelegation
// 1. Submits covenant signatures to activate a BTC delegation
// 2. Verifies the delegation is activated on Babylon
// 3. Checks that Babylon sends IBC packets to update the consumer chain
// 4. Verifies the delegation details in the consumer chain contract match Babylon
// 5. Confirms the consumer FP voting power equals the total stake amount
func (s *BCDConsumerIntegrationTestSuite) Test5ActivateDelegation() {
	// Query consumer finality provider
	consumerFp, err := s.babylonController.QueryConsumerFinalityProvider(consumerID, bbntypes.NewBIP340PubKeyFromBTCPK(czFpBTCPK).MarshalHex())
	s.Require().NoError(err)
	s.Require().NotNil(consumerFp)

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
	}, time.Second*30, time.Second)

	// Assert delegation details
	s.Empty(dataFromContract.Delegations[0].UndelegationInfo.DelegatorUnbondingInfo)
	s.Equal(activeDel.BtcPk.MarshalHex(), dataFromContract.Delegations[0].BtcPkHex)
	s.Len(dataFromContract.Delegations[0].FpBtcPkList, 2)
	s.Equal(activeDel.FpBtcPkList[0].MarshalHex(), dataFromContract.Delegations[0].FpBtcPkList[0])
	s.Equal(activeDel.FpBtcPkList[1].MarshalHex(), dataFromContract.Delegations[0].FpBtcPkList[1])
	s.Equal(activeDel.StartHeight, dataFromContract.Delegations[0].StartHeight)
	s.Equal(activeDel.EndHeight, dataFromContract.Delegations[0].EndHeight)
	s.Equal(activeDel.TotalSat, dataFromContract.Delegations[0].TotalSat)
	s.Equal(hex.EncodeToString(activeDel.StakingTx), hex.EncodeToString(dataFromContract.Delegations[0].StakingTx))
	s.Equal(activeDel.SlashingTx.ToHexStr(), hex.EncodeToString(dataFromContract.Delegations[0].SlashingTx))

	// Query and assert finality provider voting power is equal to the total stake
	s.Eventually(func() bool {
		fpInfo, err := s.cosmwasmController.QueryFinalityProviderInfo(consumerFp.BtcPk.MustToBTCPK())
		if err != nil {
			s.T().Logf("Error querying finality provider info: %v", err)
			return false
		}

		return fpInfo != nil && fpInfo.Power == activeDel.TotalSat && fpInfo.BtcPkHex == consumerFp.BtcPk.MarshalHex()
	}, time.Minute, time.Second*5)
}

func (s *BCDConsumerIntegrationTestSuite) Test6ConsumerFPRewardsGeneration() {
	// Query consumer finality providers
	consumerFp, err := s.babylonController.QueryConsumerFinalityProvider(consumerID, bbntypes.NewBIP340PubKeyFromBTCPK(czFpBTCPK).MarshalHex())
	s.Require().NoError(err)
	s.Require().NotNil(consumerFp)

	// Get the activated block height and block on the consumer chain
	czActivatedHeight, err := s.cosmwasmController.QueryActivatedHeight()
	s.NoError(err)
	czActivatedBlock, err := s.cosmwasmController.QueryIndexedBlock(czActivatedHeight)
	s.NoError(err)
	s.NotNil(czActivatedBlock)

	// Ensure the staking contract balance is initially empty
	rewards, err := s.cosmwasmController.QueryStakingContractBalances()
	s.NoError(err)
	s.Empty(rewards)

	// Commit public randomness at the activated block height on the consumer chain
	randListInfo, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(r, czFpBTCSK, uint64(czActivatedHeight), 100)
	s.NoError(err)

	// Submit the public randomness to the consumer chain
	txResp, err := s.cosmwasmController.CommitPubRandList(czFpBTCPK, uint64(czActivatedHeight), 100, randListInfo.Commitment, msgCommitPubRandList.Sig.MustToBTCSig())
	s.NoError(err)
	s.NotNil(txResp)

	// Consumer finality provider submits finality signature
	txResp, err = s.cosmwasmController.SubmitFinalitySig(
		czFpBTCSK,
		czFpBTCPK,
		randListInfo.SRList[0],
		&randListInfo.PRList[0],
		randListInfo.ProofList[0].ToProto(),
		czActivatedHeight,
	)
	s.NoError(err)
	s.NotNil(txResp)

	// Ensure consumer finality provider's finality signature is received and stored in the smart contract
	s.Eventually(func() bool {
		fpSigsResponse, err := s.cosmwasmController.QueryFinalitySignature(consumerFp.BtcPk.MarshalHex(), uint64(czActivatedHeight))
		if err != nil {
			s.T().Logf("failed to query finality signature: %s", err.Error())
			return false
		}
		if fpSigsResponse == nil || fpSigsResponse.Signature == nil || len(fpSigsResponse.Signature) == 0 {
			return false
		}
		return true
	}, time.Minute, time.Second*5)

	// Once the vote is cast, ensure the block is finalised
	finalizedBlock, err := s.cosmwasmController.QueryIndexedBlock(uint64(czActivatedHeight))
	s.NoError(err)
	s.NotEmpty(finalizedBlock)
	s.Equal(hex.EncodeToString(finalizedBlock.AppHash), hex.EncodeToString(czActivatedBlock.AppHash))
	s.True(finalizedBlock.Finalized)

	// Ensure consumer rewards are generated and sent to the staking contract
	s.Eventually(func() bool {
		rewards, err := s.cosmwasmController.QueryStakingContractBalances()
		if err != nil {
			s.T().Logf("failed to query rewards: %s", err.Error())
			return false
		}
		if len(rewards) == 0 {
			return false
		}
		fmt.Println("Consumer rewards: ", rewards)
		return true
	}, time.Minute, time.Second*5)
}

// Test7BabylonFPCascadedSlashing
// 1. Submits a Babylon FP valid finality sig to Babylon
// 2. Block is finalized.
// 3. Equivocates/ Submits a invalid finality sig to Babylon
// 4. Babylon FP is slashed
// 4. Babylon notifies involved consumer about the delegations.
// 5. Consumer discounts the voting power of other involved consumer FP's in the affected delegations
func (s *BCDConsumerIntegrationTestSuite) Test7BabylonFPCascadedSlashing() {
	// get the activated height
	activatedHeight, err := s.babylonController.QueryActivatedHeight()
	s.NoError(err)
	s.NotNil(activatedHeight)

	// get the block at the activated height
	activatedHeightBlock, err := s.babylonController.QueryCometBlock(activatedHeight.Height)
	s.NoError(err)
	s.NotNil(activatedHeightBlock)

	// get the babylon finality provider
	babylonFp, err := s.babylonController.QueryFinalityProviders()
	s.NoError(err)
	s.NotNil(babylonFp)

	babylonFpBIP340PK := bbntypes.NewBIP340PubKeyFromBTCPK(babylonFpBTCPK)
	randIdx := activatedHeight.Height - 1 // pub rand was committed from height 1-100

	// submit finality signature
	txResp, err := s.babylonController.SubmitFinalitySignature(
		babylonFpBTCSK,
		babylonFpBIP340PK,
		randListInfo1.SRList[randIdx],
		&randListInfo1.PRList[randIdx],
		randListInfo1.ProofList[randIdx].ToProto(),
		activatedHeight.Height)
	s.NoError(err)
	s.NotNil(txResp)

	// ensure vote is eventually cast
	var votes []bbntypes.BIP340PubKey
	s.Eventually(func() bool {
		votes, err = s.babylonController.QueryVotesAtHeight(activatedHeight.Height)
		if err != nil {
			s.T().Logf("Error querying votes: %v", err)
			return false
		}
		return len(votes) > 0
	}, time.Minute, time.Second*5)
	s.Equal(1, len(votes))
	s.Equal(votes[0].MarshalHex(), babylonFpBIP340PK.MarshalHex())

	// once the vote is cast, ensure block is finalised
	finalizedBlock, err := s.babylonController.QueryIndexedBlock(activatedHeight.Height)
	s.NoError(err)
	s.NotEmpty(finalizedBlock)
	s.Equal(strings.ToUpper(hex.EncodeToString(finalizedBlock.AppHash)), activatedHeightBlock.Block.AppHash.String())
	s.True(finalizedBlock.Finalized)

	// equivocate by submitting invalid finality signature
	txResp, err = s.babylonController.SubmitInvalidFinalitySignature(
		r,
		babylonFpBTCSK,
		babylonFpBIP340PK,
		randListInfo1.SRList[randIdx],
		&randListInfo1.PRList[randIdx],
		randListInfo1.ProofList[randIdx].ToProto(),
		activatedHeight.Height,
	)
	s.NoError(err)
	s.NotNil(txResp)

	// check the babylon finality provider is slashed
	babylonFpBIP340PKHex := bbntypes.NewBIP340PubKeyFromBTCPK(babylonFpBTCPK).MarshalHex()
	s.Eventually(func() bool {
		fp, err := s.babylonController.QueryFinalityProvider(babylonFpBIP340PKHex)
		if err != nil {
			s.T().Logf("Error querying finality provider: %v", err)
			return false
		}
		return fp != nil &&
			fp.FinalityProvider.SlashedBtcHeight > 0
	}, time.Minute, time.Second*5)

	// query consumer finality provider
	consumerFp, err := s.babylonController.QueryConsumerFinalityProvider(consumerID, bbntypes.NewBIP340PubKeyFromBTCPK(czFpBTCPK).MarshalHex())
	s.Require().NoError(err)
	s.Require().NotNil(consumerFp)

	// query and assert finality provider voting power is zero after slashing
	s.Eventually(func() bool {
		fpInfo, err := s.cosmwasmController.QueryFinalityProviderInfo(consumerFp.BtcPk.MustToBTCPK())
		if err != nil {
			s.T().Logf("Error querying finality providers by power: %v", err)
			return false
		}

		return fpInfo != nil && fpInfo.Power == 0 && fpInfo.BtcPkHex == consumerFp.BtcPk.MarshalHex()
	}, time.Minute, time.Second*5)
}

func (s *BCDConsumerIntegrationTestSuite) Test8ConsumerFPCascadedSlashing() {
	// create a new consumer finality provider
	resp, czFpBTCSK2, czFpBTCPK2 := s.createVerifyConsumerFP()
	consumerFp, err := s.babylonController.QueryConsumerFinalityProvider(consumerID, resp.BtcPk.MarshalHex())
	s.NoError(err)

	// register a babylon finality provider
	babylonFp := s.createVerifyBabylonFP(babylonFpBTCSK2)

	// create a new delegation and restake to both Babylon and consumer finality provider
	// NOTE: this will create delegation in pending state as covenant sigs are not provided
	_, stakingTxHash := s.createBabylonDelegation(babylonFp, consumerFp)

	// check delegation
	delegation, err := s.babylonController.QueryBTCDelegation(stakingTxHash)
	s.Require().NoError(err)
	s.NotNil(delegation)

	// activate the delegation by submitting covenant sigs
	s.submitCovenantSigs(consumerFp)

	// query the staking contract for delegations on the consumer chain
	var dataFromContract *cosmwasm.ConsumerDelegationsResponse
	s.Eventually(func() bool {
		dataFromContract, err = s.cosmwasmController.QueryDelegations()
		return err == nil && dataFromContract != nil && len(dataFromContract.Delegations) == 2
	}, time.Second*20, time.Second)

	// query and assert consumer finality provider's voting power is equal to the total stake
	s.Eventually(func() bool {
		fpInfo, err := s.cosmwasmController.QueryFinalityProviderInfo(consumerFp.BtcPk.MustToBTCPK())
		if err != nil {
			s.T().Logf("Error querying finality provider info: %v", err)
			return false
		}

		return fpInfo != nil && fpInfo.Power == delegation.TotalSat && fpInfo.BtcPkHex == consumerFp.BtcPk.MarshalHex()
	}, time.Minute, time.Second*5)

	// get the latest block height and block on the consumer chain
	czNodeStatus, err := s.cosmwasmController.GetCometNodeStatus()
	s.NoError(err)
	s.NotNil(czNodeStatus)
	czlatestBlockHeight := czNodeStatus.SyncInfo.LatestBlockHeight
	czLatestBlock, err := s.cosmwasmController.QueryIndexedBlock(uint64(czlatestBlockHeight))
	s.NoError(err)
	s.NotNil(czLatestBlock)

	// commit public randomness at the latest block height on the consumer chain
	randListInfo, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(r, czFpBTCSK2, uint64(czlatestBlockHeight), 100)
	s.NoError(err)

	// submit the public randomness to the consumer chain
	txResp, err := s.cosmwasmController.CommitPubRandList(czFpBTCPK2, uint64(czlatestBlockHeight), 100, randListInfo.Commitment, msgCommitPubRandList.Sig.MustToBTCSig())
	s.NoError(err)
	s.NotNil(txResp)

	// consumer finality provider submits finality signature
	txResp, err = s.cosmwasmController.SubmitFinalitySig(
		czFpBTCSK2,
		czFpBTCPK2,
		randListInfo.SRList[0],
		&randListInfo.PRList[0],
		randListInfo.ProofList[0].ToProto(),
		uint64(czlatestBlockHeight),
	)
	s.NoError(err)
	s.NotNil(txResp)

	// ensure consumer finality provider's finality signature is received and stored in the smart contract
	s.Eventually(func() bool {
		fpSigsResponse, err := s.cosmwasmController.QueryFinalitySignature(consumerFp.BtcPk.MarshalHex(), uint64(czlatestBlockHeight))
		if err != nil {
			s.T().Logf("failed to query finality signature: %s", err.Error())
			return false
		}
		if fpSigsResponse == nil || fpSigsResponse.Signature == nil || len(fpSigsResponse.Signature) == 0 {
			return false
		}
		return true
	}, time.Minute, time.Second*5)

	// consumer finality provider submits invalid finality signature
	txResp, err = s.cosmwasmController.SubmitInvalidFinalitySig(
		r,
		czFpBTCSK2,
		czFpBTCPK2,
		randListInfo.SRList[0],
		&randListInfo.PRList[0],
		randListInfo.ProofList[0].ToProto(),
		czlatestBlockHeight,
	)
	s.NoError(err)
	s.NotNil(txResp)

	// ensure consumer finality provider is slashed
	s.Eventually(func() bool {
		fp, err := s.cosmwasmController.QueryFinalityProvider(consumerFp.BtcPk.MarshalHex())
		return err == nil && fp != nil && fp.SlashedHeight > 0
	}, time.Minute, time.Second*5)

	// query and assert consumer finality provider's voting power is zero after slashing
	s.Eventually(func() bool {
		fpInfo, err := s.cosmwasmController.QueryFinalityProviderInfo(consumerFp.BtcPk.MustToBTCPK())
		if err != nil {
			s.T().Logf("Error querying finality providers by power: %v", err)
			return false
		}

		return fpInfo != nil && fpInfo.Power == 0 && fpInfo.BtcPkHex == consumerFp.BtcPk.MarshalHex()
	}, time.Minute, time.Second*5)

	// check the babylon finality provider's voting power is discounted (cascaded slashing)
	babylonFpBIP340PKHex := bbntypes.NewBIP340PubKeyFromBTCPK(babylonFpBTCPK2).MarshalHex()
	s.Eventually(func() bool {
		fp, err := s.babylonController.QueryFinalityProvider(babylonFpBIP340PKHex)
		if err != nil {
			s.T().Logf("Error querying finality provider: %v", err)
			return false
		}
		return fp != nil &&
			fp.FinalityProvider.SlashedBtcHeight == 0 // should not be slashed
	}, time.Minute, time.Second*5)

	// check consumer FP record in Babylon is updated
	consumerFpBIP340PKHex := consumerFp.BtcPk.MarshalHex()
	s.Eventually(func() bool {
		fp, err := s.babylonController.QueryFinalityProvider(consumerFpBIP340PKHex)
		if err != nil {
			s.T().Logf("Error querying finality provider: %v", err)
			return false
		}
		return fp != nil &&
			fp.FinalityProvider.SlashedBtcHeight > 0 // should be recorded slashed
	}, time.Minute, time.Second*5)
}

// helper function: submitCovenantSigs submits the covenant signatures to activate the BTC delegation
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

	// ensure the BTC delegation has covenant sigs and is active now
	s.Eventually(func() bool {
		activeDelsSet, err := s.babylonController.QueryFinalityProviderDelegations(consumerFp.BtcPk.MarshalHex(), 1)
		s.NoError(err)
		if len(activeDelsSet) != 1 {
			return false
		}
		if len(activeDelsSet[0].Dels) != 1 {
			return false
		}
		if !activeDelsSet[0].Dels[0].Active {
			return false
		}

		activeDels, err := ParseRespsBTCDelToBTCDel(activeDelsSet[0])
		s.NoError(err)
		s.NotNil(activeDels)
		if len(activeDels.Dels) != 1 {
			return false
		}
		if !activeDels.Dels[0].HasCovenantQuorums(1) {
			return false
		}
		return true
	}, time.Minute, time.Second*5, "BTC staking was not activated within the expected time")

	// ensure BTC staking is activated
	s.Eventually(func() bool {
		activatedHeight, err := s.babylonController.QueryActivatedHeight()
		if err != nil {
			s.T().Logf("Error querying activated height: %v", err)
			return false
		}
		return activatedHeight != nil && activatedHeight.Height > 0
	}, time.Minute, time.Second*5)
}

// helper function: createBabylonDelegation creates a random BTC delegation restaking to Babylon and consumer finality providers
func (s *BCDConsumerIntegrationTestSuite) createBabylonDelegation(babylonFp *bstypes.FinalityProviderResponse, consumerFp *bsctypes.FinalityProviderResponse) (*btcec.PublicKey, string) {
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
	stakingTimeBlocks := uint16(10000)
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
		params.SlashingPkScript,
		params.SlashingRate,
		unbondingTime,
	)

	stakingMsgTx := testStakingInfo.StakingTx
	stakingTxBytes, err := bbntypes.SerializeBTCTx(stakingMsgTx)
	s.NoError(err)
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
	inclusionProof := bstypes.NewInclusionProofFromSpvProof(blockWithStakingTx.SpvProof)

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
		params.SlashingPkScript,
		params.SlashingRate,
		unbondingTime,
	)
	delUnbondingSlashingSig, err := testUnbondingInfo.GenDelSlashingTxSig(czDelBtcSk)
	s.NoError(err)

	// submit the message for creating BTC delegation
	delBTCPK := *bbntypes.NewBIP340PubKeyFromBTCPK(czDelBtcPk)

	serializedUnbondingTx, err := bbntypes.SerializeBTCTx(testUnbondingInfo.UnbondingTx)
	s.NoError(err)

	// submit the BTC delegation to Babylon
	_, err = s.babylonController.CreateBTCDelegation(
		&delBTCPK,
		[]*btcec.PublicKey{babylonFp.BtcPk.MustToBTCPK(), consumerFp.BtcPk.MustToBTCPK()},
		pop,
		uint32(stakingTimeBlocks),
		stakingValue,
		stakingTxBytes,
		inclusionProof,
		testStakingInfo.SlashingTx,
		delegatorSig,
		serializedUnbondingTx,
		uint32(unbondingTime),
		unbondingValue,
		testUnbondingInfo.SlashingTx,
		delUnbondingSlashingSig,
	)
	s.NoError(err)

	return czDelBtcPk, stakingTxHash
}

// helper function: createVerifyBabylonFP creates a random Babylon finality provider and verifies it
func (s *BCDConsumerIntegrationTestSuite) createVerifyBabylonFP(babylonFpBTCSK *btcec.PrivateKey) *bstypes.FinalityProviderResponse {
	// NOTE: we use the node's secret key as Babylon secret key for the finality provider
	// babylonFpBTCSK, _, _ := datagen.GenRandomBTCKeyPair(r)
	sdk.SetAddrCacheEnabled(false)
	bbnparams.SetAddressPrefixes()
	fpBabylonAddr, err := sdk.AccAddressFromBech32(s.babylonController.MustGetTxSigner())
	s.NoError(err)
	babylonFp, err := datagen.GenCustomFinalityProvider(r, babylonFpBTCSK, fpBabylonAddr, "")
	s.NoError(err)
	babylonFp.Commission = &minCommissionRate
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

	actualFp, err := s.babylonController.QueryFinalityProvider(babylonFp.BtcPk.MarshalHex())
	s.NoError(err)
	s.Equal(babylonFp.Description, actualFp.FinalityProvider.Description)
	s.Equal(babylonFp.Commission, actualFp.FinalityProvider.Commission)
	s.Equal(babylonFp.BtcPk, actualFp.FinalityProvider.BtcPk)
	s.Equal(babylonFp.Pop, actualFp.FinalityProvider.Pop)
	s.Equal(babylonFp.SlashedBabylonHeight, actualFp.FinalityProvider.SlashedBabylonHeight)
	s.Equal(babylonFp.SlashedBtcHeight, actualFp.FinalityProvider.SlashedBtcHeight)
	return actualFp.FinalityProvider
}

// helper function: commitAndFinalizePubRand commits public randomness at the given start height and finalizes it
func (s *BCDConsumerIntegrationTestSuite) commitAndFinalizePubRand(babylonFpBTCSK *btcec.PrivateKey, babylonFpBTCPK *btcec.PublicKey, commitStartHeight uint64) *datagen.RandListInfo {
	// commit public randomness list
	numPubRand := uint64(100)
	randList, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(r, babylonFpBTCSK, commitStartHeight, numPubRand)
	s.NoError(err)

	_, err = s.babylonController.CommitPublicRandomness(msgCommitPubRandList)
	s.NoError(err)

	pubRandCommitMap, err := s.babylonController.QueryLastCommittedPublicRand(babylonFpBTCPK, commitStartHeight)
	s.NoError(err)
	s.Len(pubRandCommitMap, 1)

	var firstPubRandCommit *ftypes.PubRandCommitResponse
	for _, commit := range pubRandCommitMap {
		firstPubRandCommit = commit
		break
	}

	commitEpoch := firstPubRandCommit.EpochNum
	// finalise until the epoch of the first public randomness commit
	s.finalizeUntilEpoch(commitEpoch)
	return randList
}

// helper function: createVerifyConsumerFP creates a random consumer finality provider on Babylon
// and verifies its existence.
func (s *BCDConsumerIntegrationTestSuite) createVerifyConsumerFP() (*bstypes.FinalityProvider, *btcec.PrivateKey, *btcec.PublicKey) {
	/*
		create a random consumer finality provider on Babylon
	*/
	// NOTE: we use the node's secret key as Babylon secret key for the finality provider
	czFpBTCSecretKey, czFpBTCPublicKey, _ := datagen.GenRandomBTCKeyPair(r)
	sdk.SetAddrCacheEnabled(false)
	bbnparams.SetAddressPrefixes()
	fpBabylonAddr, err := sdk.AccAddressFromBech32(s.babylonController.MustGetTxSigner())
	s.NoError(err)
	czFp, err := datagen.GenCustomFinalityProvider(r, czFpBTCSecretKey, fpBabylonAddr, consumerID)
	s.NoError(err)
	czFp.Commission = &minCommissionRate
	czFpPop, err := czFp.Pop.Marshal()
	s.NoError(err)
	czDescription, err := czFp.Description.Marshal()
	s.NoError(err)

	_, err = s.babylonController.RegisterFinalityProvider(
		consumerID,
		czFp.BtcPk,
		czFpPop,
		czFp.Commission,
		czDescription,
	)
	s.NoError(err)

	// query the existence of finality provider and assert equivalence
	actualFp, err := s.babylonController.QueryConsumerFinalityProvider(consumerID, czFp.BtcPk.MarshalHex())
	s.NoError(err)
	s.Equal(czFp.Description, actualFp.Description)
	s.Equal(czFp.Commission.String(), actualFp.Commission.String())
	s.Equal(czFp.BtcPk, actualFp.BtcPk)
	s.Equal(czFp.Pop, actualFp.Pop)
	s.Equal(czFp.SlashedBabylonHeight, actualFp.SlashedBabylonHeight)
	s.Equal(czFp.SlashedBtcHeight, actualFp.SlashedBtcHeight)
	s.Equal(consumerID, actualFp.ConsumerId)
	return czFp, czFpBTCSecretKey, czFpBTCPublicKey
}

// helper function: initBabylonController initializes the Babylon controller with the default configuration.
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
	cfg.KeyDirectory = filepath.Join(currentDir, "../../contrib/images/ibcsim-bcd/.testnets/node0/babylond")
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

// helper function: initCosmwasmController initializes the Cosmwasm controller with the default configuration.
func (s *BCDConsumerIntegrationTestSuite) initCosmwasmController() error {
	cfg := cwconfig.DefaultCosmwasmConfig()

	// Get the current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		s.T().Fatalf("Failed to get current working directory: %v", err)
	}

	cfg.BtcStakingContractAddress = "bbnc1nc5tatafv6eyq7llkr2gv50ff9e22mnf70qgjlv737ktmt4eswrqgn0kq0"
	cfg.BtcFinalityContractAddress = "bbnc17p9rzwnnfxcjp32un9ug7yhhzgtkhvl9jfksztgw5uh69wac2pgssg3nft"
	cfg.ChainID = "bcd-test"
	cfg.KeyDirectory = filepath.Join(currentDir, "../../contrib/images/ibcsim-bcd/.testnets/bcd/bcd-test")
	cfg.AccountPrefix = "bbnc"

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

// helper function: waitForIBCConnection waits for the IBC connection to be established between Babylon and the consumer.
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

// helper function: verifyConsumerRegistration verifies the automatic registration of a consumer
// and returns the consumer details.
func (s *BCDConsumerIntegrationTestSuite) registerVerifyConsumer() *bsctypes.ConsumerRegister {
	var registeredConsumer *bsctypes.ConsumerRegister
	var err error

	// wait until the consumer is registered
	s.Eventually(func() bool {
		// Register a random consumer on Babylon
		registeredConsumer = bsctypes.NewCosmosConsumerRegister(
			consumerID,
			datagen.GenRandomHexStr(r, 5),
			"Chain description: "+datagen.GenRandomHexStr(r, 15),
		)
		_, err = s.babylonController.RegisterConsumerChain(registeredConsumer.ConsumerId, registeredConsumer.ConsumerName, registeredConsumer.ConsumerDescription)
		if err != nil {
			return false
		}

		consumerRegistryResp, err := s.babylonController.QueryConsumerRegistry(consumerID)
		if err != nil {
			return false
		}
		s.Require().NotNil(consumerRegistryResp)
		s.Require().Len(consumerRegistryResp.GetConsumersRegister(), 1)
		s.Require().Equal(registeredConsumer.ConsumerId, consumerRegistryResp.GetConsumersRegister()[0].ConsumerId)
		s.Require().Equal(registeredConsumer.ConsumerName, consumerRegistryResp.GetConsumersRegister()[0].ConsumerName)
		s.Require().Equal(registeredConsumer.ConsumerDescription, consumerRegistryResp.GetConsumersRegister()[0].ConsumerDescription)

		return true
	}, 2*time.Minute, 5*time.Second, "Consumer was not registered within the expected time")

	s.T().Logf("Consumer registered: ID=%s, Name=%s, Description=%s",
		registeredConsumer.ConsumerId,
		registeredConsumer.ConsumerName,
		registeredConsumer.ConsumerDescription)

	return registeredConsumer
}

func (s *BCDConsumerIntegrationTestSuite) finalizeUntilEpoch(epoch uint64) {
	bbnClient := s.babylonController.GetBBNClient()

	// wait until the checkpoint of this epoch is sealed
	s.Eventually(func() bool {
		lastSealedCkpt, err := bbnClient.LatestEpochFromStatus(ckpttypes.Sealed)
		if err != nil {
			return false
		}
		return epoch <= lastSealedCkpt.RawCheckpoint.EpochNum
	}, 1*time.Minute, 1*time.Second)

	s.T().Logf("start finalizing epochs till %d", epoch)
	// Random source for the generation of BTC data
	r := rand.New(rand.NewSource(time.Now().Unix()))

	// get all checkpoints of these epochs
	pagination := &sdkquerytypes.PageRequest{
		Key:   ckpttypes.CkptsObjectKey(0),
		Limit: epoch,
	}
	resp, err := bbnClient.RawCheckpoints(pagination)
	s.NoError(err)
	s.Equal(int(epoch), len(resp.RawCheckpoints))

	submitter := s.babylonController.GetKeyAddress()

	for _, checkpoint := range resp.RawCheckpoints {
		currentBtcTipResp, err := s.babylonController.QueryBtcLightClientTip()
		s.NoError(err)
		tipHeader, err := bbntypes.NewBTCHeaderBytesFromHex(currentBtcTipResp.HeaderHex)
		s.NoError(err)

		rawCheckpoint, err := checkpoint.Ckpt.ToRawCheckpoint()
		s.NoError(err)

		btcCheckpoint, err := ckpttypes.FromRawCkptToBTCCkpt(rawCheckpoint, submitter)
		s.NoError(err)

		babylonTagBytes, err := hex.DecodeString("01020304")
		s.NoError(err)

		p1, p2, err := txformat.EncodeCheckpointData(
			babylonTagBytes,
			txformat.CurrentVersion,
			btcCheckpoint,
		)
		s.NoError(err)

		tx1 := datagen.CreatOpReturnTransaction(r, p1)

		opReturn1 := datagen.CreateBlockWithTransaction(r, tipHeader.ToBlockHeader(), tx1)
		tx2 := datagen.CreatOpReturnTransaction(r, p2)
		opReturn2 := datagen.CreateBlockWithTransaction(r, opReturn1.HeaderBytes.ToBlockHeader(), tx2)

		// insert headers and proofs
		_, err = s.babylonController.InsertBtcBlockHeaders([]bbntypes.BTCHeaderBytes{
			opReturn1.HeaderBytes,
			opReturn2.HeaderBytes,
		})
		s.NoError(err)

		_, err = s.babylonController.InsertSpvProofs(submitter.String(), []*btcctypes.BTCSpvProof{
			opReturn1.SpvProof,
			opReturn2.SpvProof,
		})
		s.NoError(err)

		// wait until this checkpoint is submitted
		s.Eventually(func() bool {
			ckpt, err := bbnClient.RawCheckpoint(checkpoint.Ckpt.EpochNum)
			if err != nil {
				return false
			}
			return ckpt.RawCheckpoint.Status == ckpttypes.Submitted
		}, 1*time.Minute, 1*time.Second)
	}

	// insert w BTC headers
	err = s.babylonController.InsertWBTCHeaders(r)
	s.NoError(err)

	// wait until the checkpoint of this epoch is finalised
	s.Eventually(func() bool {
		lastFinalizedCkpt, err := bbnClient.LatestEpochFromStatus(ckpttypes.Finalized)
		if err != nil {
			s.T().Logf("failed to get last finalized epoch: %v", err)
			return false
		}
		return epoch <= lastFinalizedCkpt.RawCheckpoint.EpochNum
	}, 1*time.Minute, 1*time.Second)

	s.T().Logf("epoch %d is finalised", epoch)
}

// helper function: getDeterministicCovenantKey returns a single, constant private key and its corresponding public key.
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

	if publicKeyHex != "bb50e2d89a4ed70663d080659fe0ad4b9bc3e06c17a227433966cb59ceee020d" {
		return nil, nil, "", fmt.Errorf("public key hex is not expected")
	}

	return privateKey, publicKey, publicKeyHex, nil
}
