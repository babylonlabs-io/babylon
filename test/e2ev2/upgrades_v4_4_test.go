package e2e2

import (
	"testing"

	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	v44 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v4_4"
	"github.com/babylonlabs-io/babylon/v4/test/e2ev2/tmanager"
)

// TestUpgradeV44 exercises the v4.4 upgrade, which carries no state migration.
// Its only job is to coordinate the binary swap for the cosmos-sdk v0.53.8
// security release, so what this test has to prove is that the chain survives
// that swap and that the transaction paths the SDK bump touches still work
// afterwards.
//
// The pre-upgrade image runs cosmos-sdk v0.53.4 -- the version mainnet runs --
// and the locally built image runs v0.53.8, so the swap crosses the exact SDK
// delta production will cross.
func TestUpgradeV44(t *testing.T) {
	t.Parallel()

	tm := tmanager.NewTmWithUpgrade(t, 0, "")
	chainVal := tm.ChainValidator()
	n := tm.Chains[tmanager.CHAIN_ID_BABYLON].Nodes[0]

	tm.Start()
	chainVal.WaitUntilBlkHeight(3)

	del := n.CreateWallet("del")
	del.VerifySentTx = true

	initAmt := sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(100_000000))
	n.SendCoins(del.Address.String(), sdk.NewCoins(initAmt))
	n.WaitForNextBlock()
	n.UpdateWalletAccSeqNumber(del.KeyName)

	// Build staking and distribution state that has to survive the swap.
	delAmt := sdkmath.NewInt(10_000000)
	n.WrappedDelegate(del.KeyName, chainVal.Wallet.ValidatorAddress, delAmt)
	n.WaitForEpochEnd()

	delBefore := n.QueryDelegation(del.Address, chainVal.Wallet.ValidatorAddress)
	require.Equal(t, delAmt, delBefore.Balance.Amount)
	balBefore := n.QueryAllBalances(del.Address.String())

	// Height 0 lets the harness place the plan at current height + 20. v4.4 has
	// no epoch-boundary constraint, unlike the migrating upgrades.
	govMsg, preUpgradeFunc := createGovPropAndPreUpgradeFunc(
		t, chainVal.Wallet.WalletSender, v44.UpgradeName, 0,
	)

	tm.Upgrade(govMsg, preUpgradeFunc)

	// The chain is still producing blocks on the new binary.
	heightAtUpgrade, err := n.LatestBlockNumber()
	require.NoError(t, err)
	n.WaitForNextBlocks(2)
	heightLater, err := n.LatestBlockNumber()
	require.NoError(t, err)
	require.Greater(t, heightLater, heightAtUpgrade)

	// v4.4 migrates nothing, so pre-upgrade state must be untouched.
	delAfter := n.QueryDelegation(del.Address, chainVal.Wallet.ValidatorAddress)
	require.Equal(t, delBefore.Balance, delAfter.Balance)
	require.Equal(t, balBefore, n.QueryAllBalances(del.Address.String()))

	n.UpdateWalletAccSeqNumber(n.DefaultWallet().KeyName)
	n.UpdateWalletAccSeqNumber(del.KeyName)

	// A bank send exercises the ante chain, where most of the audited v0.53.8
	// changes live (SetPubKeyDecorator, signature verification, tx decoding).
	recipient := n.CreateWallet("recipient")
	sendAmt := sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1_000000))
	n.SendCoins(recipient.Address.String(), sdk.NewCoins(sendAmt))
	n.WaitForNextBlock()
	require.Equal(t, sdk.NewCoins(sendAmt), n.QueryAllBalances(recipient.Address.String()))

	// A second delegation exercises the epoching -> x/staking -> x/distribution
	// path that v0.53.8 changed (getBeginInfo, strict historical rewards reads).
	n.WrappedDelegate(del.KeyName, chainVal.Wallet.ValidatorAddress, delAmt)
	n.WaitForEpochEnd()

	delFinal := n.QueryDelegation(del.Address, chainVal.Wallet.ValidatorAddress)
	require.Equal(t, delAmt.MulRaw(2), delFinal.Balance.Amount)
}

// createGovPropAndPreUpgradeFunc builds a MsgSoftwareUpgrade gov proposal for
// the named plan, plus a no-op pre-upgrade hook. v4.4 needs no chain state set
// up before the swap, unlike the migrating upgrades.
func createGovPropAndPreUpgradeFunc(t *testing.T, valWallet *tmanager.WalletSender, upgradeName string, upgradeHeight int64) (*govtypes.MsgSubmitProposal, tmanager.PreUpgradeFunc) {
	upgradeMsg := &upgradetypes.MsgSoftwareUpgrade{
		Authority: "bbn10d07y265gmmuvt4z0w9aw880jnsr700jduz5f2",
		Plan: upgradetypes.Plan{
			Name:   upgradeName,
			Height: upgradeHeight,
			Info:   "Upgrade to " + upgradeName,
		},
	}

	anyMsg, err := types.NewAnyWithValue(upgradeMsg)
	require.NoError(t, err)

	govMsg := &govtypes.MsgSubmitProposal{
		Messages:       []*types.Any{anyMsg},
		InitialDeposit: []sdk.Coin{sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1_000_000))},
		Proposer:       valWallet.Address.String(),
		Metadata:       "",
		Title:          upgradeName,
		Summary:        "upgrade",
		Expedited:      false,
	}

	preUpgradeFunc := func(nodes []*tmanager.Node) {}
	return govMsg, preUpgradeFunc
}
