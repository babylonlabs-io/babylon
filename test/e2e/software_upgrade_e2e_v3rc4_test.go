package e2e

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v3/app/signingcontext"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	v3rc4 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v3rc4"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

const (
	commitStartHeightV3RC4 = uint64(5)
	govPropFileV3RC4       = "v3rc4_upgrade_temp.json"
)

type SoftwareUpgradeV3RC4TestSuite struct {
	BaseBtcRewardsDistribution

	fp1BTCSK  *btcec.PrivateKey
	fp2BTCSK  *btcec.PrivateKey
	del1BTCSK *btcec.PrivateKey
	del2BTCSK *btcec.PrivateKey

	fp1 *bstypes.FinalityProvider
	fp2 *bstypes.FinalityProvider

	// BTC Staking amounts for delegations
	fp1Del1StakingAmt int64
	fp1Del2StakingAmt int64
	fp2Del1StakingAmt int64

	// Baby staking amounts for delegations (to validators) to make them co-stakers
	del1BabyAmt int64
	del2BabyAmt int64

	// bech32 addresses
	del1Addr string
	del2Addr string
	fp1Addr  string
	fp2Addr  string

	// finality helpers
	finalityIdx              uint64
	finalityBlockHeightVoted uint64
	fp1RandListInfo          *datagen.RandListInfo
	fp2RandListInfo          *datagen.RandListInfo

	configurer *configurer.UpgradeConfigurer

	// temporary upgrade config file path
	tempUpgradeConfigPath string
}

func (s *SoftwareUpgradeV3RC4TestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite for v3.0.0-rc3 to v3rc4 upgrade...")
	s.r = rand.New(rand.NewSource(time.Now().UnixNano()))
	var err error

	s.net = &chaincfg.SigNetParams
	s.fp1BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.fp2BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.del1BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)
	s.del2BTCSK, _, _ = datagen.GenRandomBTCKeyPair(s.r)

	s.fp1Del1StakingAmt = int64(2 * 10e8)
	s.fp1Del2StakingAmt = int64(4 * 10e8)
	s.fp2Del1StakingAmt = int64(2 * 10e8)

	s.del1BabyAmt = int64(1000000) // 1 Baby
	s.del2BabyAmt = int64(2000000) // 2 Baby

	covenantSKs, _, _ := bstypes.DefaultCovenantCommittee()
	s.covenantSKs = covenantSKs

	// Create temporary upgrade configuration file
	s.tempUpgradeConfigPath, err = s.createTempUpgradeConfig()
	s.NoError(err)

	// func runs right before the upgrade proposal is sent
	preUpgradeFunc := func(chains []*chain.Config) {
		chainA := chains[0]
		n := chainA.NodeConfigs[1]

		chainA.WaitUntilHeight(2)
		s.SetupFps(n)
		s.SetupVerifiedBtcDelegationsWithBabyStaking(n)
		s.FpCommitPubRandAndVote(n)
	}

	s.configurer, err = configurer.NewSoftwareUpgradeConfigurerWithCurrentTag(
		s.T(),
		true,
		s.tempUpgradeConfigPath,
		preUpgradeFunc,
		"v3.0.0-rc.3", // Start from this tag
	)
	s.NoError(err)

	err = s.configurer.ConfigureChains()
	s.NoError(err)
	err = s.configurer.RunSetup()
	s.Require().NoError(err)
}

