package e2e

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cometbft/cometbft/libs/bytes"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sync/errgroup"

	"github.com/babylonlabs-io/babylon/v4/crypto/eots"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v4/testutil/coins"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
	itypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

const (
	stakingTimeBlocks = uint16(math.MaxUint16)
	wDel1             = "del1"
	wDel2             = "del2"
	wFp1              = "fp1"
	wFp2              = "fp2"
	numPubRand        = uint64(600)
)

type BtcRewardsDistribution struct {
	suite.Suite

	r   *rand.Rand
	net *chaincfg.Params

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

	// The lastet delegation will stake 6_00000000 to (fp2, del2).
	// Since the rewards are combined by their bech32 address, del2
	// will have 10_00000000 and del1 will have 4_00000000 as voting power,
	// meaning that del1 will receive only 40% of the amount of rewards
	// that del2 will receive once every delegation is active and blocks
	// are being rewarded.
	fp2Del2StakingAmt int64

	// bech32 address of the delegators
	del1Addr string
	del2Addr string
	// bech32 address of the finality providers
	fp1Addr string
	fp2Addr string

	// covenant helpers
	covenantSKs     []*btcec.PrivateKey
	covenantWallets []string

	// finality helpers
	finalityIdx              uint64
	finalityBlockHeightVoted uint64
	fp1RandListInfo          *datagen.RandListInfo
	fp2RandListInfo          *datagen.RandListInfo

	configurer configurer.Configurer
}

func (s *BtcRewardsDistribution) SetupSuite() {
	s.T().Log("setting up e2e integration test suite...")
	var err error

	s.r = rand.New(rand.NewSource(time.Now().Unix()))
	s.net = &chaincfg.SimNetParams
	s.fp1BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.fp2BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.del1BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.del2BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)

	s.fp1Del1StakingAmt = int64(2 * 10e8)
	s.fp1Del2StakingAmt = int64(4 * 10e8)
	s.fp2Del1StakingAmt = int64(2 * 10e8)
	s.fp2Del2StakingAmt = int64(6 * 10e8)

	covenantSKs, _, _ := bstypes.DefaultCovenantCommittee()
	s.covenantSKs = covenantSKs

	s.configurer, err = configurer.NewBTCStakingConfigurer(s.T(), true)
	s.NoError(err)
	err = s.configurer.ConfigureChains()
	s.NoError(err)
	err = s.configurer.RunSetup()
	s.NoError(err)
}

// Test1CreateFinalityProviders creates all finality providers
func (s *BtcRewardsDistribution) Test1CreateFinalityProviders() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(2)

	n1, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)
	n2, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	n1.WaitForNextBlock()
	n2.WaitForNextBlock()

	s.fp1Addr = n1.KeysAdd(wFp1)
	s.fp2Addr = n2.KeysAdd(wFp2)

	n2.BankMultiSendFromNode([]string{s.fp1Addr, s.fp2Addr}, "1000000ubbn")

	n2.WaitForNextBlock()

	s.fp1 = CreateNodeFP(
		s.T(),
		s.r,
		s.fp1BTCSK,
		n1,
		s.fp1Addr,
	)
	s.NotNil(s.fp1)

	s.fp2 = CreateNodeFP(
		s.T(),
		s.r,
		s.fp2BTCSK,
		n2,
		s.fp2Addr,
	)
	s.NotNil(s.fp2)

	actualFps := n2.QueryFinalityProviders()
	s.Len(actualFps, 2)
}

