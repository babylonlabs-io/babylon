package e2e

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	sdkmath "cosmossdk.io/math"
	bbn "github.com/babylonlabs-io/babylon/v2/types"
	"github.com/babylonlabs-io/babylon/v2/x/mint/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cometbft/cometbft/libs/bytes"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sync/errgroup"

	appparams "github.com/babylonlabs-io/babylon/v2/app/params"
	"github.com/babylonlabs-io/babylon/v2/app/signingcontext"
	v2 "github.com/babylonlabs-io/babylon/v2/app/upgrades/v2"
	"github.com/babylonlabs-io/babylon/v2/testutil/coins"
	"github.com/babylonlabs-io/babylon/v2/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v2/testutil/sample"
	btclighttypes "github.com/babylonlabs-io/babylon/v2/x/btclightclient/types"
	bstypes "github.com/babylonlabs-io/babylon/v2/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/v2/x/finality/types"
	itypes "github.com/babylonlabs-io/babylon/v2/x/incentive/types"

	"github.com/babylonlabs-io/babylon/v2/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v2/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v2/test/e2e/configurer/config"
)

const (
	TokenFactoryModulePath = "tokenfactory"
)

type SoftwareUpgradeV2TestSuite struct {
	suite.Suite
	configurer *configurer.UpgradeConfigurer

	r   *rand.Rand
	net *chaincfg.Params

	// BTC Reward data
	fp1BTCSK  *btcec.PrivateKey
	del1BTCSK *btcec.PrivateKey
	del2BTCSK *btcec.PrivateKey

	fp1 *bstypes.FinalityProvider

	// (fp1, del1) fp1Del1StakingAmt => 2_00000000
	// (fp1, del2) fp1Del2StakingAmt => 4_00000000
	fp1Del1StakingAmt int64
	fp1Del2StakingAmt int64

	// bech32 address of the delegators
	del1Addr string
	del2Addr string
	// bech32 address of the finality provider
	fp1Addr string

	// covenant helpers
	covenantSKs     []*btcec.PrivateKey
	covenantWallets []string

	// finality helpers
	finalityIdx              uint64
	finalityBlockHeightVoted uint64
	fp1RandListInfo          *datagen.RandListInfo
}

func (s *SoftwareUpgradeV2TestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite for v1.1 to v2 upgrade...")
	var err error

	btcHeaderGenesis := sample.SignetBtcHeader195552(s.T())

	s.r = rand.New(rand.NewSource(time.Now().Unix()))
	s.net = &chaincfg.SimNetParams
	s.fp1BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.del1BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.del2BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)

	s.fp1Del1StakingAmt = int64(2 * 10e8)
	s.fp1Del2StakingAmt = int64(4 * 10e8)

	covenantSKs, _, _ := bstypes.DefaultCovenantCommittee()
	s.covenantSKs = covenantSKs

	cfg, err := configurer.NewSoftwareUpgradeConfigurer(
		s.T(),
		true,
		config.UpgradeV2FilePath,
		[]*btclighttypes.BTCHeaderInfo{btcHeaderGenesis},
		s.PreUpgrade,
	)
	s.NoError(err)
	s.configurer = cfg

	err = s.configurer.ConfigureChains()
	s.NoError(err)
	err = s.configurer.RunSetup() // upgrade happens at the setup of configurer.
	s.Require().NoError(err)
}

func (s *SoftwareUpgradeV2TestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

// PreUpgrade runs right before the upgrade handler to v2
// Note: Preupgrade funcs need to use the node and chains from params
// the test suite still doesn't have it set
func (s *SoftwareUpgradeV2TestSuite) PreUpgrade(chains []*chain.Config) {
	chainA := chains[0]

	n, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)

	n.WaitForNextBlocks(2)

	s.preUpgradeCreateFp1(n)
	s.preUpgradeCreateBtcDels(n)
	s.preUpgradeSubmitCovdSigs(n)
	s.preUpgradeAddFinalitySigs(n)
	s.preUpgradeWithdrawRewardsBtcDel(n)
}