func (s *SoftwareUpgradeV3RC4TestSuite) TearDownSuite() {
	// Clean up temporary upgrade config file
	if s.tempUpgradeConfigPath != "" {
		// Remove the local file created in govProps directory
		localFilePath := filepath.Join("govProps", govPropFileV3RC4)
		os.Remove(localFilePath)
	}

	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

// createTempUpgradeConfig creates a temporary upgrade configuration file for v3rc4
func (s *SoftwareUpgradeV3RC4TestSuite) createTempUpgradeConfig() (string, error) {
	upgradeConfig := map[string]interface{}{
		"messages": []map[string]interface{}{
			{
				"@type":     "/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade",
				"authority": "bbn10d07y265gmmuvt4z0w9aw880jnsr700jduz5f2",
				"plan": map[string]interface{}{
					"name":                  "v3rc4",
					"time":                  "0001-01-01T00:00:00Z",
					"height":                "221",
					"info":                  "Upgrade to v3rc4",
					"upgraded_client_state": nil,
				},
			},
		},
		"metadata":  "",
		"deposit":   "500000000ubbn",
		"title":     "Upgrade to Babylon v3rc4",
		"summary":   "This upgrade introduces the costaking module for BTC stakers with Baby delegation",
		"expedited": false,
	}

	// Create the file in govProps directory which is accessible to Docker containers
	govPropsDir := "govProps"
	if err := os.MkdirAll(govPropsDir, 0755); err != nil {
		return "", err
	}

	// Create temporary file in the govProps directory with a fixed name for this test
	tempFilePath := filepath.Join(govPropsDir, govPropFileV3RC4)
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	// Write config to temp file
	configBytes, err := json.Marshal(upgradeConfig)
	if err != nil {
		return "", err
	}

	if _, err := tempFile.Write(configBytes); err != nil {
		return "", err
	}

	// Return the path that will be accessible in Docker containers
	return "/govProps/" + govPropFileV3RC4, nil
}

func (s *SoftwareUpgradeV3RC4TestSuite) SetupFps(n *chain.NodeConfig) {
	n.WaitForNextBlock()

	s.fp1Addr = n.KeysAdd(wFp1)
	s.fp2Addr = n.KeysAdd(wFp2)
	n.BankMultiSendFromNode([]string{s.fp1Addr, s.fp2Addr}, "100000000000ubbn")

	n.WaitForNextBlock()

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
	n.WaitForNextBlock()

	actualFps := n.QueryFinalityProviders(n.ChainID())
	s.Require().Len(actualFps, 2)
}

// SetupVerifiedBtcDelegationsWithBabyStaking sets up BTC delegations and also delegates Baby tokens
// This is important for the v3rc4 upgrade test as it creates co-stakers (BTC stakers who also stake Baby)
func (s *SoftwareUpgradeV3RC4TestSuite) SetupVerifiedBtcDelegationsWithBabyStaking(n *chain.NodeConfig) {
	s.del1Addr = n.KeysAdd(wDel1)
	s.del2Addr = n.KeysAdd(wDel2)

	// Fund delegators with both ubbn and additional amount for Baby staking
	n.BankMultiSendFromNode([]string{s.del1Addr, s.del2Addr}, "10000000ubbn")

	n.WaitForNextBlock()

	// Create BTC delegations first
	s.CreateBTCDelegationAndCheck(n, wDel1, s.fp1, s.del1BTCSK, s.del1Addr, s.fp1Del1StakingAmt)
	s.CreateBTCDelegationAndCheck(n, wDel1, s.fp2, s.del1BTCSK, s.del1Addr, s.fp2Del1StakingAmt)
	s.CreateBTCDelegationAndCheck(n, wDel2, s.fp1, s.del2BTCSK, s.del2Addr, s.fp1Del2StakingAmt)

	// Verify BTC delegations
	resp := n.QueryBtcDelegations(bstypes.BTCDelegationStatus_ANY)
	require.Len(s.T(), resp.BtcDelegations, 3)

	s.CreateCovenantsAndSubmitSignaturesToPendDels(n, s.fp1, s.fp2)

	// Now create Baby delegations to validators to make them co-stakers
	// This is crucial for the v3rc4 upgrade test as it will register these as CostakerRewardsTracker
	validators, err := n.QueryValidators()
	s.NoError(err)
	s.Require().NotEmpty(validators, "No validators found")

	validatorAddr := validators[0].OperatorAddress

	// Delegate Baby tokens for del1 (making them a co-staker)
	n.Delegate(s.del1Addr, validatorAddr, fmt.Sprintf("%dubbn", s.del1BabyAmt))

	// Delegate Baby tokens for del2 (making them a co-staker)
	n.Delegate(s.del2Addr, validatorAddr, fmt.Sprintf("%dubbn", s.del2BabyAmt))

	// Wait for next epoch as delegations are queued and executed at epoch end
	s.T().Logf("Waiting for next epoch to process Baby delegations...")
	nextEpoch, err := n.WaitForNextEpoch()
	s.NoError(err)
	s.T().Logf("Now in epoch %d, delegations should be processed", nextEpoch)

	// Verify delegations exist after epoch boundary
	del1Delegations, err := n.QueryDelegatorDelegations(s.del1Addr)
	s.NoError(err)
	s.Require().NotEmpty(del1Delegations, "del1 should have Baby delegations after epoch boundary")

	del2Delegations, err := n.QueryDelegatorDelegations(s.del2Addr)
	s.NoError(err)
	s.Require().NotEmpty(del2Delegations, "del2 should have Baby delegations after epoch boundary")
}

func (s *SoftwareUpgradeV3RC4TestSuite) FpCommitPubRandAndVote(n *chain.NodeConfig) {
	fp1RandListInfo, fp1CommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(s.r, s.fp1BTCSK, commitStartHeightV3RC4, numPubRand)
	s.NoError(err)

	fp2RandListInfo, fp2CommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(s.r, s.fp2BTCSK, commitStartHeightV3RC4, numPubRand)
	s.NoError(err)
	s.fp2RandListInfo = fp2RandListInfo

	n.CommitPubRandList(
		wFp1,
		fp1CommitPubRandList.FpBtcPk,
		fp1CommitPubRandList.StartHeight,
		fp1CommitPubRandList.NumPubRand,
		fp1CommitPubRandList.Commitment,
		fp1CommitPubRandList.Sig,
	)

	n.CommitPubRandList(
		wFp2,
		fp2CommitPubRandList.FpBtcPk,
		fp2CommitPubRandList.StartHeight,
		fp2CommitPubRandList.NumPubRand,
		fp2CommitPubRandList.Commitment,
		fp2CommitPubRandList.Sig,
	)

	n.WaitForNextBlocks(2)

	fp1CommitPubRand := n.QueryListPubRandCommit(fp1CommitPubRandList.FpBtcPk)
	fp1PubRand := fp1CommitPubRand[commitStartHeightV3RC4]
	s.Require().Equal(fp1PubRand.NumPubRand, numPubRand)

	fp2CommitPubRand := n.QueryListPubRandCommit(fp2CommitPubRandList.FpBtcPk)
	fp2PubRand := fp2CommitPubRand[commitStartHeightV3RC4]
	s.Require().Equal(fp2PubRand.NumPubRand, numPubRand)

	finalizedEpoch := n.WaitUntilCurrentEpochIsSealedAndFinalized(1)
	s.Require().GreaterOrEqual(finalizedEpoch, fp1PubRand.EpochNum)
	s.Require().GreaterOrEqual(finalizedEpoch, fp2PubRand.EpochNum)

	fps := n.QueryFinalityProviders(n.ChainID())
	s.Require().Len(fps, 2)

	for _, fp := range fps {
		s.Require().False(fp.Jailed, "fp is jailed")
		s.Require().Zero(fp.SlashedBabylonHeight, "fp is slashed")

		fpDels := n.QueryFinalityProviderDelegations(fp.BtcPk.MarshalHex())
		if fp.BtcPk.Equals(s.fp1.BtcPk) {
			s.Require().Len(fpDels, 2)
		} else {
			s.Require().Len(fpDels, 1)
		}

		for _, fpDelStaker := range fpDels {
			for _, fpDel := range fpDelStaker.Dels {
				s.Require().True(fpDel.Active)
				s.Require().GreaterOrEqual(fpDel.TotalSat, uint64(0))
			}
		}
	}

	s.finalityBlockHeightVoted = n.WaitFinalityIsActivated()
	s.finalityIdx = s.finalityBlockHeightVoted - commitStartHeightV3RC4

	n.WaitForNextBlockWithSleep50ms()

	// Submit finality votes similar to v3 test
	n.AddFinalitySignatureToBlockWithContext(
		s.fp2BTCSK,
		s.fp2.BtcPk,
		s.finalityBlockHeightVoted,
		s.fp2RandListInfo.SRList[s.finalityIdx],
		&s.fp2RandListInfo.PRList[s.finalityIdx],
		*s.fp2RandListInfo.ProofList[s.finalityIdx].ToProto(),
		"",
		fmt.Sprintf("--from=%s", wFp2),
	)

	_ = n.AddFinalitySignatureToBlockWithContext(
		s.fp1BTCSK,
		s.fp1.BtcPk,
		s.finalityBlockHeightVoted,
		s.fp1RandListInfo.SRList[s.finalityIdx],
		&s.fp1RandListInfo.PRList[s.finalityIdx],
		*s.fp1RandListInfo.ProofList[s.finalityIdx].ToProto(),
		"",
		fmt.Sprintf("--from=%s", wFp1),
	)

	n.WaitForNextBlocks(2)

	s.finalityIdx++
	s.finalityBlockHeightVoted++
	s.AddFinalityVoteUntilCurrentHeight(n, "")
}

// TestUpgradeV3RC4 checks if the upgrade from v3.0.0-rc3 to v3rc4 was successful
func (s *SoftwareUpgradeV3RC4TestSuite) Test1UpgradeV3RC4() {
	chainA := s.configurer.GetChainConfig(0)
	n, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)

	n.WaitForNextBlock()

	govProp, err := s.configurer.ParseGovPropFromFile()
	s.NoError(err)
	chainA.WaitUntilHeight(govProp.Plan.Height + 1)

	expectedUpgradeHeight := govProp.Plan.Height
	resp := n.QueryAppliedPlan(v3rc4.UpgradeName)
	s.EqualValues(expectedUpgradeHeight, resp.Height, "the plan should be applied at the height %d", expectedUpgradeHeight)

	s.CheckCostakerRewardsTrackerAfterUpgrade(n)

	n.WaitForNextBlock()

	// Send finality votes until upgrade height plus 10 blocks
	fpFinVoteContext := signingcontext.FpFinVoteContextV0(n.ChainID(), appparams.AccFinality.String())
	s.AddFinalityVoteUntilCurrentHeight(n, fpFinVoteContext)
}

