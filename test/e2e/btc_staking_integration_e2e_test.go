package e2e

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/babylonchain/babylon/crypto/eots"
	"github.com/babylonchain/babylon/test/e2e/configurer"
	"github.com/babylonchain/babylon/test/e2e/configurer/chain"
	"github.com/babylonchain/babylon/test/e2e/initialization"
	"github.com/babylonchain/babylon/testutil/datagen"
	bbn "github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	bsctypes "github.com/babylonchain/babylon/x/btcstkconsumer/types"
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/suite"
)

type BTCStakingIntegrationTestSuite struct {
	suite.Suite

	configurer configurer.Configurer
}

func (s *BTCStakingIntegrationTestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite...")
	var (
		err error
	)

	// The e2e test flow is as follows:
	//
	// 1. Configure 1 chain with some validator nodes
	// 2. Execute various e2e tests
	s.configurer, err = configurer.NewBTCStakingIntegrationConfigurer(s.T(), true)

	s.Require().NoError(err)

	err = s.configurer.ConfigureChains()
	s.Require().NoError(err)

	err = s.configurer.RunSetup()
	s.Require().NoError(err)
}

func (s *BTCStakingIntegrationTestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	s.Require().NoError(err)
}

func (s *BTCStakingIntegrationTestSuite) Test1RegisterNewConsumerChain() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)
	s.registerVerifyConsumerChain(nonValidatorNode)
}

func (s *BTCStakingIntegrationTestSuite) Test2CreateConsumerChainFinalityProvider() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	// get the chain registered in Test1
	chainRegistryList := nonValidatorNode.QueryChainRegistryList(&query.PageRequest{Limit: 1})
	consumerChainId := chainRegistryList.ChainIds[0]

	// create a random num of finality providers from 1 to 5 on the consumer chain
	numFPs := datagen.RandomInt(r, 5) + 1
	for i := 0; i < int(numFPs); i++ {
		s.createVerifyConsumerChainFP(nonValidatorNode, consumerChainId)
	}
}

func (s *BTCStakingIntegrationTestSuite) Test3RestakeDelegationToMultipleFPs() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	// get the chain registered in Test1
	chainRegistryList := nonValidatorNode.QueryChainRegistryList(&query.PageRequest{Limit: 1})
	consumerChainId := chainRegistryList.ChainIds[0]
	// get the consumer chain created in Test2
	consumerChainFp := nonValidatorNode.QueryConsumerFinalityProviders(consumerChainId)[0]

	// register a babylon finality provider
	babylonFp := s.createVerifyBabylonFP(nonValidatorNode)

	// create a delegation and restake to both Babylon and consumer chain finality providers
	// NOTE: this will create delegation in pending state as covenant sigs are not provided
	delBtcPk, stakingTxHash := s.createBabylonDelegation(nonValidatorNode, babylonFp, consumerChainFp)

	// check delegation
	delegation := nonValidatorNode.QueryBtcDelegation(stakingTxHash)
	s.NotNil(delegation)

	// check consumer chain finality provider delegation
	czPendingDelSet := nonValidatorNode.QueryFinalityProviderDelegations(consumerChainFp.BtcPk.MarshalHex())
	s.Len(czPendingDelSet, 1)
	czPendingDels := czPendingDelSet[0]
	s.Len(czPendingDels.Dels, 1)
	s.Equal(delBtcPk.SerializeCompressed()[1:], czPendingDels.Dels[0].BtcPk.MustToBTCPK().SerializeCompressed()[1:])
	s.Len(czPendingDels.Dels[0].CovenantSigs, 0)

	// check Babylon finality provider delegation
	pendingDelSet := nonValidatorNode.QueryFinalityProviderDelegations(babylonFp.BtcPk.MarshalHex())
	s.Len(pendingDelSet, 1)
	pendingDels := pendingDelSet[0]
	s.Len(pendingDels.Dels, 1)
	s.Equal(delBtcPk.SerializeCompressed()[1:], pendingDels.Dels[0].BtcPk.MustToBTCPK().SerializeCompressed()[1:])
	s.Len(pendingDels.Dels[0].CovenantSigs, 0)
}

// helper function: register a random chain on Babylon and verify it
func (s *BTCStakingIntegrationTestSuite) registerVerifyConsumerChain(babylonNode *chain.NodeConfig) *bsctypes.ChainRegister {
	// Register a random chain on Babylon
	randomChain := datagen.GenRandomChainRegister(r)
	babylonNode.RegisterConsumerChain(randomChain.ChainId, randomChain.ChainName, randomChain.ChainDescription)
	babylonNode.WaitForNextBlock()

	// Query the chain registry to verify the chain was registered
	chainRegistry := babylonNode.QueryChainRegistry(randomChain.ChainId)
	s.Require().Len(chainRegistry, 1)
	s.Require().Equal(randomChain.ChainId, chainRegistry[0].ChainId)
	s.Require().Equal(randomChain.ChainName, chainRegistry[0].ChainName)
	s.Require().Equal(randomChain.ChainDescription, chainRegistry[0].ChainDescription)
	return randomChain
}