// Test1UpgradeV2 checks if the upgrade from v1.1 to v2 was successful
func (s *SoftwareUpgradeV2TestSuite) Test1UpgradeV2() {
	// Chain is already upgraded, check for new modules and state changes
	chainA := s.configurer.GetChainConfig(0)

	n, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)

	govProp, err := s.configurer.ParseGovPropFromFile()
	s.NoError(err)
	chainA.WaitUntilHeight(govProp.Plan.Height + 1) // waits for chain to produce blocks

	expectedUpgradeHeight := govProp.Plan.Height
	resp := n.QueryAppliedPlan(v2.UpgradeName)
	s.EqualValues(expectedUpgradeHeight, resp.Height, "the plan should be applied at the height %d", expectedUpgradeHeight)

	// Check that the module exists by querying parameters with the QueryParams helper
	var tokenfactoryParams map[string]interface{}
	n.QueryParams(TokenFactoryModulePath, &tokenfactoryParams)
	s.T().Logf("Tokenfactory params: %v", tokenfactoryParams)

	params, ok := tokenfactoryParams["params"].(map[string]interface{})
	s.Require().True(ok, "params field should exist and be a map")

	denomCreationFee, ok := params["denom_creation_fee"].([]interface{})
	s.Require().True(ok, "denom_creation_fee should be a list")
	s.Require().Len(denomCreationFee, 1, "denom_creation_fee should have one entry")

	feeEntry, ok := denomCreationFee[0].(map[string]interface{})
	s.Require().True(ok, "fee entry should be a map")
	s.Equal(types.DefaultBondDenom, feeEntry["denom"])
	s.Equal("10000000", feeEntry["amount"])

	s.Equal("2000000", params["denom_creation_gas_consume"])

	n.WaitForNextBlock()

	// TODO: Add more functionality checks here as they are added
}

func (s *SoftwareUpgradeV2TestSuite) preUpgradeCreateFp1(n *chain.NodeConfig) {
	s.fp1Addr = n.KeysAdd(wFp1)
	n.BankSendFromNode(s.fp1Addr, "1000000ubbn")
	n.WaitForNextBlock()

	s.fp1 = CreateNodeFP(
		s.T(),
		s.r,
		s.fp1BTCSK,
		n,
		s.fp1Addr,
		"",
	)
	s.NotNil(s.fp1)

	actualFps := n.QueryFinalityProviders()
	s.Len(actualFps, 1)
}

func (s *SoftwareUpgradeV2TestSuite) preUpgradeCreateBtcDels(n *chain.NodeConfig) {
	s.del1Addr = n.KeysAdd(wDel1)
	s.del2Addr = n.KeysAdd(wDel2)

	n.BankMultiSendFromNode([]string{s.del1Addr, s.del2Addr}, "1000000ubbn")

	n.WaitForNextBlock()

	// fp1Del1
	n.CreateBTCDel(s.r, s.T(), s.net, wDel1, s.fp1, s.del1BTCSK, s.del1Addr, stakingTimeBlocks, s.fp1Del1StakingAmt, "")
	// fp1Del2
	n.CreateBTCDel(s.r, s.T(), s.net, wDel2, s.fp1, s.del2BTCSK, s.del2Addr, stakingTimeBlocks, s.fp1Del2StakingAmt, "")

	n.WaitForNextBlocks(2)
	resp := n.QueryBtcDelegations(bstypes.BTCDelegationStatus_ANY)
	require.Len(s.T(), resp.BtcDelegations, 2)
}

func (s *SoftwareUpgradeV2TestSuite) preUpgradeSubmitCovdSigs(n *chain.NodeConfig) {
	params := n.QueryBTCStakingParams()

	covAddrs := make([]string, params.CovenantQuorum)
	covWallets := make([]string, params.CovenantQuorum)
	for i := 0; i < int(params.CovenantQuorum); i++ {
		covWallet := fmt.Sprintf("cov%d", i)
		covWallets[i] = covWallet
		covAddrs[i] = n.KeysAdd(covWallet)
	}
	s.covenantWallets = covWallets

	n.BankMultiSendFromNode(covAddrs, "1000000ubbn")

	// tx bank send needs to take effect
	n.WaitForNextBlock()

	AddCovdSigsToPendingBtcDels(s.r, s.T(), n, s.net, params, s.covenantSKs, s.covenantWallets, s.fp1.BtcPk.MarshalHex())

	// ensure the BTC delegation has covenant sigs now
	activeDelsSet := AllBtcDelsActive(s.T(), n, s.fp1.BtcPk.MarshalHex())
	s.Require().Len(activeDelsSet, 2)
}