// Test2CreateFinalityProviders creates the first 3 btc delegations
// with the same values, but different satoshi staked amounts
func (s *BtcRewardsDistribution) Test2CreateFirstBtcDelegations() {
	n2, err := s.configurer.GetChainConfig(0).GetNodeAtIndex(2)
	s.NoError(err)

	s.del1Addr = n2.KeysAdd(wDel1)
	s.del2Addr = n2.KeysAdd(wDel2)

	n2.BankMultiSendFromNode([]string{s.del1Addr, s.del2Addr}, "1000000ubbn")

	n2.WaitForNextBlock()

	// fp1Del1
	s.CreateBTCDelegationAndCheck(n2, wDel1, s.fp1, s.del1BTCSK, s.del1Addr, s.fp1Del1StakingAmt)
	// fp1Del2
	s.CreateBTCDelegationAndCheck(n2, wDel2, s.fp1, s.del2BTCSK, s.del2Addr, s.fp1Del2StakingAmt)
	// fp2Del1
	s.CreateBTCDelegationAndCheck(n2, wDel1, s.fp2, s.del1BTCSK, s.del1Addr, s.fp2Del1StakingAmt)

	resp := n2.QueryBtcDelegations(bstypes.BTCDelegationStatus_ANY)
	require.Len(s.T(), resp.BtcDelegations, 3)
}

// Test3SubmitCovenantSignature covenant approves all the 3 BTC delegation
func (s *BtcRewardsDistribution) Test3SubmitCovenantSignature() {
	n1, err := s.configurer.GetChainConfig(0).GetNodeAtIndex(1)
	s.NoError(err)

	params := n1.QueryBTCStakingParams()

	covAddrs := make([]string, params.CovenantQuorum)
	covWallets := make([]string, params.CovenantQuorum)
	for i := 0; i < int(params.CovenantQuorum); i++ {
		covWallet := fmt.Sprintf("cov%d", i)
		covWallets[i] = covWallet
		covAddrs[i] = n1.KeysAdd(covWallet)
	}
	s.covenantWallets = covWallets

	n1.BankMultiSendFromNode(covAddrs, "1000000ubbn")

	// tx bank send needs to take effect
	n1.WaitForNextBlock()

	AddCovdSigsToPendingBtcDels(s.r, s.T(), n1, s.net, params, s.covenantSKs, s.covenantWallets, s.fp1.BtcPk.MarshalHex(), s.fp2.BtcPk.MarshalHex())

	// ensure the BTC delegation has covenant sigs now
	activeDelsSet := AllBtcDelsActive(s.T(), n1, s.fp1.BtcPk.MarshalHex(), s.fp2.BtcPk.MarshalHex())
	s.Require().Len(activeDelsSet, 3)
}

