package e2e

import (
	"fmt"
	"math/rand"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cometbft/cometbft/libs/bytes"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	"github.com/babylonlabs-io/babylon/v3/app/signingcontext"
	v3 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v3"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer/config"
	"github.com/babylonlabs-io/babylon/v3/testutil/coins"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/v3/x/finality/types"
	itypes "github.com/babylonlabs-io/babylon/v3/x/incentive/types"
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
	finalityIdx              uint64
	finalityBlockHeightVoted uint64
	fp1RandListInfo          *datagen.RandListInfo
	fp2RandListInfo          *datagen.RandListInfo

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

	s.configurer, err = configurer.NewSoftwareUpgradeConfigurer(
		s.T(),
		true,
		config.UpgradeV3FilePath,
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
}

func (s *SoftwareUpgradeV3TestSuite) SetupVerifiedBtcDelegations(n *chain.NodeConfig) {
	s.del1Addr = n.KeysAdd(wDel1)
	s.del2Addr = n.KeysAdd(wDel2)

	n.BankMultiSendFromNode([]string{s.del1Addr, s.del2Addr}, "1000000ubbn")

	n.WaitForNextBlock()

	// fp1Del1
	s.CreateBTCDelegationV2AndCheck(n, wDel1, s.fp1, s.del1BTCSK, s.del1Addr, s.fp1Del1StakingAmt)
	// fp2Del1
	s.CreateBTCDelegationV2AndCheck(n, wDel1, s.fp2, s.del1BTCSK, s.del1Addr, s.fp2Del1StakingAmt)
	// fp1Del2
	s.CreateBTCDelegationV2AndCheck(n, wDel2, s.fp1, s.del2BTCSK, s.del2Addr, s.fp1Del2StakingAmt)

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

	// submit finality signature
	s.finalityIdx = s.finalityBlockHeightVoted - commitStartHeight

	n.WaitForNextBlockWithSleep50ms()

	// send vote of fp2 first to avoid finalizing block only with fp1 vote
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
	appHash := n.AddFinalitySignatureToBlockWithContext(
		s.fp1BTCSK,
		s.fp1.BtcPk,
		s.finalityBlockHeightVoted,
		s.fp1RandListInfo.SRList[s.finalityIdx],
		&s.fp1RandListInfo.PRList[s.finalityIdx],
		*s.fp1RandListInfo.ProofList[s.finalityIdx].ToProto(),
		"",
		fmt.Sprintf("--from=%s", wFp1),
	)

	n.WaitForNextBlocks(2)

	// ensure vote is eventually cast
	var finalizedBlocks []*ftypes.IndexedBlock
	s.Eventually(func() bool {
		finalizedBlocks = n.QueryListBlocks(ftypes.QueriedBlockStatus_FINALIZED)
		return len(finalizedBlocks) > 0
	}, time.Minute, time.Millisecond*50)

	s.Equal(s.finalityBlockHeightVoted, finalizedBlocks[0].Height)
	s.Equal(appHash.Bytes(), finalizedBlocks[0].AppHash)
	s.T().Logf("the block %d is finalized", s.finalityBlockHeightVoted)

	s.finalityIdx++
	s.finalityBlockHeightVoted++
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
	n.WaitForNextBlocks(20)

	s.CheckBtcRewardsAfterUpgrade(expectedUpgradeHeight)
}

func (s *SoftwareUpgradeV3TestSuite) CheckFpAfterUpgrade() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
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

<<<<<<< HEAD
	btcstakingparams, exists := stakingParams["params"]
	s.Require().True(exists, "btcstakingparams params should exist")

	btcparamsMap, ok := btcstakingparams.(map[string]interface{})
	s.Require().True(ok, "btcstakingparams params should exist")

	maxFP, ok := btcparamsMap["max_finality_providers"]
	s.Require().True(ok, "max_finality_providers param should exist")
	s.Require().Equal(float64(10), maxFP, "max_finality_providers should be 10")

	s.Require().Equal(float64(264773), btcparamsMap["btc_activation_height"],
		"BtcActivationHeight should be incremented correctly")

	// check that the module exists by querying parameters with the QueryParams helper
	var btcstkconsumerParams map[string]interface{}
	n.QueryParams(btcstkconsumerModulePath, &btcstkconsumerParams)
	s.T().Logf("btcstkconsumer params: %v", btcstkconsumerParams)

	btcConsParams, exists := btcstkconsumerParams["params"]
	s.Require().True(exists, "btcstkconsumer params should exist")

	btcConsParamsMap, ok := btcConsParams.(map[string]interface{})
	s.Require().True(ok, "btcstkconsumer params should be a map")
	s.Require().Equal(false, btcConsParamsMap["permissioned_integration"],
		"permissioned_integration should be false")
=======
	btcStkConsParams := n.QueryBTCStkConsumerParams()
	s.Require().False(btcStkConsParams.PermissionedIntegration, "btcstkconsumer permissioned integration should be false")
>>>>>>> 14095de (chore: e2e test for incentives migration in upgrade v3 (#1473))

	zoneConciergeParams := n.QueryZoneConciergeParams()
	s.Require().Equal(uint32(2419200), zoneConciergeParams.IbcPacketTimeoutSeconds, "ibc_packet_timeout_seconds should be 2419200")

	btcStkParams := n.QueryBTCStakingParams()
	s.Require().Equal(uint32(10), btcStkParams.MaxFinalityProviders, "max_finality_providers should be 10")
	s.Require().Equal(uint32(264773), btcStkParams.BtcActivationHeight, "btc activation height should be 264773")
}

func (s *SoftwareUpgradeV3TestSuite) CheckBtcRewardsAfterUpgrade(expectedUpgradeHeight int64) {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)
	n, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)

	// Current setup of voting power
	// (fp1, del1) => 2_00000000
	// (fp1, del2) => 4_00000000
	// (fp2, del1) => 2_00000000

	// The sum per bech32 address will be
	// (fp1)  => 6_00000000
	// (fp2)  => 2_00000000
	// (del1) => 4_00000000
	// (del2) => 4_00000000

	// check for rewards from the finality activation height until last finalized block
	finalizedBlocks := n.QueryListBlocks(ftypes.QueriedBlockStatus_FINALIZED)
	firstFinalizedBlock := finalizedBlocks[0]
	lastFinalizedBlock := finalizedBlocks[len(finalizedBlocks)-1]
	// at least one block should be finalized after the upgrade
	s.Require().GreaterOrEqual(lastFinalizedBlock.Height, uint64(expectedUpgradeHeight))

	// assuming that both fps were rewarded the same amounts
	fp1Rwds, fp2Rwds, del1, del2 := s.QueryRewardGauges(n)

	vpFp1 := s.fp1Del1StakingAmt + s.fp1Del2StakingAmt
	vpFp2 := s.fp2Del1StakingAmt
	totalVp := vpFp1 + vpFp2

	fp1Rate := sdkmath.LegacyNewDec(vpFp1).QuoTruncate(sdkmath.LegacyNewDec(totalVp))
	fp2Rate := sdkmath.LegacyNewDec(vpFp2).QuoTruncate(sdkmath.LegacyNewDec(totalVp))

	totalCoinsRewarded := sdk.NewCoins()
	fp1CommTotal, fp2CommTotal, fp1CoinsForDels, fp2CoinsForDels := sdk.NewCoins(), sdk.NewCoins(), sdk.NewCoins(), sdk.NewCoins()
	for blkHeight := firstFinalizedBlock.Height; blkHeight <= lastFinalizedBlock.Height; blkHeight++ {
		fpsVoted := n.QueryVotesAtHeight(blkHeight)
		s.Require().Len(fpsVoted, 2, "two fps should have voted")
		for _, fpVoted := range fpsVoted {
			if fpVoted.Equals(s.fp1.BtcPk) {
				continue
			}
			if fpVoted.Equals(s.fp2.BtcPk) {
				continue
			}
			s.Require().Fail("some other fp voted?")
		}

		fp1Vp := n.QueryFinalityProviderPowerAtHeight(s.fp1.BtcPk, blkHeight)
		s.Require().EqualValues(vpFp1, fp1Vp)
		fp2Vp := n.QueryFinalityProviderPowerAtHeight(s.fp2.BtcPk, blkHeight)
		s.Require().EqualValues(vpFp2, fp2Vp)

		// calculate the rewards given for each block
		coinsInBlk, err := n.QueryBtcStkGauge(blkHeight)
		s.Require().NoError(err)
		totalCoinsRewarded = totalCoinsRewarded.Add(coinsInBlk...)

		coinsForFp1AndDels := itypes.GetCoinsPortion(coinsInBlk, fp1Rate)
		fp1Comm := itypes.GetCoinsPortion(coinsForFp1AndDels, *s.fp1.Commission)
		fp1CommTotal = fp1CommTotal.Add(fp1Comm...)
		fp1CoinsForDels = fp1CoinsForDels.Add(coinsForFp1AndDels.Sub(fp1Comm...)...)

		coinsForFp2AndDels := itypes.GetCoinsPortion(coinsInBlk, fp2Rate)
		fp2Comm := itypes.GetCoinsPortion(coinsForFp2AndDels, *s.fp2.Commission)
		fp2CommTotal = fp2CommTotal.Add(fp2Comm...)
		fp2CoinsForDels = fp2CoinsForDels.Add(coinsForFp2AndDels.Sub(fp2Comm...)...)
	}

	// fp commissions check
	coins.RequireCoinsDiffInPointOnePercentMargin(s.T(), fp1CommTotal, fp1Rwds.Coins)
	coins.RequireCoinsDiffInPointOnePercentMargin(s.T(), fp2CommTotal, fp2Rwds.Coins)

	// BTC delegators check
	// (fp1, del1) => 2_00000000
	// (fp1, del2) => 4_00000000
	// (fp2, del1) => 2_00000000

	// del1 receives what is left after fp commission for fp2 since it is the only staker
	// plus his share in fp1CoinsForDels 1/3
	del1ShareFp1 := itypes.GetCoinsPortion(fp1CoinsForDels, sdkmath.LegacyMustNewDecFromStr("0.333333333"))
	expCoinsDel1 := fp2CoinsForDels.Add(del1ShareFp1...)
	coins.RequireCoinsDiffInPointOnePercentMargin(s.T(), expCoinsDel1, del1.Coins)

	// del2 receives 2/3 of fp1CoinsForDels
	expCoinsDel2 := itypes.GetCoinsPortion(fp1CoinsForDels, sdkmath.LegacyMustNewDecFromStr("0.666666667"))
	coins.RequireCoinsDiffInPointOnePercentMargin(s.T(), expCoinsDel2, del2.Coins)

	// since the fp commission is fixed at 20% and both delegators have the same amount staked over all dels 4_00000000
	// they should earn roughly the same amounts, with rounding diffs
	s.Require().Equal(s.fp1.Commission.String(), s.fp2.Commission.String())
	coins.RequireCoinsDiffInPointOnePercentMargin(s.T(), del1.Coins, del2.Coins)
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
	// send vote of fp2 first to avoid finalizing block only with fp1 vote. Vps:
	// (fp1)  => 6_00000000
	// (fp2)  => 2_00000000
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

	s.finalityIdx++
	s.finalityBlockHeightVoted++
	return appHash
}

