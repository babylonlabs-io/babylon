package e2e

import (
	"fmt"
	"math/rand"
	"time"

	"cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/babylonlabs-io/babylon/v3/app/params"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v3/testutil/coins"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	bsctypes "github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
	itypes "github.com/babylonlabs-io/babylon/v3/x/incentive/types"
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
	// Since the rewards are combined by their bech32 address, del2
	// will have 10_00000000 and del1 will have 4_00000000 as voting power
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

	actualFps := n2.QueryFinalityProviders("")
	require.Len(s.T(), actualFps, 4, "should have created all the FPs to start the test")
	s.T().Log("All Fps created")
}

// Test2CreateFinalityProviders creates the first 3 btc delegations
// with the same values, but different satoshi staked amounts
func (s *BtcRewardsDistributionBsnRollup) Test2CreateFirstBtcDelegations() {
	n2, err := s.configurer.GetChainConfig(0).GetNodeAtIndex(2)
	s.NoError(err)

	s.del1Addr = n2.KeysAdd(wDel1)
	s.del2Addr = n2.KeysAdd(wDel2)

	n2.BankMultiSendFromNode([]string{s.del1Addr, s.del2Addr}, "1000000ubbn")

	// TODO(rafilx): add bsn rewards to FP without adding voting power, should fail

	n2.WaitForNextBlocks(2)

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
	require.Greater(s.T(), rewardCoins.Len(), 2, "should have 2 or more denoms to give out as rewards")

	fp2Ratio, fp3Ratio := math.LegacyMustNewDecFromStr("0.7"), math.LegacyMustNewDecFromStr("0.3")

	bbnCommDiff, del1Diff, del2Diff, fp1bbnDiff, fp2cons0Diff, fp3cons0Diff, fp4cons4Diff := s.SuiteRewardsDiff(n2, func() {
		n2.AddBsnRewards(n2.WalletName, s.bsn0.ConsumerId, rewardCoins, []bstypes.FpRatio{
			bstypes.FpRatio{
				BtcPk: s.fp2cons0.BtcPk,
				Ratio: fp2Ratio,
			},
			bstypes.FpRatio{
				BtcPk: s.fp3cons0.BtcPk,
				Ratio: fp3Ratio,
			},
		})
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

	// Current setup of voting power
	// (fp2, del1) => 2_00000000
	// (fp2, del2) => 4_00000000
	// (fp3, del1) => 2_00000000

	fp2RemainingBtcRewards := fp2AfterRatio.Sub(fp2CommExp...)
	fp3RemainingBtcRewards := fp3AfterRatio.Sub(fp3CommExp...)

	// del1 will receive all the rewards of fp3 and 1/3 of the fp2 rewards
	expectedRewardsDel1 := coins.CalculatePercentageOfCoins(fp2RemainingBtcRewards, 33).Add(fp3RemainingBtcRewards...)
	coins.RequireCoinsDiffInPointOnePercentMargin(s.T(), expectedRewardsDel1, del1Diff)
	// del2 will receive 2/3 of the fp2 rewards
	expectedRewardsDel2 := coins.CalculatePercentageOfCoins(fp2RemainingBtcRewards, 66)
	coins.RequireCoinsDiffInPointOnePercentMargin(s.T(), expectedRewardsDel2, del2Diff)

	require.True(s.T(), fp1bbnDiff.IsZero(), "fp1 was not rewarded")
	require.True(s.T(), fp4cons4Diff.IsZero(), "fp4 was not rewarded")

	// check reward gauges accordingly to ratio and commissions

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
	// fps := n2.QueryFinalityProviders("")
	// s.Len(fps, 4)
	// for _, fp := range fps {
	// 	s.Equal(fp.SlashedBabylonHeight, uint64(0))
	// 	s.Equal(fp.SlashedBtcHeight, uint32(0))
	// }

	// dels := n2.QueryFinalityProvidersDelegations(s.fp1bbn.BtcPk.MarshalHex())
	// s.Len(dels, 3)
	// for _, del := range dels {
	// 	s.True(del.Active)
	// }

	// // makes sure there is some reward there
	// s.Eventually(func() bool {
	// 	_, errFp1 := n2.QueryRewardGauge(s.fp1bbn.Address())
	// 	_, errFp2 := n2.QueryRewardGauge(s.fp2cons0.Address())
	// 	_, errDel1 := n2.QueryRewardGauge(sdk.MustAccAddressFromBech32(s.del1Addr))
	// 	_, errDel2 := n2.QueryRewardGauge(sdk.MustAccAddressFromBech32(s.del2Addr))
	// 	return errFp1 == nil && errFp2 == nil && errDel1 == nil && errDel2 == nil
	// }, time.Minute*2, time.Second*3, "wait to have some rewards available in the gauge")

	// // The rewards distributed for the finality providers should be fp1 => 3x, fp2 => 1x
	// fp1DiffRewards, fp2DiffRewards, del1DiffRewards, del2DiffRewards := s.QueryRewardGauges(n2)

	// coins.RequireCoinsDiffInPointOnePercentMargin(
	// 	s.T(),
	// 	fp2DiffRewards.Coins.MulInt(sdkmath.NewIntFromUint64(3)),
	// 	fp1DiffRewards.Coins,
	// )

	// // The rewards distributed to the delegators should be the same for each delegator
	// coins.RequireCoinsDiffInPointOnePercentMargin(s.T(), del1DiffRewards.Coins, del2DiffRewards.Coins)

	// CheckWithdrawReward(s.T(), n2, wDel2, s.del2Addr)

	// s.AddFinalityVoteUntilCurrentHeight()
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

// QueryDelRewards returns the rewards available for del1, del2
func (s *BtcRewardsDistributionBsnRollup) QueryDelRewards(n *chain.NodeConfig) (
	del1coins, del2coins sdk.Coins,
) {
	g := new(errgroup.Group)
	var (
		err                 error
		btcDel1RewardGauges map[string]*itypes.RewardGaugesResponse
		btcDel2RewardGauges map[string]*itypes.RewardGaugesResponse
	)
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

	_ = g.Wait()

	btcDel1RewardCoins := sdk.NewCoins()
	btcDel1RewardGauge, ok := btcDel1RewardGauges[itypes.BTC_STAKER.String()]
	if ok {
		btcDel1RewardCoins = btcDel1RewardGauge.Coins
	}

	btcDel2RewardCoins := sdk.NewCoins()
	btcDel2RewardGauge, ok := btcDel2RewardGauges[itypes.BTC_STAKER.String()]
	if ok {
		btcDel2RewardCoins = btcDel2RewardGauge.Coins
	}
	return btcDel1RewardCoins, btcDel2RewardCoins
}

// QuerySuiteRewards returns the babylon commission account balance and fp, dels
// available rewards
func (s *BtcRewardsDistributionBsnRollup) QuerySuiteRewards(n *chain.NodeConfig) (
	bbnComm, del1, del2, fp1bbn, fp2cons0, fp3cons0, fp4cons4 sdk.Coins,
) {
	bbnComm, err := n.QueryBalances(params.AccBbnComissionCollectorBsn.String())
	require.NoError(s.T(), err)

	fp1bbn, fp2cons0, fp3cons0, fp4cons4 = s.QueryFpRewards(n)
	del1, del2 = s.QueryDelRewards(n)
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

// // Test6ActiveLastDelegation creates a new btc delegation
// // (fp2, del2) with 6_00000000 sats and sends the covenant signatures
// // needed.
// func (s *BtcRewardsDistributionBsnRollup) Test6ActiveLastDelegation() {
// 	chainA := s.configurer.GetChainConfig(0)
// 	n2, err := chainA.GetNodeAtIndex(2)
// 	s.NoError(err)
// 	// covenants are at n1
// 	n1, err := chainA.GetNodeAtIndex(1)
// 	s.NoError(err)

// 	// fp2Del2
// 	s.CreateBTCDelegationAndCheck(n2, wDel2, s.fp2cons0, s.del2BTCSK, s.del2Addr, s.fp2Del2StakingAmt)

// 	s.AddFinalityVoteUntilCurrentHeight()

// 	allDelegations := n2.QueryFinalityProvidersDelegations(s.fp1bbn.BtcPk.MarshalHex(), s.fp2cons0.BtcPk.MarshalHex())
// 	s.Equal(len(allDelegations), 4)

// 	pendingDels := make([]*bstypes.BTCDelegationResponse, 0)
// 	for _, delegation := range allDelegations {
// 		if !strings.EqualFold(delegation.StatusDesc, bstypes.BTCDelegationStatus_PENDING.String()) {
// 			continue
// 		}
// 		pendingDels = append(pendingDels, delegation)
// 	}

// 	s.Equal(len(pendingDels), 1)
// 	pendingDel, err := chain.ParseRespBTCDelToBTCDel(pendingDels[0])
// 	s.NoError(err)

// 	SendCovenantSigsToPendingDel(s.r, s.T(), n1, s.net, s.covenantSKs, s.covenantWallets, pendingDel)

// 	// wait for a block so that covenant txs take effect
// 	n1.WaitForNextBlock()

// 	s.AddFinalityVoteUntilCurrentHeight()

// 	// ensure that all BTC delegation are active
// 	allDelegations = n1.QueryFinalityProvidersDelegations(s.fp1bbn.BtcPk.MarshalHex(), s.fp2cons0.BtcPk.MarshalHex())
// 	s.Len(allDelegations, 4)
// 	for _, activeDel := range allDelegations {
// 		s.True(activeDel.Active)
// 	}
// }

// // Test7CheckRewards verifies the rewards of all the delegations
// // and finality provider
// func (s *BtcRewardsDistributionBsnRollup) Test7CheckRewards() {
// 	n2, err := s.configurer.GetChainConfig(0).GetNodeAtIndex(2)
// 	s.NoError(err)

// 	n2.WaitForNextBlock()
// 	s.AddFinalityVoteUntilCurrentHeight()

// 	// Current setup of voting power
// 	// (fp1, del1) => 2_00000000
// 	// (fp1, del2) => 4_00000000
// 	// (fp2, del1) => 2_00000000
// 	// (fp2, del2) => 6_00000000

// 	// The sum per bech32 address will be
// 	// (fp1)  => 6_00000000
// 	// (fp2)  => 8_00000000
// 	// (del1) => 4_00000000
// 	// (del2) => 10_00000000

// 	// gets the difference in rewards in 4 blocks range
// 	fp1DiffRewards, fp2DiffRewards, del1DiffRewards, del2DiffRewards := s.GetRewardDifferences(4)

// 	// Check the difference in the finality providers
// 	// fp1 should receive ~75% of the rewards received by fp2
// 	expectedRwdFp1 := coins.CalculatePercentageOfCoins(fp2DiffRewards, 75)
// 	coins.RequireCoinsDiffInPointOnePercentMargin(s.T(), fp1DiffRewards, expectedRwdFp1)

// 	// Check the difference in the delegators
// 	// the del1 should receive ~40% of the rewards received by del2
// 	expectedRwdDel1 := coins.CalculatePercentageOfCoins(del2DiffRewards, 40)
// 	coins.RequireCoinsDiffInPointOnePercentMargin(s.T(), del1DiffRewards, expectedRwdDel1)

// 	fp1DiffRewardsStr := fp1DiffRewards.String()
// 	fp2DiffRewardsStr := fp2DiffRewards.String()
// 	del1DiffRewardsStr := del1DiffRewards.String()
// 	del2DiffRewardsStr := del2DiffRewards.String()

// 	s.NotEmpty(fp1DiffRewardsStr)
// 	s.NotEmpty(fp2DiffRewardsStr)
// 	s.NotEmpty(del1DiffRewardsStr)
// 	s.NotEmpty(del2DiffRewardsStr)
// }

// // Test8SlashFp slashes the finality provider, but should continue to produce blocks
// func (s *BtcRewardsDistributionBsnRollup) Test8SlashFp() {
// 	chainA := s.configurer.GetChainConfig(0)
// 	n2, err := chainA.GetNodeAtIndex(2)
// 	s.NoError(err)

// 	badBlockHeightToVote := s.finalityBlockHeightVoted + 1

// 	blockToVote, err := n2.QueryBlock(int64(badBlockHeightToVote))
// 	s.NoError(err)
// 	appHash := blockToVote.AppHash

// 	// generate bad EOTS signature with a diff block height to vote
// 	fpFinVoteContext := signingcontext.FpFinVoteContextV0(n2.ChainID(), appparams.AccFinality.String())

// 	msgToSign := []byte(fpFinVoteContext)
// 	msgToSign = append(msgToSign, sdk.Uint64ToBigEndian(s.finalityBlockHeightVoted)...)
// 	msgToSign = append(msgToSign, appHash...)

// 	fp1Sig, err := eots.Sign(s.fp2cons0BTCSK, s.fp2RandListInfo.SRList[s.finalityIdx], msgToSign)
// 	s.NoError(err)

// 	finalitySig := bbn.NewSchnorrEOTSSigFromModNScalar(fp1Sig)

// 	// submit finality signature to slash
// 	n2.AddFinalitySigFromVal(
// 		s.fp2cons0.BtcPk,
// 		s.finalityBlockHeightVoted,
// 		&s.fp2RandListInfo.PRList[s.finalityIdx],
// 		*s.fp2RandListInfo.ProofList[s.finalityIdx].ToProto(),
// 		appHash,
// 		finalitySig,
// 	)

// 	n2.WaitForNextBlocks(2)

// 	fps := n2.QueryFinalityProviders("")
// 	require.Len(s.T(), fps, 2)
// 	for _, fp := range fps {
// 		if strings.EqualFold(fp.Addr, s.fp1Addr) {
// 			require.Zero(s.T(), fp.SlashedBabylonHeight)
// 			continue
// 		}
// 		require.NotZero(s.T(), fp.SlashedBabylonHeight)
// 	}

// 	// wait a few blocks to check if it doesn't panic when rewards are being produced
// 	n2.WaitForNextBlocks(5)
// }

// func (s *BtcRewardsDistributionBsnRollup) GetRewardDifferences(blocksDiff uint64) (
// 	fp1DiffRewards, fp2DiffRewards, del1DiffRewards, del2DiffRewards sdk.Coins,
// ) {
// 	chainA := s.configurer.GetChainConfig(0)
// 	n2, err := chainA.GetNodeAtIndex(2)
// 	s.NoError(err)

// 	fp1RewardGaugePrev, fp2RewardGaugePrev, btcDel1RewardGaugePrev, btcDel2RewardGaugePrev := s.QueryRewardGauges(n2)
// 	// wait a few block of rewards to calculate the difference
// 	for i := 1; i <= int(blocksDiff); i++ {
// 		if i%2 == 0 {
// 			s.AddFinalityVoteUntilCurrentHeight()
// 		}
// 		n2.WaitForNextBlock()
// 	}

// 	fp1RewardGauge, fp2RewardGauge, btcDel1RewardGauge, btcDel2RewardGauge := s.QueryRewardGauges(n2)

// 	// since varius block were created, it is needed to get the difference
// 	// from a certain point where all the delegations were active to properly
// 	// calculate the distribution with the voting power structure with 4 BTC delegations active
// 	// Note: if a new block is mined during the query of reward gauges, the calculation might be a
// 	// bit off by some ubbn
// 	fp1DiffRewards = fp1RewardGauge.Coins.Sub(fp1RewardGaugePrev.Coins...)
// 	fp2DiffRewards = fp2RewardGauge.Coins.Sub(fp2RewardGaugePrev.Coins...)
// 	del1DiffRewards = btcDel1RewardGauge.Coins.Sub(btcDel1RewardGaugePrev.Coins...)
// 	del2DiffRewards = btcDel2RewardGauge.Coins.Sub(btcDel2RewardGaugePrev.Coins...)

// 	s.AddFinalityVoteUntilCurrentHeight()
// 	return fp1DiffRewards, fp2DiffRewards, del1DiffRewards, del2DiffRewards
// }

// func (s *BtcRewardsDistributionBsnRollup) AddFinalityVoteUntilCurrentHeight() {
// 	chainA := s.configurer.GetChainConfig(0)
// 	n1, err := chainA.GetNodeAtIndex(1)
// 	s.NoError(err)
// 	n2, err := chainA.GetNodeAtIndex(2)
// 	s.NoError(err)

// 	currentBlock := n2.LatestBlockNumber()

// 	accN1, err := n1.QueryAccount(s.fp1bbn.Addr)
// 	s.NoError(err)
// 	accN2, err := n1.QueryAccount(s.fp2cons0.Addr)
// 	s.NoError(err)

// 	accNumberN1 := accN1.GetAccountNumber()
// 	accSequenceN1 := accN1.GetSequence()

// 	accNumberN2 := accN2.GetAccountNumber()
// 	accSequenceN2 := accN2.GetSequence()

// 	for s.finalityBlockHeightVoted < currentBlock {
// 		n1Flags := []string{
// 			"--offline",
// 			fmt.Sprintf("--account-number=%d", accNumberN1),
// 			fmt.Sprintf("--sequence=%d", accSequenceN1),
// 			fmt.Sprintf("--from=%s", wFp1),
// 		}
// 		n2Flags := []string{
// 			"--offline",
// 			fmt.Sprintf("--account-number=%d", accNumberN2),
// 			fmt.Sprintf("--sequence=%d", accSequenceN2),
// 			fmt.Sprintf("--from=%s", wFp2),
// 		}
// 		s.AddFinalityVote(n1Flags, n2Flags)

// 		accSequenceN1++
// 		accSequenceN2++
// 	}
// }

// func (s *BtcRewardsDistributionBsnRollup) AddFinalityVote(flagsN1, flagsN2 []string) (appHash bytes.HexBytes) {
// 	chainA := s.configurer.GetChainConfig(0)
// 	n2, err := chainA.GetNodeAtIndex(2)
// 	s.NoError(err)
// 	n1, err := chainA.GetNodeAtIndex(1)
// 	s.NoError(err)

// 	s.finalityIdx++
// 	s.finalityBlockHeightVoted++

// 	appHash = n1.AddFinalitySignatureToBlock(
// 		s.fp1bbnBTCSK,
// 		s.fp1bbn.BtcPk,
// 		s.finalityBlockHeightVoted,
// 		s.fp1RandListInfo.SRList[s.finalityIdx],
// 		&s.fp1RandListInfo.PRList[s.finalityIdx],
// 		*s.fp1RandListInfo.ProofList[s.finalityIdx].ToProto(),
// 		flagsN1...,
// 	)

// 	n2.AddFinalitySignatureToBlock(
// 		s.fp2cons0BTCSK,
// 		s.fp2cons0.BtcPk,
// 		s.finalityBlockHeightVoted,
// 		s.fp2RandListInfo.SRList[s.finalityIdx],
// 		&s.fp2RandListInfo.PRList[s.finalityIdx],
// 		*s.fp2RandListInfo.ProofList[s.finalityIdx].ToProto(),
// 		flagsN2...,
// 	)

// 	return appHash
// }

// // QueryRewardGauges returns the rewards available for fp1, fp2, del1, del2
// func (s *BtcRewardsDistributionBsnRollup) QueryRewardGauges(n *chain.NodeConfig) (
// 	fp1, fp2, del1, del2 *itypes.RewardGaugesResponse,
// ) {
// 	n.WaitForNextBlockWithSleep50ms()

// 	g := new(errgroup.Group)
// 	var (
// 		err                 error
// 		fp1RewardGauges     map[string]*itypes.RewardGaugesResponse
// 		fp2RewardGauges     map[string]*itypes.RewardGaugesResponse
// 		btcDel1RewardGauges map[string]*itypes.RewardGaugesResponse
// 		btcDel2RewardGauges map[string]*itypes.RewardGaugesResponse
// 	)

// 	g.Go(func() error {
// 		fp1RewardGauges, err = n.QueryRewardGauge(s.fp1bbn.Address())
// 		if err != nil {
// 			return fmt.Errorf("failed to query rewards for fp1: %w", err)
// 		}
// 		return nil
// 	})
// 	g.Go(func() error {
// 		fp2RewardGauges, err = n.QueryRewardGauge(s.fp2cons0.Address())
// 		if err != nil {
// 			return fmt.Errorf("failed to query rewards for fp2: %w", err)
// 		}
// 		return nil
// 	})
// 	g.Go(func() error {
// 		btcDel1RewardGauges, err = n.QueryRewardGauge(sdk.MustAccAddressFromBech32(s.del1Addr))
// 		if err != nil {
// 			return fmt.Errorf("failed to query rewards for del1: %w", err)
// 		}
// 		return nil
// 	})
// 	g.Go(func() error {
// 		btcDel2RewardGauges, err = n.QueryRewardGauge(sdk.MustAccAddressFromBech32(s.del2Addr))
// 		if err != nil {
// 			return fmt.Errorf("failed to query rewards for del2: %w", err)
// 		}
// 		return nil
// 	})
// 	s.NoError(g.Wait())

// 	fp1RewardGauge, ok := fp1RewardGauges[itypes.FINALITY_PROVIDER.String()]
// 	s.True(ok)
// 	s.True(fp1RewardGauge.Coins.IsAllPositive())

// 	fp2RewardGauge, ok := fp2RewardGauges[itypes.FINALITY_PROVIDER.String()]
// 	s.True(ok)
// 	s.True(fp2RewardGauge.Coins.IsAllPositive())

// 	btcDel1RewardGauge, ok := btcDel1RewardGauges[itypes.BTC_STAKER.String()]
// 	s.True(ok)
// 	s.True(btcDel1RewardGauge.Coins.IsAllPositive())

// 	btcDel2RewardGauge, ok := btcDel2RewardGauges[itypes.BTC_STAKER.String()]
// 	s.True(ok)
// 	s.True(btcDel2RewardGauge.Coins.IsAllPositive())

// 	return fp1RewardGauge, fp2RewardGauge, btcDel1RewardGauge, btcDel2RewardGauge
// }