func (s *SoftwareUpgradeV2TestSuite) preUpgradeAddFinalitySigs(n *chain.NodeConfig) {
	// commit public randomness list
	commitStartHeight := uint64(5)

	fp1RandListInfo, fp1CommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(s.r, s.fp1BTCSK, "", commitStartHeight, numPubRand)
	s.NoError(err)
	s.fp1RandListInfo = fp1RandListInfo

	n.CommitPubRandList(
		fp1CommitPubRandList.FpBtcPk,
		fp1CommitPubRandList.StartHeight,
		fp1CommitPubRandList.NumPubRand,
		fp1CommitPubRandList.Commitment,
		fp1CommitPubRandList.Sig,
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

	finalizedEpoch := n.WaitUntilCurrentEpochIsSealedAndFinalized(1)
	s.Require().GreaterOrEqual(finalizedEpoch, fp1PubRand.EpochNum)

	fps := n.QueryFinalityProviders()
	s.Require().Len(fps, 1)

	fp := fps[0]
	s.Require().False(fp.Jailed, "fp is jailed")
	s.Require().Zero(fp.SlashedBabylonHeight, "fp is slashed")
	fpDels := n.QueryFinalityProviderDelegations(fp.BtcPk.MarshalHex())
	s.Require().Len(fpDels, 2)
	del1BtcPk := bbn.NewBIP340PubKeyFromBTCPK(s.del1BTCSK.PubKey())
	del2BtcPk := bbn.NewBIP340PubKeyFromBTCPK(s.del2BTCSK.PubKey())

	for _, fpDelStaker := range fpDels {
		for _, fpDel := range fpDelStaker.Dels {
			s.Require().True(fpDel.Active)

			if strings.EqualFold(del1BtcPk.MarshalHex(), fpDel.BtcPk.MarshalHex()) {
				s.Require().GreaterOrEqual(fpDel.TotalSat, uint64(s.fp1Del1StakingAmt))
				continue
			}

			if strings.EqualFold(del2BtcPk.MarshalHex(), fpDel.BtcPk.MarshalHex()) {
				s.Require().GreaterOrEqual(fpDel.TotalSat, uint64(s.fp1Del2StakingAmt))
				continue
			}

			s.FailNow("found a weird delegation")
		}
	}

	s.finalityBlockHeightVoted = n.WaitFinalityIsActivated()

	// submit finality signature
	s.finalityIdx = s.finalityBlockHeightVoted - commitStartHeight

	n.WaitForNextBlockWithSleep50ms()

	// pre-upgrade there is no context, so we pass an empty string
	appHash := n.AddFinalitySignatureToBlock(
		s.fp1BTCSK,
		s.fp1.BtcPk,
		s.finalityBlockHeightVoted,
		s.fp1RandListInfo.SRList[s.finalityIdx],
		&s.fp1RandListInfo.PRList[s.finalityIdx],
		*s.fp1RandListInfo.ProofList[s.finalityIdx].ToProto(),
		"",
		fmt.Sprintf("--from=%s", wFp1),
	)

	n.WaitForNextBlockWithSleep50ms()

	var finalizedBlocks []*ftypes.IndexedBlock
	s.Eventually(func() bool {
		finalizedBlocks = n.QueryListBlocks(ftypes.QueriedBlockStatus_FINALIZED)
		return len(finalizedBlocks) > 0
	}, time.Minute, time.Millisecond*50)

	s.Equal(s.finalityBlockHeightVoted, finalizedBlocks[0].Height)
	s.Equal(appHash.Bytes(), finalizedBlocks[0].AppHash)
	s.T().Logf("the block %d is finalized", s.finalityBlockHeightVoted)
}

func (s *SoftwareUpgradeV2TestSuite) preUpgradeWithdrawRewardsBtcDel(n *chain.NodeConfig) {
	// Current setup of voting power
	// (fp1, del1) => 2_00000000
	// (fp1, del2) => 4_00000000

	// The sum per bech32 address will be
	// (fp1)  => 6_00000000
	// (del1) => 2_00000000
	// (del2) => 4_00000000

	// verifies that everyone is active and not slashed
	fps := n.QueryFinalityProviders()
	s.Len(fps, 1)
	fp := fps[0]
	s.Equal(fp.SlashedBabylonHeight, uint64(0))
	s.Equal(fp.SlashedBtcHeight, uint32(0))

	dels := n.QueryFinalityProvidersDelegations(s.fp1.BtcPk.MarshalHex())
	s.Len(dels, 2)
	for _, del := range dels {
		s.True(del.Active)
	}

	// makes sure there is some reward there
	s.Eventually(func() bool {
		_, errFp1 := n.QueryRewardGauge(s.fp1.Address())
		_, errDel1 := n.QueryRewardGauge(sdk.MustAccAddressFromBech32(s.del1Addr))
		_, errDel2 := n.QueryRewardGauge(sdk.MustAccAddressFromBech32(s.del2Addr))
		return errFp1 == nil && errDel1 == nil && errDel2 == nil
	}, time.Minute*4, time.Second*3, "wait to have some rewards available in the gauge")

	_, del1DiffRewards, del2DiffRewards := s.QueryRewardGauges(n)

	// The rewards distributed to the delegators should be 1x for del1 and 2x for del2
	// to compare the rewards, duplicates the amount of rewards in del1 to check with del2
	coins.RequireCoinsDiffInPointOnePercentMargin(s.T(), del1DiffRewards.Coins.MulInt(sdkmath.NewIntFromUint64(2)), del2DiffRewards.Coins)

	// withdraw both BTC del rewards
	CheckWithdrawReward(s.T(), n, wDel1, s.del1Addr)
	CheckWithdrawReward(s.T(), n, wDel2, s.del2Addr)

	s.AddFinalityVoteUntilCurrentHeight(n)
}

// QueryRewardGauges returns the rewards available for fp1, fp2, del1, del2
func (s *SoftwareUpgradeV2TestSuite) QueryRewardGauges(n *chain.NodeConfig) (
	fp1, del1, del2 *itypes.RewardGaugesResponse,
) {
	n.WaitForNextBlockWithSleep50ms()

	g := new(errgroup.Group)
	var (
		err                 error
		fp1RewardGauges     map[string]*itypes.RewardGaugesResponse
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

	btcDel1RewardGauge, ok := btcDel1RewardGauges[itypes.BTC_STAKER.String()]
	s.True(ok)
	s.True(btcDel1RewardGauge.Coins.IsAllPositive())

	btcDel2RewardGauge, ok := btcDel2RewardGauges[itypes.BTC_STAKER.String()]
	s.True(ok)
	s.True(btcDel2RewardGauge.Coins.IsAllPositive())

	s.T().Logf("query reward: fp1 - %s", fp1RewardGauge.Coins.String())
	s.T().Logf("query reward: del1 - %s", btcDel1RewardGauge.Coins.String())
	s.T().Logf("query reward: del2 - %s", btcDel2RewardGauge.Coins.String())
	return fp1RewardGauge, btcDel1RewardGauge, btcDel2RewardGauge
}

func (s *SoftwareUpgradeV2TestSuite) GetRewardDifferences(blocksDiff uint64) (
	fp1DiffRewards, del1DiffRewards, del2DiffRewards sdk.Coins,
) {
	chainA := s.configurer.GetChainConfig(0)
	n1, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)

	fp1RewardGaugePrev, btcDel1RewardGaugePrev, btcDel2RewardGaugePrev := s.QueryRewardGauges(n1)
	// wait a few block of rewards to calculate the difference
	for i := 1; i <= int(blocksDiff); i++ {
		if i%2 == 0 {
			s.AddFinalityVoteUntilCurrentHeight(n1)
		}
		n1.WaitForNextBlock()
	}

	fp1RewardGauge, btcDel1RewardGauge, btcDel2RewardGauge := s.QueryRewardGauges(n1)

	// since varius block were created, it is needed to get the difference
	// from a certain point where all the delegations were active to properly
	// calculate the distribution with the voting power structure with 4 BTC delegations active
	// Note: if a new block is mined during the query of reward gauges, the calculation might be a
	// bit off by some ubbn
	fp1DiffRewards = fp1RewardGauge.Coins.Sub(fp1RewardGaugePrev.Coins...)
	del1DiffRewards = btcDel1RewardGauge.Coins.Sub(btcDel1RewardGaugePrev.Coins...)
	del2DiffRewards = btcDel2RewardGauge.Coins.Sub(btcDel2RewardGaugePrev.Coins...)

	s.AddFinalityVoteUntilCurrentHeight(n1)
	return fp1DiffRewards, del1DiffRewards, del2DiffRewards
}

func (s *SoftwareUpgradeV2TestSuite) AddFinalityVoteUntilCurrentHeight(n *chain.NodeConfig) {
	currentBlock := n.LatestBlockNumber()

	accN1, err := n.QueryAccount(s.fp1.Addr)
	s.NoError(err)

	accNumberN1 := accN1.GetAccountNumber()
	accSequenceN1 := accN1.GetSequence()

	for s.finalityBlockHeightVoted < currentBlock {
		n1Flags := []string{
			"--offline",
			fmt.Sprintf("--account-number=%d", accNumberN1),
			fmt.Sprintf("--sequence=%d", accSequenceN1),
			fmt.Sprintf("--from=%s", wFp1),
		}
		s.AddFinalityVote(n, n1Flags)

		accSequenceN1++
	}
}

func (s *SoftwareUpgradeV2TestSuite) AddFinalityVote(n *chain.NodeConfig, flagsFp1 []string) (appHash bytes.HexBytes) {
	s.finalityIdx++
	s.finalityBlockHeightVoted++

	appHash = n.AddFinalitySignatureToBlock(
		s.fp1BTCSK,
		s.fp1.BtcPk,
		s.finalityBlockHeightVoted,
		s.fp1RandListInfo.SRList[s.finalityIdx],
		&s.fp1RandListInfo.PRList[s.finalityIdx],
		*s.fp1RandListInfo.ProofList[s.finalityIdx].ToProto(),
		signingcontext.FpFinVoteContextV0(n.ChainID(), appparams.AccFinality.String()),
		flagsFp1...,
	)

	return appHash
}

// Test2CheckRewardsAfterUpgrade verifies the rewards of all the delegations
// and finality provider
func (s *SoftwareUpgradeV2TestSuite) Test2CheckRewardsAfterUpgrade() {
	n1, err := s.configurer.GetChainConfig(0).GetNodeAtIndex(1)
	s.NoError(err)

	n1.WaitForNextBlock()
	s.AddFinalityVoteUntilCurrentHeight(n1)

	// Current setup of voting power
	// (fp1, del1) => 2_00000000
	// (fp1, del2) => 4_00000000

	// The sum per bech32 address will be
	// (fp1)  => 6_00000000
	// (del1) => 2_00000000
	// (del2) => 4_00000000

	// gets the difference in rewards in 4 blocks range
	fp1DiffRewards, del1DiffRewards, del2DiffRewards := s.GetRewardDifferences(4)

	// Check the difference in the delegators
	// the del1 should receive ~50% of the rewards received by del2
	expectedRwdDel1 := coins.CalculatePercentageOfCoins(del2DiffRewards, 50)
	coins.RequireCoinsDiffInPointOnePercentMargin(s.T(), del1DiffRewards, expectedRwdDel1)

	fp1DiffRewardsStr := fp1DiffRewards.String()
	del1DiffRewardsStr := del1DiffRewards.String()
	del2DiffRewardsStr := del2DiffRewards.String()

	s.NotEmpty(fp1DiffRewardsStr)
	s.NotEmpty(del1DiffRewardsStr)
	s.NotEmpty(del2DiffRewardsStr)

	// withdraw the rewards
	CheckWithdrawReward(s.T(), n1, wDel1, s.del1Addr)
	CheckWithdrawReward(s.T(), n1, wDel2, s.del2Addr)
}
