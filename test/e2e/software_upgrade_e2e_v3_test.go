package e2e

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cometbft/cometbft/libs/bytes"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	"github.com/babylonlabs-io/babylon/v3/app/signingcontext"
	v3 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v3"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer/config"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v3/testutil/sample"
	btclighttypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/v3/x/finality/types"
)

const (
	// commit public randomness list
	commitStartHeight = uint64(5)
)

type SoftwareUpgradeV3TestSuite struct {
	BaseBtcRewardsDistribution

	fp1BTCSK  *btcec.PrivateKey
	fp2BTCSK  *btcec.PrivateKey
	del1BTCSK *btcec.PrivateKey
	del2BTCSK *btcec.PrivateKey

	fp1 *bstypes.FinalityProvider
	fp2 *bstypes.FinalityProvider

	// 3 Delegations will start closely and possibly in the same block
	// (fp1, del1), (fp1, del2), (fp2, del1)

	// (fp1, del1) fp1Del1StakingAmt => 2_00000000
	// (fp1, del2) fp1Del2StakingAmt => 4_00000000
	// (fp2, del1) fp2Del2StakingAmt => 2_00000000
	fp1Del1StakingAmt int64
	fp1Del2StakingAmt int64
	fp2Del1StakingAmt int64

	// bech32 address of the delegators
	del1Addr string
	del2Addr string
	// bech32 address of the finality providers
	fp1Addr string
	fp2Addr string

	// finality helpers
	finalityIdx               uint64
	firstFinalizedBlockHeight uint64
	finalityBlockHeightVoted  uint64
	fp1RandListInfo           *datagen.RandListInfo
	fp2RandListInfo           *datagen.RandListInfo

	configurer *configurer.UpgradeConfigurer
}

func (s *SoftwareUpgradeV3TestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite for v2.2.0 to v3 upgrade...")
	s.r = rand.New(rand.NewSource(time.Now().UnixNano()))
	var err error

	s.net = &chaincfg.SigNetParams
	s.fp1BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.fp2BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.del1BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.del2BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)

	s.fp1Del1StakingAmt = int64(2 * 10e8)
	s.fp1Del2StakingAmt = int64(4 * 10e8)
	s.fp2Del1StakingAmt = int64(2 * 10e8)

	covenantSKs, _, _ := bstypes.DefaultCovenantCommittee()
	s.covenantSKs = covenantSKs

	// func runs right before the upgrade proposal is sent
	preUpgradeFunc := func(chains []*chain.Config) {
		chainA := chains[0]
		n := chainA.NodeConfigs[1]

		chainA.WaitUntilHeight(2)
		s.SetupFps(n)
		s.SetupVerifiedBtcDelegations(n)
		s.FpCommitPubRandAndVote(n)
	}

	btcHeaderGenesis := sample.SignetBtcHeader195552(s.T())

	s.configurer, err = configurer.NewSoftwareUpgradeConfigurer(
		s.T(),
		true,
		config.UpgradeV3FilePath,
		[]*btclighttypes.BTCHeaderInfo{btcHeaderGenesis},
		preUpgradeFunc,
	)
	s.NoError(err)

	err = s.configurer.ConfigureChains()
	s.NoError(err)
	err = s.configurer.RunSetup() // upgrade happens at the setup of configurer.
	s.Require().NoError(err)
}