// CheckCostakerRewardsTrackerAfterUpgrade verifies that the CostakerRewardsTracker was properly initialized
func (s *SoftwareUpgradeV3RC4TestSuite) CheckCostakerRewardsTrackerAfterUpgrade(n *chain.NodeConfig) {
	// Query costaker rewards tracker for del1 (who has both BTC and Baby delegations)
	del1Tracker, err := n.QueryCostakerRewardsTracker(s.del1Addr)
	s.NoError(err, "should be able to query costaker rewards tracker for del1")
	s.Require().NotNil(del1Tracker, "del1 should have a costaker rewards tracker")

	s.T().Logf("del1 costaker rewards tracker: ActiveSatoshis=%s, ActiveBaby=%s, TotalScore=%s",
		del1Tracker.ActiveSatoshis.String(), del1Tracker.ActiveBaby.String(), del1Tracker.TotalScore.String())

	// Verify del1 has non-zero active satoshis, baby and score
	s.Require().True(del1Tracker.TotalScore.GT(sdkmath.ZeroInt()), "del1 should have a total score")
	s.Require().Equal(uint64(1), del1Tracker.StartPeriodCumulativeReward, "del1 should start at period 1")
	expectedDel1Sats := sdkmath.NewIntFromUint64(uint64(s.fp1Del1StakingAmt + s.fp2Del1StakingAmt))
	s.Require().True(del1Tracker.ActiveSatoshis.Equal(expectedDel1Sats),
		"del1 active satoshis should match expected BTC delegations: expected %s, got %s",
		expectedDel1Sats.String(), del1Tracker.ActiveSatoshis.String())

	expectedDel1Baby := sdkmath.NewIntFromUint64(uint64(s.del1BabyAmt))
	s.Require().True(del1Tracker.ActiveBaby.Equal(expectedDel1Baby),
		"del1 active baby should match expected Baby delegations: expected %s, got %s",
		expectedDel1Baby.String(), del1Tracker.ActiveBaby.String())

	// Query costaker rewards tracker for del2 (who has both BTC and Baby delegations)
	del2Tracker, err := n.QueryCostakerRewardsTracker(s.del2Addr)
	s.NoError(err, "should be able to query costaker rewards tracker for del2")
	s.Require().NotNil(del2Tracker, "del2 should have a costaker rewards tracker")

	s.T().Logf("del2 costaker rewards tracker: ActiveSatoshis=%s, ActiveBaby=%s, TotalScore=%s",
		del2Tracker.ActiveSatoshis.String(), del2Tracker.ActiveBaby.String(), del2Tracker.TotalScore.String())

	// Verify del2 values
	s.Require().True(del2Tracker.TotalScore.GT(sdkmath.ZeroInt()), "del2 should have a total score")
	s.Require().Equal(uint64(1), del2Tracker.StartPeriodCumulativeReward, "del2 should start at period 1")

	expectedDel2Sats := sdkmath.NewIntFromUint64(uint64(s.fp1Del2StakingAmt))
	s.Require().True(del2Tracker.ActiveSatoshis.Equal(expectedDel2Sats),
		"del2 active satoshis should match expected BTC delegations: expected %s, got %s",
		expectedDel2Sats.String(), del2Tracker.ActiveSatoshis.String())

	expectedDel2Baby := sdkmath.NewIntFromUint64(uint64(s.del2BabyAmt))
	s.Require().True(del2Tracker.ActiveBaby.Equal(expectedDel2Baby),
		"del2 active baby should match expected Baby delegations: expected %s, got %s",
		expectedDel2Baby.String(), del2Tracker.ActiveBaby.String())
}

