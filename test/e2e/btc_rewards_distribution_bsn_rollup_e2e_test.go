package e2e

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v4/testutil/coins"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	bsctypes "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
	itypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

const (
	wFp3       = "fp3"
	wFp4       = "fp4"
	bsnIdCons0 = "bsn-consumer-0"
	bsnIdCons4 = "bsn-consumer-4"
)

type BtcRewardsDistributionBsnRollup struct {
	BaseBtcRewardsDistribution

	// consumer
	bsn0 *bsctypes.ConsumerRegister
	bsn4 *bsctypes.ConsumerRegister

	// 4 fps
	// babylon => fp1
	// consumer0 => fp2, fp3
	// consumer4 => fp4
	fp1bbnBTCSK   *btcec.PrivateKey
	fp2cons0BTCSK *btcec.PrivateKey
	fp3cons0BTCSK *btcec.PrivateKey
	fp4cons4BTCSK *btcec.PrivateKey

	del1BTCSK *btcec.PrivateKey
	del2BTCSK *btcec.PrivateKey

	fp1bbn   *bstypes.FinalityProvider
	fp2cons0 *bstypes.FinalityProvider
	fp3cons0 *bstypes.FinalityProvider
	fp4cons4 *bstypes.FinalityProvider

	// 3 BTC Delegations will be made at the beginning
	// (fp1bbn,fp2cons0,fp4cons4 del1), (fp1bbn,fp3cons0,fp4cons4 del1), (fp1bbn, fp2cons0 del2)

	// (fp1bbn,fp2cons0,fp4cons4 del1) fp2fp4Del1StkAmt => 2_00000000
	// (fp1bbn,fp3cons0,fp4cons4 del1) fp3fp4Del1StkAmt => 2_00000000
	// (fp1bbn,fp2cons0 del2) fp2Del2StkAmt => 4_00000000
	fp2fp4Del1StkAmt int64
	fp3fp4Del1StkAmt int64
	fp2Del2StkAmt    int64

	// The lastet delegation will stake 6_00000000 to (fp4cons4, del2).
	fp4Del2StakingAmt int64

	// bech32 address of the delegators
	del1Addr string
	del2Addr string
	// bech32 address of the finality providers
	fp1bbnAddr   string
	fp2cons0Addr string
	fp3cons0Addr string
	fp4cons4Addr string

	configurer configurer.Configurer
}

func (s *BtcRewardsDistributionBsnRollup) SetupSuite() {
	s.T().Log("setting up e2e integration test suite...")
	var err error

	s.r = rand.New(rand.NewSource(time.Now().Unix()))
	s.net = &chaincfg.SimNetParams
	s.fp1bbnBTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.fp2cons0BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.fp3cons0BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.fp4cons4BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.del1BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.del2BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)

	s.fp2fp4Del1StkAmt = int64(2 * 10e8)
	s.fp3fp4Del1StkAmt = int64(2 * 10e8)
	s.fp2Del2StkAmt = int64(4 * 10e8)
	s.fp4Del2StakingAmt = int64(6 * 10e8)

	covenantSKs, _, _ := bstypes.DefaultCovenantCommittee()
	s.covenantSKs = covenantSKs

	s.configurer, err = configurer.NewBabylonConfigurer(s.T(), true)
	s.NoError(err)
	err = s.configurer.ConfigureChains()
	s.NoError(err)
	err = s.configurer.RunSetup()
	s.NoError(err)
}

func (s *BtcRewardsDistributionBsnRollup) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

