package e2e

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cometbft/cometbft/libs/bytes"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/testutil/coins"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/types"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/x/finality/types"
	itypes "github.com/babylonlabs-io/babylon/x/incentive/types"
)

const (
	stakingTimeBlocks = uint16(math.MaxUint16)
	wDel1             = "del1"
	wDel2             = "del2"
	wFp1              = "fp1"
	wFp2              = "fp2"
	numPubRand        = uint64(350)
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

	// Staking amount of each delegation
	// 3 Delegations will start closely and possibly in the same block
	// (fp1, del1), (fp1, del2), (fp2, del1)

	// (fp1, del1) fp1Del1StakingAmt => 2_00000000
	// (fp1, del2) fp1Del2StakingAmt => 4_00000000
	// (fp2, del1) fp2Del2StakingAmt => 2_00000000
	// for this top configure the reward distribution should
	// be 25%, 50%, 25% respectively (if they will be processed in the same block)
	// since the rewards distribution is by their bech32 address the delegations
	// are combined the voting power and each delegator should receive the same
	// amount of rewards, finality providers on the other hand will have different amounts
	// fp2 with 2_00000000 and fp1 with 6_00000000. This means the fp1 will
	// receive 3x the amount of rewards then fp2.
	fp1Del1StakingAmt int64
	fp1Del2StakingAmt int64
	fp2Del1StakingAmt int64

	// The lastet delegation will come right after (del2) had withdraw his rewards
	// and stake 6_00000000 to (fp2, del2). Since the rewards are combined by
	// their bech32 address, del2 will have 10_00000000 and del1 will have 4_00000000
	// as voting power, meaning that del1 will receive only 40% of the amount of rewards
	// that del2 will receive once every delegation is active
	fp2Del2StakingAmt int64

	// bech32 address of the delegators
	del1Addr string
	del2Addr string

	// covenant helpers
	covenantSKs     []*btcec.PrivateKey
	covenantWallets []string

	// finality helpers
	finalityIdx              uint64
	finalityBlockHeightVoted uint64
	fp1RandListInfo          *datagen.RandListInfo
	fp2RandListInfo          *datagen.RandListInfo

	// reward helpers
	fp1LastRewardGauge     *itypes.RewardGaugesResponse
	fp2LastRewardGauge     *itypes.RewardGaugesResponse
	btcDel2LastRewardGauge *itypes.RewardGaugesResponse
	btcDel1LastRewardGauge *itypes.RewardGaugesResponse

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
	chainA.WaitUntilHeight(1)

	n1, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)
	n2, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	s.fp1 = CreateNodeFP(
		s.T(),
		s.r,
		s.fp1BTCSK,
		n1,
	)
	s.NotNil(s.fp1)

	s.fp2 = CreateNodeFP(
		s.T(),
		s.r,
		s.fp2BTCSK,
		n2,
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

	n2.BankMultiSendFromNode([]string{s.del1Addr, s.del2Addr}, "100000ubbn")

	// fp1Del1
	s.CreateBTCDelegationAndCheck(n2, wDel1, s.fp1, s.del1BTCSK, s.del1Addr, s.fp1Del1StakingAmt)
	// fp1Del2
	s.CreateBTCDelegationAndCheck(n2, wDel2, s.fp1, s.del2BTCSK, s.del2Addr, s.fp1Del2StakingAmt)
	// fp2Del1
	s.CreateBTCDelegationAndCheck(n2, wDel1, s.fp2, s.del1BTCSK, s.del1Addr, s.fp2Del1StakingAmt)

	n2.WaitForNextBlock()
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

	n1.BankMultiSendFromNode(covAddrs, "100000ubbn")

	// tx bank send needs to take effect
	n1.WaitForNextBlock()

	pendingDelsResp := n1.QueryFinalityProvidersDelegations(s.fp1.BtcPk.MarshalHex(), s.fp2.BtcPk.MarshalHex())
	s.Equal(len(pendingDelsResp), 3)

	for _, pendingDelResp := range pendingDelsResp {
		pendingDel, err := ParseRespBTCDelToBTCDel(pendingDelResp)
		s.NoError(err)

		SendCovenantSigsToPendingDel(s.r, s.T(), n1, s.net, s.covenantSKs, s.covenantWallets, pendingDel)

		n1.WaitForNextBlock()
	}

	// wait for a block so that above txs take effect
	n1.WaitForNextBlock()

	// ensure the BTC delegation has covenant sigs now
	activeDelsSet := n1.QueryFinalityProvidersDelegations(s.fp1.BtcPk.MarshalHex(), s.fp2.BtcPk.MarshalHex())
	s.Len(activeDelsSet, 3)
	for _, activeDel := range activeDelsSet {
		s.True(activeDel.Active)
	}
}

