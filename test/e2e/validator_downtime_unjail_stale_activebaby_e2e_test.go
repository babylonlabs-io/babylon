package e2e

import (
	"bytes"
	"math/rand"
	"net/url"
	"os"
	"strconv"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	ckpttypes "github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
	costakingtypes "github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	"github.com/btcsuite/btcd/chaincfg"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/initialization"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
)

type DowntimeSlashUnjailStaleActiveBabyTestSuite struct {
	suite.Suite

	configurer configurer.Configurer
	envBackup  map[string]*string
}

func (s *DowntimeSlashUnjailStaleActiveBabyTestSuite) SetupSuite() {
	s.T().Log("setting up downtime slash+unjail stale ActiveBaby e2e integration test suite...")
	var err error

	s.envBackup = make(map[string]*string)

	s.configurer, err = configurer.NewBabylonConfigurer(s.T(), true)
	s.Require().NoError(err)
	err = s.configurer.ConfigureChains()
	s.Require().NoError(err)
	err = s.configurer.RunSetup()
	s.Require().NoError(err)
}

func (s *DowntimeSlashUnjailStaleActiveBabyTestSuite) TearDownSuite() {
	for key, oldVal := range s.envBackup {
		if oldVal == nil {
			_ = os.Unsetenv(key)
			continue
		}
		_ = os.Setenv(key, *oldVal)
	}

	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

func (s *DowntimeSlashUnjailStaleActiveBabyTestSuite) setEnv(key, val string) {
	if _, ok := s.envBackup[key]; !ok {
		if oldVal, found := os.LookupEnv(key); found {
			s.envBackup[key] = &oldVal
		} else {
			s.envBackup[key] = nil
		}
	}
	_ = os.Setenv(key, val)
}

// validator gets slashed+jailed for downtime mid-epoch
// validator unjails mid epoch
// validator is active for the next epoch
// costaking ActiveBaby for a delegator stays at the preslash amount across the epoch boundary
// impact: the delegator can create BTC stake in the next epoch and receive TotalScore based on burned BABY collateral
func (s *DowntimeSlashUnjailStaleActiveBabyTestSuite) TestDowntimeSlashUnjailStaleActiveBaby() {
	chainA := s.configurer.GetChainConfig(0)
	chainA.WaitUntilHeight(1)

	valN1, err := chainA.GetNodeAtIndex(0)
	s.NoError(err)
	valN2, err := chainA.GetNodeAtIndex(1)
	s.NoError(err)
	n, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	// Map node containers. onchain validators.
	s.T().Log("Mapping validator nodes to on-chain validators...")
	node0ConsPubKey := valN1.ValidatorConsPubKey()
	node1ConsPubKey := valN2.ValidatorConsPubKey()

	validators, err := n.QueryValidators()
	s.NoError(err)
	s.Require().Len(validators, 2, "Need exactly 2 validators")

	var val1, val2 *stakingtypes.Validator
	var val2ConsAddr string
	for i := range validators {
		var valConsPubKey cryptotypes.PubKey
		err := util.Cdc.UnpackAny(validators[i].ConsensusPubkey, &valConsPubKey)
		s.Require().NoError(err, "failed to unmarshal consensus pubkey")

		if bytes.Equal(valConsPubKey.Bytes(), node0ConsPubKey.Bytes()) {
			val1 = &validators[i]
		}
		if bytes.Equal(valConsPubKey.Bytes(), node1ConsPubKey.Bytes()) {
			val2 = &validators[i]
			val2ConsAddr = sdk.ConsAddress(valConsPubKey.Address()).String()
		}
	}
	s.Require().NotNil(val1, "Could not map node 0 to validator")
	s.Require().NotNil(val2, "Could not map node 1 to validator")
	s.Require().NotEmpty(val2ConsAddr, "Could not derive validator 2 consensus address")
	val1OpAddr := val1.OperatorAddress
	val2OpAddr := val2.OperatorAddress

	// Read parameters we need
	scoreRatioBtcByBaby := s.getScoreRatioBtcByBaby(n)
	s.Require().True(scoreRatioBtcByBaby.IsPositive(), "score_ratio_btc_by_baby must be > 0")
	btcParams := n.QueryBTCStakingParams()
	s.Require().GreaterOrEqual(btcParams.MinStakingValueSat, int64(1), "min_staking_value_sat must be > 0")
	btcStakeSatAmt := btcParams.MinStakingValueSat
	// Size the BABY delegation so that preslash collateral exactly covers the minimum BTC stake
	// Cap formula capSats = ActiveBaby / score_ratio_btc_by_bab
	delegationAmt := scoreRatioBtcByBaby.MulRaw(btcStakeSatAmt)

	// Create a dedicated costaker wallet (so assertions aren't affected by validator self delegations).
	costakerWallet := "costaker-downtime"
	costakerAddr := n.KeysAdd(costakerWallet)
	honestWallet := "costaker-honest"
	honestAddr := n.KeysAdd(honestWallet)

	balPre, err := n.QueryBalances(costakerAddr)
	s.NoError(err)
	// Fund enough for BABY delegation + BTC-staking tx fees.
	valN1.BankSend(initialization.ValidatorWalletName, costakerAddr, delegationAmt.AddRaw(5_000_000).String()+initialization.BabylonDenom)
	valN1.BankSend(initialization.ValidatorWalletName, honestAddr, delegationAmt.AddRaw(5_000_000).String()+initialization.BabylonDenom)
	s.Require().Eventually(func() bool {
		balPost, err := n.QueryBalances(costakerAddr)
		if err != nil {
			return false
		}
		return balPost.AmountOf(initialization.BabylonDenom).GT(balPre.AmountOf(initialization.BabylonDenom))
	}, 30*time.Second, 500*time.Millisecond, "costaker funding tx should be committed")

	// Queue delegations
	// 1) Give validator1 >66% voting power so chain continues while validator2 is down.
	// Use a fixed large delegation to avoid depending on initial token distribution details.
	txHashGiveVal1VP := n.Delegate(initialization.ValidatorWalletName, val1OpAddr, "200000000000ubbn", "--gas=auto", "--gas-adjustment=1.5")
	s.waitForTxSuccess(n, txHashGiveVal1VP)

	// 2) Delegate from costaker -> validator2,this is the amount we track for ActiveBaby staleness
	txHashCostakerDel := n.Delegate(costakerWallet, val2OpAddr, delegationAmt.String()+initialization.BabylonDenom, "--gas=auto", "--gas-adjustment=1.5")
	s.waitForTxSuccess(n, txHashCostakerDel)

	// 3) Honest costaker delegates to validator1, not slashed
	txHashHonestDel := n.Delegate(honestWallet, val1OpAddr, delegationAmt.String()+initialization.BabylonDenom, "--gas=auto", "--gas-adjustment=1.5")
	s.waitForTxSuccess(n, txHashHonestDel)

	// Wait (up to a few epoch boundaries) for queued delegations to be executed
	var val1Percentage sdkmath.LegacyDec
	for i := 0; i < 4; i++ {
		_, err = n.WaitForNextEpoch()
		s.NoError(err)
		chainA.WaitForNumHeights(3)

		validators, err = n.QueryValidators()
		s.NoError(err)
		for i := range validators {
			if validators[i].OperatorAddress == val1OpAddr {
				val1 = &validators[i]
			}
			if validators[i].OperatorAddress == val2OpAddr {
				val2 = &validators[i]
			}
		}
		totalVotingPower := val1.Tokens.Add(val2.Tokens)
		val1Percentage = val1.Tokens.ToLegacyDec().Quo(totalVotingPower.ToLegacyDec()).MulInt64(100)
		s.T().Logf("Validator1 tokens=%s (%s%%), Validator2 tokens=%s, total=%s",
			val1.Tokens.String(),
			val1Percentage.String(),
			val2.Tokens.String(),
			totalVotingPower.String(),
		)
		if val1Percentage.GT(sdkmath.LegacyMustNewDecFromStr("66")) {
			break
		}
	}
	s.Require().True(val1Percentage.GT(sdkmath.LegacyMustNewDecFromStr("66")), "validator1 must have >66%% voting power")

	// Record pre-slash staking delegation tokens and costaking ActiveBaby.
	trackerPreSlash, err := n.QueryCostakerRewardsTracker(costakerAddr)
	s.NoError(err)
	require.Equal(s.T(), delegationAmt.String(), trackerPreSlash.ActiveBaby.String(), "pre-slash ActiveBaby should match delegation amount")
	require.Equal(s.T(), "0", trackerPreSlash.ActiveSatoshis.String(), "pre-slash ActiveSatoshis should be zero")
	require.Equal(s.T(), "0", trackerPreSlash.TotalScore.String(), "pre-slash TotalScore should be zero")

	delTokensPreSlash := s.getDelegationTokensToValidator(n, costakerAddr, val2.OperatorAddress)
	require.Equal(s.T(), delegationAmt, delTokensPreSlash, "pre-slash staking delegation tokens should match delegation amount")

	// Slashing parameters determine how many blocks need to be missed.
	slashingParams, err := n.QuerySlashingParams()
	s.NoError(err)
	s.Require().True(slashingParams.SlashFractionDowntime.IsPositive(), "SlashFractionDowntime must be > 0 to assert token burn")

	minSignedPerWindow := slashingParams.MinSignedPerWindow
	signedBlocksWindow := slashingParams.SignedBlocksWindow
	maxMissedBlocks := signedBlocksWindow - minSignedPerWindow.MulInt64(signedBlocksWindow).TruncateInt64()
	blocksToMiss := maxMissedBlocks + 3

	// epochAtSlash is the epoch in which the downtime slash/jail is observed to occur.
	// record it before stopping the validator and later assert the jail/slash happens mid-epoch
	// by checking the epoch number is unchanged at the time the validator becomes jailed.
	epochAtSlash, err := n.QueryCurrentEpoch()
	s.NoError(err)
	epochInterval := s.getEpochInterval(n)
	s.T().Logf("Epoch interval=%d blocks, current epoch=%d, signed_blocks_window=%d, blocks_to_miss=%d, downtime_jail_duration=%s, slash_fraction_downtime=%s",
		epochInterval,
		epochAtSlash,
		signedBlocksWindow,
		blocksToMiss,
		slashingParams.DowntimeJailDuration,
		slashingParams.SlashFractionDowntime.String(),
	)

	// Stop validator2 to trigger downtime slash+jail mid-epoch.
	currentHeight, err := n.QueryCurrentHeight()
	s.NoError(err)
	s.T().Logf("Stopping validator2 at height=%d epoch=%d to simulate downtime...", currentHeight, epochAtSlash)
	err = valN2.Stop()
	s.NoError(err)

	targetHeight := currentHeight + signedBlocksWindow + blocksToMiss + 5
	s.waitForHeight(n, targetHeight)

	// Verify validator2 is jailed and record jailed-until.
	s.Require().Eventually(func() bool {
		v, err := n.QueryValidator(val2.OperatorAddress)
		if err != nil {
			return false
		}
		return v.Jailed
	}, 60*time.Second, 1*time.Second, "validator2 should be jailed after downtime")
	signingInfo, err := n.QuerySigningInfo(val2ConsAddr)
	s.NoError(err)

	epochAfterJail, err := n.QueryCurrentEpoch()
	s.NoError(err)
	s.T().Logf("Validator jailed at epoch %d (slash started at epoch %d)", epochAfterJail, epochAtSlash)

	// Assert staking tokens are reduced immediately (mid-epoch).
	delTokensPostSlash := s.getDelegationTokensToValidator(n, costakerAddr, val2.OperatorAddress)
	s.Require().True(delTokensPostSlash.LT(delTokensPreSlash), "staking delegation tokens should decrease immediately after downtime slash")

	expectedPostSlash := sdkmath.LegacyNewDecFromInt(delTokensPreSlash).
		Mul(sdkmath.LegacyOneDec().Sub(slashingParams.SlashFractionDowntime)).
		TruncateInt()
	s.Require().Equal(expectedPostSlash, delTokensPostSlash, "delegation tokens should reflect SlashFractionDowntime immediately")

	// Validator is jailed, so the active baby should be zero.
	// Assert costaking ActiveBaby is still pre-slash mid-epoch.
	v, err := n.QueryValidator(val2.OperatorAddress)
	s.NoError(err)
	s.True(v.Jailed)
	trackerMidEpoch, err := n.QueryCostakerRewardsTracker(costakerAddr)
	s.NoError(err)
	// s.Require().Equal(trackerPreSlash.ActiveBaby, trackerMidEpoch.ActiveBaby, "ActiveBaby should remain stale mid-epoch after slash+jail")
	require.Equal(s.T(), "0", trackerMidEpoch.ActiveBaby.String(), "mid epoch after slash and jailing ActiveBaby should be Zero")
	require.Equal(s.T(), "0", trackerMidEpoch.ActiveSatoshis.String(), "mid epoch after slash ActiveSatoshis should be zero")
	require.Equal(s.T(), "0", trackerMidEpoch.TotalScore.String(), "mid epoch after slash TotalScore should be zero")

	// Restart validator2 and unjail once the jail duration passes (still within the same epoch).
	err = valN2.Run()
	s.NoError(err)

	sleepDur := time.Until(signingInfo.JailedUntil.Add(2 * time.Second))
	if sleepDur < 0 {
		sleepDur = 0
	}
	if sleepDur > 10*time.Minute {
		s.T().Skipf("downtime jail duration too long for e2e: %s", sleepDur)
	}
	s.T().Logf("Waiting %s for downtime jail duration before unjail...", sleepDur)
	time.Sleep(sleepDur)

	epochBeforeUnjail, err := n.QueryCurrentEpoch()
	s.NoError(err)
	s.T().Logf("Unjailing validator at epoch %d", epochBeforeUnjail)

	txHash := valN2.UnjailValidator(initialization.ValidatorWalletName)
	s.T().Logf("Unjail tx hash: %s", txHash)
	s.waitForTxSuccess(n, txHash)

	// Wait a few blocks for state to update.
	h, err := n.QueryCurrentHeight()
	s.NoError(err)
	chainA.WaitUntilHeight(h + 3)

	requireEventually := func(cond func() bool, msg string) {
		s.Require().Eventually(cond, 60*time.Second, 1*time.Second, msg)
	}
	requireEventually(func() bool {
		v, err := n.QueryValidator(val2.OperatorAddress)
		if err != nil {
			return false
		}
		return !v.Jailed
	}, "validator2 should become unjailed after unjail tx")

	// Wait for the epoch boundary, then verify validator2 stays active into the next epoch.
	_, err = n.WaitForNextEpoch()
	s.NoError(err)
	chainA.WaitForNumHeights(3)
	epochAfterBoundary, err := n.QueryCurrentEpoch()
	s.NoError(err)
	s.Require().Greater(epochAfterBoundary, epochAtSlash, "epoch should advance after waiting for epoch end")

	valSet, err := n.QueryCurrentEpochValSet()
	s.NoError(err)
	val2ValAddr, err := sdk.ValAddressFromBech32(val2.OperatorAddress)
	s.NoError(err)
	foundVal2 := false
	for _, v := range valSet.Validators {
		if bytes.Equal(v.Addr, val2ValAddr) {
			foundVal2 = true
			break
		}
	}
	s.Require().True(foundVal2, "validator2 should be in the next epoch validator set after unjail")

	// Validator is not jailed, so it should have some active baby
	// Final assertion: ActiveBaby remains at the pre-slash amount across the epoch boundary.
	v, err = n.QueryValidator(val2.OperatorAddress)
	s.NoError(err)
	s.False(v.Jailed)
	trackerNextEpoch, err := n.QueryCostakerRewardsTracker(costakerAddr)
	s.NoError(err)
	// s.Require().Equal(trackerPreSlash.ActiveBaby.String(), trackerNextEpoch.ActiveBaby.String(), "ActiveBaby remained stale across epoch boundary")
	require.Equal(s.T(), expectedPostSlash.String(), trackerNextEpoch.ActiveBaby.String(), "tracker next epoch where the validator unjailed ActiveBaby should match the expectedPostSlash")
	require.Equal(s.T(), "0", trackerNextEpoch.ActiveSatoshis.String(), "tracker next epoch where the validator unjailed ActiveSatoshis should be zero")
	require.Equal(s.T(), "0", trackerNextEpoch.TotalScore.String(), "tracker next epoch where the validator unjailed TotalScore should be zero")

	// Impact assertion: BTC collateral cap bypass in the next epoch
	// Create a finality provider and a BTC delegation for the costaker in the next epoch (after the boundary).
	// onchain staking tokens backing this delegator have already been burned (delTokensPostSlash), but the
	// costaking tracker still uses the pre-slash ActiveBaby to compute TotalScore.
	r := rand.New(rand.NewSource(2))
	fpSk, _, err := datagen.GenRandomBTCKeyPair(r)
	s.NoError(err)
	fp := chain.CreateFpFromNodeAddr(s.T(), r, fpSk, n)

	btcStakerSK, _, err := datagen.GenRandomBTCKeyPair(r)
	s.NoError(err)
	btcStakerSK2, _, err := datagen.GenRandomBTCKeyPair(r)
	s.NoError(err)
	btcNet := &chaincfg.SimNetParams

	// Create a BTC delegation at the module's minimum staking value.
	stakingTimeBlocks := uint16(btcParams.MinStakingTimeBlocks)
	if stakingTimeBlocks < 400 {
		stakingTimeBlocks = 400
	}
	testStakingInfo := n.CreateBTCDelegationAndCheck(
		r,
		s.T(),
		btcNet,
		costakerWallet,
		fp,
		btcStakerSK,
		costakerAddr,
		stakingTimeBlocks,
		btcStakeSatAmt,
	)
	testStakingInfo2 := n.CreateBTCDelegationAndCheck(
		r,
		s.T(),
		btcNet,
		honestWallet,
		fp,
		btcStakerSK2,
		honestAddr,
		stakingTimeBlocks,
		btcStakeSatAmt,
	)

	// Provide covenant signatures to activate the BTC delegation.
	btcDelResp := n.QueryBtcDelegation(testStakingInfo.StakingTx.TxHash().String())
	s.Require().NotNil(btcDelResp)
	pendingDel, err := chain.ParseRespBTCDelToBTCDel(btcDelResp.BtcDelegation)
	s.NoError(err)
	s.Require().NotNil(pendingDel)

	btcDelResp2 := n.QueryBtcDelegation(testStakingInfo2.StakingTx.TxHash().String())
	s.Require().NotNil(btcDelResp2)
	pendingDel2, err := chain.ParseRespBTCDelToBTCDel(btcDelResp2.BtcDelegation)
	s.NoError(err)
	s.Require().NotNil(pendingDel2)

	covenantSKs, _, _ := bstypes.DefaultCovenantCommittee()
	covWallets := make([]string, len(covenantSKs))
	for i := range covWallets {
		covWallets[i] = costakerWallet
	}
	txHashes := n.SendCovenantSigs(r, s.T(), btcNet, covenantSKs, covWallets, pendingDel)
	for _, txHash := range txHashes {
		s.waitForTxSuccess(n, txHash)
	}
	txHashes2 := n.SendCovenantSigs(r, s.T(), btcNet, covenantSKs, covWallets, pendingDel2)
	for _, txHash := range txHashes2 {
		s.waitForTxSuccess(n, txHash)
	}

	// Wait until the BTC delegation is ACTIVE.
	s.Require().Eventually(func() bool {
		active := n.QueryActiveDelegations()
		for _, del := range active {
			if del.StakingTxHex == btcDelResp.BtcDelegation.StakingTxHex {
				return true
			}
		}
		return false
	}, 60*time.Second, 500*time.Millisecond, "BTC delegation should become ACTIVE after covenant quorum")
	s.Require().Eventually(func() bool {
		active := n.QueryActiveDelegations()
		for _, del := range active {
			if del.StakingTxHex == btcDelResp2.BtcDelegation.StakingTxHex {
				return true
			}
		}
		return false
	}, 60*time.Second, 500*time.Millisecond, "honest BTC delegation should become ACTIVE after covenant quorum")

	// Activate the fp so costaking counts sats delegated to it
	// fp needs to be in the active set and needs BTC-timestamped public randomness
	for {
		epochNow, err := n.QueryCurrentEpoch()
		s.NoError(err)
		if epochNow >= 1 {
			break
		}
		_, err = n.WaitForNextEpoch()
		s.NoError(err)
	}

	currentHeightForPR, err := n.QueryCurrentHeight()
	s.NoError(err)
	commitStartHeight := uint64(currentHeightForPR)
	numPubRand := uint64(200)
	_, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(r, fpSk, commitStartHeight, numPubRand)
	s.NoError(err)
	n.CommitPubRandList(
		costakerWallet,
		msgCommitPubRandList.FpBtcPk,
		msgCommitPubRandList.StartHeight,
		msgCommitPubRandList.NumPubRand,
		msgCommitPubRandList.Commitment,
		msgCommitPubRandList.Sig,
	)
	n.WaitForNextBlock()

	epochOfPRCommit, err := n.QueryCurrentEpoch()
	s.NoError(err)
	s.Require().GreaterOrEqual(epochOfPRCommit, uint64(1))

	// Wait until the epoch containing the PR commit is sealed, then finalize it (BTC timestamping),
	// so the FP becomes eligible for the active set.
	_, err = n.WaitForNextEpoch()
	s.NoError(err)
	chainA.WaitForNumHeights(3)
	s.Require().Eventually(func() bool {
		resp, err := n.QueryRawCheckpoint(epochOfPRCommit)
		if err != nil {
			return false
		}
		return resp.Status == ckpttypes.Sealed
	}, 60*time.Second, 500*time.Millisecond, "epoch checkpoint should become sealed before finalization")

	// btccheckpoint tracks submissions with ancestry; finalize consecutively from epoch 1.
	n.FinalizeSealedEpochs(1, epochOfPRCommit)

	// Ensure the checkpoint reaches FINALIZED (w-deep). Finalization is driven by BTC header insertions,
	// so if we're close to the threshold, add a few more empty BTC headers to cross it deterministically.
	ckptFinalized := false
	for i := 0; i < 20; i++ {
		resp, err := n.QueryRawCheckpoint(epochOfPRCommit)
		if err == nil {
			s.T().Logf("checkpoint epoch=%d status=%s (try %d/20)", epochOfPRCommit, resp.Status.String(), i+1)
			if resp.Status == ckpttypes.Finalized {
				ckptFinalized = true
				break
			}
		}
		n.InsertNewEmptyBtcHeader(r)
		n.WaitForNextBlock()
	}
	s.Require().True(ckptFinalized, "epoch checkpoint should become finalized for PR commit to be BTC-timestamped")

	expAmtSats := sdkmath.NewInt(btcStakeSatAmt)
	// Now the FP should enter the finality active set, triggering costaking hooks to set ActiveSatoshis.
	var trackerWithBtc *costakingtypes.QueryCostakerRewardsTrackerResponse
	s.Require().Eventually(func() bool {
		trackerWithBtc, err = n.QueryCostakerRewardsTracker(costakerAddr)
		if err != nil {
			return false
		}
		return trackerWithBtc.ActiveSatoshis.Equal(expAmtSats)
	}, 2*time.Minute, 500*time.Millisecond, "ActiveSatoshis should reflect BTC delegation once FP is active")
	var trackerHonestWithBtc *costakingtypes.QueryCostakerRewardsTrackerResponse
	s.Require().Eventually(func() bool {
		trackerHonestWithBtc, err = n.QueryCostakerRewardsTracker(honestAddr)
		if err != nil {
			return false
		}
		return trackerHonestWithBtc.ActiveSatoshis.Equal(expAmtSats)
	}, 2*time.Minute, 500*time.Millisecond, "honest ActiveSatoshis should reflect BTC delegation once FP is active")

	correctCapSats := delTokensPostSlash.Quo(scoreRatioBtcByBaby)
	s.Require().True(correctCapSats.LT(trackerWithBtc.ActiveSatoshis), "test requires post-slash collateral cap to be smaller than the BTC stake")
	s.Require().Equal(trackerWithBtc.ActiveSatoshis, trackerWithBtc.TotalScore, "stale ActiveBaby allows TotalScore to equal ActiveSatoshis")
	s.Require().True(trackerWithBtc.TotalScore.GT(correctCapSats),
		"TotalScore should exceed the cap implied by burned staking tokens: score=%s correct_cap=%s post_slash_baby=%s ratio=%s",
		trackerWithBtc.TotalScore.String(),
		correctCapSats.String(),
		delTokensPostSlash.String(),
		scoreRatioBtcByBaby.String(),
	)

	//Impact assertion. coins actually leave the pool on withdraw, and the attacker is overpaid
	// Ensure the withdrawal happens sufficiently after the slash epoch (not just the first boundary).
	epochNow, err := n.QueryCurrentEpoch()
	s.NoError(err)
	if epochNow < epochAtSlash+2 {
		_, err = n.WaitForNextEpoch()
		s.NoError(err)
		epochNow, err = n.QueryCurrentEpoch()
		s.NoError(err)
	}
	s.Require().GreaterOrEqual(epochNow, epochAtSlash+2, "withdrawal must occur at least 2 epochs after the slash epoch")

	trackerLate, err := n.QueryCostakerRewardsTracker(costakerAddr)
	s.NoError(err)
	// s.Require().Equal(trackerPreSlash.ActiveBaby, trackerLate.ActiveBaby, "ActiveBaby still stale at withdrawal time")
	require.Equal(s.T(), expectedPostSlash.String(), trackerLate.ActiveBaby.String(), "tracker late after the btc delegation ActiveBaby should match the expectedPostSlash")
	require.Equal(s.T(), expAmtSats.String(), trackerLate.ActiveSatoshis.String(), "tracker late after the btc delegation ActiveSatoshis should be the amount of sats staked")

	expScore := sdkmath.MaxInt(expAmtSats, expectedPostSlash.Quo(scoreRatioBtcByBaby))
	require.Equal(s.T(), expScore.String(), trackerLate.TotalScore.String(), "tracker late after the btc delegation TotalScore should be the min (ActiveSatoshis, ActiveBaby / ScoreRatioBtcByBaby)")

	// Inject some fees into fee_collector so costaking accumulates nonzero rewards.
	sinkWallet := "reward-sink"
	sinkAddr := n.KeysAdd(sinkWallet)
	for i := 0; i < 3; i++ {
		valN1.BankSend(initialization.ValidatorWalletName, sinkAddr, "1"+initialization.BabylonDenom)
		n.WaitForNextBlock()
	}
	chainA.WaitForNumHeights(2)

	curRewards, err := s.queryCostakingCurrentRewards(n)
	s.NoError(err)
	rewardsUbbnWithDecimals := curRewards.Rewards.AmountOf(initialization.BabylonDenom)
	s.Require().True(rewardsUbbnWithDecimals.IsPositive(), "expected non-zero costaking rewards after fee injection")
	rewardsUbbn := rewardsUbbnWithDecimals.Quo(ictvtypes.DecimalRewards)
	s.Require().True(rewardsUbbn.IsPositive(), "expected non-zero costaking rewards (after removing DecimalRewards)")

	// Under correct collateralization, attacker score would be capped at correctCapSats.
	// Honest score stays at full btcStakeSatAmt.
	expectedAttackerUbbnCorrect := rewardsUbbn.Mul(correctCapSats).Quo(correctCapSats.Add(trackerHonestWithBtc.TotalScore))
	s.Require().True(expectedAttackerUbbnCorrect.IsPositive(), "expected attacker payout (correctly capped) should be > 0")

	// Withdraw costaker rewards for the attacker and compute the received amount excluding tx fees.
	s.Require().Equal(sdkmath.NewInt(btcStakeSatAmt), trackerHonestWithBtc.TotalScore, "honest costaker should be fully collateralized and not capped")

	attackerReceived := s.withdrawRewardAndGetReceived(n, ictvtypes.COSTAKER.String(), costakerWallet, costakerAddr)
	attackerReceivedUbbn := attackerReceived.AmountOf(initialization.BabylonDenom)
	s.Require().True(attackerReceivedUbbn.IsPositive(), "attacker should receive non-zero rewards")
	s.Require().True(attackerReceivedUbbn.GT(expectedAttackerUbbnCorrect),
		"attacker should be overpaid vs correct cap: received=%s expected_correct=%s (rewards=%s correct_cap_sats=%s honest_score=%s)",
		attackerReceivedUbbn.String(),
		expectedAttackerUbbnCorrect.String(),
		rewardsUbbn.String(),
		correctCapSats.String(),
		trackerHonestWithBtc.TotalScore.String(),
	)
}

func (s *DowntimeSlashUnjailStaleActiveBabyTestSuite) getEpochInterval(node *chain.NodeConfig) uint64 {
	var epochingParamsWrapper struct {
		Params struct {
			EpochInterval string `json:"epoch_interval"`
		} `json:"params"`
	}
	node.QueryParams("epoching", &epochingParamsWrapper)
	epochInterval, err := strconv.ParseUint(epochingParamsWrapper.Params.EpochInterval, 10, 64)
	s.Require().NoError(err)
	s.Require().Greater(epochInterval, uint64(0))
	return epochInterval
}

func (s *DowntimeSlashUnjailStaleActiveBabyTestSuite) getScoreRatioBtcByBaby(node *chain.NodeConfig) sdkmath.Int {
	var costakingParamsWrapper struct {
		Params struct {
			ScoreRatioBtcByBaby sdkmath.Int `json:"score_ratio_btc_by_baby"`
		} `json:"params"`
	}
	node.QueryParams("costaking", &costakingParamsWrapper)
	s.Require().True(costakingParamsWrapper.Params.ScoreRatioBtcByBaby.IsPositive())
	return costakingParamsWrapper.Params.ScoreRatioBtcByBaby
}

func (s *DowntimeSlashUnjailStaleActiveBabyTestSuite) queryCostakingCurrentRewards(node *chain.NodeConfig) (*costakingtypes.QueryCurrentRewardsResponse, error) {
	bz, err := node.QueryGRPCGateway("/babylon/costaking/v1/current_rewards", url.Values{})
	if err != nil {
		return nil, err
	}
	var resp costakingtypes.QueryCurrentRewardsResponse
	if err := util.Cdc.UnmarshalJSON(bz, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *DowntimeSlashUnjailStaleActiveBabyTestSuite) withdrawRewardAndGetReceived(
	node *chain.NodeConfig,
	stakeholderType string,
	fromWallet string,
	receiverAddr string,
) sdk.Coins {
	balanceBefore, err := node.QueryBalances(receiverAddr)
	s.Require().NoError(err)

	txHash := node.WithdrawReward(stakeholderType, fromWallet)
	s.waitForTxSuccess(node, txHash)
	_, txAuth, err := node.QueryTxWithError(txHash)
	s.Require().NoError(err)

	balanceAfter, err := node.QueryBalances(receiverAddr)
	s.Require().NoError(err)

	// received = (after + fee) - before
	return balanceAfter.Add(txAuth.AuthInfo.Fee.Amount...).Sub(balanceBefore...)
}

func (s *DowntimeSlashUnjailStaleActiveBabyTestSuite) waitForTxSuccess(node *chain.NodeConfig, txHash string) {
	s.Require().NotEmpty(txHash)
	var resp sdk.TxResponse
	s.Require().Eventually(func() bool {
		r, _, err := node.QueryTxWithError(txHash)
		if err != nil {
			return false
		}
		resp = r
		return true
	}, 45*time.Second, 500*time.Millisecond, "tx %s should be committed", txHash)
	s.Require().Equal(uint32(0), resp.Code, "tx %s should succeed (code=0), raw_log=%s", txHash, resp.RawLog)
}

func (s *DowntimeSlashUnjailStaleActiveBabyTestSuite) waitForHeight(node *chain.NodeConfig, targetHeight int64) {
	maxAttempts := 500
	for i := 0; i < maxAttempts; i++ {
		currentHeight, err := node.QueryCurrentHeight()
		if err == nil && currentHeight >= targetHeight {
			s.T().Logf("Reached target height %d (current: %d)", targetHeight, currentHeight)
			return
		}
		if i%10 == 0 {
			s.T().Logf("Waiting for height %d, current: %d (attempt %d/%d)", targetHeight, currentHeight, i+1, maxAttempts)
		}
		time.Sleep(2 * time.Second)
	}
	s.FailNow("Timeout waiting for height %d", targetHeight)
}

func (s *DowntimeSlashUnjailStaleActiveBabyTestSuite) getDelegationTokensToValidator(
	node *chain.NodeConfig,
	delegatorAddr string,
	validatorAddr string,
) sdkmath.Int {
	delegations, err := node.QueryDelegatorDelegations(delegatorAddr)
	s.Require().NoError(err)
	for _, d := range delegations {
		if d.Delegation.ValidatorAddress == validatorAddr {
			return d.Balance.Amount
		}
	}
	s.FailNow("delegation not found for delegator=%s validator=%s", delegatorAddr, validatorAddr)
	return sdkmath.ZeroInt()
}