// Test1CreateFinalityProviders creates all finality providers
func (s *BtcRewardsDistributionBsnRollup) Test1CreateFinalityProviders() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(2)

	n1, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)
	n2, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	n1.WaitForNextBlock()
	n2.WaitForNextBlock()

	s.fp1bbnAddr = n1.KeysAdd(wFp1)
	s.fp2cons0Addr = n2.KeysAdd(wFp2)
	s.fp3cons0Addr = n2.KeysAdd(wFp3)
	s.fp4cons4Addr = n2.KeysAdd(wFp4)

	n2.BankMultiSendFromNode([]string{s.fp1bbnAddr, s.fp2cons0Addr, s.fp3cons0Addr, s.fp4cons4Addr}, "1000000ubbn")

	n1.WaitForNextBlock()
	n2.WaitForNextBlock()

	rollupContractCons0 := n1.CreateFinalityContract(bsnIdCons0)

	bsn0 := bsctypes.NewCosmosConsumerRegister(
		bsnIdCons0,
		datagen.GenRandomHexStr(s.r, 5),
		"Chain description: "+datagen.GenRandomHexStr(s.r, 15),
		datagen.GenBabylonRewardsCommission(s.r),
	)
	n1.RegisterRollupConsumerChain(n1.WalletName, bsn0.ConsumerId, bsn0.ConsumerName, bsn0.ConsumerDescription, bsn0.BabylonRewardsCommission.String(), rollupContractCons0)
	s.bsn0 = bsn0

	rollupContractCons4 := n2.CreateFinalityContract(bsnIdCons4)

	bsn4 := bsctypes.NewCosmosConsumerRegister(
		bsnIdCons4,
		datagen.GenRandomHexStr(s.r, 5),
		"Chain description: "+datagen.GenRandomHexStr(s.r, 15),
		datagen.GenBabylonRewardsCommission(s.r),
	)
	n2.RegisterRollupConsumerChain(n2.WalletName, bsn4.ConsumerId, bsn4.ConsumerName, bsn4.ConsumerDescription, bsn4.BabylonRewardsCommission.String(), rollupContractCons4)
	s.bsn4 = bsn4

	n1.WaitForNextBlock()
	n2.WaitForNextBlock()

	consumers := n2.QueryBTCStkConsumerConsumers()
	require.Len(s.T(), consumers, 2)
	s.T().Log("All Consumers created")

	s.fp1bbn = CreateNodeFP(
		s.T(),
		s.r,
		s.fp1bbnBTCSK,
		n1,
		s.fp1bbnAddr,
		n1.ChainID(),
	)
	require.NotNil(s.T(), s.fp1bbn)

	s.fp2cons0 = CreateNodeFP(
		s.T(),
		s.r,
		s.fp2cons0BTCSK,
		n2,
		s.fp2cons0Addr,
		bsnIdCons0,
	)
	s.NotNil(s.fp2cons0)

	s.fp3cons0 = CreateNodeFP(
		s.T(),
		s.r,
		s.fp3cons0BTCSK,
		n2,
		s.fp3cons0Addr,
		bsnIdCons0,
	)
	s.NotNil(s.fp3cons0)

	s.fp4cons4 = CreateNodeFP(
		s.T(),
		s.r,
		s.fp4cons4BTCSK,
		n2,
		s.fp4cons4Addr,
		bsnIdCons4,
	)
	s.NotNil(s.fp4cons4)

	n1.WaitForNextBlock()
	n2.WaitForNextBlock()

	babylonFps := n2.QueryFinalityProviders(n1.ChainID())
	cons0Fps := n2.QueryFinalityProviders(bsn0.ConsumerId)
	cons4Fps := n2.QueryFinalityProviders(bsn4.ConsumerId)

	require.Len(s.T(), append(babylonFps, append(cons0Fps, cons4Fps...)...), 4, "should have created all the FPs to start the test")
	s.T().Log("All Fps created")
}

