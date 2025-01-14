package e2e

import (
	"math/rand"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/app"
	appparams "github.com/babylonlabs-io/babylon/app/params"
	v1 "github.com/babylonlabs-io/babylon/app/upgrades/v1"
	"github.com/babylonlabs-io/babylon/app/upgrades/v1/testnet"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	"github.com/babylonlabs-io/babylon/testutil/sample"
	btclighttypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"

	"github.com/babylonlabs-io/babylon/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/test/e2e/configurer/config"
	"github.com/babylonlabs-io/babylon/test/e2e/util"
)

type SoftwareUpgradeV1TestnetTestSuite struct {
	suite.Suite

	configurer            *configurer.UpgradeConfigurer
	balancesBeforeUpgrade map[string]sdk.Coin
}

func (s *SoftwareUpgradeV1TestnetTestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite...")
	var err error
	s.balancesBeforeUpgrade = make(map[string]sdk.Coin)

	btcHeaderGenesis := sample.SignetBtcHeader195552(s.T())

	tokenDistData, err := v1.LoadTokenDistributionFromData(testnet.TokensDistributionStr)
	s.NoError(err)

	balanceToMintByAddr := make(map[string]int64)
	for _, td := range tokenDistData.TokenDistribution {
		balanceToMintByAddr[td.AddressSender] += td.Amount
		balanceToMintByAddr[td.AddressReceiver] += 0
	}

	// func only runs right before the upgrade proposal is sent
	preUpgradeFunc := func(chains []*chain.Config) {
		node := chains[0].NodeConfigs[1]
		uniqueAddrsTokenReceivers := make(map[string]any)

		for addr, amountToMint := range balanceToMintByAddr {
			s.balancesBeforeUpgrade[addr] = sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.ZeroInt())
			if amountToMint <= 0 {
				continue
			}

			uniqueAddrsTokenReceivers[addr] = struct{}{}
			amountToSend := sdk.NewCoin(appparams.BaseCoinUnit, sdkmath.NewInt(amountToMint))
			node.BankSendFromNode(addr, amountToSend.String())
		}

		// needs to wait for a block to make sure the send tx was processed and
		// it queries the real balances before upgrade.
		node.WaitForNextBlock()

		// only verifies the balances of the ones that had to receive something
		// from the node and all the others should be zero
		for addr := range uniqueAddrsTokenReceivers {
			balance, err := node.QueryBalance(addr, appparams.DefaultBondDenom)
			s.NoError(err)
			s.balancesBeforeUpgrade[addr] = *balance
		}
	}

	cfg, err := configurer.NewSoftwareUpgradeConfigurer(
		s.T(),
		true,
		config.UpgradeSignetLaunchFilePath,
		[]*btclighttypes.BTCHeaderInfo{btcHeaderGenesis},
		preUpgradeFunc,
	)
	s.NoError(err)
	s.configurer = cfg

	err = s.configurer.ConfigureChains()
	s.NoError(err)
	err = s.configurer.RunSetup() // upgrade happens at the setup of configurer.
	s.Require().NoError(err)
}

