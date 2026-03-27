package e2e2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	v43 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v4_3"
	"github.com/babylonlabs-io/babylon/v4/test/e2ev2/tmanager"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	costktypes "github.com/babylonlabs-io/babylon/v4/x/costaking/types"
)

var ZeroInt = sdkmath.ZeroInt()

// TestUpgradeV43 reproduces two costaking reward tracker bugs from
// v4.2.x and verifies the v4.3 upgrade corrects them.
//
// Scenario 1 (unbond/re-delegate from slashed validator):
//  1. del1 creates two BABY delegations: healthy chain validator and val1
//  2. val1 gets slashed (jailed by downtime)
//  3. del1 unbonds from slashed val1
//  4. del1 delegates again to slashed val1
//  5. del1 unbonds again from slashed val1
//
// Result: del1's ActiveBaby lower than expected
//
// Scenario 2 (jail/unjail + delegate in same epoch):
//  1. del2 delegates BABY to val2
//  2. val2 gets jailed by downtime
//  3. val2 is unjailed
//  4. del3 delegates BABY to val2 and healthy chain validator (same epoch)
//
// Result: del3's ActiveBaby lower than expected
//
// The v4.3 upgrade recalculates all ActiveBaby and scores.
func TestUpgradeV43(t *testing.T) {
	t.Parallel()

	// =====================================================================
	// Chain and wallets setup
	// =====================================================================
	tm := tmanager.NewTmWithUpgrade(t, 0, "", func(cfg *tmanager.ChainConfig) {
		cfg.EpochLength = 40
		cfg.DowntimeJailDuration = 10 * time.Second
	})
	chainVal := tm.ChainValidator()
	n := tm.Chains[tmanager.CHAIN_ID_BABYLON].Nodes[0]

	tm.Start()
	chainVal.WaitUntilBlkHeight(3)

	// Scenario 1 wallets
	wVal1 := n.CreateWallet("val1")
	wVal1.VerifySentTx = true
	del1 := n.CreateWallet("del1")
	del1.VerifySentTx = true
	wFp1 := n.CreateWallet("fp1")
	wFp1.VerifySentTx = true

	// Scenario 2 wallets
	wVal2 := n.CreateWallet("val2")
	wVal2.VerifySentTx = true
	del2 := n.CreateWallet("del2")
	del2.VerifySentTx = true
	del3 := n.CreateWallet("del3")
	del3.VerifySentTx = true

	initAmtOfWallets := sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(100_000000))
	allWalletAddrs := []string{
		wVal1.Address.String(),
		wFp1.Address.String(),
		del1.Address.String(),
		wVal2.Address.String(),
		del2.Address.String(),
		del3.Address.String(),
	}
	for _, addr := range allWalletAddrs {
		n.SendCoins(addr, sdk.NewCoins(initAmtOfWallets))
		n.WaitForNextBlock()
	}
	n.UpdateWalletAccSeqNumber(
		wVal1.KeyName, del1.KeyName, wFp1.KeyName,
		wVal2.KeyName, del2.KeyName, del3.KeyName,
	)

	// In this point every wallet from scenario 1 and 2 is funded and has updated acc sequence and number

	// =====================================================================
	// Finality providers + BTC delegations
	// =====================================================================
	fp := n.NewFpWithWallet(wFp1)
	fp.CommitPubRand()
	fpPk := fp.PublicKey.MustToBTCPK()
	btcDel1 := n.CreateBtcDelegation(del1, fpPk)
	btcDel2 := n.CreateBtcDelegation(del2, fpPk)
	btcDel3 := n.CreateBtcDelegation(del3, fpPk)

	n.WaitUntilCurrentEpochIsSealedAndFinalized(1)
	n.WaitFinalityIsActivated()
	fp.AddFinalityVoteUntilCurrentHeight()

	btcDelsFromQuery := n.QueryBTCDelegations(bstypes.BTCDelegationStatus_ACTIVE)
	require.Len(t, btcDelsFromQuery, 3)
	for _, btcDelFromQuery := range btcDelsFromQuery {
		switch btcDelFromQuery.StakingTxHex {
		case btcDel1.StakingTxHex, btcDel2.StakingTxHex, btcDel3.StakingTxHex:
			continue
		default:
			t.Error("failed to find active BTC delegation")
		}
	}

	expSat := sdkmath.NewInt(int64(btcDel1.TotalSat))

	// On this step all the btc delegations were created, one for each delegator.
	// Scenario 1:
	//   Fp => Active and with delegations
	//   BabyVal1 => Not created yet
	//   Del1 => 2BTC to FP, no baby delegations
	n.CheckCostaking(del1.Address, expSat, ZeroInt, ZeroInt)

	// Scenario 2:
	//   Fp => Active and with delegations
	//   BabyVal2 => Not created yet
	//   Del2 => 2BTC to FP, no baby delegations
	//   Del3 => 2BTC to FP, no baby delegations
	n.CheckCostaking(del2.Address, expSat, ZeroInt, ZeroInt)
	n.CheckCostaking(del3.Address, expSat, ZeroInt, ZeroInt)

	// =====================================================================
	// Create validators (both created in the same epoch, both will be
	// jailed by downtime after ~85 blocks since they never sign any block)
	// =====================================================================
	val1Addr := sdk.ValAddress(wVal1.Address)
	n.WrappedCreateValidator(wVal1.KeyName, wVal1.Address)

	val2Addr := sdk.ValAddress(wVal2.Address)
	n.WrappedCreateValidator(wVal2.KeyName, wVal2.Address)

	n.WaitForEpochEnd()

	val1 := n.QueryValidator(val1Addr)
	require.True(t, val1.IsBonded(), "val1 should be bonded")
	val2 := n.QueryValidator(val2Addr)
	require.True(t, val2.IsBonded(), "val2 should be bonded")

	// =====================================================================
	// Delegate BABY to validators
	// =====================================================================
	// Scenario 1: del1 delegates to healthy chain validator and val1
	amtHealthyDel := sdkmath.NewInt(10_000000)
	amtSlashDel := sdkmath.NewInt(2_000000)
	n.WrappedDelegate(del1.KeyName, chainVal.Wallet.ValidatorAddress, amtHealthyDel)
	n.WrappedDelegate(del1.KeyName, val1Addr, amtSlashDel)

	// Scenario 2: del2 delegates to val2 before jailing
	amtDel2toVal2 := sdkmath.NewInt(5_000000)
	n.WrappedDelegate(del2.KeyName, val2Addr, amtDel2toVal2)

	n.WaitForEpochEnd()

	fp.AddFinalityVoteUntilCurrentHeight()
	costkP := n.QueryCostkParams()

	// Check everyone costaking amounts before continuing
	// Scenario 1:
	//   Fp => Active and with delegations
	//   BabyVal1 => Created with baby delegation
	//   Del1 => 2BTC to FP, 10BABY to healthy and 2BABY to val1
	expBabyAmtDel1 := amtHealthyDel.Add(amtSlashDel)
	expScoreDel1 := costktypes.CalculateScore(costkP.ScoreRatioBtcByBaby, expBabyAmtDel1, expSat)
	n.CheckCostaking(del1.Address, expSat, expBabyAmtDel1, expScoreDel1)

	// Scenario 2:
	//   Fp => Active and with delegations
	//   BabyVal2 => Created with baby delegation
	//   Del2 => 2BTC to FP, 5BABY to val2
	//   Del3 =>  2BTC to FP, no baby delegations
	expBabyAmtDel2 := amtDel2toVal2
	expScoreDel2 := costktypes.CalculateScore(costkP.ScoreRatioBtcByBaby, expBabyAmtDel2, expSat)
	n.CheckCostaking(del2.Address, expSat, expBabyAmtDel2, expScoreDel2)
	n.CheckCostaking(del3.Address, expSat, ZeroInt, ZeroInt)

	// =====================================================================
	// Wait for val1 and val2 to be jailed by downtime
	// (both were created in the same epoch and never sign blocks)
	// =====================================================================
	n.WaitForEpochEnd()

	val1 = n.QueryValidator(val1Addr)
	require.False(t, val1.Jailed, "val1 should not be jailed yet")
	val2 = n.QueryValidator(val2Addr)
	require.False(t, val2.Jailed, "val2 should not be jailed yet")

	// val1 is jailed after ~85 blocks of missing signatures
	val1 = n.WaitForValidatorBeJailed(val1Addr)
	require.True(t, val1.Jailed, "val1 should be jailed")

	// val2 was created at the same time as val1, so should also be jailed
	val2 = n.WaitForValidatorBeJailed(val2Addr)
	require.True(t, val2.Jailed, "val2 should be jailed")

	// =====================================================================
	// Scenario 1: unbond, re-delegate, unbond from slashed val1
	// =====================================================================
	slashDelegation := n.QueryDelegation(del1.Address, val1Addr)
	sharesToUbd := val1.TokensFromShares(slashDelegation.Delegation.Shares)
	n.WrappedUndelegate(del1.KeyName, val1Addr, sharesToUbd.TruncateInt())

	// NOTE: amount must be less than first delegation to slashed validator
	amtSlashDel2 := sdkmath.NewInt(1_500000)
	n.WrappedDelegate(del1.KeyName, val1Addr, amtSlashDel2)
	n.WrappedUndelegate(del1.KeyName, val1Addr, amtSlashDel2)

	// Scenario 1's txs above add several blocks. Wait additional blocks
	// to ensure DowntimeJailDuration (10s) has fully elapsed.
	n.WaitForEpochEnd()

	// =====================================================================
	// Verify scenario 1: del1's costaking is in a bad state (v4.2.2 bug)
	// =====================================================================
	costkRwdTrackerBeforeUpgrade := n.QueryCostkRwdTrckCli(del1.Address)
	t.Logf(
		"scenario 1: del1's costaker reward tracker should have %s, but has %s due to bug",
		amtHealthyDel.String(), costkRwdTrackerBeforeUpgrade.ActiveBaby.String(),
	)
	require.True(t, amtHealthyDel.GT(costkRwdTrackerBeforeUpgrade.ActiveBaby))
	require.Equal(t, expSat.String(), costkRwdTrackerBeforeUpgrade.ActiveSatoshis.String())

	expScoreAfterSlash := costktypes.CalculateScore(
		costkP.ScoreRatioBtcByBaby, costkRwdTrackerBeforeUpgrade.ActiveBaby, expSat,
	)
	require.Equal(t, expScoreAfterSlash.String(), costkRwdTrackerBeforeUpgrade.TotalScore.String())

	currRwdBeforeUpgrade := n.QueryCostkCurrRwdCli()
	require.True(t, currRwdBeforeUpgrade.Period > costkRwdTrackerBeforeUpgrade.StartPeriodCumulativeReward)

	// Scenario 2:
	//   Fp => Active and with delegations
	//   BabyVal2 => Jailed with baby delegation
	//   Del2 => 2BTC to FP, 5BABY to val2(jailed)
	//   Del3 =>  2BTC to FP, no baby delegations
	n.CheckCostaking(del2.Address, expSat, ZeroInt, ZeroInt)
	n.CheckCostaking(del3.Address, expSat, ZeroInt, ZeroInt)

	// =====================================================================
	// Scenario 2: wait until next epoch do execute all the actions of scenario 2 bug in a single epoch
	// =====================================================================
	// - Unjail val2
	// - del3 delegates to healthy val
	// - del3 delegates to val2
	n.WaitForEpochEnd()

	n.Unjail(wVal2.KeyName, val2Addr)
	val2AfterUnjail := n.QueryValidator(val2Addr)
	require.False(t, val2AfterUnjail.Jailed, "val2 should be unjailed")

	// Del3 10BABY to healthy val and 2BABY to just unjailed validator
	amtDel3UnjailedVal := sdkmath.NewInt(2_000000)
	n.WrappedDelegate(del3.KeyName, chainVal.Wallet.ValidatorAddress, amtHealthyDel)
	n.WrappedDelegate(del3.KeyName, val2Addr, amtDel3UnjailedVal)

	// Scenario 2:
	//   Fp => Active and with delegations
	//   BabyVal2 => Just unjailed (takes one epoch to take effect) with baby delegation
	//   Del2 => 2BTC to FP, 5BABY to val2(jailed)
	//   Del3 => 2BTC to FP, 10BABY to healthy val (takes one epoch to have effect) and 2BABY to val2(jailed) so zero baby
	n.CheckCostaking(del2.Address, expSat, ZeroInt, ZeroInt)
	n.CheckCostaking(del3.Address, expSat, ZeroInt, ZeroInt)

	// =====================================================================
	// Update BTC staking params to create a second params version.
	// This ensures the HeightToVersionMap has multiple entries and the
	// migration correctly preserves them across the upgrade.
	// =====================================================================
	paramsVersionsBeforeParamsUpdate := n.QueryBtcStakingParamsVersions()
	require.Len(t, paramsVersionsBeforeParamsUpdate, 1)

	updatedParams := n.QueryBtcStakingParams()
	updatedParams.BtcActivationHeight += 1000
	submitAndPassBtcStakingParamsUpdate(t, chainVal, *updatedParams)

	paramsVersionsAfterParamsUpdate := n.QueryBtcStakingParamsVersions()
	require.Len(t, paramsVersionsAfterParamsUpdate, 2)
	require.Equal(t, updatedParams.BtcActivationHeight, paramsVersionsAfterParamsUpdate[1].Params.BtcActivationHeight)

	// =====================================================================
	// Submit upgrade proposal and execute, verifying the epochs
	// =====================================================================
	paramsVersionsBeforeUpgrade := paramsVersionsAfterParamsUpdate

	epochBeforeUpgrade := n.QueryCurrentEpoch()

	// The params update governance proposal takes ~12s (voting period) which
	// may advance the epoch, so recalculate the upgrade height from current state.
	secondBlockOfNextEpoch := epochBeforeUpgrade.EpochBoundary + 2
	govMsg, preUpgradeFunc := createGovPropAndPreUpgradeFunc(
		t, chainVal.Wallet.WalletSender, int64(secondBlockOfNextEpoch),
	)

	costkRwdTrackerBeforeUpgrade = n.QueryCostkRwdTrckCli(del1.Address)
	// Upgrades and verify that only one epoch pass since the upgrade
	tm.Upgrade(govMsg, preUpgradeFunc)
	epochAfterUpgrade := n.QueryCurrentEpoch()
	require.Equal(t, epochAfterUpgrade.CurrentEpoch, epochBeforeUpgrade.CurrentEpoch+1)

	// =====================================================================
	// Verify post-upgrade state
	// =====================================================================
	paramsVersionsAfterUpgrade := n.QueryBtcStakingParamsVersions()
	require.Equal(t, paramsVersionsBeforeUpgrade, paramsVersionsAfterUpgrade)

	// =====================================================================
	// Verify HeightToVersionMap is consistent after migration
	// =====================================================================
	// Each stored param version should be queryable by its BtcActivationHeight,
	// confirming the HeightToVersionMap correctly maps heights to versions.
	for _, sp := range paramsVersionsAfterUpgrade {
		params, version := n.QueryBtcStakingParamsByBTCHeight(uint32(sp.Params.BtcActivationHeight))
		require.Equal(t, sp.Version, version,
			"HeightToVersionMap should map BtcActivationHeight %d to version %d",
			sp.Params.BtcActivationHeight, sp.Version)
		require.Equal(t, sp.Params.BtcActivationHeight, params.BtcActivationHeight)
	}

	btcDelsResp := chainVal.QueryBTCDelegations(bstypes.BTCDelegationStatus_ACTIVE)
	require.Len(t, btcDelsResp, 3)

	// Scenario 1: del1's costaking should now reflect only the healthy delegation
	expScoreDel1 = costktypes.CalculateScore(costkP.ScoreRatioBtcByBaby, amtHealthyDel, expSat)
	costkRwdTrackerAfterUpgrade := n.CheckCostaking(del1.Address, expSat, amtHealthyDel, expScoreDel1)

	// Reward periods should have advanced after the upgrade recalculated all trackers
	currRwdAfterUpgrade := n.QueryCostkCurrRwdCli()
	require.True(t, costkRwdTrackerAfterUpgrade.StartPeriodCumulativeReward > costkRwdTrackerBeforeUpgrade.StartPeriodCumulativeReward)
	require.True(t, currRwdAfterUpgrade.Period > currRwdBeforeUpgrade.Period)

	// Scenario 2:
	//   Fp => Active and with delegations
	//   BabyVal2 => Active with baby delegation
	//   Del2 => 2BTC to FP, 5BABY to val2
	//   Del3 =>  2BTC to FP, 10BABY to healthy val and 2BABY to val2
	val2 = n.QueryValidator(val2Addr)
	require.False(t, val2.Jailed, "val2 should not be jailed")
	require.True(t, val2.IsBonded(), "val2 should be bonded")

	// del2: val2 was slashed for downtime, so del2's tokens are less than the original 5BABY
	del2ToVal2 := n.QueryDelegation(del2.Address, val2Addr)
	expBabyAmtDel2 = del2ToVal2.Balance.Amount
	expScoreDel2 = costktypes.CalculateScore(costkP.ScoreRatioBtcByBaby, expBabyAmtDel2, expSat)
	n.CheckCostaking(del2.Address, expSat, expBabyAmtDel2, expScoreDel2)

	// del3
	expBabyAmtDel3 := amtHealthyDel.Add(amtDel3UnjailedVal)
	expScoreDel3 := costktypes.CalculateScore(costkP.ScoreRatioBtcByBaby, expBabyAmtDel3, expSat)
	n.CheckCostaking(del3.Address, expSat, expBabyAmtDel3, expScoreDel3)
}