// Test2CreateFirstBtcDelegations creates the first 3 btc delegations
// with the same values, but different satoshi staked amounts
func (s *BtcRewardsDistributionBsnRollup) Test2CreateFirstBtcDelegations() {
	n2, err := s.configurer.GetChainConfig(0).GetNodeAtIndex(2)
	s.NoError(err)

	s.del1Addr = n2.KeysAdd(wDel1)
	s.del2Addr = n2.KeysAdd(wDel2)

	n2.BankMultiSendFromNode([]string{s.del1Addr, s.del2Addr}, "1000000ubbn")

	n2.WaitForNextBlock()

	// add bsn rewards to FP without adding voting power, should fail
	failRwdCoins := sdk.NewCoins(sdk.NewCoin(nativeDenom, math.NewInt(1000)))
	failRatios := []bstypes.FpRatio{{BtcPk: s.fp2cons0.BtcPk, Ratio: math.LegacyOneDec()}}
	outBuf, _, _ := n2.AddBsnRewards(n2.WalletName, s.fp3cons0.BsnId, failRwdCoins, failRatios)

	txHash := chain.GetTxHashFromOutput(outBuf.String())
	n2.WaitForNextBlock()

	txRespAddBsnRewards, _ := n2.QueryTx(txHash)
	require.Contains(s.T(), txRespAddBsnRewards.RawLog, "unable to allocate BTC rewards")

	n2.WaitForNextBlock()

	s.CreateBTCDelegationMultipleFPsAndCheck(n2, wDel1, s.del1BTCSK, s.del1Addr, s.fp2fp4Del1StkAmt, s.fp1bbn, s.fp2cons0, s.fp4cons4)
	s.CreateBTCDelegationMultipleFPsAndCheck(n2, wDel1, s.del1BTCSK, s.del1Addr, s.fp3fp4Del1StkAmt, s.fp1bbn, s.fp3cons0, s.fp4cons4)
	s.CreateBTCDelegationMultipleFPsAndCheck(n2, wDel2, s.del2BTCSK, s.del2Addr, s.fp2Del2StkAmt, s.fp1bbn, s.fp2cons0)

	resp := n2.QueryBtcDelegations(bstypes.BTCDelegationStatus_ANY)
	require.Len(s.T(), resp.BtcDelegations, 3)
}

// Test3SubmitCovenantSignature covenant approves all the 3 BTC delegation
func (s *BtcRewardsDistributionBsnRollup) Test3SubmitCovenantSignature() {
	n1, err := s.configurer.GetChainConfig(0).GetNodeAtIndex(1)
	s.NoError(err)

	s.CreateCovenantsAndSubmitSignaturesToPendDels(n1, s.fp1bbn)
}

// Test5CheckRewardsFirstDelegations verifies the rewards of all the 3 created BTC delegations
// Since it is a BSN rewards, it doesn't depend on the blocks, but when the MsgBsnAddRewards
// is sent.
func (s *BtcRewardsDistributionBsnRollup) Test5CheckRewardsFirstDelegations() {
	n2, err := s.configurer.GetChainConfig(0).GetNodeAtIndex(2)
	s.NoError(err)

	nodeBalances, err := n2.QueryBalances(n2.PublicAddress)
	s.NoError(err)

	rewardCoins := nodeBalances.Sub(sdk.NewCoin(nativeDenom, nodeBalances.AmountOf(nativeDenom))).QuoInt(math.NewInt(4))
	require.GreaterOrEqual(s.T(), rewardCoins.Len(), 2, "should have 2 or more denoms to give out as rewards")

	fp2Ratio, fp3Ratio := math.LegacyMustNewDecFromStr("0.7"), math.LegacyMustNewDecFromStr("0.3")

	bbnCommDiff, del1Diff, del2Diff, fp1bbnDiff, fp2cons0Diff, fp3cons0Diff, fp4cons4Diff := s.SuiteRewardsDiff(n2, func() {
		outBuf, _, _ := n2.AddBsnRewards(n2.WalletName, s.bsn0.ConsumerId, rewardCoins, []bstypes.FpRatio{
			{
				BtcPk: s.fp2cons0.BtcPk,
				Ratio: fp2Ratio,
			},
			{
				BtcPk: s.fp3cons0.BtcPk,
				Ratio: fp3Ratio,
			},
		})

		txHash := chain.GetTxHashFromOutput(outBuf.String())
		n2.WaitForNextBlock()

		txRespAddBsnRewards, _ := n2.QueryTx(txHash)
		require.Truef(s.T(), len(txRespAddBsnRewards.RawLog) == 0, "raw log should be empty %s", txRespAddBsnRewards.RawLog)
	})

	bbnCommExp := itypes.GetCoinsPortion(rewardCoins, s.bsn0.BabylonRewardsCommission)
	require.Equal(s.T(), bbnCommExp.String(), bbnCommDiff.String(), "babylon commission")

	rewardCoinsAfterBbnComm := rewardCoins.Sub(bbnCommExp...)

	fp2AfterRatio := itypes.GetCoinsPortion(rewardCoinsAfterBbnComm, fp2Ratio)
	fp2CommExp := itypes.GetCoinsPortion(fp2AfterRatio, *s.fp2cons0.Commission)
	require.Equal(s.T(), fp2CommExp.String(), fp2cons0Diff.String(), "fp2 consumer 0 commission")

	fp3AfterRatio := itypes.GetCoinsPortion(rewardCoinsAfterBbnComm, fp3Ratio)
	fp3CommExp := itypes.GetCoinsPortion(fp3AfterRatio, *s.fp3cons0.Commission)
	require.Equal(s.T(), fp3CommExp.String(), fp3cons0Diff.String(), "fp3 consumer 0 commission")

	// Current setup of voting power for consumer 0
	// (fp2cons0, del1) => 2_00000000
	// (fp2cons0, del2) => 4_00000000
	// (fp3cons0, del1) => 2_00000000

	fp2RemainingBtcRewards := fp2AfterRatio.Sub(fp2CommExp...)
	fp3RemainingBtcRewards := fp3AfterRatio.Sub(fp3CommExp...)

	// del1 will receive all the rewards of fp3 and 1/3 of the fp2 rewards
	expectedRewardsDel1 := itypes.GetCoinsPortion(fp2RemainingBtcRewards, math.LegacyMustNewDecFromStr("0.333333333333334")).Add(fp3RemainingBtcRewards...)
	coins.RequireCoinsDiffInPointOnePercentMargin(s.T(), expectedRewardsDel1, del1Diff)
	// del2 will receive 2/3 of the fp2 rewards
	expectedRewardsDel2 := itypes.GetCoinsPortion(fp2RemainingBtcRewards, math.LegacyMustNewDecFromStr("0.666666666666666"))
	coins.RequireCoinsDiffInPointOnePercentMargin(s.T(), expectedRewardsDel2, del2Diff)

	require.True(s.T(), fp1bbnDiff.IsZero(), "fp1 was not rewarded")
	require.True(s.T(), fp4cons4Diff.IsZero(), "fp4 was not rewarded")
}