// Test4CommitPublicRandomnessAndSealed commits public randomness for
// each finality provider and seals the epoch.
func (s *BtcRewardsDistribution) Test4CommitPublicRandomnessAndSealed() {
	chainA := s.configurer.GetChainConfig(0)
	n1, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)
	n2, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	// commit public randomness list
	commitStartHeight := uint64(5)

	fp1RandListInfo, fp1CommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(s.r, s.fp1BTCSK, commitStartHeight, numPubRand)
	s.NoError(err)
	s.fp1RandListInfo = fp1RandListInfo

	fp2RandListInfo, fp2CommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(s.r, s.fp2BTCSK, commitStartHeight, numPubRand)
	s.NoError(err)
	s.fp2RandListInfo = fp2RandListInfo

	n1.CommitPubRandList(
		fp1CommitPubRandList.FpBtcPk,
		fp1CommitPubRandList.StartHeight,
		fp1CommitPubRandList.NumPubRand,
		fp1CommitPubRandList.Commitment,
		fp1CommitPubRandList.Sig,
	)

	n2.CommitPubRandList(
		fp2CommitPubRandList.FpBtcPk,
		fp2CommitPubRandList.StartHeight,
		fp2CommitPubRandList.NumPubRand,
		fp2CommitPubRandList.Commitment,
		fp2CommitPubRandList.Sig,
	)

	// needs to wait for a block to make sure the pub rand is committed
	// prior to epoch finalization
	n2.WaitForNextBlockWithSleep50ms()

	// check all FPs requirement to be active
	// TotalBondedSat > 0
	// IsTimestamped
	// !IsJailed
	// !IsSlashed

	fp1CommitPubRand := n1.QueryListPubRandCommit(fp1CommitPubRandList.FpBtcPk)
	fp1PubRand := fp1CommitPubRand[commitStartHeight]
	s.Require().Equal(fp1PubRand.NumPubRand, numPubRand)

	fp2CommitPubRand := n2.QueryListPubRandCommit(fp2CommitPubRandList.FpBtcPk)
	fp2PubRand := fp2CommitPubRand[commitStartHeight]
	s.Require().Equal(fp2PubRand.NumPubRand, numPubRand)

	finalizedEpoch := n1.WaitUntilCurrentEpochIsSealedAndFinalized(1)
	s.Require().GreaterOrEqual(finalizedEpoch, fp1PubRand.EpochNum)
	s.Require().GreaterOrEqual(finalizedEpoch, fp2PubRand.EpochNum)

	fps := n2.QueryFinalityProviders()
	s.Require().Len(fps, 2)
	for _, fp := range fps {
		s.Require().False(fp.Jailed, "fp is jailed")
		s.Require().Zero(fp.SlashedBabylonHeight, "fp is slashed")
		fpDels := n2.QueryFinalityProviderDelegations(fp.BtcPk.MarshalHex())
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

	s.finalityBlockHeightVoted = n1.WaitFinalityIsActivated()

	// submit finality signature
	s.finalityIdx = s.finalityBlockHeightVoted - commitStartHeight

	n1.WaitForNextBlockWithSleep50ms()
	var (
		wg      sync.WaitGroup
		appHash bytes.HexBytes
	)
	wg.Add(2)

	go func() {
		defer wg.Done()
		appHash = n1.AddFinalitySignatureToBlock(
			s.fp1BTCSK,
			s.fp1.BtcPk,
			s.finalityBlockHeightVoted,
			s.fp1RandListInfo.SRList[s.finalityIdx],
			&s.fp1RandListInfo.PRList[s.finalityIdx],
			*s.fp1RandListInfo.ProofList[s.finalityIdx].ToProto(),
			fmt.Sprintf("--from=%s", wFp1),
		)
	}()

	go func() {
		defer wg.Done()
		n2.AddFinalitySignatureToBlock(
			s.fp2BTCSK,
			s.fp2.BtcPk,
			s.finalityBlockHeightVoted,
			s.fp2RandListInfo.SRList[s.finalityIdx],
			&s.fp2RandListInfo.PRList[s.finalityIdx],
			*s.fp2RandListInfo.ProofList[s.finalityIdx].ToProto(),
			fmt.Sprintf("--from=%s", wFp2),
		)
	}()

	wg.Wait()
	n1.WaitForNextBlockWithSleep50ms()

	// ensure vote is eventually cast
	var finalizedBlocks []*ftypes.IndexedBlock
	s.Eventually(func() bool {
		finalizedBlocks = n1.QueryListBlocks(ftypes.QueriedBlockStatus_FINALIZED)
		return len(finalizedBlocks) > 0
	}, time.Minute, time.Millisecond*50)

	s.Equal(s.finalityBlockHeightVoted, finalizedBlocks[0].Height)
	s.Equal(appHash.Bytes(), finalizedBlocks[0].AppHash)
	s.T().Logf("the block %d is finalized", s.finalityBlockHeightVoted)
}