func createGovPropAndPreUpgradeFunc(t *testing.T, valWallet *tmanager.WalletSender, upgradeHeight int64) (*govtypes.MsgSubmitProposal, tmanager.PreUpgradeFunc) {
	upgradeMsg := &upgradetypes.MsgSoftwareUpgrade{
		Authority: "bbn10d07y265gmmuvt4z0w9aw880jnsr700jduz5f2",
		Plan: upgradetypes.Plan{
			Name:   v43.UpgradeName,
			Height: upgradeHeight,
			Info:   "Upgrade to v4.3",
		},
	}

	anyMsg, err := types.NewAnyWithValue(upgradeMsg)
	require.NoError(t, err)

	govMsg := &govtypes.MsgSubmitProposal{
		Messages:       []*types.Any{anyMsg},
		InitialDeposit: []sdk.Coin{sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1000000))},
		Proposer:       valWallet.Address.String(),
		Metadata:       "",
		Title:          v43.UpgradeName,
		Summary:        "upgrade",
		Expedited:      false,
	}

	preUpgradeFunc := func(nodes []*tmanager.Node) {}
	return govMsg, preUpgradeFunc
}

func submitAndPassBtcStakingParamsUpdate(
	t *testing.T,
	chainVal *tmanager.ValidatorNode,
	newParams bstypes.Params,
) {
	updateMsg := &bstypes.MsgUpdateParams{
		Authority: appparams.AccGov.String(),
		Params:    newParams,
	}

	anyMsg, err := types.NewAnyWithValue(updateMsg)
	require.NoError(t, err)

	govMsg := &govtypes.MsgSubmitProposal{
		Messages:       []*types.Any{anyMsg},
		InitialDeposit: []sdk.Coin{sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1000000))},
		Proposer:       chainVal.Wallet.Address.String(),
		Metadata:       "",
		Title:          "Update BTC Staking Params",
		Summary:        "update btc staking params to create new version",
		Expedited:      false,
	}

	chainVal.UpdateWalletAccSeqNumber(chainVal.Wallet.KeyName)
	_, tx := chainVal.Wallet.SubmitMsgs(govMsg)
	require.NotNil(t, tx, "params update proposal tx should not be nil")
	chainVal.WaitForNextBlock()

	propsResp := chainVal.QueryProposals()
	require.NotEmpty(t, propsResp.Proposals)

	proposalID := propsResp.Proposals[len(propsResp.Proposals)-1].Id
	voteMsg := &govtypes.MsgVote{
		ProposalId: proposalID,
		Voter:      chainVal.Wallet.Address.String(),
		Option:     govtypes.VoteOption_VOTE_OPTION_YES,
	}
	_, voteTx := chainVal.Wallet.SubmitMsgs(voteMsg)
	require.NotNil(t, voteTx, "vote tx should not be nil")

	chainVal.WaitForCondition(func() bool {
		resp := chainVal.QueryProposals()
		for _, p := range resp.Proposals {
			if p.Id == proposalID {
				return p.Status == govtypes.ProposalStatus_PROPOSAL_STATUS_PASSED
			}
		}
		return false
	}, "waiting for btc staking params update proposal to pass")
}