// Test6ActiveLastDelegation creates a new btc delegation
// (fp1bbn, fp4cons4, del2) with 6_00000000 sats and sends the covenant signatures
// needed.
func (s *BtcRewardsDistributionBsnRollup) Test6ActiveLastDelegation() {
	chainA := s.configurer.GetChainConfig(0)
	n2, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	// covenants are at n1
	n1, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)

	s.CreateBTCDelegationMultipleFPsAndCheck(n2, wDel2, s.del2BTCSK, s.del2Addr, s.fp4Del2StakingAmt, s.fp1bbn, s.fp4cons4)

	allDelegations := n2.QueryFinalityProvidersDelegations(s.fp1bbn.BtcPk.MarshalHex())
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

	n1.SendCovenantSigs(s.r, s.T(), s.net, s.covenantSKs, s.covenantWallets, pendingDel)

	// wait for a block so that covenant txs take effect
	n1.WaitForNextBlock()

	// ensure that all BTC delegation are active
	allDelegations = n1.QueryFinalityProvidersDelegations(s.fp1bbn.BtcPk.MarshalHex())
	s.Len(allDelegations, 4)
	for _, activeDel := range allDelegations {
		s.True(activeDel.Active)
	}
}

// Test7CheckRewardsBsn4 verifies the rewards of all the BTC delegations to consumer 4
// are correctly distributed between the single fp that represents it
func (s *BtcRewardsDistributionBsnRollup) Test7CheckRewardsBsn4() {
	n2, err := s.configurer.GetChainConfig(0).GetNodeAtIndex(2)
	s.NoError(err)

	nodeBalances, err := n2.QueryBalances(n2.PublicAddress)
	s.NoError(err)

	rewardCoins := nodeBalances.Sub(sdk.NewCoin(nativeDenom, nodeBalances.AmountOf(nativeDenom))).QuoInt(math.NewInt(4))
	require.Greater(s.T(), rewardCoins.Len(), 1, "should have 2 or more denoms to give out as rewards")

	fp4Ratio := math.LegacyOneDec()

	bbnCommDiff, del1Diff, del2Diff, fp1bbnDiff, fp2cons0Diff, fp3cons0Diff, fp4cons4Diff := s.SuiteRewardsDiff(n2, func() {
		outBuf, _, _ := n2.AddBsnRewards(n2.WalletName, s.bsn4.ConsumerId, rewardCoins, []bstypes.FpRatio{{
			BtcPk: s.fp4cons4.BtcPk,
			Ratio: fp4Ratio,
		}})

		txHash := chain.GetTxHashFromOutput(outBuf.String())
		n2.WaitForNextBlock()

		txRespAddBsnRewards, _ := n2.QueryTx(txHash)
		require.Truef(s.T(), len(txRespAddBsnRewards.RawLog) == 0, "raw log should be empty %s", txRespAddBsnRewards.RawLog)
	})

	bbnCommExp := itypes.GetCoinsPortion(rewardCoins, s.bsn4.BabylonRewardsCommission)
	require.Equal(s.T(), bbnCommExp.String(), bbnCommDiff.String(), "babylon commission")

	rewardCoinsAfterBbnComm := rewardCoins.Sub(bbnCommExp...)

	// there is only one fp in the consumer 4, so the entire rewards after bbn commission goes
	// to the fp and his BTC stakers
	fp4CommExp := itypes.GetCoinsPortion(rewardCoinsAfterBbnComm, *s.fp4cons4.Commission)
	require.Equal(s.T(), fp4CommExp.String(), fp4cons4Diff.String(), "fp4 consumer 4 commission")

	// Current setup of voting power for consumer 4
	// (fp4cons4, del1) => 2_00000000
	// (fp4cons4, del1) => 2_00000000
	// (fp4cons4, del2) => 6_00000000

	// sum up by dels
	// (del1) => 4_00000000
	// (del2) => 6_00000000

	fp4RemainingBtcRewards := rewardCoinsAfterBbnComm.Sub(fp4CommExp...)

	// del1 will receive 4/10 of the fp4 rewards
	expectedRewardsDel1 := itypes.GetCoinsPortion(fp4RemainingBtcRewards, math.LegacyMustNewDecFromStr("0.4"))
	coins.RequireCoinsDiffInPointOnePercentMargin(s.T(), expectedRewardsDel1, del1Diff)
	// del2 will receive 6/10 of the fp4 rewards
	expectedRewardsDel2 := itypes.GetCoinsPortion(fp4RemainingBtcRewards, math.LegacyMustNewDecFromStr("0.6"))
	coins.RequireCoinsDiffInPointOnePercentMargin(s.T(), expectedRewardsDel2, del2Diff)

	require.True(s.T(), fp1bbnDiff.IsZero(), "fp1 was not rewarded")
	require.True(s.T(), fp2cons0Diff.IsZero(), "fp2 was not rewarded")
	require.True(s.T(), fp3cons0Diff.IsZero(), "fp3 was not rewarded")
}