// Test5CheckRewardsFirstDelegations verifies the rewards independent of mint amounts
// There might be a difference in rewards if the BTC delegations were included in different blocks
// that is the reason to get the difference in rewards between a block range to assert
// the reward difference between fps and delegators.
func (s *BtcRewardsDistribution) Test5CheckRewardsFirstDelegations() {
	n2, err := s.configurer.GetChainConfig(0).GetNodeAtIndex(2)
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

	// verifies that everyone is active and not slashed
	fps := n2.QueryFinalityProviders()
	s.Len(fps, 2)
	s.Equal(fps[0].Commission.String(), fps[1].Commission.String())
	for _, fp := range fps {
		s.Equal(fp.SlashedBabylonHeight, uint64(0))
		s.Equal(fp.SlashedBtcHeight, uint32(0))
	}

	dels := n2.QueryFinalityProvidersDelegations(s.fp1.BtcPk.MarshalHex(), s.fp2.BtcPk.MarshalHex())
	s.Len(dels, 3)
	for _, del := range dels {
		s.True(del.Active)
	}

	// makes sure there is some reward there
	s.Eventually(func() bool {
		_, errFp1 := n2.QueryRewardGauge(s.fp1.Address())
		_, errFp2 := n2.QueryRewardGauge(s.fp2.Address())
		_, errDel1 := n2.QueryRewardGauge(sdk.MustAccAddressFromBech32(s.del1Addr))
		_, errDel2 := n2.QueryRewardGauge(sdk.MustAccAddressFromBech32(s.del2Addr))
		return errFp1 == nil && errFp2 == nil && errDel1 == nil && errDel2 == nil
	}, time.Minute*4, time.Second*3, "wait to have some rewards available in the gauge")

	// The rewards distributed for the finality providers should be fp1 => 3x, fp2 => 1x
	fp1DiffRewards, fp2DiffRewards, del1DiffRewards, del2DiffRewards := s.QueryRewardGauges(n2)
	s.AddFinalityVoteUntilCurrentHeight()

	coins.RequireCoinsDiffInPointOnePercentMargin(
		s.T(),
		fp2DiffRewards.Coins.MulInt(sdkmath.NewIntFromUint64(3)),
		fp1DiffRewards.Coins,
	)

	// The rewards distributed to the delegators should be the same for each delegator
	coins.RequireCoinsDiffInPointOnePercentMargin(s.T(), del1DiffRewards.Coins, del2DiffRewards.Coins)

	CheckWithdrawReward(s.T(), n2, wDel2, s.del2Addr)

	s.AddFinalityVoteUntilCurrentHeight()
}

// Test6ActiveLastDelegation creates a new btc delegation
// (fp2, del2) with 6_00000000 sats and sends the covenant signatures
// needed.
func (s *BtcRewardsDistribution) Test6ActiveLastDelegation() {
	chainA := s.configurer.GetChainConfig(0)
	n2, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)
	// covenants are at n1
	n1, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)

	// fp2Del2
	s.CreateBTCDelegationAndCheck(n2, wDel2, s.fp2, s.del2BTCSK, s.del2Addr, s.fp2Del2StakingAmt)

	s.AddFinalityVoteUntilCurrentHeight()

	allDelegations := n2.QueryFinalityProvidersDelegations(s.fp1.BtcPk.MarshalHex(), s.fp2.BtcPk.MarshalHex())
	s.Equal(len(allDelegations), 4)

	pendingDels := make([]*bstypes.BTCDelegationResponse, 0)
	for _, delegation := range allDelegations {
		if !strings.EqualFold(delegation.StatusDesc, bstypes.BTCDelegationStatus_PENDING.String()) {
			continue
		}
		pendingDels = append(pendingDels, delegation)
	}

	s.Equal(len(pendingDels), 1)
	pendingDel, err := chain.ParseRespBTCDelToBTCDel(pendingDels[0])
	s.NoError(err)

	SendCovenantSigsToPendingDel(s.r, s.T(), n1, s.net, s.covenantSKs, s.covenantWallets, pendingDel)

	// wait for a block so that covenant txs take effect
	n1.WaitForNextBlock()

	s.AddFinalityVoteUntilCurrentHeight()

	// ensure that all BTC delegation are active
	allDelegations = n1.QueryFinalityProvidersDelegations(s.fp1.BtcPk.MarshalHex(), s.fp2.BtcPk.MarshalHex())
	s.Len(allDelegations, 4)
	for _, activeDel := range allDelegations {
		s.True(activeDel.Active)
	}
}