// Test4CommitPublicRandomnessAndSealed commits public randomness for
// each finality provider and seals the epoch.
func (s *BtcRewardsDistribution) Test4CommitPublicRandomnessAndSealed() {
	chainA := s.configurer.GetChainConfig(0)
	n1, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)
	n2, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	/*
		commit a number of public randomness
	*/
	// commit public randomness list
	commitStartHeight := uint64(1)

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

	n1.WaitUntilCurrentEpochIsSealedAndFinalized(1)

	// activated height is never returned
	s.finalityBlockHeightVoted = n1.WaitFinalityIsActivated()

	// submit finality signature
	s.finalityIdx = s.finalityBlockHeightVoted - commitStartHeight

	appHash := n1.AddFinalitySignatureToBlock(
		s.fp1BTCSK,
		s.fp1.BtcPk,
		s.finalityBlockHeightVoted,
		s.fp1RandListInfo.SRList[s.finalityIdx],
		&s.fp1RandListInfo.PRList[s.finalityIdx],
		*s.fp1RandListInfo.ProofList[s.finalityIdx].ToProto(),
	)

	n2.AddFinalitySignatureToBlock(
		s.fp2BTCSK,
		s.fp2.BtcPk,
		s.finalityBlockHeightVoted,
		s.fp2RandListInfo.SRList[s.finalityIdx],
		&s.fp2RandListInfo.PRList[s.finalityIdx],
		*s.fp2RandListInfo.ProofList[s.finalityIdx].ToProto(),
	)

	n2.WaitForNextBlocks(2)
	s.AddFinalityVote([]string{}, []string{})

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

	// The rewards distributed for the finality providers should be fp1 => 3x, fp2 => 1x
	s.UpdateRewardGauges(n2)
	// fp1 ~2674ubbn
	// fp2 ~891ubbn
	coins.RequireCoinsDiffInPointOnePercentMargin(
		s.T(),
		s.fp2LastRewardGauge.Coins.MulInt(sdkmath.NewIntFromUint64(3)), // ~2673ubbn
		s.fp1LastRewardGauge.Coins,
	)

	// The rewards distributed to the delegators should be the same for each delegator
	// del1 ~7130ubbn
	// del2 ~7130ubbn
	s.Equal(s.btcDel1LastRewardGauge.Coins.String(), s.btcDel2LastRewardGauge.Coins.String())
	// note that the rewards will not be precise as more or less blocks were produced and given out rewards.

	// Withdraw the reward just for del2 to check it is possible
	del2BalanceBeforeWithdraw, err := n2.QueryBalances(s.del2Addr)
	s.NoError(err)

	n2.WithdrawReward(itypes.BTCDelegationType.String(), wDel2)
	n2.WaitForNextBlock()

	del2BalanceAfterWithdraw, err := n2.QueryBalances(s.del2Addr)
	s.NoError(err)

	s.Equal(del2BalanceAfterWithdraw.String(), del2BalanceBeforeWithdraw.Add(s.btcDel2LastRewardGauge.Coins...).String())
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
	pendingDel, err := ParseRespBTCDelToBTCDel(pendingDels[0])
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

	// cache the reward gauges prior to the BTC delegation is active, so it
	// can calculate the rewards properly based on the difference
	// even then it could be that a block is mined between the query of this
	// 4 address, but the difference would be very small
	// s.UpdateRewardGauges()
}

// Test7FinalityVoteBlock votes in one more block from both finality providers
func (s *BtcRewardsDistribution) Test7FinalityVoteBlock() {
	chainA := s.configurer.GetChainConfig(0)
	// n2, err := chainA.GetNodeAtIndex(2)
	// s.NoError(err)
	// covenants are at n1
	n1, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)

	// lastFinalizedEpoch, err := n1.QueryLastFinalizedEpoch()
	// s.NoError(err)
	// lastFinalizedEpoch++
	// n1.WaitUntilCurrentEpochIsSealedAndFinalized(lastFinalizedEpoch)

	appHash := s.AddFinalityVote([]string{}, []string{})
	n1.WaitForNextBlock()

	// ensure vote is eventually cast
	var finalizedBlocks []*ftypes.IndexedBlock
	s.Eventually(func() bool {
		finalizedBlocks = n1.QueryListBlocks(ftypes.QueriedBlockStatus_FINALIZED)
		return len(finalizedBlocks) > 1
	}, time.Minute, time.Millisecond*50)

	lastFinalizedBlock := finalizedBlocks[len(finalizedBlocks)-1]
	s.Equal(s.finalityBlockHeightVoted, lastFinalizedBlock.Height)
	s.Equal(appHash.Bytes(), lastFinalizedBlock.AppHash)
	s.T().Logf("the block %d is finalized", s.finalityBlockHeightVoted)
}