// QueryFpRewards returns the rewards available for fp1, fp2, fp3, fp4
func (s *BtcRewardsDistributionBsnRollup) QueryFpRewards(n *chain.NodeConfig) (
	fp1bbn, fp2cons0, fp3cons0, fp4cons4 sdk.Coins,
) {
	g := new(errgroup.Group)
	var (
		err                  error
		fp1bbnRewardGauges   map[string]*itypes.RewardGaugesResponse
		fp2cons0RewardGauges map[string]*itypes.RewardGaugesResponse
		fp3cons0RewardGauges map[string]*itypes.RewardGaugesResponse
		fp4cons4RewardGauges map[string]*itypes.RewardGaugesResponse
	)

	g.Go(func() error {
		fp1bbnRewardGauges, err = n.QueryRewardGauge(s.fp1bbn.Address())
		if err != nil {
			return fmt.Errorf("failed to query rewards for fp1bbn: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		fp2cons0RewardGauges, err = n.QueryRewardGauge(s.fp2cons0.Address())
		if err != nil {
			return fmt.Errorf("failed to query rewards for fp2cons0: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		fp3cons0RewardGauges, err = n.QueryRewardGauge(s.fp3cons0.Address())
		if err != nil {
			return fmt.Errorf("failed to query rewards for fp3cons0: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		fp4cons4RewardGauges, err = n.QueryRewardGauge(s.fp4cons4.Address())
		if err != nil {
			return fmt.Errorf("failed to query rewards for fp4cons4: %w", err)
		}
		return nil
	})

	_ = g.Wait()
	fp1bbnRewardCoins := sdk.NewCoins()
	fp1bbnRewardGauge, ok := fp1bbnRewardGauges[itypes.FINALITY_PROVIDER.String()]
	if ok {
		fp1bbnRewardCoins = fp1bbnRewardGauge.Coins
	}
	fp2cons0RewardCoins := sdk.NewCoins()
	fp2cons0RewardGauge, ok := fp2cons0RewardGauges[itypes.FINALITY_PROVIDER.String()]
	if ok {
		fp2cons0RewardCoins = fp2cons0RewardGauge.Coins
	}
	fp3cons0RewardCoins := sdk.NewCoins()
	fp3cons0RewardGauge, ok := fp3cons0RewardGauges[itypes.FINALITY_PROVIDER.String()]
	if ok {
		fp3cons0RewardCoins = fp3cons0RewardGauge.Coins
	}
	fp4cons4RewardCoins := sdk.NewCoins()
	fp4cons4RewardGauge, ok := fp4cons4RewardGauges[itypes.FINALITY_PROVIDER.String()]
	if ok {
		fp4cons4RewardCoins = fp4cons4RewardGauge.Coins
	}

	return fp1bbnRewardCoins, fp2cons0RewardCoins, fp3cons0RewardCoins, fp4cons4RewardCoins
}

// QuerySuiteRewards returns the babylon commission account balance and fp, dels
// available rewards
func (s *BtcRewardsDistributionBsnRollup) QuerySuiteRewards(n *chain.NodeConfig) (
	bbnComm, del1, del2, fp1bbn, fp2cons0, fp3cons0, fp4cons4 sdk.Coins,
) {
	bbnComm, err := n.QueryBalances(params.AccBbnComissionCollectorBsn.String())
	require.NoError(s.T(), err)

	fp1bbn, fp2cons0, fp3cons0, fp4cons4 = s.QueryFpRewards(n)
	delRwd := s.QueryDelRewards(n, s.del1Addr, s.del2Addr)
	del1, del2 = delRwd[s.del1Addr], delRwd[s.del2Addr]

	return bbnComm, del1, del2, fp1bbn, fp2cons0, fp3cons0, fp4cons4
}

func (s *BtcRewardsDistributionBsnRollup) SuiteRewardsDiff(n *chain.NodeConfig, f func()) (
	bbnComm, del1, del2, fp1bbn, fp2cons0, fp3cons0, fp4cons4 sdk.Coins,
) {
	bbnCommBefore, del1Before, del2Before, fp1bbnBefore, fp2cons0Before, fp3cons0Before, fp4cons4Before := s.QuerySuiteRewards(n)

	f()
	n.WaitForNextBlock()

	bbnCommAfter, del1After, del2After, fp1bbnAfter, fp2cons0After, fp3cons0After, fp4cons4After := s.QuerySuiteRewards(n)

	bbnCommDiff := bbnCommAfter.Sub(bbnCommBefore...)
	del1Diff := del1After.Sub(del1Before...)
	del2Diff := del2After.Sub(del2Before...)
	fp1bbnDiff := fp1bbnAfter.Sub(fp1bbnBefore...)
	fp2cons0Diff := fp2cons0After.Sub(fp2cons0Before...)
	fp3cons0Diff := fp3cons0After.Sub(fp3cons0Before...)
	fp4cons4Diff := fp4cons4After.Sub(fp4cons4Before...)

	return bbnCommDiff, del1Diff, del2Diff, fp1bbnDiff, fp2cons0Diff, fp3cons0Diff, fp4cons4Diff
}
