package e2e2

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/btcsuite/btcd/btcec/v2"

	sdkmath "cosmossdk.io/math"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	v43 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v4_3"
	"github.com/babylonlabs-io/babylon/v4/test/e2ev2/tmanager"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
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

	initAmtOfWallets := sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(100_000000))
	n.SendCoins(valSlashWallet.Address.String(), sdk.NewCoins(initAmtOfWallets))
	n.WaitForNextBlock()

	n.UpdateWalletAccSeqNumber(valSlashWallet.KeyName)

	// create validator to be slashed
	valSlashAddr := sdk.ValAddress(valSlashWallet.Address)
	n.WrappedCreateValidator(valSlashWallet.KeyName, valSlashWallet.Address)

	n.SendCoins(delegator.Address.String(), sdk.NewCoins(initAmtOfWallets))
	n.WaitForEpochEnd() // validator must wait for end of epoch
	n.UpdateWalletAccSeqNumber(delegator.KeyName)

	valSlash := n.QueryValidator(valSlashAddr)
	require.True(t, valSlash.IsBonded())

	// creates healthy two delegations
	amtHealthyDel, amtSlashDel := sdkmath.NewInt(10_000000), sdkmath.NewInt(2_000000)
	n.WrappedDelegate(delegator.KeyName, validator.Wallet.ValidatorAddress, amtHealthyDel)
	n.WrappedDelegate(delegator.KeyName, valSlashAddr, amtSlashDel)

	fpSK := setupFp(t, tm.R, n)
	n.CreateBtcDelegation(delegator, fpSK.PubKey())

	n.WaitForEpochEnd() // to process the new baby delegations

	costkRwdTracker := n.QueryCostkRwdTrckCli(delegator.Address)
	require.Equal(t, amtHealthyDel.Add(amtSlashDel).String(), costkRwdTracker.ActiveBaby.String())

	slashedVal := n.QueryValidator(valSlashAddr)
	require.False(t, slashedVal.Jailed)

	slashedVal = n.WaitForValidatorBeJailed(valSlashAddr)

	slashDelegation := n.QueryDelegation(delegator.Address, valSlashAddr)

	sharesToUbd := slashedVal.TokensFromShares(slashDelegation.Delegation.Shares)
	n.WrappedUndelegate(delegator.KeyName, valSlashAddr, sharesToUbd.TruncateInt())

	amtSlashDel2 := sdkmath.NewInt(1_500000) // this amount needs to be less than amtSlashDel
	n.WrappedDelegate(delegator.KeyName, valSlashAddr, amtSlashDel2)
	n.WaitForNextBlock()
	n.WrappedUndelegate(delegator.KeyName, valSlashAddr, amtSlashDel2)

	n.WaitForEpochEnd()

	// costk is in an bad state created by the bug in v4.2.2
	// 2 stakes, val slash, unbond, bond, unbond again
	costkRwdTracker = n.QueryCostkRwdTrckCli(delegator.Address)
	t.Logf("costaker reward tracker is in bad state where it should have the amount %s, but has %s due to bug", amtHealthyDel.String(), costkRwdTracker.ActiveBaby.String())
	require.True(t, amtHealthyDel.GT(costkRwdTracker.ActiveBaby))

	govMsg, preUpgradeFunc := createGovPropAndPreUpgradeFunc(t, validator.Wallet.WalletSender)
	// execute preUpgradeFunc, submit a proposal, vote, and then process upgrade
	tm.Upgrade(govMsg, preUpgradeFunc)

	// post-upgrade state verification

	// The costaking should reflect the actual amount of baby staked to the healthy validator
	costkRwdTracker = n.QueryCostkRwdTrckCli(delegator.Address)
	require.Equal(t, amtHealthyDel.String(), costkRwdTracker.ActiveBaby.String())

	btcDelsResp := validator.QueryBTCDelegations(bstypes.BTCDelegationStatus_ACTIVE)
	require.Len(t, btcDelsResp, 1)
}

func createGovPropAndPreUpgradeFunc(t *testing.T, valWallet *tmanager.WalletSender) (*govtypes.MsgSubmitProposal, tmanager.PreUpgradeFunc) {
	// create the upgrade message
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

func setupFp(t *testing.T, r *rand.Rand, n *tmanager.Node) *btcec.PrivateKey {
	fpSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	fp, err := datagen.GenCustomFinalityProvider(r, fpSK, n.DefaultWallet().Address)
	require.NoError(t, err)
	n.CreateFinalityProvider(n.DefaultWallet().KeyName, fp)
	n.WaitForNextBlock()

	fpResp := n.QueryFinalityProvider(fp.BtcPk.MarshalHex())
	require.NotNil(t, fpResp)

	return fpSK
}
