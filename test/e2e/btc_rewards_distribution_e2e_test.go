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

	covenantSKs     []*btcec.PrivateKey
	covenantWallets []string

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
	n0, err := s.configurer.GetChainConfig(0).GetNodeAtIndex(0)
	s.NoError(err)

	wDel1, wDel2 := "del1", "del2"
	s.del1Addr = n0.KeysAdd(wDel1)
	s.del2Addr = n0.KeysAdd(wDel2)

	n0.BankMultiSendFromNode([]string{s.del1Addr, s.del2Addr}, "100000ubbn")

	// fp1Del1
	s.CreateBTCDelegationAndCheck(n0, wDel1, s.fp1, s.del1BTCSK, s.del1Addr, s.fp1Del1StakingAmt)
	// fp1Del2
	s.CreateBTCDelegationAndCheck(n0, wDel2, s.fp1, s.del2BTCSK, s.del2Addr, s.fp1Del2StakingAmt)
	// fp2Del1
	s.CreateBTCDelegationAndCheck(n0, wDel1, s.fp2, s.del1BTCSK, s.del1Addr, s.fp2Del1StakingAmt)
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
	numPubRand := uint64(150)
	commitStartHeight := uint64(1)

	fp1RandListInfo, fp1CommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(s.r, s.fp1BTCSK, commitStartHeight, numPubRand)
	s.NoError(err)
	fp2RandListInfo, fp2CommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(s.r, s.fp2BTCSK, commitStartHeight, numPubRand)
	s.NoError(err)

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

	n1.WaitUntilCurrentEpochIsSealedAndFinalized()

	// activated height is never returned
	activatedHeight := n1.WaitFinalityIsActivated()

	// submit finality signature
	idx := activatedHeight - commitStartHeight

	appHash := n1.AddFinalitySignatureToBlock(
		s.fp1BTCSK,
		s.fp1.BtcPk,
		activatedHeight,
		fp1RandListInfo.SRList[idx],
		&fp1RandListInfo.PRList[idx],
		*fp1RandListInfo.ProofList[idx].ToProto(),
	)

	n2.AddFinalitySignatureToBlock(
		s.fp2BTCSK,
		s.fp2.BtcPk,
		activatedHeight,
		fp2RandListInfo.SRList[idx],
		&fp2RandListInfo.PRList[idx],
		*fp2RandListInfo.ProofList[idx].ToProto(),
	)

	// ensure vote is eventually cast
	var finalizedBlocks []*ftypes.IndexedBlock
	s.Eventually(func() bool {
		finalizedBlocks = n1.QueryListBlocks(ftypes.QueriedBlockStatus_FINALIZED)
		return len(finalizedBlocks) > 0
	}, time.Minute, time.Millisecond*50)

	s.Equal(activatedHeight, finalizedBlocks[0].Height)
	s.Equal(appHash.Bytes(), finalizedBlocks[0].AppHash)
	s.T().Logf("the block %d is finalized", activatedHeight)
}

// Test5CheckRewardsFirstDelegations verifies the rewards independent of mint amounts
func (s *BtcRewardsDistribution) Test5CheckRewardsFirstDelegations() {
	n1, err := s.configurer.GetChainConfig(0).GetNodeAtIndex(1)
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
	fp1RewardGauges, err := n1.QueryRewardGauge(s.fp1.Address())
	s.NoError(err)
	fp1RewardGauge, ok := fp1RewardGauges[itypes.FinalityProviderType.String()]
	s.True(ok)
	s.True(fp1RewardGauge.Coins.IsAllPositive()) // 2674ubbn

	fp2RewardGauges, err := n1.QueryRewardGauge(s.fp2.Address())
	s.NoError(err)
	fp2RewardGauge, ok := fp2RewardGauges[itypes.FinalityProviderType.String()]
	s.True(ok)
	s.True(fp2RewardGauge.Coins.IsAllPositive()) // 891ubbn

	coins.RequireCoinsDiffInPointOnePercentMargin(
		s.T(),
		fp2RewardGauge.Coins.MulInt(sdkmath.NewIntFromUint64(3)), // 2673ubbn
		fp1RewardGauge.Coins,
	)

	// The rewards distributed to the delegators should be the same for each delegator
	btcDel1RewardGauges, err := n1.QueryRewardGauge(sdk.MustAccAddressFromBech32(s.del1Addr))
	s.NoError(err)
	btcDel1RewardGauge, ok := btcDel1RewardGauges[itypes.BTCDelegationType.String()]
	s.True(ok)
	s.True(btcDel1RewardGauge.Coins.IsAllPositive()) // 7130ubbn

	btcDel2RewardGauges, err := n1.QueryRewardGauge(sdk.MustAccAddressFromBech32(s.del2Addr))
	s.NoError(err)
	btcDel2RewardGauge, ok := btcDel2RewardGauges[itypes.BTCDelegationType.String()]
	s.True(ok)
	s.True(btcDel2RewardGauge.Coins.IsAllPositive()) // 7130ubbn

	s.Equal(btcDel1RewardGauge.Coins.String(), btcDel2RewardGauge.Coins.String())

	// Withdraw the reward just for del2
	del2BalanceBeforeWithdraw, err := n1.QueryBalances(s.del2Addr)
	s.NoError(err)

	n1.WithdrawReward(itypes.BTCDelegationType.String(), s.del2Addr)
	n1.WaitForNextBlock()

	del2BalanceAfterWithdraw, err := n1.QueryBalances(s.del2Addr)
	s.NoError(err)

	s.Equal(del2BalanceAfterWithdraw.String(), del2BalanceBeforeWithdraw.Add(btcDel2RewardGauge.Coins...).String())
}

func (s *BtcRewardsDistribution) Test6ActiveLastDelegation() {
	n2, err := s.configurer.GetChainConfig(0).GetNodeAtIndex(2)
	s.NoError(err)

	wDel2 := "del2"
	// fp2Del2
	s.CreateBTCDelegationAndCheck(n2, wDel2, s.fp2, s.del2BTCSK, s.del2Addr, s.fp2Del2StakingAmt)

	allDelegations := n2.QueryFinalityProvidersDelegations(s.fp1.BtcPk.MarshalHex(), s.fp2.BtcPk.MarshalHex())
	s.Equal(len(allDelegations), 4)

	for _, delegation := range allDelegations {
		if !strings.EqualFold(delegation.StatusDesc, bstypes.BTCDelegationStatus_PENDING.String()) {
			continue
		}
		pendingDel, err := ParseRespBTCDelToBTCDel(delegation)
		s.NoError(err)

		SendCovenantSigsToPendingDel(s.r, s.T(), n2, s.net, s.covenantSKs, s.covenantWallets, pendingDel)

		n2.WaitForNextBlock()
	}

	// wait for a block so that above txs take effect
	n2.WaitForNextBlock()

	// ensure the BTC delegation has covenant sigs now
	allDelegations = n2.QueryFinalityProvidersDelegations(s.fp1.BtcPk.MarshalHex(), s.fp2.BtcPk.MarshalHex())
	s.Len(allDelegations, 4)
	for _, activeDel := range allDelegations {
		s.True(activeDel.Active)
	}
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
