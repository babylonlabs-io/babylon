package e2e

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/types"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/x/finality/types"
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
	fp1Del1StakingAmt int64
	fp1Del2StakingAmt int64
	fp2Del1StakingAmt int64

	// The lastet delegation will come right after (fp1, del2) and (fp2, del1)
	// had withdraw his rewards, and stake 6_00000000 to (fp2, del2) receive the same
	// amount of rewards as the sum of rewards (fp1, del2) and (fp2, del1)
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

	s.fp1Del1StakingAmt = int64(9 * 10e8)
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

	stakingTimeBlocks := uint16(math.MaxUint16)

	// fp1Del1
	s.CreateBTCDelegationAndCheck(n0, wDel1, s.fp1, s.del1BTCSK, s.del1Addr, stakingTimeBlocks, s.fp1Del1StakingAmt)
	// fp1Del2
	s.CreateBTCDelegationAndCheck(n0, wDel2, s.fp1, s.del2BTCSK, s.del2Addr, stakingTimeBlocks, s.fp1Del2StakingAmt)
	// fp2Del1
	s.CreateBTCDelegationAndCheck(n0, wDel1, s.fp2, s.del1BTCSK, s.del1Addr, stakingTimeBlocks, s.fp2Del1StakingAmt)
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
	n1.CommitPubRandList(
		fp1CommitPubRandList.FpBtcPk,
		fp1CommitPubRandList.StartHeight,
		fp1CommitPubRandList.NumPubRand,
		fp1CommitPubRandList.Commitment,
		fp1CommitPubRandList.Sig,
	)

	n1.WaitUntilCurrentEpochIsSealedAndFinalized()

	// activated height is never returned
	activatedHeight := n1.WaitFinalityIsActivated()

	fp2RandListInfo, fp2CommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(s.r, s.fp2BTCSK, commitStartHeight, numPubRand)
	s.NoError(err)
	n2.CommitPubRandList(
		fp2CommitPubRandList.FpBtcPk,
		fp2CommitPubRandList.StartHeight,
		fp2CommitPubRandList.NumPubRand,
		fp2CommitPubRandList.Commitment,
		fp2CommitPubRandList.Sig,
	)
	// latestBlock := n1.LatestBlockNumber()
	/*
		submit finality signature
	*/
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

	// // ensure finality provider has received rewards after the block is finalised
	// fpRewardGauges, err := n1.QueryRewardGauge(fpBabylonAddr)
	// s.NoError(err)
	// fpRewardGauge, ok := fpRewardGauges[itypes.FinalityProviderType.String()]
	// s.True(ok)
	// s.True(fpRewardGauge.Coins.IsAllPositive())
	// // ensure BTC delegation has received rewards after the block is finalised
	// btcDelRewardGauges, err := n1.QueryRewardGauge(delBabylonAddr)
	// s.NoError(err)
	// btcDelRewardGauge, ok := btcDelRewardGauges[itypes.BTCDelegationType.String()]
	// s.True(ok)
	// s.True(btcDelRewardGauge.Coins.IsAllPositive())
	// s.T().Logf("the finality provider received rewards for providing finality")
}

// func (s *BtcRewardsDistribution) Test4WithdrawReward() {
// 	chainA := s.configurer.GetChainConfig(0)
// 	nonValidatorNode, err := chainA.GetNodeAtIndex(2)
// 	s.NoError(err)

// 	// finality provider balance before withdraw
// 	fpBabylonAddr, err := sdk.AccAddressFromBech32(s.fp1.Addr)
// 	s.NoError(err)
// 	delBabylonAddr := fpBabylonAddr

// 	fpBalance, err := nonValidatorNode.QueryBalances(fpBabylonAddr.String())
// 	s.NoError(err)
// 	// finality provider reward gauge should not be fully withdrawn
// 	fpRgs, err := nonValidatorNode.QueryRewardGauge(fpBabylonAddr)
// 	s.NoError(err)
// 	fpRg := fpRgs[itypes.FinalityProviderType.String()]
// 	s.T().Logf("finality provider's withdrawable reward before withdrawing: %s", convertToRewardGauge(fpRg).GetWithdrawableCoins().String())
// 	s.False(convertToRewardGauge(fpRg).IsFullyWithdrawn())

// 	// withdraw finality provider reward
// 	nonValidatorNode.WithdrawReward(itypes.FinalityProviderType.String(), initialization.ValidatorWalletName)
// 	nonValidatorNode.WaitForNextBlock()

// 	// balance after withdrawing finality provider reward
// 	fpBalance2, err := nonValidatorNode.QueryBalances(fpBabylonAddr.String())
// 	s.NoError(err)
// 	s.T().Logf("fpBalance2: %s; fpBalance: %s", fpBalance2.String(), fpBalance.String())
// 	s.True(fpBalance2.IsAllGT(fpBalance))
// 	// finality provider reward gauge should be fully withdrawn now
// 	fpRgs2, err := nonValidatorNode.QueryRewardGauge(fpBabylonAddr)
// 	s.NoError(err)
// 	fpRg2 := fpRgs2[itypes.FinalityProviderType.String()]
// 	s.T().Logf("finality provider's withdrawable reward after withdrawing: %s", convertToRewardGauge(fpRg2).GetWithdrawableCoins().String())
// 	s.True(convertToRewardGauge(fpRg2).IsFullyWithdrawn())

// 	// BTC delegation balance before withdraw
// 	btcDelBalance, err := nonValidatorNode.QueryBalances(delBabylonAddr.String())
// 	s.NoError(err)
// 	// BTC delegation reward gauge should not be fully withdrawn
// 	btcDelRgs, err := nonValidatorNode.QueryRewardGauge(delBabylonAddr)
// 	s.NoError(err)
// 	btcDelRg := btcDelRgs[itypes.BTCDelegationType.String()]
// 	s.T().Logf("BTC delegation's withdrawable reward before withdrawing: %s", convertToRewardGauge(btcDelRg).GetWithdrawableCoins().String())
// 	s.False(convertToRewardGauge(btcDelRg).IsFullyWithdrawn())

// 	// withdraw BTC delegation reward
// 	nonValidatorNode.WithdrawReward(itypes.BTCDelegationType.String(), initialization.ValidatorWalletName)
// 	nonValidatorNode.WaitForNextBlock()

// 	// balance after withdrawing BTC delegation reward
// 	btcDelBalance2, err := nonValidatorNode.QueryBalances(delBabylonAddr.String())
// 	s.NoError(err)
// 	s.T().Logf("btcDelBalance2: %s; btcDelBalance: %s", btcDelBalance2.String(), btcDelBalance.String())
// 	s.True(btcDelBalance2.IsAllGT(btcDelBalance))
// 	// BTC delegation reward gauge should be fully withdrawn now
// 	btcDelRgs2, err := nonValidatorNode.QueryRewardGauge(delBabylonAddr)
// 	s.NoError(err)
// 	btcDelRg2 := btcDelRgs2[itypes.BTCDelegationType.String()]
// 	s.T().Logf("BTC delegation's withdrawable reward after withdrawing: %s", convertToRewardGauge(btcDelRg2).GetWithdrawableCoins().String())
// 	s.True(convertToRewardGauge(btcDelRg2).IsFullyWithdrawn())
// }

func (s *BtcRewardsDistribution) CreateBTCDelegationAndCheck(
	n *chain.NodeConfig,
	wDel string,
	fp *bstypes.FinalityProvider,
	btcStakerSK *btcec.PrivateKey,
	delAddr string,
	stakingTimeBlocks uint16,
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
