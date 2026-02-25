package e2e2

import (
	"testing"

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

// TestUpgradeV43 reproduces a costaking reward tracker miscalculation
// from v4.2.2 and verifies the v4.3 upgrade corrects it.
//
// Bug scenario:
//  1. Delegator creates two BABY delegations: healthy validator A
//     and validator B
//  2. Validator B gets slashed (jailed by downtime)
//  3. Delegator unbonds from slashed B
//  4. Delegator delegates again to slashed B
//  5. Delegator unbonds again from slashed B
//
// This causes ActiveBaby to be lower than expected because post-slash
// delegations (never added to ActiveBaby) are incorrectly subtracted
// on unbond. The v4.3 upgrade recalculates all ActiveBaby and scores.
func TestUpgradeV43(t *testing.T) {
	t.Parallel()

	// --- Chain and wallets setup ---
	tm := tmanager.NewTmWithUpgrade(t, 0, "")
	validator := tm.ChainValidator()
	n := tm.Chains[tmanager.CHAIN_ID_BABYLON].Nodes[0]

	// start chain with v4.2.x
	tm.Start()
	validator.WaitUntilBlkHeight(3)

	valSlashWallet := n.CreateWallet("slashed")
	valSlashWallet.VerifySentTx = true
	delegator := n.CreateWallet("delegator")
	delegator.VerifySentTx = true
	wFp := n.CreateWallet("healthy_fp")
	wFp.VerifySentTx = true

	initAmtOfWallets := sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(100_000000))
	n.SendCoins(valSlashWallet.Address.String(), sdk.NewCoins(initAmtOfWallets))
	n.WaitForNextBlock()
	n.SendCoins(wFp.Address.String(), sdk.NewCoins(initAmtOfWallets))
	n.WaitForNextBlock()
	n.SendCoins(delegator.Address.String(), sdk.NewCoins(initAmtOfWallets))
	n.WaitForNextBlock()
	n.UpdateWalletAccSeqNumber(valSlashWallet.KeyName, delegator.KeyName, wFp.KeyName)

	// --- Finality provider + BTC delegation ---
	fp := n.NewFpWithWallet(wFp)
	fp.CommitPubRand()
	btcDel := n.CreateBtcDelegation(delegator, fp.PublicKey.MustToBTCPK())

	n.WaitUntilCurrentEpochIsSealedAndFinalized(1)
	n.WaitFinalityIsActivated()
	fp.AddFinalityVoteUntilCurrentHeight()

	// --- Create second validator (will be slashed later) ---
	valSlashAddr := sdk.ValAddress(valSlashWallet.Address)
	n.WrappedCreateValidator(valSlashWallet.KeyName, valSlashWallet.Address)
	n.WaitForEpochEnd()

	valSlash := n.QueryValidator(valSlashAddr)
	require.True(t, valSlash.IsBonded())

	// --- Delegate BABY to both validators ---
	amtHealthyDel := sdkmath.NewInt(10_000000)
	amtSlashDel := sdkmath.NewInt(2_000000)
	n.WrappedDelegate(delegator.KeyName, validator.Wallet.ValidatorAddress, amtHealthyDel)
	n.WrappedDelegate(delegator.KeyName, valSlashAddr, amtSlashDel)
	n.WaitForEpochEnd()

	fp.AddFinalityVoteUntilCurrentHeight()

	// --- Verify costaking state before slashing ---
	costkP := n.QueryCostkParams()
	expSat := sdkmath.NewInt(int64(btcDel.TotalSat))
	expBaby := amtHealthyDel.Add(amtSlashDel)
	expScore := costktypes.CalculateScore(costkP.ScoreRatioBtcByBaby, expBaby, expSat)

	costkRwdTracker := n.QueryCostkRwdTrckCli(delegator.Address)
	require.Equal(t, expBaby.String(), costkRwdTracker.ActiveBaby.String())
	require.Equal(t, expSat.String(), costkRwdTracker.ActiveSatoshis.String())
	require.Equal(t, expScore.String(), costkRwdTracker.TotalScore.String())

	// --- Slash validator B (jail by downtime) ---
	n.WaitForEpochEnd()

	slashedVal := n.QueryValidator(valSlashAddr)
	require.False(t, slashedVal.Jailed)

	slashedVal = n.WaitForValidatorBeJailed(valSlashAddr)

	// --- Reproduce bug: unbond, re-delegate, unbond from slashed validator ---
	slashDelegation := n.QueryDelegation(delegator.Address, valSlashAddr)
	sharesToUbd := slashedVal.TokensFromShares(slashDelegation.Delegation.Shares)
	n.WrappedUndelegate(delegator.KeyName, valSlashAddr, sharesToUbd.TruncateInt())

	// NOTE: amount must be less than first delegation to slashed validator
	amtSlashDel2 := sdkmath.NewInt(1_500000)
	n.WrappedDelegate(delegator.KeyName, valSlashAddr, amtSlashDel2)
	n.WrappedUndelegate(delegator.KeyName, valSlashAddr, amtSlashDel2)

	n.WaitForEpochEnd()

	// --- Verify costaking is in a bad state (v4.2.2 bug) ---
	costkRwdTrackerBeforeUpgrade := n.QueryCostkRwdTrckCli(delegator.Address)
	t.Logf(
		"costaker reward tracker is in bad state where it should have the amount %s, but has %s due to bug",
		amtHealthyDel.String(), costkRwdTrackerBeforeUpgrade.ActiveBaby.String(),
	)
	require.True(t, amtHealthyDel.GT(costkRwdTrackerBeforeUpgrade.ActiveBaby))
	require.Equal(t, expSat.String(), costkRwdTrackerBeforeUpgrade.ActiveSatoshis.String())

	expScoreAfterSlash := costktypes.CalculateScore(
		costkP.ScoreRatioBtcByBaby, costkRwdTrackerBeforeUpgrade.ActiveBaby, expSat,
	)
	require.Equal(t, expScoreAfterSlash.String(), costkRwdTrackerBeforeUpgrade.TotalScore.String())

	currRwdBeforeUpgrade := n.QueryCostkCurrRwdCli()
	require.Equal(t, currRwdBeforeUpgrade.Period, costkRwdTrackerBeforeUpgrade.StartPeriodCumulativeReward+1)
	require.Equal(t, currRwdBeforeUpgrade.TotalScore.String(), costkRwdTrackerBeforeUpgrade.TotalScore.String())

	// --- Submit upgrade proposal and execute ---
	paramsVersionsBeforeUpgrade := n.QueryBtcStakingParamsVersions()

	currEpoch := n.QueryCurrentEpoch()
	firstBlockOfNextEpoch := currEpoch.EpochBoundary + 1
	govMsg, preUpgradeFunc := createGovPropAndPreUpgradeFunc(
		t, validator.Wallet.WalletSender, int64(firstBlockOfNextEpoch),
	)
	tm.Upgrade(govMsg, preUpgradeFunc)

	// --- Verify post-upgrade state ---
	paramsVersionsAfterUpgrade := n.QueryBtcStakingParamsVersions()
	require.Equal(t, paramsVersionsBeforeUpgrade, paramsVersionsAfterUpgrade)

	btcDelsResp := validator.QueryBTCDelegations(bstypes.BTCDelegationStatus_ACTIVE)
	require.Len(t, btcDelsResp, 1)

	// Costaking should now reflect only the healthy delegation
	expScore = costktypes.CalculateScore(costkP.ScoreRatioBtcByBaby, amtHealthyDel, expSat)

	costkRwdTrackerAfterUpgrade := n.QueryCostkRwdTrckCli(delegator.Address)
	require.Equal(t, amtHealthyDel.String(), costkRwdTrackerAfterUpgrade.ActiveBaby.String())
	require.Equal(t, expSat.String(), costkRwdTrackerAfterUpgrade.ActiveSatoshis.String())
	require.Equal(t, expScore.String(), costkRwdTrackerAfterUpgrade.TotalScore.String())

	// Reward period should have advanced by 1
	currRwdAfterUpgrade := n.QueryCostkCurrRwdCli()
	require.Equal(t, costkRwdTrackerBeforeUpgrade.StartPeriodCumulativeReward+1, costkRwdTrackerAfterUpgrade.StartPeriodCumulativeReward)
	require.Equal(t, currRwdAfterUpgrade.Period, costkRwdTrackerAfterUpgrade.StartPeriodCumulativeReward+1)
	require.Equal(t, currRwdAfterUpgrade.Period, currRwdBeforeUpgrade.Period+1)
	require.Equal(t, currRwdAfterUpgrade.TotalScore.String(), costkRwdTrackerAfterUpgrade.TotalScore.String())
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