// helper function: create a random consumer chain finality provider on Babylon and verify it
func (s *BTCStakingIntegrationTestSuite) createVerifyConsumerChainFP(babylonNode *chain.NodeConfig, consumerChainId string) *bstypes.FinalityProvider {
	/*
		create a random consumer finality provider on Babylon
	*/
	// NOTE: we use the node's secret key as Babylon secret key for the finality provider
	czFpBTCSK, _, _ := datagen.GenRandomBTCKeyPair(r)
	sr, _, err := eots.NewMasterRandPair(r)
	s.NoError(err)
	czFp, err := datagen.GenRandomCustomFinalityProvider(r, czFpBTCSK, babylonNode.SecretKey, sr)
	s.NoError(err)
	babylonNode.CreateConsumerFinalityProvider(
		czFp.BabylonPk, czFp.BtcPk, czFp.Pop, czFp.MasterPubRand, consumerChainId, czFp.Description.Moniker,
		czFp.Description.Identity, czFp.Description.Website, czFp.Description.SecurityContact,
		czFp.Description.Details, czFp.Commission,
	)

	// wait for a block so that above txs take effect
	babylonNode.WaitForNextBlock()

	// query the existence of finality provider and assert equivalence
	actualFp := babylonNode.QueryConsumerFinalityProvider(consumerChainId, czFp.BtcPk.MarshalHex())
	s.Equal(czFp.Description, actualFp.Description)
	s.Equal(czFp.Commission, actualFp.Commission)
	s.Equal(czFp.BabylonPk, actualFp.BabylonPk)
	s.Equal(czFp.BtcPk, actualFp.BtcPk)
	s.Equal(czFp.Pop, actualFp.Pop)
	s.Equal(czFp.SlashedBabylonHeight, actualFp.SlashedBabylonHeight)
	s.Equal(czFp.SlashedBtcHeight, actualFp.SlashedBtcHeight)
	s.Equal(consumerChainId, actualFp.ChainId)
	return czFp
}

// helper function: create a random Babylon finality provider and verify it
func (s *BTCStakingIntegrationTestSuite) createVerifyBabylonFP(babylonNode *chain.NodeConfig) *bstypes.FinalityProviderResponse {
	/*
		create a random finality provider on Babylon
	*/
	// NOTE: we use the node's secret key as Babylon secret key for the finality provider
	babylonFpBTCSK, _, _ := datagen.GenRandomBTCKeyPair(r)
	sr, _, err := eots.NewMasterRandPair(r)
	s.NoError(err)
	babylonFp, err := datagen.GenRandomCustomFinalityProvider(r, babylonFpBTCSK, babylonNode.SecretKey, sr)
	s.NoError(err)
	babylonNode.CreateFinalityProvider(babylonFp.BabylonPk, babylonFp.BtcPk, babylonFp.Pop, babylonFp.MasterPubRand, babylonFp.Description.Moniker, babylonFp.Description.Identity, babylonFp.Description.Website, babylonFp.Description.SecurityContact, babylonFp.Description.Details, babylonFp.Commission)

	// wait for a block so that above txs take effect
	babylonNode.WaitForNextBlock()

	// query the existence of finality provider and assert equivalence
	actualFps := babylonNode.QueryFinalityProviders()
	s.Len(actualFps, 1)
	s.Equal(babylonFp.Description, actualFps[0].Description)
	s.Equal(babylonFp.Commission, actualFps[0].Commission)
	s.Equal(babylonFp.BabylonPk, actualFps[0].BabylonPk)
	s.Equal(babylonFp.BtcPk, actualFps[0].BtcPk)
	s.Equal(babylonFp.Pop, actualFps[0].Pop)
	s.Equal(babylonFp.SlashedBabylonHeight, actualFps[0].SlashedBabylonHeight)
	s.Equal(babylonFp.SlashedBtcHeight, actualFps[0].SlashedBtcHeight)

	return actualFps[0]
}