// Test7CheckRewards verifies the rewards of all the delegations
// and finality provider
func (s *BtcRewardsDistribution) Test7CheckRewards() {
	n2, err := s.configurer.GetChainConfig(0).GetNodeAtIndex(2)
	s.NoError(err)

	n2.WaitForNextBlock()
	s.AddFinalityVoteUntilCurrentHeight()

	// Current setup of voting power
	// (fp1, del1) => 2_00000000
	// (fp1, del2) => 4_00000000
	// (fp2, del1) => 2_00000000
	// (fp2, del2) => 6_00000000

	// The sum per bech32 address will be
	// (fp1)  => 6_00000000
	// (fp2)  => 8_00000000
	// (del1) => 4_00000000
	// (del2) => 10_00000000

	// gets the difference in rewards in 4 blocks range
	fp1DiffRewards, fp2DiffRewards, del1DiffRewards, del2DiffRewards := s.GetRewardDifferences(4)

	// Check the difference in the finality providers
	// fp1 should receive ~75% of the rewards received by fp2
	expectedRwdFp1 := coins.CalculatePercentageOfCoins(fp2DiffRewards, 75)
	coins.RequireCoinsDiffInPointOnePercentMargin(s.T(), fp1DiffRewards, expectedRwdFp1)

	// Check the difference in the delegators
	// the del1 should receive ~40% of the rewards received by del2
	expectedRwdDel1 := coins.CalculatePercentageOfCoins(del2DiffRewards, 40)
	coins.RequireCoinsDiffInPointOnePercentMargin(s.T(), del1DiffRewards, expectedRwdDel1)

	fp1DiffRewardsStr := fp1DiffRewards.String()
	fp2DiffRewardsStr := fp2DiffRewards.String()
	del1DiffRewardsStr := del1DiffRewards.String()
	del2DiffRewardsStr := del2DiffRewards.String()

	s.NotEmpty(fp1DiffRewardsStr)
	s.NotEmpty(fp2DiffRewardsStr)
	s.NotEmpty(del1DiffRewardsStr)
	s.NotEmpty(del2DiffRewardsStr)
}

// Test8SlashFp slashes the finality provider, but should continue to produce blocks
func (s *BtcRewardsDistribution) Test8SlashFp() {
	chainA := s.configurer.GetChainConfig(0)
	n2, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	badBlockHeightToVote := s.finalityBlockHeightVoted + 1

	blockToVote, err := n2.QueryBlock(int64(badBlockHeightToVote))
	s.NoError(err)
	appHash := blockToVote.AppHash

	// generate bad EOTS signature with a diff block height to vote
	msgToSign := append(sdk.Uint64ToBigEndian(s.finalityBlockHeightVoted), appHash...)
	fp1Sig, err := eots.Sign(s.fp2BTCSK, s.fp2RandListInfo.SRList[s.finalityIdx], msgToSign)
	s.NoError(err)

	finalitySig := bbn.NewSchnorrEOTSSigFromModNScalar(fp1Sig)

	// submit finality signature to slash
	n2.AddFinalitySigFromVal(
		s.fp2.BtcPk,
		s.finalityBlockHeightVoted,
		&s.fp2RandListInfo.PRList[s.finalityIdx],
		*s.fp2RandListInfo.ProofList[s.finalityIdx].ToProto(),
		appHash,
		finalitySig,
	)

	n2.WaitForNextBlocks(2)

	fps := n2.QueryFinalityProviders()
	require.Len(s.T(), fps, 2)
	for _, fp := range fps {
		if strings.EqualFold(fp.Addr, s.fp1Addr) {
			require.Zero(s.T(), fp.SlashedBabylonHeight)
			continue
		}
		require.NotZero(s.T(), fp.SlashedBabylonHeight)
	}

	// wait a few blocks to check if it doesn't panic when rewards are being produced
	n2.WaitForNextBlocks(5)
}

