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
	"github.com/babylonlabs-io/babylon/v3/x/finality/types"

	"github.com/babylonlabs-io/babylon/v3/app/signingcontext"
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
	fp1CommitPubRandList  *types.MsgCommitPubRandList
	fp2CommitPubRandList  *types.MsgCommitPubRandList
	commitStartHeight     uint64
}

func (s *SoftwareUpgradeV3TestSuite) SetupSuite() {
	s.T().Skip("Temporarily skipping v3 upgrade test")
	s.T().Log("setting up e2e integration test suite for v2.2.0 to v3 upgrade...")
	var err error
	s.balancesBeforeUpgrade = make(map[string]sdk.Coin)

	btcHeaderGenesis := sample.SignetBtcHeader195552(s.T())

	// func runs right before the upgrade proposal is sent
	preUpgradeFunc := func(chains []*chain.Config) {
		n := chains[0].NodeConfigs[1]

		s.fp1Addr = n.KeysAdd("fp1Addr")
		s.fp2Addr = n.KeysAdd("fp2Addr")
		n.BankMultiSendFromNode([]string{s.fp1Addr, s.fp2Addr}, "900000ubbn")

		n.WaitForNextBlock()

		_, err := n.QueryBalance(s.fp1Addr, appparams.DefaultBondDenom)
		s.NoError(err)
		_, err = n.QueryBalance(s.fp2Addr, appparams.DefaultBondDenom)
		s.NoError(err)

		s.r = rand.New(rand.NewSource(time.Now().UnixNano()))

		s.fp1BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
		s.fp2BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)

		s.fp1 = CreateNodeFPV2(
			s.T(),
			s.r,
			s.fp1BTCSK,
			n,
			s.fp1Addr,
		)

		s.fp2 = CreateNodeFPV2(
			s.T(),
			s.r,
			s.fp2BTCSK,
			n,
			s.fp2Addr,
		)

		n.WaitForNextBlock()

		randCommitContext := signingcontext.FpRandCommitContextV0(n.ChainID(), appparams.AccFinality.String())
		numPubRand := uint64(100)

		s.commitStartHeight = n.LatestBlockNumber()
		_, s.fp1CommitPubRandList, err = datagen.GenRandomMsgCommitPubRandList(s.r, s.fp1BTCSK, randCommitContext, s.commitStartHeight, numPubRand)
		s.NoError(err)
		_, s.fp2CommitPubRandList, err = datagen.GenRandomMsgCommitPubRandList(s.r, s.fp2BTCSK, randCommitContext, s.commitStartHeight, numPubRand)
		s.NoError(err)

		s.Require().NotNil(s.fp1CommitPubRandList, "fp1CommitPubRandList should not be nil")
		s.Require().NotNil(s.fp2CommitPubRandList, "fp2CommitPubRandList should not be nil")

		n.CommitPubRandList(
			s.fp1CommitPubRandList.FpBtcPk,
			s.fp1CommitPubRandList.StartHeight,
			s.fp1CommitPubRandList.NumPubRand,
			s.fp1CommitPubRandList.Commitment,
			s.fp1CommitPubRandList.Sig,
		)
		n.CommitPubRandList(
			s.fp2CommitPubRandList.FpBtcPk,
			s.fp2CommitPubRandList.StartHeight,
			s.fp2CommitPubRandList.NumPubRand,
			s.fp2CommitPubRandList.Commitment,
			s.fp2CommitPubRandList.Sig,
		)
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

	n.WaitForNextBlock()

	govProp, err := s.configurer.ParseGovPropFromFile()
	s.NoError(err)
	chainA.WaitUntilHeight(govProp.Plan.Height + 1) // waits for chain to produce blocks

	expectedUpgradeHeight := govProp.Plan.Height
	resp := n.QueryAppliedPlan(v3.UpgradeName)
	s.EqualValues(expectedUpgradeHeight, resp.Height, "the plan should be applied at the height %d", expectedUpgradeHeight)

	// check fps have the same chain id
	s.Require().Equal(n.ChainID(), s.fp1.BsnId)
	s.Require().Equal(n.ChainID(), s.fp2.BsnId)

	// query pub randomness
	fp1CommitPubRand := n.QueryListPubRandCommit(s.fp1CommitPubRandList.FpBtcPk)
	s.Require().NotNil(fp1CommitPubRand, "fp1CommitPubRand should not be nil")
	_, ok := fp1CommitPubRand[s.commitStartHeight]
	s.Require().True(ok, "fp1CommitPubRand should contain commitStartHeight")

	fp2CommitPubRand := n.QueryListPubRandCommit(s.fp2CommitPubRandList.FpBtcPk)
	s.Require().NotNil(fp2CommitPubRand, "fp2CommitPubRand should not be nil")
	_, ok = fp2CommitPubRand[s.commitStartHeight]
	s.Require().True(ok, "fp2CommitPubRand should contain commitStartHeight")

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