// helper function: create a Babylon delegation and restake to Babylon and consumer chain finality providers
func (s *BTCStakingIntegrationTestSuite) createBabylonDelegation(nonValidatorNode *chain.NodeConfig, babylonFp *bstypes.FinalityProviderResponse, consumerChainFp *bsctypes.FinalityProviderResponse) (*btcec.PublicKey, string) {
	// finalise epochs until the registered epoch of the finality provider
	// so that the finality provider can receive BTC delegations
	// TODO: it is assumed here that babylonFp is registered after consumerChainFp so
	//  if we finalize registered epoch of babylonFp the other would also get finalized
	//  ideally we should get registered epoch of each restaked fp and finalize it.
	var (
		startEpoch = uint64(1)
		endEpoch   = babylonFp.RegisteredEpoch
	)
	// wait until the end epoch is sealed
	s.Eventually(func() bool {
		ch, _ := nonValidatorNode.QueryCurrentHeight()
		ce, _ := nonValidatorNode.QueryCurrentEpoch()
		fmt.Println("current height", ch)
		fmt.Println("current epoch", ce)
		resp, err := nonValidatorNode.QueryRawCheckpoint(endEpoch)
		if err != nil {
			if !strings.Contains(err.Error(), "codespace checkpointing code 1201") {
				return false
			}
		}
		return resp.Status == ckpttypes.Sealed
	}, time.Minute, time.Second*5)
	// finalise these epochs
	nonValidatorNode.FinalizeSealedEpochs(startEpoch, endEpoch)

	/*
		create a random BTC delegation restaking to Babylon and consumer chain finality providers
	*/

	delBbnSk := nonValidatorNode.SecretKey
	delBtcSk, delBtcPk, _ := datagen.GenRandomBTCKeyPair(r)

	// BTC staking params, BTC delegation key pairs and PoP
	params := nonValidatorNode.QueryBTCStakingParams()

	// minimal required unbonding time
	unbondingTime := uint16(initialization.BabylonBtcFinalizationPeriod) + 1

	// get covenant BTC PKs
	covenantBTCPKs := []*btcec.PublicKey{}
	for _, covenantPK := range params.CovenantPks {
		covenantBTCPKs = append(covenantBTCPKs, covenantPK.MustToBTCPK())
	}
	// NOTE: we use the node's secret key as Babylon secret key for the BTC delegation
	pop, err := bstypes.NewPoP(delBbnSk, delBtcSk)
	s.NoError(err)
	// generate staking tx and slashing tx
	stakingTimeBlocks := uint16(math.MaxUint16)
	testStakingInfo := datagen.GenBTCStakingSlashingInfo(
		r,
		s.T(),
		net,
		delBtcSk,
		[]*btcec.PublicKey{babylonFp.BtcPk.MustToBTCPK(), consumerChainFp.BtcPk.MustToBTCPK()},
		covenantBTCPKs,
		covenantQuorum,
		stakingTimeBlocks,
		stakingValue,
		params.SlashingAddress,
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
		delBtcSk,
	)
	s.NoError(err)

	// submit staking tx to Bitcoin and get inclusion proof
	currentBtcTipResp, err := nonValidatorNode.QueryTip()
	s.NoError(err)
	currentBtcTip, err := chain.ParseBTCHeaderInfoResponseToInfo(currentBtcTipResp)
	s.NoError(err)

	blockWithStakingTx := datagen.CreateBlockWithTransaction(r, currentBtcTip.Header.ToBlockHeader(), stakingMsgTx)
	nonValidatorNode.InsertHeader(&blockWithStakingTx.HeaderBytes)
	// make block k-deep
	for i := 0; i < initialization.BabylonBtcConfirmationPeriod; i++ {
		nonValidatorNode.InsertNewEmptyBtcHeader(r)
	}
	stakingTxInfo := btcctypes.NewTransactionInfoFromSpvProof(blockWithStakingTx.SpvProof)

	// generate BTC undelegation stuff
	stkTxHash := testStakingInfo.StakingTx.TxHash()
	unbondingValue := stakingValue - datagen.UnbondingTxFee // TODO: parameterise fee
	testUnbondingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r,
		s.T(),
		net,
		delBtcSk,
		[]*btcec.PublicKey{babylonFp.BtcPk.MustToBTCPK(), consumerChainFp.BtcPk.MustToBTCPK()},
		covenantBTCPKs,
		covenantQuorum,
		wire.NewOutPoint(&stkTxHash, datagen.StakingOutIdx),
		stakingTimeBlocks,
		unbondingValue,
		params.SlashingAddress,
		params.SlashingRate,
		unbondingTime,
	)
	delUnbondingSlashingSig, err := testUnbondingInfo.GenDelSlashingTxSig(delBtcSk)
	s.NoError(err)

	// submit the message for creating BTC delegation
	delBTCPKs := []bbn.BIP340PubKey{*bbn.NewBIP340PubKeyFromBTCPK(delBtcPk)}
	nonValidatorNode.CreateBTCDelegation(
		delBbnSk.PubKey().(*secp256k1.PubKey),
		delBTCPKs,
		pop,
		stakingTxInfo,
		[]*bbn.BIP340PubKey{babylonFp.BtcPk, consumerChainFp.BtcPk},
		stakingTimeBlocks,
		btcutil.Amount(stakingValue),
		testStakingInfo.SlashingTx,
		delegatorSig,
		testUnbondingInfo.UnbondingTx,
		testUnbondingInfo.SlashingTx,
		uint16(unbondingTime),
		btcutil.Amount(unbondingValue),
		delUnbondingSlashingSig,
	)

	// wait for a block so that above txs take effect
	nonValidatorNode.WaitForNextBlock()
	nonValidatorNode.WaitForNextBlock()

	return delBtcPk, stakingTxHash
}