func (s *SoftwareUpgradeV1TestnetTestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

// TestUpgradeV1 Checks if the BTC Headers were inserted.
func (s *SoftwareUpgradeV1TestnetTestSuite) TestUpgradeV1() {
	// chain is already upgraded, only checks for differences in state are expected
	chainA := s.configurer.GetChainConfig(0)

	n, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	govProp, err := s.configurer.ParseGovPropFromFile()
	s.NoError(err)
	chainA.WaitUntilHeight(govProp.Plan.Height + 1) // waits for chain to produce blocks

	r := rand.New(rand.NewSource(time.Now().Unix()))
	fptBTCSK, _, _ := datagen.GenRandomBTCKeyPair(r)
	fp := CreateNodeFPFromNodeAddr(
		s.T(),
		r,
		fptBTCSK,
		n,
	)

	bbnApp := app.NewTmpBabylonApp()

	// makes sure that the upgrade was actually executed
	expectedUpgradeHeight := govProp.Plan.Height
	resp := n.QueryAppliedPlan(v1.UpgradeName)
	s.EqualValues(expectedUpgradeHeight, resp.Height, "the plan should be applied at the height %d", expectedUpgradeHeight)

	btcHeadersInserted, err := v1.LoadBTCHeadersFromData(bbnApp.AppCodec(), testnet.NewBtcHeadersStr)
	s.NoError(err)

	lenHeadersInserted := len(btcHeadersInserted)
	oldHeadersStoredLen := 1 // only block zero is set by default in genesis for e2e test

	// needs to do pagination as the default query only returns the first 100
	storedBtcHeadersResp := n.QueryBtcLightClientMainchainAll()
	storedHeadersLen := len(storedBtcHeadersResp)
	s.Equal(storedHeadersLen, oldHeadersStoredLen+lenHeadersInserted)

	// ensure the headers were inserted at the end
	for i := 0; i < lenHeadersInserted; i++ {
		headerInserted := btcHeadersInserted[i]
		reversedStoredIndex := storedHeadersLen - (oldHeadersStoredLen + i + 1)
		headerStoredResp := storedBtcHeadersResp[reversedStoredIndex] // reverse reading

		s.EqualValues(headerInserted.Header.MarshalHex(), headerStoredResp.HeaderHex)
	}

	// check that staking params correctly deserialize and that they are the same
	// as the one from the data
	stakingParams := n.QueryBTCStakingParams()

	bsParamsFromUpgrade, err := v1.LoadBtcStakingParamsFromData(testnet.BtcStakingParamsStr)
	s.NoError(err)

	lastParamInUpgradeData := bsParamsFromUpgrade[len(bsParamsFromUpgrade)-1]
	s.EqualValues(lastParamInUpgradeData, *stakingParams)

	// expected version starts at 0 since the upgrade overwrites the params
	for expVersion, paramsInUpgradeData := range bsParamsFromUpgrade {
		bsParamsAtBtcHeight := n.QueryBTCStakingParamsByVersion(uint32(expVersion))
		s.Equal(*bsParamsAtBtcHeight, paramsInUpgradeData)
	}

	// check that finality params correctly deserialize and that they are the same
	// as the one from the data
	finalityParams := n.QueryFinalityParams()

	finalityParamsFromData, err := v1.LoadFinalityParamsFromData(bbnApp.AppCodec(), testnet.FinalityParamStr)
	s.NoError(err)
	s.EqualValues(finalityParamsFromData, *finalityParams)

	// check that incentive params correctly deserialize and that they are the same
	// as the one from the data
	incentiveParams, err := n.QueryIncentiveParams()
	s.NoError(err)

	incentiveParamsFromData, err := v1.LoadIncentiveParamsFromData(bbnApp.AppCodec(), testnet.IncentiveParamStr)
	s.NoError(err)
	s.EqualValues(incentiveParamsFromData, *incentiveParams)

	// FP tries to commit with start height before finality activation height
	// it should fail, after commits with start height = finality activation height
	// and it should work.
	_, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(r, fptBTCSK, finalityParamsFromData.FinalityActivationHeight-1, finalityParamsFromData.MinPubRand)
	s.NoError(err)
	n.CommitPubRandList(
		fp.BtcPk,
		msgCommitPubRandList.StartHeight,
		msgCommitPubRandList.NumPubRand,
		msgCommitPubRandList.Commitment,
		msgCommitPubRandList.Sig,
	)
	// the tx does not fails, but it actually
	// does not commits for that height.
	listByHeight := n.QueryListPubRandCommit(fp.BtcPk)
	_, listFound := listByHeight[finalityParamsFromData.FinalityActivationHeight]
	s.False(listFound, "this list should not exists, because the msg should have failed")

	// commits with valid start height
	_, msgCommitPubRandList, err = datagen.GenRandomMsgCommitPubRandList(r, fptBTCSK, finalityParamsFromData.FinalityActivationHeight, finalityParamsFromData.MinPubRand)
	s.NoError(err)
	n.WaitForNextBlock()
	n.CommitPubRandList(
		msgCommitPubRandList.FpBtcPk,
		msgCommitPubRandList.StartHeight,
		msgCommitPubRandList.NumPubRand,
		msgCommitPubRandList.Commitment,
		msgCommitPubRandList.Sig,
	)

	n.WaitForNextBlock()

	listByHeight = n.QueryListPubRandCommit(msgCommitPubRandList.FpBtcPk)
	_, listFound = listByHeight[finalityParamsFromData.FinalityActivationHeight]
	s.True(listFound, "this list should exists, because the msg sent is after the activation height")

	// Verifies the balance differences were really executed
	tokenDistData, err := v1.LoadTokenDistributionFromData(testnet.TokensDistributionStr)
	s.NoError(err)

	balanceDiffByAddr := make(map[string]int64)
	for _, td := range tokenDistData.TokenDistribution {
		balanceDiffByAddr[td.AddressSender] -= td.Amount
		balanceDiffByAddr[td.AddressReceiver] += td.Amount
	}

	for addr, diff := range balanceDiffByAddr {
		coinDiff := sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(util.Abs(diff)))
		expectedBalance := s.balancesBeforeUpgrade[addr].Add(coinDiff)
		if diff < 0 {
			expectedBalance = s.balancesBeforeUpgrade[addr].Sub(coinDiff)
		}

		balanceAfterUpgrade, err := n.QueryBalance(addr, appparams.DefaultBondDenom)
		s.NoError(err)

		expBalance := expectedBalance.String()
		actBalance := balanceAfterUpgrade.String()
		s.Equal(expBalance, actBalance, "addr %s has different balances. Expected %s != %s Actual", addr, expBalance, actBalance)
	}
}