func (s *SoftwareUpgradeV3TestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

func (s *SoftwareUpgradeV3TestSuite) SetupFps(n *chain.NodeConfig) {
	n.WaitForNextBlock()

	s.fp1Addr = n.KeysAdd(wFp1)
	s.fp2Addr = n.KeysAdd(wFp2)
	n.BankMultiSendFromNode([]string{s.fp1Addr, s.fp2Addr}, "1000000ubbn")

	n.WaitForNextBlock()

	s.fp1 = CreateNodeFpV2(
		s.T(),
		s.r,
		s.fp1BTCSK,
		n,
		s.fp1Addr,
	)

	s.fp2 = CreateNodeFpV2(
		s.T(),
		s.r,
		s.fp2BTCSK,
		n,
		s.fp2Addr,
	)
	n.WaitForNextBlock()

	actualFps := n.QueryFinalityProvidersV2()
	s.Require().Len(actualFps, 2)

	// s.commitStartHeight = n.LatestBlockNumber()

	// var err error
	// _, s.fp1CommitPubRandList, err = datagen.GenRandomMsgCommitPubRandList(s.r, s.fp1BTCSK, "", s.commitStartHeight, numPubRand)
	// s.NoError(err)
	// _, s.fp2CommitPubRandList, err = datagen.GenRandomMsgCommitPubRandList(s.r, s.fp2BTCSK, "", s.commitStartHeight, numPubRand)
	// s.NoError(err)

	// s.Require().NotNil(s.fp1CommitPubRandList, "fp1CommitPubRandList should not be nil")
	// s.Require().NotNil(s.fp2CommitPubRandList, "fp2CommitPubRandList should not be nil")

	// n.CommitPubRandList(
	// 	s.fp1CommitPubRandList.FpBtcPk,
	// 	s.fp1CommitPubRandList.StartHeight,
	// 	s.fp1CommitPubRandList.NumPubRand,
	// 	s.fp1CommitPubRandList.Commitment,
	// 	s.fp1CommitPubRandList.Sig,
	// )
	// n.CommitPubRandList(
	// 	s.fp2CommitPubRandList.FpBtcPk,
	// 	s.fp2CommitPubRandList.StartHeight,
	// 	s.fp2CommitPubRandList.NumPubRand,
	// 	s.fp2CommitPubRandList.Commitment,
	// 	s.fp2CommitPubRandList.Sig,
	// )
}

func (s *SoftwareUpgradeV3TestSuite) SetupVerifiedBtcDelegations(n *chain.NodeConfig) {
	s.del1Addr = n.KeysAdd(wDel1)
	s.del2Addr = n.KeysAdd(wDel2)

	n.BankMultiSendFromNode([]string{s.del1Addr, s.del2Addr}, "1000000ubbn")

	n.WaitForNextBlock()

	// fp1Del1
	s.CreateBTCDelegationV2AndCheck(n, wDel1, s.fp1, s.del1BTCSK, s.del1Addr, s.fp1Del1StakingAmt)
	// fp1Del2
	s.CreateBTCDelegationV2AndCheck(n, wDel2, s.fp1, s.del2BTCSK, s.del2Addr, s.fp1Del2StakingAmt)
	// fp2Del1
	s.CreateBTCDelegationV2AndCheck(n, wDel1, s.fp2, s.del1BTCSK, s.del1Addr, s.fp2Del1StakingAmt)

	resp := n.QueryBtcDelegations(bstypes.BTCDelegationStatus_ANY)
	require.Len(s.T(), resp.BtcDelegations, 3)

	s.CreateCovenantsAndSubmitSignaturesToPendDels(n, s.fp1, s.fp2)
}

// CreateBTCDelegationV2AndCheck creates a btc delegation with empty signing context
func (s *SoftwareUpgradeV3TestSuite) CreateBTCDelegationV2AndCheck(
	n *chain.NodeConfig,
	wDel string,
	fp *bstypes.FinalityProvider,
	btcStakerSK *btcec.PrivateKey,
	delAddr string,
	stakingSatAmt int64,
) {
	n.CreateBTCDelegationMultipleFPsAndCheckWithPopContext(
		s.r,
		s.T(),
		s.net,
		wDel,
		[]*bstypes.FinalityProvider{fp},
		btcStakerSK,
		delAddr,
		stakingTimeBlocks,
		stakingSatAmt,
		"",
	)
}

func (s *SoftwareUpgradeV3TestSuite) FpCommitPubRandAndVote(n *chain.NodeConfig) {
	// v2 is empty context
	randCommitContext := ""

	fp1RandListInfo, fp1CommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(s.r, s.fp1BTCSK, randCommitContext, commitStartHeight, numPubRand)
	s.NoError(err)
	s.fp1RandListInfo = fp1RandListInfo

	fp2RandListInfo, fp2CommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(s.r, s.fp2BTCSK, randCommitContext, commitStartHeight, numPubRand)
	s.NoError(err)
	s.fp2RandListInfo = fp2RandListInfo

	n.CommitPubRandList(
		wFp1,
		fp1CommitPubRandList.FpBtcPk,
		fp1CommitPubRandList.StartHeight,
		fp1CommitPubRandList.NumPubRand,
		fp1CommitPubRandList.Commitment,
		fp1CommitPubRandList.Sig,
	)

	n.CommitPubRandList(
		wFp2,
		fp2CommitPubRandList.FpBtcPk,
		fp2CommitPubRandList.StartHeight,
		fp2CommitPubRandList.NumPubRand,
		fp2CommitPubRandList.Commitment,
		fp2CommitPubRandList.Sig,
	)

	// needs to wait for a block to make sure the pub rand is committed
	// prior to epoch finalization
	n.WaitForNextBlockWithSleep50ms()

	// check all FPs requirement to be active
	// TotalBondedSat > 0
	// IsTimestamped
	// !IsJailed
	// !IsSlashed

	fp1CommitPubRand := n.QueryListPubRandCommit(fp1CommitPubRandList.FpBtcPk)
	fp1PubRand := fp1CommitPubRand[commitStartHeight]
	s.Require().Equal(fp1PubRand.NumPubRand, numPubRand)

	fp2CommitPubRand := n.QueryListPubRandCommit(fp2CommitPubRandList.FpBtcPk)
	fp2PubRand := fp2CommitPubRand[commitStartHeight]
	s.Require().Equal(fp2PubRand.NumPubRand, numPubRand)

	finalizedEpoch := n.WaitUntilCurrentEpochIsSealedAndFinalized(1)
	s.Require().GreaterOrEqual(finalizedEpoch, fp1PubRand.EpochNum)
	s.Require().GreaterOrEqual(finalizedEpoch, fp2PubRand.EpochNum)

	fps := n.QueryFinalityProvidersV2()
	s.Require().Len(fps, 2)

	for _, fp := range fps {
		s.Require().False(fp.Jailed, "fp is jailed")
		s.Require().Zero(fp.SlashedBabylonHeight, "fp is slashed")

		fpDels := n.QueryFinalityProviderDelegations(fp.BtcPk.MarshalHex())
		if fp.BtcPk.Equals(s.fp1.BtcPk) {
			s.Require().Len(fpDels, 2)
		} else {
			s.Require().Len(fpDels, 1)
		}

		for _, fpDelStaker := range fpDels {
			for _, fpDel := range fpDelStaker.Dels {
				s.Require().True(fpDel.Active)
				s.Require().GreaterOrEqual(fpDel.TotalSat, uint64(0))
			}
		}
	}

	s.finalityBlockHeightVoted = n.WaitFinalityIsActivated()
	s.firstFinalizedBlockHeight = s.finalityBlockHeightVoted

	// submit finality signature
	s.finalityIdx = s.finalityBlockHeightVoted - commitStartHeight

	n.WaitForNextBlockWithSleep50ms()
	var (
		wg      sync.WaitGroup
		appHash bytes.HexBytes
	)
	wg.Add(2)

	go func() {
		defer wg.Done()
		appHash = n.AddFinalitySignatureToBlockWithContext(
			s.fp1BTCSK,
			s.fp1.BtcPk,
			s.finalityBlockHeightVoted,
			s.fp1RandListInfo.SRList[s.finalityIdx],
			&s.fp1RandListInfo.PRList[s.finalityIdx],
			*s.fp1RandListInfo.ProofList[s.finalityIdx].ToProto(),
			"",
			fmt.Sprintf("--from=%s", wFp1),
		)
	}()

	go func() {
		defer wg.Done()
		n.AddFinalitySignatureToBlockWithContext(
			s.fp2BTCSK,
			s.fp2.BtcPk,
			s.finalityBlockHeightVoted,
			s.fp2RandListInfo.SRList[s.finalityIdx],
			&s.fp2RandListInfo.PRList[s.finalityIdx],
			*s.fp2RandListInfo.ProofList[s.finalityIdx].ToProto(),
			"",
			fmt.Sprintf("--from=%s", wFp2),
		)
	}()

	wg.Wait()

	s.finalityIdx++
	s.finalityBlockHeightVoted++
	n.WaitForNextBlockWithSleep50ms()

	// ensure vote is eventually cast
	var finalizedBlocks []*ftypes.IndexedBlock
	s.Eventually(func() bool {
		finalizedBlocks = n.QueryListBlocks(ftypes.QueriedBlockStatus_FINALIZED)
		return len(finalizedBlocks) > 0
	}, time.Minute, time.Millisecond*50)

	s.Equal(s.finalityBlockHeightVoted, finalizedBlocks[0].Height)
	s.Equal(appHash.Bytes(), finalizedBlocks[0].AppHash)
	s.T().Logf("the block %d is finalized", s.finalityBlockHeightVoted)

	s.AddFinalityVoteUntilCurrentHeight(n, "")
}

// TestUpgradeV3 checks if the upgrade from v2.2.0 to v3 was successful
func (s *SoftwareUpgradeV3TestSuite) Test1UpgradeV3() {
	chainA := s.configurer.GetChainConfig(0)
	n, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)

	n.WaitForNextBlock()

	govProp, err := s.configurer.ParseGovPropFromFile()
	s.NoError(err)
	chainA.WaitUntilHeight(govProp.Plan.Height + 1) // waits for chain to produce blocks

	expectedUpgradeHeight := govProp.Plan.Height
	resp := n.QueryAppliedPlan(v3.UpgradeName)
	s.EqualValues(expectedUpgradeHeight, resp.Height, "the plan should be applied at the height %d", expectedUpgradeHeight)

	s.CheckFpAfterUpgrade()
	s.CheckParamsAfterUpgrade()

	n.WaitForNextBlock()

	// send finality votes until upgrade height plus 10 blocks
	fpFinVoteContext := signingcontext.FpFinVoteContextV0(n.ChainID(), appparams.AccFinality.String())
	s.AddFinalityVoteUntilCurrentHeight(n, fpFinVoteContext)

	// wait a few blocks for the reward to be allocated
	n.WaitForNextBlocks(10)

	// check for rewards from the finality activation height until last finalized block
	totalRewardsAllocated, err := n.QueryBtcStkGaugeFromBlocks(s.firstFinalizedBlockHeight, s.finalityBlockHeightVoted-1)
	s.Require().NoError(err)
	s.Require().NotNil(totalRewardsAllocated)
}