func (s *SoftwareUpgradeV3RC4TestSuite) AddFinalityVoteUntilCurrentHeight(n *chain.NodeConfig, fpFinalityVoteContext string) {
	currentBlock := n.LatestBlockNumber()

	accFp1, err := n.QueryAccount(s.fp1.Addr)
	s.NoError(err)
	accFp2, err := n.QueryAccount(s.fp2.Addr)
	s.NoError(err)

	accNumberFp1 := accFp1.GetAccountNumber()
	accSequenceFp1 := accFp1.GetSequence()

	accNumberFp2 := accFp2.GetAccountNumber()
	accSequenceFp2 := accFp2.GetSequence()

	n.WaitForNextBlockWithSleep50ms()

	for s.finalityBlockHeightVoted < currentBlock {
		fp1Flags := []string{
			"--offline",
			fmt.Sprintf("--account-number=%d", accNumberFp1),
			fmt.Sprintf("--sequence=%d", accSequenceFp1),
			fmt.Sprintf("--from=%s", s.fp1.Addr),
		}
		fp2Flags := []string{
			"--offline",
			fmt.Sprintf("--account-number=%d", accNumberFp2),
			fmt.Sprintf("--sequence=%d", accSequenceFp2),
			fmt.Sprintf("--from=%s", s.fp2.Addr),
		}
		s.AddFinalityVote(n, fpFinalityVoteContext, fp1Flags, fp2Flags)

		accSequenceFp1++
		accSequenceFp2++
	}
}

func (s *SoftwareUpgradeV3RC4TestSuite) AddFinalityVote(n *chain.NodeConfig, fpFinalityVoteContext string, flagsFp1, flagsFp2 []string) {
	n.AddFinalitySignatureToBlockWithContext(
		s.fp2BTCSK,
		s.fp2.BtcPk,
		s.finalityBlockHeightVoted,
		s.fp2RandListInfo.SRList[s.finalityIdx],
		&s.fp2RandListInfo.PRList[s.finalityIdx],
		*s.fp2RandListInfo.ProofList[s.finalityIdx].ToProto(),
		fpFinalityVoteContext,
		flagsFp2...,
	)

	n.AddFinalitySignatureToBlockWithContext(
		s.fp1BTCSK,
		s.fp1.BtcPk,
		s.finalityBlockHeightVoted,
		s.fp1RandListInfo.SRList[s.finalityIdx],
		&s.fp1RandListInfo.PRList[s.finalityIdx],
		*s.fp1RandListInfo.ProofList[s.finalityIdx].ToProto(),
		fpFinalityVoteContext,
		flagsFp1...,
	)

	s.finalityIdx++
	s.finalityBlockHeightVoted++
}