func (s *BtcRewardsDistribution) GetRewardDifferences(blocksDiff uint64) (
	fp1DiffRewards, fp2DiffRewards, del1DiffRewards, del2DiffRewards sdk.Coins,
) {
	chainA := s.configurer.GetChainConfig(0)
	n2, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	fp1RewardGaugePrev, fp2RewardGaugePrev, btcDel1RewardGaugePrev, btcDel2RewardGaugePrev := s.QueryRewardGauges(n2)
	// wait a few block of rewards to calculate the difference
	for i := 1; i <= int(blocksDiff); i++ {
		if i%2 == 0 {
			s.AddFinalityVoteUntilCurrentHeight()
		}
		n2.WaitForNextBlock()
	}

	fp1RewardGauge, fp2RewardGauge, btcDel1RewardGauge, btcDel2RewardGauge := s.QueryRewardGauges(n2)

	// since varius block were created, it is needed to get the difference
	// from a certain point where all the delegations were active to properly
	// calculate the distribution with the voting power structure with 4 BTC delegations active
	// Note: if a new block is mined during the query of reward gauges, the calculation might be a
	// bit off by some ubbn
	fp1DiffRewards = fp1RewardGauge.Coins.Sub(fp1RewardGaugePrev.Coins...)
	fp2DiffRewards = fp2RewardGauge.Coins.Sub(fp2RewardGaugePrev.Coins...)
	del1DiffRewards = btcDel1RewardGauge.Coins.Sub(btcDel1RewardGaugePrev.Coins...)
	del2DiffRewards = btcDel2RewardGauge.Coins.Sub(btcDel2RewardGaugePrev.Coins...)

	s.AddFinalityVoteUntilCurrentHeight()
	return fp1DiffRewards, fp2DiffRewards, del1DiffRewards, del2DiffRewards
}

func (s *BtcRewardsDistribution) AddFinalityVoteUntilCurrentHeight() {
	chainA := s.configurer.GetChainConfig(0)
	n1, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)
	n2, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	currentBlock := n2.LatestBlockNumber()

	accN1, err := n1.QueryAccount(s.fp1.Addr)
	s.NoError(err)
	accN2, err := n1.QueryAccount(s.fp2.Addr)
	s.NoError(err)

	accNumberN1 := accN1.GetAccountNumber()
	accSequenceN1 := accN1.GetSequence()

	accNumberN2 := accN2.GetAccountNumber()
	accSequenceN2 := accN2.GetSequence()

	for s.finalityBlockHeightVoted < currentBlock {
		n1Flags := []string{
			"--offline",
			fmt.Sprintf("--account-number=%d", accNumberN1),
			fmt.Sprintf("--sequence=%d", accSequenceN1),
			fmt.Sprintf("--from=%s", wFp1),
		}
		n2Flags := []string{
			"--offline",
			fmt.Sprintf("--account-number=%d", accNumberN2),
			fmt.Sprintf("--sequence=%d", accSequenceN2),
			fmt.Sprintf("--from=%s", wFp2),
		}
		s.AddFinalityVote(n1Flags, n2Flags)

		accSequenceN1++
		accSequenceN2++
	}
}

