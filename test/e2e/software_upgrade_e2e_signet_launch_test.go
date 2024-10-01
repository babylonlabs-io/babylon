package e2e

import (
	"sort"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/app"
	appparams "github.com/babylonlabs-io/babylon/app/params"
	v1 "github.com/babylonlabs-io/babylon/app/upgrades/v1"
	"github.com/babylonlabs-io/babylon/app/upgrades/v1/testnet"
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

	btcHeaderGenesis, err := app.SignetBtcHeaderGenesis(app.NewTmpBabylonApp().AppCodec())
	s.NoError(err)

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
		uniqueAddrs := make(map[string]any)

		for addr, amountToMint := range balanceToMintByAddr {
			uniqueAddrs[addr] = struct{}{}
			if amountToMint <= 0 {
				continue
			}

			amountToSend := sdk.NewCoin(appparams.BaseCoinUnit, sdkmath.NewInt(amountToMint))
			node.BankSendFromNode(addr, amountToSend.String())
		}

		// needs to wait for a block to make sure the send tx was processed and
		// it queries the real balances before upgrade.
		node.WaitForNextBlock()
		for addr := range uniqueAddrs {
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

// TestUpgradeSignetLaunch Checks if the BTC Headers were inserted.
func (s *SoftwareUpgradeV1TestnetTestSuite) TestUpgradeSignetLaunch() {
	// chain is already upgraded, only checks for differences in state are expected
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(30) // five blocks more than upgrade

	n, err := chainA.GetDefaultNode()
	s.NoError(err)

	govProp, err := s.configurer.ParseGovPropFromFile()
	s.NoError(err)

	bbnApp := app.NewTmpBabylonApp()

	// makes sure that the upgrade was actually executed
	expectedUpgradeHeight := govProp.Plan.Height
	resp := n.QueryAppliedPlan(v1.UpgradeName)
	s.EqualValues(expectedUpgradeHeight, resp.Height, "the plan should be applied at the height %d", expectedUpgradeHeight)

	btcHeadersInserted, err := v1.LoadBTCHeadersFromData(bbnApp.AppCodec(), testnet.NewBtcHeadersStr)
	s.NoError(err)

	lenHeadersInserted := len(btcHeadersInserted)
	oldHeadersStoredLen := 1 // only block zero is set by default in genesis for e2e test

	storedBtcHeadersResp := n.QueryBtcLightClientMainchain()
	storedHeadersLen := len(storedBtcHeadersResp)
	s.Equal(storedHeadersLen, oldHeadersStoredLen+lenHeadersInserted)

	// ensure the headers were inserted at the end
	for i := 0; i < lenHeadersInserted; i++ {
		headerInserted := btcHeadersInserted[i]
		reversedStoredIndex := storedHeadersLen - (oldHeadersStoredLen + i + 1)
		headerStoredResp := storedBtcHeadersResp[reversedStoredIndex] // reverse reading

		s.EqualValues(headerInserted.Header.MarshalHex(), headerStoredResp.HeaderHex)
	}

	oldFPsLen := 0 // it should not have any FP
	fpsFromNode := n.QueryFinalityProviders()

	fpsInserted, err := v1.LoadSignedFPsFromData(bbnApp.AppCodec(), bbnApp.TxConfig().TxJSONDecoder(), testnet.FinalityParamStr)
	s.NoError(err)
	s.Equal(len(fpsInserted), len(fpsFromNode)+oldFPsLen)

	// sorts all the FPs from node to match the ones from loaded string json
	sort.Slice(fpsFromNode, func(i, j int) bool {
		return fpsFromNode[i].Addr > fpsFromNode[j].Addr
	})

	for i, fpInserted := range fpsInserted {
		fpFromKeeper := fpsFromNode[i]
		s.EqualValues(fpFromKeeper.Addr, fpInserted.Addr)
		s.EqualValues(fpFromKeeper.Description, fpInserted.Description)
		s.EqualValues(fpFromKeeper.Commission.String(), fpInserted.Commission.String())
		s.EqualValues(fpFromKeeper.Pop.String(), fpInserted.Pop.String())
	}

	// check that staking params correctly deserialize and that they are the same
	// as the one from the data
	stakingParams := n.QueryBTCStakingParams()

	stakingParamsFromData, err := v1.LoadBtcStakingParamsFromData(bbnApp.AppCodec(), testnet.BtcStakingParamStr)
	s.NoError(err)

	s.EqualValues(stakingParamsFromData, *stakingParams)

	// check that finality params correctly deserialize and that they are the same
	// as the one from the data
	finalityParams := n.QueryFinalityParams()

	finalityParamsFromData, err := v1.LoadFinalityParamsFromData(bbnApp.AppCodec(), testnet.FinalityParamStr)
	s.NoError(err)
	s.EqualValues(finalityParamsFromData, *finalityParams)

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