// QueryRewardGauges returns the rewards available for fp1, fp2, del1, del2
func (s *SoftwareUpgradeV3TestSuite) QueryRewardGauges(n *chain.NodeConfig) (
	fp1, fp2, del1, del2 *itypes.RewardGaugesResponse,
) {
	n.WaitForNextBlockWithSleep50ms()

	g := new(errgroup.Group)
	var (
		err                 error
		fp1RewardGauges     map[string]*itypes.RewardGaugesResponse
		fp2RewardGauges     map[string]*itypes.RewardGaugesResponse
		btcDel1RewardGauges map[string]*itypes.RewardGaugesResponse
		btcDel2RewardGauges map[string]*itypes.RewardGaugesResponse
	)

	g.Go(func() error {
		fp1RewardGauges, err = n.QueryRewardGauge(s.fp1.Address())
		if err != nil {
			return fmt.Errorf("failed to query rewards for fp1: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		fp2RewardGauges, err = n.QueryRewardGauge(s.fp2.Address())
		if err != nil {
			return fmt.Errorf("failed to query rewards for fp2: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		btcDel1RewardGauges, err = n.QueryRewardGauge(sdk.MustAccAddressFromBech32(s.del1Addr))
		if err != nil {
			return fmt.Errorf("failed to query rewards for del1: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		btcDel2RewardGauges, err = n.QueryRewardGauge(sdk.MustAccAddressFromBech32(s.del2Addr))
		if err != nil {
			return fmt.Errorf("failed to query rewards for del2: %w", err)
		}
		return nil
	})
	s.NoError(g.Wait())

	fp1RewardGauge, ok := fp1RewardGauges[itypes.FINALITY_PROVIDER.String()]
	s.True(ok)
	s.True(fp1RewardGauge.Coins.IsAllPositive())

	fp2RewardGauge, ok := fp2RewardGauges[itypes.FINALITY_PROVIDER.String()]
	s.True(ok)
	s.True(fp2RewardGauge.Coins.IsAllPositive())

	btcDel1RewardGauge, ok := btcDel1RewardGauges[itypes.BTC_STAKER.String()]
	s.True(ok)
	s.True(btcDel1RewardGauge.Coins.IsAllPositive())

	btcDel2RewardGauge, ok := btcDel2RewardGauges[itypes.BTC_STAKER.String()]
	s.True(ok)
	s.True(btcDel2RewardGauge.Coins.IsAllPositive())

	return fp1RewardGauge, fp2RewardGauge, btcDel1RewardGauge, btcDel2RewardGauge
}
