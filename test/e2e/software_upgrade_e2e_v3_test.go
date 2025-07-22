package e2e

import (
	"math/rand"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"

	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	v3 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v3"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v3/testutil/sample"
	btclighttypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"

	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer/config"
)

const (
	btcstkconsumerModulePath = "btcstkconsumer"
	zoneconciergeModulePath  = "zoneconcierge"
)

type SoftwareUpgradeV3TestSuite struct {
	suite.Suite

	configurer            *configurer.UpgradeConfigurer
	balancesBeforeUpgrade map[string]sdk.Coin
	fp1Addr               string
	fp2Addr               string
	r                     *rand.Rand
	fp1BTCSK              *btcec.PrivateKey
	fp2BTCSK              *btcec.PrivateKey
	fp1                   *bstypes.FinalityProvider
	fp2                   *bstypes.FinalityProvider
}

func (s *SoftwareUpgradeV3TestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite for v2.2.0 to v3 upgrade...")
	var err error
	s.balancesBeforeUpgrade = make(map[string]sdk.Coin)

	btcHeaderGenesis := sample.SignetBtcHeader195552(s.T())

	// func runs right before the upgrade proposal is sent
	preUpgradeFunc := func(chains []*chain.Config) {
		node := chains[0].NodeConfigs[1]

		// Record some balances before the upgrade to verify after
		addresses := []string{
			node.PublicAddress,
			chains[0].NodeConfigs[0].PublicAddress,
		}

		for _, addr := range addresses {
			balance, err := node.QueryBalance(addr, appparams.DefaultBondDenom)
			s.NoError(err)
			s.balancesBeforeUpgrade[addr] = *balance
		}
	}

	cfg, err := configurer.NewSoftwareUpgradeConfigurer(
		s.T(),
		true,
		config.UpgradeV3FilePath,
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

func (s *SoftwareUpgradeV3TestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

// TestUpgradeV3 checks if the upgrade from v2.2.0 to v3 was successful
func (s *SoftwareUpgradeV3TestSuite) TestUpgradeV3() {
	chainA := s.configurer.GetChainConfig(0)

	n, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)

	s.fp1Addr = n.KeysAdd("fp1Addr")
	s.fp2Addr = n.KeysAdd("fp2Addr")
	n.BankMultiSendFromNode([]string{s.fp1Addr, s.fp2Addr}, "1000000ubbn")

	n.WaitForNextBlock()

	s.r = rand.New(rand.NewSource(time.Now().UnixNano()))

	s.fp1BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.fp2BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)

	s.fp1 = CreateNodeFP(
		s.T(),
		s.r,
		s.fp1BTCSK,
		n,
		s.fp1Addr,
		n.ChainID(),
	)

	s.fp2 = CreateNodeFP(
		s.T(),
		s.r,
		s.fp2BTCSK,
		n,
		s.fp2Addr,
		n.ChainID(),
	)

	govProp, err := s.configurer.ParseGovPropFromFile()
	s.NoError(err)
	chainA.WaitUntilHeight(govProp.Plan.Height + 1) // waits for chain to produce blocks

	expectedUpgradeHeight := govProp.Plan.Height
	resp := n.QueryAppliedPlan(v3.UpgradeName)
	s.EqualValues(expectedUpgradeHeight, resp.Height, "the plan should be applied at the height %d", expectedUpgradeHeight)

	// check fps have the same chain id
	s.Require().Equal(s.fp2.BsnId, n.ChainID())

	fp1CommitPubRand := n.QueryListPubRandCommit(fp1CommitPubRandList.FpBtcPk)
	fp1PubRand := fp1CommitPubRand[commitStartHeight]
	s.Require().Equal(fp1PubRand.NumPubRand, numPubRand)

	fp2CommitPubRand := n.QueryListPubRandCommit(fp2CommitPubRandList.FpBtcPk)
	fp2PubRand := fp2CommitPubRand[commitStartHeight]
	s.Require().Equal(fp2PubRand.NumPubRand, numPubRand)

	// check btcstaking params has max finality provider set to 1
	var stakingParams map[string]interface{}
	n.QueryParams("btcstaking", &stakingParams)
	s.T().Logf("staking params: %v", stakingParams)

	btcstakingparams, exists := stakingParams["params"]
	s.Require().True(exists, "btcstakingparams params should exist")

	btcparamsMap, ok := btcstakingparams.(map[string]interface{})
	s.Require().True(ok, "btcstakingparams params should exist")

	maxFP, ok := btcparamsMap["max_finality_providers"]
	s.Require().True(ok, "max_finality_providers param should exist")
	s.Require().Equal(float64(1), maxFP, "max_finality_providers should be 1")

	// check that the module exists by querying parameters with the QueryParams helper
	var btcstkconsumerParams map[string]interface{}
	n.QueryParams(btcstkconsumerModulePath, &btcstkconsumerParams)
	s.T().Logf("btcstkconsumer params: %v", btcstkconsumerParams)

	params, exists := btcstkconsumerParams["params"]
	s.Require().True(exists, "btcstkconsumer params should exist")

	paramsMap, ok := params.(map[string]interface{})
	s.Require().True(ok, "btcstkconsumer params should be a map")

	_, permissionedExists := paramsMap["permissioned_integration"]
	s.Require().True(permissionedExists, "permissioned_integration field should exist in btcstkconsumer params")

	registeredConsumers := n.QueryBTCStkConsumerConsumers()
	s.T().Logf("registered consumers: %v", registeredConsumers)

	if len(registeredConsumers) > 0 {
		consumerIDs := make([]string, len(registeredConsumers))
		for i, consumer := range registeredConsumers {
			consumerIDs[i] = consumer.ConsumerId
		}

		finalisedBsnsInfoResp := n.QueryZoneConciergeFinalizedBsnsInfo(consumerIDs, false)
		s.NoError(err, "zoneconcierge FinalizedBsnsInfo query should succeed")
		s.T().Logf("zoneconcierge FinalizedBsnsInfo: %v", finalisedBsnsInfoResp)
	} else {
		s.T().Log("No registered consumers found, skipping finalized-bsns-info query")
	}

	n.WaitForNextBlock()

	// TODO: Add more functionality checks here as they are added
}
