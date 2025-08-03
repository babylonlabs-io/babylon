package e2e

import (
	"math/rand"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/suite"

	v3 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v3"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/v3/x/finality/types"

	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer/config"
)

type SoftwareUpgradeV3TestSuite struct {
	suite.Suite

	r          *rand.Rand
	configurer *configurer.UpgradeConfigurer

	fp1Addr              string
	fp2Addr              string
	fp1BTCSK             *btcec.PrivateKey
	fp2BTCSK             *btcec.PrivateKey
	fp1                  *bstypes.FinalityProvider
	fp2                  *bstypes.FinalityProvider
	fp1CommitPubRandList *types.MsgCommitPubRandList
	fp2CommitPubRandList *types.MsgCommitPubRandList
	commitStartHeight    uint64
}

func (s *SoftwareUpgradeV3TestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite for v2.2.0 to v3 upgrade...")
	s.r = rand.New(rand.NewSource(time.Now().UnixNano()))
	var err error

	// func runs right before the upgrade proposal is sent
	preUpgradeFunc := func(chains []*chain.Config) {
		chainA := chains[0]
		n := chainA.NodeConfigs[1]

		chainA.WaitUntilHeight(2)
		s.SetupFps(n)
	}

	s.configurer, err = configurer.NewSoftwareUpgradeConfigurer(
		s.T(),
		true,
		config.UpgradeV3FilePath,
		nil,
		preUpgradeFunc,
	)
	s.NoError(err)

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

func (s *SoftwareUpgradeV3TestSuite) SetupFps(n *chain.NodeConfig) {
	n.WaitForNextBlock()

	s.fp1Addr = n.KeysAdd("fp1Addr")
	s.fp2Addr = n.KeysAdd("fp2Addr")
	n.BankMultiSendFromNode([]string{s.fp1Addr, s.fp2Addr}, "900000ubbn")

	n.WaitForNextBlock()

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
	s.commitStartHeight = n.LatestBlockNumber()

	var err error
	_, s.fp1CommitPubRandList, err = datagen.GenRandomMsgCommitPubRandList(s.r, s.fp1BTCSK, "", s.commitStartHeight, numPubRand)
	s.NoError(err)
	_, s.fp2CommitPubRandList, err = datagen.GenRandomMsgCommitPubRandList(s.r, s.fp2BTCSK, "", s.commitStartHeight, numPubRand)
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

	s.CheckFpAfterUpgrade()
	s.CheckParamsAfterUpgrade()

	n.WaitForNextBlock()
}

func (s *SoftwareUpgradeV3TestSuite) CheckFpAfterUpgrade() {
	chainA := s.configurer.GetChainConfig(0)
	n, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)

	fp1 := n.QueryFinalityProvider(s.fp1.BtcPk.MarshalHex())
	s.Require().Equal(fp1.BsnId, n.ChainID())
	fp2 := n.QueryFinalityProvider(s.fp2.BtcPk.MarshalHex())
	s.Require().Equal(fp2.BsnId, n.ChainID())

	// query pub randomness
	fp1CommitPubRand := n.QueryListPubRandCommit(s.fp1CommitPubRandList.FpBtcPk)
	s.Require().NotNil(fp1CommitPubRand, "fp1CommitPubRand should not be nil")
	_, ok := fp1CommitPubRand[s.commitStartHeight]
	s.Require().True(ok, "fp1CommitPubRand should contain commitStartHeight")

	fp2CommitPubRand := n.QueryListPubRandCommit(s.fp2CommitPubRandList.FpBtcPk)
	s.Require().NotNil(fp2CommitPubRand, "fp2CommitPubRand should not be nil")
	_, ok = fp2CommitPubRand[s.commitStartHeight]
	s.Require().True(ok, "fp2CommitPubRand should contain commitStartHeight")
}

func (s *SoftwareUpgradeV3TestSuite) CheckParamsAfterUpgrade() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)

	n, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)

	btcStkConsParams := n.QueryBTCStkConsumerParams()
	s.Require().False(btcStkConsParams.PermissionedIntegration, "btcstkconsumer permissioned integration should be false")

	zoneConciergeParams := n.QueryZoneConciergeParams()
	s.Require().Equal(uint32(2419200), zoneConciergeParams.IbcPacketTimeoutSeconds, "ibc_packet_timeout_seconds should be 2419200")

	btcStkParams := n.QueryBTCStakingParams()
	s.Require().Equal(uint32(10), btcStkParams.MaxFinalityProviders, "max_finality_providers should be 10")
	s.Require().Equal(uint32(260000), btcStkParams.BtcActivationHeight, "btc activation height should be 260000")
}