func (s *BtcRewardsDistribution) AddFinalityVote(flagsN1, flagsN2 []string) (appHash bytes.HexBytes) {
	chainA := s.configurer.GetChainConfig(0)
	n2, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)
	n1, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)

	s.finalityIdx++
	s.finalityBlockHeightVoted++

	appHash = n1.AddFinalitySignatureToBlock(
		s.fp1BTCSK,
		s.fp1.BtcPk,
		s.finalityBlockHeightVoted,
		s.fp1RandListInfo.SRList[s.finalityIdx],
		&s.fp1RandListInfo.PRList[s.finalityIdx],
		*s.fp1RandListInfo.ProofList[s.finalityIdx].ToProto(),
		flagsN1...,
	)

	n2.AddFinalitySignatureToBlock(
		s.fp2BTCSK,
		s.fp2.BtcPk,
		s.finalityBlockHeightVoted,
		s.fp2RandListInfo.SRList[s.finalityIdx],
		&s.fp2RandListInfo.PRList[s.finalityIdx],
		*s.fp2RandListInfo.ProofList[s.finalityIdx].ToProto(),
		flagsN2...,
	)

	return appHash
}

// QueryRewardGauges returns the rewards available for fp1, fp2, del1, del2
func (s *BtcRewardsDistribution) QueryRewardGauges(n *chain.NodeConfig) (
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

func (s *BtcRewardsDistribution) CreateBTCDelegationAndCheck(
	n *chain.NodeConfig,
	wDel string,
	fp *bstypes.FinalityProvider,
	btcStakerSK *btcec.PrivateKey,
	delAddr string,
	stakingSatAmt int64,
) {
	n.CreateBTCDelegationAndCheck(s.r, s.T(), s.net, wDel, fp, btcStakerSK, delAddr, stakingTimeBlocks, stakingSatAmt)
}

// CheckWithdrawReward withdraw rewards for one delegation and check the balance
func CheckWithdrawReward(
	t testing.TB,
	n *chain.NodeConfig,
	delWallet, delAddr string,
) {
	accDelAddr := sdk.MustAccAddressFromBech32(delAddr)
	n.WaitForNextBlockWithSleep50ms()

	delRwdGaugeBefore, errRwdGauge := n.QueryRewardGauge(accDelAddr)
	require.NoError(t, errRwdGauge)

	delBalanceBeforeWithdraw, err := n.QueryBalances(delAddr)
	require.NoError(t, err)

	txHash := n.WithdrawReward(itypes.BTC_STAKER.String(), delWallet)

	n.WaitForNextBlock()

	_, txResp := n.QueryTx(txHash)

	delRwdGaugeAfter, errRwdGauge := n.QueryRewardGauge(accDelAddr)
	require.NoError(t, errRwdGauge)

	delBalanceAfterWithdraw, err := n.QueryBalances(delAddr)
	require.NoError(t, err)

	// note that the rewards might not be precise as more or less blocks were produced and given out rewards
	// while the query balance / withdraw / query gauge was running
	delRewardGaugeBefore, ok := delRwdGaugeBefore[itypes.BTC_STAKER.String()]
	require.True(t, ok)
	require.True(t, delRewardGaugeBefore.Coins.IsAllPositive())
	delRewardGaugeAfter, ok := delRwdGaugeAfter[itypes.BTC_STAKER.String()]
	require.True(t, ok)
	require.True(t, delRewardGaugeAfter.Coins.IsAllPositive())

	coinsWithdraw := delRewardGaugeAfter.WithdrawnCoins.Sub(delRewardGaugeBefore.WithdrawnCoins...)
	actualAmt := delBalanceAfterWithdraw.String()
	expectedAmt := delBalanceBeforeWithdraw.Add(coinsWithdraw...).Sub(txResp.AuthInfo.Fee.Amount...).String()
	require.Equal(t, expectedAmt, actualAmt)
}

func SendCovenantSigsToPendingDel(
	r *rand.Rand,
	t testing.TB,
	n *chain.NodeConfig,
	btcNet *chaincfg.Params,
	covenantSKs []*btcec.PrivateKey,
	covWallets []string,
	pendingDel *bstypes.BTCDelegation,
) {
	require.Len(t, pendingDel.CovenantSigs, 0)

	params := n.QueryBTCStakingParams()
	slashingTx := pendingDel.SlashingTx
	stakingTx := pendingDel.StakingTx

	stakingMsgTx, err := bbn.NewBTCTxFromBytes(stakingTx)
	require.NoError(t, err)
	stakingTxHash := stakingMsgTx.TxHash().String()

	fpBTCPKs, err := bbn.NewBTCPKsFromBIP340PKs(pendingDel.FpBtcPkList)
	require.NoError(t, err)

	stakingInfo, err := pendingDel.GetStakingInfo(params, btcNet)
	require.NoError(t, err)

	stakingSlashingPathInfo, err := stakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	/*
		generate and insert new covenant signature, in order to activate the BTC delegation
	*/
	// covenant signatures on slashing tx
	covenantSlashingSigs, err := datagen.GenCovenantAdaptorSigs(
		covenantSKs,
		fpBTCPKs,
		stakingMsgTx,
		stakingSlashingPathInfo.GetPkScriptPath(),
		slashingTx,
	)
	require.NoError(t, err)

	// cov Schnorr sigs on unbonding signature
	unbondingPathInfo, err := stakingInfo.UnbondingPathSpendInfo()
	require.NoError(t, err)
	unbondingTx, err := bbn.NewBTCTxFromBytes(pendingDel.BtcUndelegation.UnbondingTx)
	require.NoError(t, err)

	covUnbondingSigs, err := datagen.GenCovenantUnbondingSigs(
		covenantSKs,
		stakingMsgTx,
		pendingDel.StakingOutputIdx,
		unbondingPathInfo.GetPkScriptPath(),
		unbondingTx,
	)
	require.NoError(t, err)

	unbondingInfo, err := pendingDel.GetUnbondingInfo(params, btcNet)
	require.NoError(t, err)
	unbondingSlashingPathInfo, err := unbondingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)
	covenantUnbondingSlashingSigs, err := datagen.GenCovenantAdaptorSigs(
		covenantSKs,
		fpBTCPKs,
		unbondingTx,
		unbondingSlashingPathInfo.GetPkScriptPath(),
		pendingDel.BtcUndelegation.SlashingTx,
	)
	require.NoError(t, err)

	for i := 0; i < int(params.CovenantQuorum); i++ {
		// add covenant sigs
		n.AddCovenantSigs(
			covWallets[i],
			covenantSlashingSigs[i].CovPk,
			stakingTxHash,
			covenantSlashingSigs[i].AdaptorSigs,
			bbn.NewBIP340SignatureFromBTCSig(covUnbondingSigs[i]),
			covenantUnbondingSlashingSigs[i].AdaptorSigs,
		)
	}
}