func (s *SoftwareUpgradeV3TestSuite) CheckFpAfterUpgrade() {
	chainA := s.configurer.GetChainConfig(0)
	n, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)

	fp1 := n.QueryFinalityProvider(s.fp1.BtcPk.MarshalHex())
	s.Require().Equal(fp1.BsnId, n.ChainID())
	fp2 := n.QueryFinalityProvider(s.fp2.BtcPk.MarshalHex())
	s.Require().Equal(fp2.BsnId, n.ChainID())

	// query pub randomness
	fp1CommitPubRand := n.QueryListPubRandCommit(s.fp1.BtcPk)
	s.Require().NotNil(fp1CommitPubRand, "fp1CommitPubRand should not be nil")
	_, ok := fp1CommitPubRand[commitStartHeight]
	s.Require().True(ok, "fp1CommitPubRand should contain commitStartHeight")

	fp2CommitPubRand := n.QueryListPubRandCommit(s.fp2.BtcPk)
	s.Require().NotNil(fp2CommitPubRand, "fp2CommitPubRand should not be nil")
	_, ok = fp2CommitPubRand[commitStartHeight]
	s.Require().True(ok, "fp2CommitPubRand should contain commitStartHeight")
}

func (s *SoftwareUpgradeV3TestSuite) CheckParamsAfterUpgrade() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)

	n, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)

	btcStkConsParams := n.QueryBTCStkConsumerParams()
	s.Require().False(btcStkConsParams.PermissionedIntegration, "btcstkconsumer permissioned integration should be false")

	zoneConciergeParams := n.QueryZoneConciergeParams()
	s.Require().Equal(uint32(2419200), zoneConciergeParams.IbcPacketTimeoutSeconds, "ibc_packet_timeout_seconds should be 2419200")

	btcStkParams := n.QueryBTCStakingParams()
	s.Require().Equal(uint32(10), btcStkParams.MaxFinalityProviders, "max_finality_providers should be 10")
	s.Require().Equal(uint32(260000), btcStkParams.BtcActivationHeight, "btc activation height should be 260000")
}

