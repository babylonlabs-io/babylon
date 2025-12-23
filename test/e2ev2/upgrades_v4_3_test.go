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

// TestUpgradeV43 creates the scenario where an misscalculation in costaking
// reward tracker in version v4.2.2 where one delegator:
// 1. Creates two healthy baby delegations (A and B)
// 2. Some epoch starts
// 3. Validator B gets slashed
// 4. Delegator unbonds from B
// 5. Delegator delegates again to B
// 6. Delegator unbonds again from B
// 7. Epoch ends
// Results in misscalculation of active baby, the upgrade to v4.3 should
// recalculate all the active baby and score in the system
func TestUpgradeV43(t *testing.T) {
	t.Parallel()
	tm := tmanager.NewTmWithUpgrade(t, 0, "")
	validator := tm.ChainValidator()

	n := tm.Chains[tmanager.CHAIN_ID_BABYLON].Nodes[0]

	tm.Start() // start chain with v4.2.2 binary
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

	// creates the FP
	fp := n.NewFpWithWallet(wFp)
	fp.CommitPubRand()

	btcDel := n.CreateBtcDelegation(delegator, fp.PublicKey.MustToBTCPK())

	n.WaitUntilCurrentEpochIsSealedAndFinalized(1)
	n.WaitFinalityIsActivated()

	fp.AddFinalityVoteUntilCurrentHeight()

	// create validator to be slashed
	valSlashAddr := sdk.ValAddress(valSlashWallet.Address)
	n.WrappedCreateValidator(valSlashWallet.KeyName, valSlashWallet.Address)

	n.WaitForEpochEnd() // validator must wait for end of epoch

	valSlash := n.QueryValidator(valSlashAddr)
	require.True(t, valSlash.IsBonded())

	// creates healthy two delegations
	amtHealthyDel, amtSlashDel := sdkmath.NewInt(10_000000), sdkmath.NewInt(2_000000)
	n.WrappedDelegate(delegator.KeyName, validator.Wallet.ValidatorAddress, amtHealthyDel)
	n.WrappedDelegate(delegator.KeyName, valSlashAddr, amtSlashDel)

	n.WaitForEpochEnd() // to process the new baby delegations

	fp.AddFinalityVoteUntilCurrentHeight()

	costkP := n.QueryCostkParams()

	expSat := sdkmath.NewInt(int64(btcDel.TotalSat))
	expBaby := amtHealthyDel.Add(amtSlashDel)
	expScore := costktypes.CalculateScore(costkP.ScoreRatioBtcByBaby, expBaby, expSat)

	costkRwdTracker := n.QueryCostkRwdTrckCli(delegator.Address)
	require.Equal(t, expBaby.String(), costkRwdTracker.ActiveBaby.String())
	require.Equal(t, expSat.String(), costkRwdTracker.ActiveSatoshis.String())
	require.Equal(t, expScore.String(), costkRwdTracker.TotalScore.String())

	n.WaitForEpochEnd() // to process the new baby delegations

	slashedVal := n.QueryValidator(valSlashAddr)
	require.False(t, slashedVal.Jailed)

	// after validator gets jailed it also slashes the amount staked in 10%
	slashedVal = n.WaitForValidatorBeJailed(valSlashAddr)

	slashDelegation := n.QueryDelegation(delegator.Address, valSlashAddr)

	sharesToUbd := slashedVal.TokensFromShares(slashDelegation.Delegation.Shares)
	n.WrappedUndelegate(delegator.KeyName, valSlashAddr, sharesToUbd.TruncateInt())

	amtSlashDel2 := sdkmath.NewInt(1_500000) // NOTE: this amount needs to be less than first del to slashed val (amtSlashDel)
	n.WrappedDelegate(delegator.KeyName, valSlashAddr, amtSlashDel2)
	n.WrappedUndelegate(delegator.KeyName, valSlashAddr, amtSlashDel2)

	n.WaitForEpochEnd()

	// costk is in an bad state created by the bug in v4.2.2
	// 2 stakes, val slash, unbond, bond, unbond again
	costkRwdTrackerBeforeUpgrade := n.QueryCostkRwdTrckCli(delegator.Address)
	t.Logf("costaker reward tracker is in bad state where it should have the amount %s, but has %s due to bug", amtHealthyDel.String(), costkRwdTrackerBeforeUpgrade.ActiveBaby.String())
	require.True(t, amtHealthyDel.GT(costkRwdTrackerBeforeUpgrade.ActiveBaby))
	require.Equal(t, expSat.String(), costkRwdTrackerBeforeUpgrade.ActiveSatoshis.String())

	expScoreAfterSlash := costktypes.CalculateScore(costkP.ScoreRatioBtcByBaby, costkRwdTrackerBeforeUpgrade.ActiveBaby, expSat)
	require.Equal(t, expScoreAfterSlash.String(), costkRwdTrackerBeforeUpgrade.TotalScore.String())

	currRwdBeforeUpgrade := n.QueryCostkCurrRwdCli()
	require.Equal(t, currRwdBeforeUpgrade.Period, costkRwdTrackerBeforeUpgrade.StartPeriodCumulativeReward+1)
	require.Equal(t, currRwdBeforeUpgrade.TotalScore.String(), costkRwdTrackerBeforeUpgrade.TotalScore.String())

	// execute preUpgradeFunc, submit a proposal, vote, and then process upgrade
	govMsg, preUpgradeFunc := createGovPropAndPreUpgradeFunc(t, validator.Wallet.WalletSender)
	tm.Upgrade(govMsg, preUpgradeFunc)

	btcDelsResp := validator.QueryBTCDelegations(bstypes.BTCDelegationStatus_ACTIVE)
	require.Len(t, btcDelsResp, 1)
	// post-upgrade state verification

	expScore = costktypes.CalculateScore(costkP.ScoreRatioBtcByBaby, amtHealthyDel, expSat)

	// The costaking should reflect the actual amount of baby staked to the healthy validator
	costkRwdTrackerAfterUpgrade := n.QueryCostkRwdTrckCli(delegator.Address)
	require.Equal(t, amtHealthyDel.String(), costkRwdTrackerAfterUpgrade.ActiveBaby.String())
	require.Equal(t, expSat.String(), costkRwdTrackerAfterUpgrade.ActiveSatoshis.String())
	require.Equal(t, expScore.String(), costkRwdTrackerAfterUpgrade.TotalScore.String())

	currRwdAfterUpgrade := n.QueryCostkCurrRwdCli()
	require.Equal(t, costkRwdTrackerBeforeUpgrade.StartPeriodCumulativeReward+1, costkRwdTrackerAfterUpgrade.StartPeriodCumulativeReward)
	require.Equal(t, currRwdAfterUpgrade.Period, costkRwdTrackerAfterUpgrade.StartPeriodCumulativeReward+1)
	require.Equal(t, currRwdAfterUpgrade.Period, currRwdBeforeUpgrade.Period+1)
	require.Equal(t, currRwdAfterUpgrade.TotalScore.String(), currRwdAfterUpgrade.TotalScore.String())
}

func createGovPropAndPreUpgradeFunc(t *testing.T, valWallet *tmanager.WalletSender) (*govtypes.MsgSubmitProposal, tmanager.PreUpgradeFunc) {
	upgradeMsg := &upgradetypes.MsgSoftwareUpgrade{
		Authority: "bbn10d07y265gmmuvt4z0w9aw880jnsr700jduz5f2",
		Plan: upgradetypes.Plan{
			Name:   v43.UpgradeName,
			Height: int64(20),
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
