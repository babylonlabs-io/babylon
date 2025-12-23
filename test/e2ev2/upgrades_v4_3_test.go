package e2e2

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"

	sdkmath "cosmossdk.io/math"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	v43 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v4_3"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v4/test/e2ev2/tmanager"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

func TestUpgradeV43(t *testing.T) {
	t.Parallel()
	tm := tmanager.NewTmWithUpgrade(t, 0, "")
	validator := tm.ChainValidator()

	n := tm.Chains[tmanager.CHAIN_ID_BABYLON].Nodes[0]

	// start chain with previous binary
	tm.Start()
	validator.WaitUntilBlkHeight(3)

	// creates bad delegation with new validator that will be jailed due to downtime
	// 1. Delegate to two healthy validators (A and B)
	// 2. Start of an epoch
	// 	3. Validator B gets slashed
	// 	4. Delegator unbonds from B
	// 	5. Delegator delegates again to B
	// 	6. Delegator unbonds again from B
	// 7. Epoch ends

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
	// validator must wait for end of epoch
	n.WaitForEpochEnd()
	n.UpdateWalletAccSeqNumber(delegator.KeyName)

	valSlash := n.QueryValidator(valSlashAddr)
	require.True(t, valSlash.IsBonded())

	// creates healthy two delegations
	amtHealthyDel, amtSlashDel := sdkmath.NewInt(10_000000), sdkmath.NewInt(2_000000)
	n.WrappedDelegate(delegator.KeyName, validator.Wallet.ValidatorAddress, amtHealthyDel)
	n.WrappedDelegate(delegator.KeyName, valSlashAddr, amtSlashDel)

	fpSK := setupFp(t, tm.R, n)
	createSingleSigBtcDel(t, tm.R, n, delegator, fpSK)

	n.WaitForEpochEnd() // to validate the baby delegations

	costkRwdTracker := n.QueryCostkRwdTrckCli(delegator.Address)
	require.Equal(t, amtHealthyDel.Add(amtSlashDel).String(), costkRwdTracker.ActiveBaby.String())

	slashedVal := n.QueryValidator(valSlashAddr)
	require.False(t, slashedVal.Jailed)

	slashedVal = n.WaitForValidatorBeJailed(valSlashAddr)

	slashDelegation := n.QueryDelegation(delegator.Address, valSlashAddr)

	sharesToUbd := slashedVal.TokensFromShares(slashDelegation.Delegation.Shares)
	n.WrappedUndelegate(delegator.KeyName, valSlashAddr, sharesToUbd.TruncateInt())

	amtSlashDel2 := sdkmath.NewInt(1_500000)
	n.WrappedDelegate(delegator.KeyName, valSlashAddr, amtSlashDel2)
	n.WaitForNextBlock()
	n.WrappedUndelegate(delegator.KeyName, valSlashAddr, amtSlashDel2)

	n.WaitForEpochEnd()

	// costk should be
	costkRwdTracker = n.QueryCostkRwdTrckCli(delegator.Address)
	require.Equal(t, amtHealthyDel.String(), costkRwdTracker.ActiveBaby.String())
	// unbonds from slash
	// bonds again
	// unbonds again

	// check that reaches bad value of active baby

	// stakerWallet := n.DefaultWallet()

	// 	validator.WaitUntilBlkHeight(25)
	govMsg, preUpgradeFunc := createGovPropAndPreUpgradeFunc(t, validator.Wallet.WalletSender)
	require.NotNil(t, govMsg)
	require.NotNil(t, preUpgradeFunc)
	// 	// execute preUpgradeFunc, submit a proposal, vote, and then process upgrade
	// 	tm.Upgrade(govMsg, preUpgradeFunc)

	// // post-upgrade state verification
	// btcDelsResp := validator.QueryBTCDelegations(bstypes.BTCDelegationStatus_ACTIVE)
	// require.Len(t, btcDelsResp, 1)
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

func createSingleSigBtcDel(t *testing.T, r *rand.Rand, n *tmanager.Node, wallet *tmanager.WalletSender, fpSK *btcec.PrivateKey) {
	n.DefaultWallet().VerifySentTx = true

	// single-sig delegation from n to fp
	stakerSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	msg, stakingInfoBuilt := BuildSingleSigDelegationMsg(
		t, r, n,
		n.DefaultWallet(),
		stakerSK,
		fpSK.PubKey(),
		int64(2*10e8),
		1000,
	)

	n.CreateBTCDelegation(n.DefaultWallet().KeyName, msg)
	n.WaitForNextBlock()

	pendingDelResp := n.QueryBTCDelegation(stakingInfoBuilt.StakingTx.TxHash().String())
	require.NotNil(t, pendingDelResp)
	require.Equal(t, "PENDING", pendingDelResp.StatusDesc)

	/*
		generate and insert new covenant signatures, in order to verify the BTC delegation
	*/
	pendingDel, err := chain.ParseRespBTCDelToBTCDel(pendingDelResp)
	require.NoError(t, err)
	require.Len(t, pendingDel.CovenantSigs, 0)
	stakingMsgTx, err := bbn.NewBTCTxFromBytes(pendingDel.StakingTx)
	require.NoError(t, err)

	slashingTx := pendingDel.SlashingTx
	stakingTxHash := stakingMsgTx.TxHash().String()
	bsParams := n.QueryBtcStakingParams()

	fpBTCPKs, err := bbn.NewBTCPKsFromBIP340PKs(pendingDel.FpBtcPkList)
	require.NoError(t, err)

	btcCfg := &chaincfg.SimNetParams
	stakingInfo, err := pendingDel.GetStakingInfo(bsParams, btcCfg)
	require.NoError(t, err)

	stakingSlashingPathInfo, err := stakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	// it should be changed when modifying covenant pk on chain start
	covSKs, _, _ := bstypes.DefaultCovenantCommittee()

	// covenant signatures on slashing tx
	covenantSlashingSigs, err := datagen.GenCovenantAdaptorSigs(
		covSKs,
		fpBTCPKs,
		stakingMsgTx,
		stakingSlashingPathInfo.GetPkScriptPath(),
		slashingTx,
	)
	require.NoError(t, err)

	// cov Schnorr sigs on unbonding signature
	unbondingPathInfo, err := stakingInfo.UnbondingPathSpendInfo()
	require.NoError(t, err)
	unbondingTx, err := bbn.NewBTCTxFromBytes(pendingDel.BtcUndelegation.UnbondingTx)
	require.NoError(t, err)

	covUnbondingSigs, err := datagen.GenCovenantUnbondingSigs(
		covSKs,
		stakingMsgTx,
		pendingDel.StakingOutputIdx,
		unbondingPathInfo.GetPkScriptPath(),
		unbondingTx,
	)
	require.NoError(t, err)

	unbondingInfo, err := pendingDel.GetUnbondingInfo(bsParams, btcCfg)
	require.NoError(t, err)
	unbondingSlashingPathInfo, err := unbondingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)
	covenantUnbondingSlashingSigs, err := datagen.GenCovenantAdaptorSigs(
		covSKs,
		fpBTCPKs,
		unbondingTx,
		unbondingSlashingPathInfo.GetPkScriptPath(),
		pendingDel.BtcUndelegation.SlashingTx,
	)
	require.NoError(t, err)

	for i := 0; i < int(bsParams.CovenantQuorum); i++ {
		n.SubmitRefundableTxWithAssertion(func() {
			n.AddCovenantSigs(
				n.DefaultWallet().KeyName,
				covenantSlashingSigs[i].CovPk,
				stakingTxHash,
				covenantSlashingSigs[i].AdaptorSigs,
				bbn.NewBIP340SignatureFromBTCSig(covUnbondingSigs[i]),
				covenantUnbondingSlashingSigs[i].AdaptorSigs,
				nil,
			)
		}, true, n.DefaultWallet().KeyName)
	}

	verifiedDelResp := n.QueryBTCDelegation(stakingTxHash)
	require.Equal(t, "VERIFIED", verifiedDelResp.StatusDesc)
	verifiedDel, err := chain.ParseRespBTCDelToBTCDel(verifiedDelResp)
	require.NoError(t, err)
	require.Len(t, verifiedDel.CovenantSigs, int(bsParams.CovenantQuorum))
	require.True(t, verifiedDel.HasCovenantQuorums(bsParams.CovenantQuorum, 0))

	/*
		generate and add inclusion proof, in order to activate the BTC delegation
	*/
	// wait for btc delegation is k-deep
	currentBtcTipResp, err := n.QueryTip()
	require.NoError(t, err)
	currentBtcTip, err := chain.ParseBTCHeaderInfoResponseToInfo(currentBtcTipResp)
	require.NoError(t, err)
	blockWithStakingTx := datagen.CreateBlockWithTransaction(r, currentBtcTip.Header.ToBlockHeader(), stakingMsgTx)
	n.InsertHeader(&blockWithStakingTx.HeaderBytes)

	inclusionProof := bstypes.NewInclusionProofFromSpvProof(blockWithStakingTx.SpvProof)
	for i := 0; i < tmanager.BabylonBtcConfirmationPeriod; i++ {
		n.InsertNewEmptyBtcHeader(r)
	}

	// add btc inclusion proof
	n.SubmitRefundableTxWithAssertion(func() {
		n.AddBTCDelegationInclusionProof(n.DefaultWallet().KeyName, stakingTxHash, inclusionProof)
	}, true, n.DefaultWallet().KeyName)

	activeBtcDelResp := n.QueryBTCDelegation(stakingTxHash)
	require.Equal(t, "ACTIVE", activeBtcDelResp.StatusDesc)
}