func (s *SoftwareUpgradeV3TestSuite) AddFinalityVoteUntilCurrentHeight(n *chain.NodeConfig, fpFinalityVoteContext string) {
	currentBlock := n.LatestBlockNumber()

	accFp1, err := n.QueryAccount(s.fp1.Addr)
	s.NoError(err)
	accFp2, err := n.QueryAccount(s.fp2.Addr)
	s.NoError(err)

	accNumberFp1 := accFp1.GetAccountNumber()
	accSequenceFp1 := accFp1.GetSequence()

	accNumberFp2 := accFp2.GetAccountNumber()
	accSequenceFp2 := accFp2.GetSequence()

	for s.finalityBlockHeightVoted < currentBlock {
		fp1Flags := []string{
			"--offline",
			fmt.Sprintf("--account-number=%d", accNumberFp1),
			fmt.Sprintf("--sequence=%d", accSequenceFp1),
			fmt.Sprintf("--from=%s", s.fp1.Addr),
		}
		fp2Flags := []string{
			"--offline",
			fmt.Sprintf("--account-number=%d", accNumberFp2),
			fmt.Sprintf("--sequence=%d", accSequenceFp2),
			fmt.Sprintf("--from=%s", s.fp2.Addr),
		}
		s.AddFinalityVote(n, fpFinalityVoteContext, fp1Flags, fp2Flags)

		accSequenceFp1++
		accSequenceFp2++
	}
}

func (s *SoftwareUpgradeV3TestSuite) AddFinalityVote(n *chain.NodeConfig, fpFinalityVoteContext string, flagsFp1, flagsFp2 []string) (appHash bytes.HexBytes) {
	appHash = n.AddFinalitySignatureToBlockWithContext(
		s.fp1BTCSK,
		s.fp1.BtcPk,
		s.finalityBlockHeightVoted,
		s.fp1RandListInfo.SRList[s.finalityIdx],
		&s.fp1RandListInfo.PRList[s.finalityIdx],
		*s.fp1RandListInfo.ProofList[s.finalityIdx].ToProto(),
		fpFinalityVoteContext,
		flagsFp1...,
	)

	n.AddFinalitySignatureToBlockWithContext(
		s.fp2BTCSK,
		s.fp2.BtcPk,
		s.finalityBlockHeightVoted,
		s.fp2RandListInfo.SRList[s.finalityIdx],
		&s.fp2RandListInfo.PRList[s.finalityIdx],
		*s.fp2RandListInfo.ProofList[s.finalityIdx].ToProto(),
		fpFinalityVoteContext,
		flagsFp2...,
	)

	s.finalityIdx++
	s.finalityBlockHeightVoted++
	return appHash
}