// Test8CheckRewards verifies the rewards of all the delegations
// and finality provider
func (s *BtcRewardsDistribution) Test8CheckRewards() {
	n2, err := s.configurer.GetChainConfig(0).GetNodeAtIndex(2)
	s.NoError(err)

	// updates the cache of the suite
	// s.UpdateRewardGauges(n2)

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

	// Current rewards for each FP
	// fp1 ~5364ubbn
	// fp2 ~1787ubbn
	// del1 ~11625ubbn
	// del2 ~16989ubbn
	fp1RewardGaugePrev, fp2RewardGaugePrev, btcDel1RewardGaugePrev, btcDel2RewardGaugePrev := s.QueryRewardGauges(n2)
	// wait a few block of rewards
	n2.WaitForNextBlocks(10)
	fp1RewardGauge, fp2RewardGauge, btcDel1RewardGauge, btcDel2RewardGauge := s.QueryRewardGauges(n2)

	// since varius block were created, it is needed to get the difference
	// from a certain point where all the delegations were active to properly
	// calculate the distribution with the voting power structure with 4 BTC delegations active
	// Note: if a new block is mined during the query of reward gauges, the calculation might be a
	// bit off by some ubbn
	fp1DiffRewards := fp1RewardGauge.Coins.Sub(fp1RewardGaugePrev.Coins...)
	fp2DiffRewards := fp2RewardGauge.Coins.Sub(fp2RewardGaugePrev.Coins...)
	del1DiffRewards := btcDel1RewardGauge.Coins.Sub(btcDel1RewardGaugePrev.Coins...)
	del2DiffRewards := btcDel2RewardGauge.Coins.Sub(btcDel2RewardGaugePrev.Coins...)

	fp1DiffRewardsStr := fp1DiffRewards.String()
	fp2DiffRewardsStr := fp2DiffRewards.String()
	del1DiffRewardsStr := del1DiffRewards.String()
	del2DiffRewardsStr := del2DiffRewards.String()

	s.NotEmpty(fp1DiffRewardsStr)
	s.NotEmpty(fp2DiffRewardsStr)
	s.NotEmpty(del1DiffRewardsStr)
	s.NotEmpty(del2DiffRewardsStr)

	// s.Equal(btcDel1RewardGauge.Coins.String(), btcDel2RewardGauge.Coins.String())

	// // Withdraw the reward just for del2
	// del2BalanceBeforeWithdraw, err := n2.QueryBalances(s.del2Addr)
	// s.NoError(err)

	// n2.WithdrawReward(itypes.BTCDelegationType.String(), wDel2)
	// n2.WaitForNextBlock()

	// del2BalanceAfterWithdraw, err := n2.QueryBalances(s.del2Addr)
	// s.NoError(err)

	// s.Equal(del2BalanceAfterWithdraw.String(), del2BalanceBeforeWithdraw.Add(btcDel2RewardGauge.Coins...).String())
}

// TODO(rafilx): Slash a FP and expect rewards to be withdraw.

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
		}
		n2Flags := []string{
			"--offline",
			fmt.Sprintf("--account-number=%d", accNumberN2),
			fmt.Sprintf("--sequence=%d", accSequenceN2),
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

// UpdateRewardGauges update the current reward gauges on the test suite
func (s *BtcRewardsDistribution) UpdateRewardGauges(n *chain.NodeConfig) {
	s.fp1LastRewardGauge, s.fp2LastRewardGauge, s.btcDel1LastRewardGauge, s.btcDel2LastRewardGauge = s.QueryRewardGauges(n)
}

func (s *BtcRewardsDistribution) QueryRewardGauges(n *chain.NodeConfig) (
	fp1, fp2, del1, del2 *itypes.RewardGaugesResponse,
) {
	n.WaitForNextBlockWithSleep50ms()

	// tries to query all in the same block
	fp1RewardGauges, errFp1 := n.QueryRewardGauge(s.fp1.Address())
	fp2RewardGauges, errFp2 := n.QueryRewardGauge(s.fp2.Address())
	btcDel1RewardGauges, errDel1 := n.QueryRewardGauge(sdk.MustAccAddressFromBech32(s.del1Addr))
	btcDel2RewardGauges, errDel2 := n.QueryRewardGauge(sdk.MustAccAddressFromBech32(s.del2Addr))
	s.NoError(errFp1)
	s.NoError(errFp2)
	s.NoError(errDel1)
	s.NoError(errDel2)

	fp1RewardGauge, ok := fp1RewardGauges[itypes.FinalityProviderType.String()]
	s.True(ok)
	s.True(fp1RewardGauge.Coins.IsAllPositive())

	fp2RewardGauge, ok := fp2RewardGauges[itypes.FinalityProviderType.String()]
	s.True(ok)
	s.True(fp2RewardGauge.Coins.IsAllPositive())

	btcDel1RewardGauge, ok := btcDel1RewardGauges[itypes.BTCDelegationType.String()]
	s.True(ok)
	s.True(btcDel1RewardGauge.Coins.IsAllPositive())

	btcDel2RewardGauge, ok := btcDel2RewardGauges[itypes.BTCDelegationType.String()]
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