func AllBtcDelsActive(t *testing.T, n *chain.NodeConfig, fpsBTCPK ...string) []*bstypes.BTCDelegationResponse {
	activeDelsSet := n.QueryFinalityProvidersDelegations(fpsBTCPK...)
	for _, activeDel := range activeDelsSet {
		require.True(t, activeDel.Active)
		require.Greater(t, activeDel.TotalSat, uint64(0))
	}
	return activeDelsSet
}

func AddCovdSigsToPendingBtcDels(
	r *rand.Rand,
	t *testing.T,
	n *chain.NodeConfig,
	btcNet *chaincfg.Params,
	bsParams *bstypes.Params,
	covenantSKs []*btcec.PrivateKey,
	covWallets []string,
	fpsBTCPK ...string,
) {
	pendingDelsResp := n.QueryFinalityProvidersDelegations(fpsBTCPK...)

	for _, pendingDelResp := range pendingDelsResp {
		pendingDel, err := chain.ParseRespBTCDelToBTCDel(pendingDelResp)
		require.NoError(t, err)

		if pendingDel.HasCovenantQuorums(bsParams.CovenantQuorum, 0) {
			continue
		}

		SendCovenantSigsToPendingDel(r, t, n, btcNet, covenantSKs, covWallets, pendingDel)

		n.WaitForNextBlock()
	}
}
