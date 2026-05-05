package e2e2

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/test/e2ev2/tmanager"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	tkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

// TestStakeExpansionChildUnbondFirstNoLongerDeadlocks_E2E is the fix-branch
// regression test for the stake-expansion child unbond deadlock. It runs
// against a real Babylon Docker chain.
//
// The exact same attack sequence the PoC test exercised is replayed here:
//
//  1. Parent A reaches ACTIVE. Child B is created as a VERIFIED stake
//     expansion of A. Covenant quorum signs B (including the
//     stake-expansion sig); child is registered with its unbonding tx +
//     covenant unbonding sigs.
//  2. Child's staking tx confirms on Bitcoin (block N), spending parent's
//     staking output. Child's unbonding tx confirms on Bitcoin (block N+1),
//     spending child's staking output.
//  3. Attacker submits MsgBTCUndelegate(child, child's unbonding tx). The
//     handler treats this as "intent-based" (registered unbonding tx), so
//     it succeeds even though the child has no inclusion proof. Child gets
//     DelegatorUnbondingInfo set ⇒ status flips UNBONDED via
//     IsUnbondedEarly.
//
// PRE-FIX behavior (the bug): subsequent MsgBTCUndelegate(parent, child's
// staking tx) would fail PERMANENTLY with "already unbonded" because the
// stake-expansion branch routes through AddBTCDelegationInclusionProof on
// the poisoned child. Parent A would stay ACTIVE while its UTXO is gone on
// BTC ⇒ phantom voting power, unslashable, persists for ~StakingTime.
//
// POST-FIX behavior (this test verifies): subsequent
// MsgBTCUndelegate(parent, child's staking tx) — submitted by anyone, not
// just the staker — SUCCEEDS. The fix in BTCUndelegate verifies the k-depth
// proof unconditionally and treats the child activation as best-effort, so
// failures on the child don't abort the parent unbonding. Parent ends up
// UNBONDED with proper DelegatorUnbondingInfo; child stays UNBONDED
// (poisoned, can never be activated, but contributes 0 voting power).
// No phantom remains.
//
// Code change being verified:
//
//	x/btcstaking/keeper/msg_server.go BTCUndelegate's stake-expansion branch
//	now calls VerifyInclusionProofAndGetHeight directly (k-depth check) and
//	then AddBTCDelegationInclusionProof as best-effort.
func TestStakeExpansionChildUnbondFirstNoLongerDeadlocks_E2E(t *testing.T) {
	t.Parallel()

	// ---- single-chain Babylon Docker setup ----
	tm := tmanager.NewTestManager(t)
	bbnCfg := tmanager.NewChainConfig(tm.TempDir, tmanager.CHAIN_ID_BABYLON)
	tm.Chains[tmanager.CHAIN_ID_BABYLON] = tmanager.NewChain(tm, bbnCfg)
	tm.Start()
	tm.ChainsWaitUntilHeight(3)

	n := tm.Chains[tmanager.CHAIN_ID_BABYLON].Nodes[0]

	// ---- wallets ----
	wFP := n.CreateWallet("fp")
	wFP.VerifySentTx = true
	wStaker := n.CreateWallet("staker")
	wStaker.VerifySentTx = true

	fundCoins := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(100_000_000)))
	n.DefaultWallet().VerifySentTx = true
	n.SendCoins(wFP.Addr(), fundCoins)
	n.SendCoins(wStaker.Addr(), fundCoins)
	n.UpdateWalletAccSeqNumber(wFP.KeyName)
	n.UpdateWalletAccSeqNumber(wStaker.KeyName)

	// ---- finality provider ----
	fp := n.NewFpWithWallet(wFP)
	fpPK := fp.PublicKey.MustToBTCPK()

	// ---- parent A: ACTIVE delegation with controlled staker SK ----
	stakingValue := int64(2 * 10e8)
	stakingTime := uint16(1000)
	stakerSK, _, err := datagen.GenRandomBTCKeyPair(n.Tm.R)
	require.NoError(t, err)
	parentResp, parentStkTx := n.CreateBtcDelegationWithSK(wStaker, stakerSK, fpPK, stakingValue, stakingTime)
	parentStkHash := parentStkTx.TxHash().String()
	requireDelegationState(t, n, "after parent A ACTIVE", parentStkHash, expectedDelState{Status: "ACTIVE"})

	// ---- child B: VERIFIED stake expansion of A ----
	childResp, expansionMsg, fundingTx := n.CreateBtcStakeExpansionVerified(
		wStaker, stakerSK, fpPK, parentResp, parentStkTx,
		stakingValue, stakingTime, 100_000,
	)
	require.NotNil(t, childResp.StkExp, "child B must carry stake-expansion metadata")

	bsParams := n.QueryBtcStakingParams()
	btcCfg := &chaincfg.SimNetParams

	childDel, err := tkeeper.ParseRespBTCDelToBTCDel(childResp)
	require.NoError(t, err)
	childStkTx, err := bbn.NewBTCTxFromBytes(childDel.StakingTx)
	require.NoError(t, err)
	childUnbondingTx, err := bbn.NewBTCTxFromBytes(childDel.BtcUndelegation.UnbondingTx)
	require.NoError(t, err)
	childStakingOutput := childStkTx.TxOut[childDel.StakingOutputIdx]
	childStkTxBz, err := bbn.SerializeBTCTx(childStkTx)
	require.NoError(t, err)
	childStkHash := childStkTx.TxHash().String()
	requireDelegationState(t, n, "after child B VERIFIED [parent]", parentStkHash, expectedDelState{Status: "ACTIVE"})
	requireDelegationState(t, n, "after child B VERIFIED [child]", childStkHash, expectedDelState{Status: "VERIFIED"})

	// ---- BTC chain replay (realistic ordering) ----
	covSKs, _, _ := bstypes.DefaultCovenantCommittee()
	expansionWitnessedBz, _ := datagen.AddWitnessToStakeExpTx(
		t,
		parentStkTx.TxOut[datagen.StakingOutIdx],
		fundingTx.TxOut[0],
		stakerSK,
		covSKs,
		bsParams.CovenantQuorum,
		[]*btcec.PublicKey{fpPK},
		stakingTime,
		stakingValue,
		childStkTx,
		btcCfg,
	)
	childUnbondingTxBz, childUnbondingWitnessed := datagen.AddWitnessToUnbondingTx(
		t,
		childStakingOutput,
		stakerSK,
		covSKs,
		bsParams.CovenantQuorum,
		[]*btcec.PublicKey{fpPK},
		uint16(childResp.StakingTime),
		int64(childResp.TotalSat),
		childUnbondingTx,
		btcCfg,
	)

	// Block N: child's staking tx confirms on BTC (spends parent's UTXO).
	tipBeforeExpansionResp, err := n.QueryTip()
	require.NoError(t, err)
	tipBeforeExpansion, err := tkeeper.ParseBTCHeaderInfoResponseToInfo(tipBeforeExpansionResp)
	require.NoError(t, err)
	expansionBlock := datagen.CreateBlockWithTransaction(
		n.Tm.R, tipBeforeExpansion.Header.ToBlockHeader(), childStkTx,
	)
	expansionBlockHeight := tipBeforeExpansion.Height + 1
	n.InsertHeader(&expansionBlock.HeaderBytes)
	expansionInclusionProof := bstypes.NewInclusionProofFromSpvProof(expansionBlock.SpvProof)

	// Block N+1: child's unbonding tx confirms on BTC.
	unbondingBlock := datagen.CreateBlockWithTransaction(
		n.Tm.R, expansionBlock.HeaderBytes.ToBlockHeader(), childUnbondingWitnessed,
	)
	n.InsertHeader(&unbondingBlock.HeaderBytes)
	unbondingInclusionProof := bstypes.NewInclusionProofFromSpvProof(unbondingBlock.SpvProof)

	// ---- ATTACK: poison the child via its registered unbonding tx ----
	//
	// This step still succeeds with the fix in place — by design, ordinary
	// undelegation is intent-based and only requires a 1-deep merkle proof.
	// The fix doesn't gate this; it gates the parent unbonding so that the
	// poisoned child no longer blocks it.
	poisonMsg := &bstypes.MsgBTCUndelegate{
		Signer:                        wStaker.Address.String(),
		StakingTxHash:                 childStkHash,
		StakeSpendingTx:               childUnbondingTxBz,
		StakeSpendingTxInclusionProof: unbondingInclusionProof,
		FundingTransactions:           [][]byte{childStkTxBz},
	}
	n.SubmitBTCUndelegate(wStaker, poisonMsg)
	requireDelegationState(t, n, "after poison [parent]", parentStkHash, expectedDelState{Status: "ACTIVE"})
	requireDelegationState(t, n, "after poison [child]", childStkHash, expectedDelState{Status: "UNBONDED", HasUnbondInfo: true})
	childAfterPoison := n.QueryBTCDelegation(childStkHash)
	require.Equal(t, "", childAfterPoison.UndelegationResponse.DelegatorUnbondingInfoResponse.SpendStakeTxHex,
		"poison's SpendStakeTxHex must be empty (registered-unbonding-tx case)")

	// ---- alternative still-blocked paths (unchanged by the fix) ----
	//
	// (a) Direct MsgAddBTCDelegationInclusionProof on the child is rejected
	//     unconditionally for stake expansions at msg_server.go:225 — this
	//     check pre-dates the deadlock fix and is unaffected.
	directMsg := &bstypes.MsgAddBTCDelegationInclusionProof{
		Signer:                  wStaker.Address.String(),
		StakingTxHash:           childStkHash,
		StakingTxInclusionProof: expansionInclusionProof,
	}
	wStaker.VerifySentTx = false
	signedDirectTx := wStaker.SignMsg(directMsg)
	directHash, err := n.SubmitTx(signedDirectTx)
	require.NoError(t, err)
	n.WaitForNextBlock()
	directResp := n.QueryTxByHash(directHash)
	require.NotZero(t, directResp.TxResponse.Code)
	require.Contains(t, directResp.TxResponse.RawLog, "stake expansion",
		"direct inclusion proof must remain rejected for stake expansions")
	requireDelegationState(t, n, "after rejected direct inclusion proof [parent]", parentStkHash, expectedDelState{Status: "ACTIVE"})
	requireDelegationState(t, n, "after rejected direct inclusion proof [child]", childStkHash, expectedDelState{Status: "UNBONDED", HasUnbondInfo: true})

	// (b) MsgBTCUndelegate(parent, parent's own unbonding tx) with a header
	//     hash that Babylon's BTC light client doesn't know — corresponds to
	//     real-world impossibility (Bitcoin's double-spend rule prevents the
	//     parent's unbonding tx from being mined while the child's staking tx
	//     occupies the same input).
	parentDel, err := tkeeper.ParseRespBTCDelToBTCDel(parentResp)
	require.NoError(t, err)
	parentUnbondingTx := parentDel.MustGetUnbondingTx()
	parentUnbondingTxBz, _ := datagen.AddWitnessToUnbondingTx(
		t,
		parentStkTx.TxOut[datagen.StakingOutIdx],
		stakerSK,
		covSKs,
		bsParams.CovenantQuorum,
		[]*btcec.PublicKey{fpPK},
		stakingTime,
		stakingValue,
		parentUnbondingTx,
		btcCfg,
	)
	bogusInclusionProof := *unbondingInclusionProof
	bogusHashBz := make([]byte, len(unbondingInclusionProof.Key.Hash.MustMarshal()))
	copy(bogusHashBz, unbondingInclusionProof.Key.Hash.MustMarshal())
	bogusHashBz[0] ^= 0xff
	bogusHash, err := bbn.NewBTCHeaderHashBytesFromBytes(bogusHashBz)
	require.NoError(t, err)
	bogusInclusionProof.Key.Hash = &bogusHash
	parentStkTxBz, err := bbn.SerializeBTCTx(parentStkTx)
	require.NoError(t, err)
	selfUnbondMsg := &bstypes.MsgBTCUndelegate{
		Signer:                        wStaker.Address.String(),
		StakingTxHash:                 parentStkHash,
		StakeSpendingTx:               parentUnbondingTxBz,
		StakeSpendingTxInclusionProof: &bogusInclusionProof,
		FundingTransactions:           [][]byte{parentStkTxBz},
	}
	selfUnbondLog := n.SubmitBTCUndelegateExpectFail(wStaker, selfUnbondMsg)
	require.Contains(t, selfUnbondLog, "stake spending tx is not on BTC chain",
		"self-unbond via parent's own unbonding tx still fails when Babylon's BTC light client does not know the header (expected)")
	requireDelegationState(t, n, "after rejected self-unbond [parent]", parentStkHash, expectedDelState{Status: "ACTIVE"})
	requireDelegationState(t, n, "after rejected self-unbond [child]", childStkHash, expectedDelState{Status: "UNBONDED", HasUnbondInfo: true})

	// ---- k-1 BTC blocks pass; child's staking tx becomes k-deep ----
	for i := 0; i < tmanager.BabylonBtcConfirmationPeriod; i++ {
		n.InsertNewEmptyBtcHeader(n.Tm.R)
	}

	// ---- FIX VERIFICATION ----
	//
	// A third-party wallet (vigilante / honest watcher / anyone observing
	// Bitcoin) — NOT the original staker — submits the legitimate
	// activation/unbonding flow. The spending tx is the child's staking tx
	// (the stake-expansion tx).
	//
	// Pre-fix: this would fail PERMANENTLY with "already unbonded" because
	// the handler routes through AddBTCDelegationInclusionProof on the
	// poisoned child.
	//
	// Post-fix: this SUCCEEDS. The fix verifies the k-depth proof
	// unconditionally and treats the child activation as best-effort —
	// the failure on the poisoned child does not abort the parent
	// unbonding.
	wThirdParty := n.CreateWallet("third_party")
	wThirdParty.VerifySentTx = true
	n.DefaultWallet().VerifySentTx = true
	n.SendCoins(wThirdParty.Addr(), fundCoins)
	n.UpdateWalletAccSeqNumber(wThirdParty.KeyName)

	parentUnbondMsg := &bstypes.MsgBTCUndelegate{
		Signer:                        wThirdParty.Address.String(), // anyone, not the staker
		StakingTxHash:                 parentStkHash,
		StakeSpendingTx:               expansionWitnessedBz,
		StakeSpendingTxInclusionProof: expansionInclusionProof,
		FundingTransactions:           [][]byte{parentStkTxBz, expansionMsg.FundingTx},
	}
	require.NotEqual(t, parentResp.StakerAddr, wThirdParty.Address.String(),
		"sanity: third-party signer must differ from the original staker")
	n.SubmitBTCUndelegate(wThirdParty, parentUnbondMsg)
	t.Logf("FIX CONFIRMED: third-party MsgBTCUndelegate(parent, child's staking tx) succeeded — "+
		"parent A is no longer phantom-ACTIVE (signer=%s, staker=%s)",
		wThirdParty.Address.String(), parentResp.StakerAddr)

	// ---- final state verification ----
	requireDelegationState(t, n, "after FIX parent unbond [parent]", parentStkHash, expectedDelState{Status: "UNBONDED", HasUnbondInfo: true})
	requireDelegationState(t, n, "after FIX parent unbond [child]", childStkHash, expectedDelState{Status: "UNBONDED", HasUnbondInfo: true})

	parentAfter := n.QueryBTCDelegation(parentStkHash)
	require.NotNil(t, parentAfter.UndelegationResponse.DelegatorUnbondingInfoResponse,
		"parent A's DelegatorUnbondingInfo must be set after the fix's parent unbond")
	require.Equal(t, "UNBONDED", parentAfter.StatusDesc,
		"FIX CONFIRMED: parent A is UNBONDED, no longer phantom-ACTIVE")

	childFinal := n.QueryBTCDelegation(childStkHash)
	require.Equal(t, "UNBONDED", childFinal.StatusDesc,
		"child remains UNBONDED — best-effort activation skipped because of the prior poison")
	require.Zero(t, childFinal.StartHeight, "child never activated, StartHeight stays 0")
	require.Zero(t, childFinal.EndHeight, "child never activated, EndHeight stays 0")

	// ---- sanity: any subsequent attempt to unbond parent fails because parent is already UNBONDED ----
	retryMsg := &bstypes.MsgBTCUndelegate{
		Signer:                        wStaker.Address.String(), // staker now retries
		StakingTxHash:                 parentStkHash,
		StakeSpendingTx:               expansionWitnessedBz,
		StakeSpendingTxInclusionProof: expansionInclusionProof,
		FundingTransactions:           [][]byte{parentStkTxBz, expansionMsg.FundingTx},
	}
	retryLog := n.SubmitBTCUndelegateExpectFail(wStaker, retryMsg)
	require.Contains(t, retryLog, "cannot unbond an unbonded BTC delegation",
		"after the fix's successful parent unbond, retries are rejected by the status gate (parent is UNBONDED)")

	// ---- BTC-side: child's staking tx is k-deep on Babylon's BTC view, parent UTXO is provably spent ----
	parentStkOutpoint := wire.OutPoint{Hash: parentStkTx.TxHash(), Index: datagen.StakingOutIdx}
	spendsParent := false
	for _, in := range childStkTx.TxIn {
		if in.PreviousOutPoint.Hash == parentStkOutpoint.Hash &&
			in.PreviousOutPoint.Index == parentStkOutpoint.Index {
			spendsParent = true
			break
		}
	}
	require.True(t, spendsParent, "child staking tx must structurally spend parent A's staking output")
	finalTipResp, err := n.QueryTip()
	require.NoError(t, err)
	finalTip, err := tkeeper.ParseBTCHeaderInfoResponseToInfo(finalTipResp)
	require.NoError(t, err)
	depth := finalTip.Height - expansionBlockHeight + 1
	require.GreaterOrEqual(t, depth, uint32(tmanager.BabylonBtcConfirmationPeriod),
		"child's staking tx must be k-deep on Babylon's BTC light client at this point")

	t.Log("=================================================================")
	t.Log("FINAL STATE — fix verification:")
	t.Log("-----------------------------------------------------------------")
	t.Logf("PARENT A (%s):", parentStkHash)
	t.Logf("  StatusDesc:                          %s   ← was ACTIVE pre-fix (phantom)", parentAfter.StatusDesc)
	t.Logf("  TotalSat:                            %d (no longer counts as voting power)", parentAfter.TotalSat)
	t.Logf("  DelegatorUnbondingInfo:              set (parent's unbonding properly recorded)")
	t.Log("-----------------------------------------------------------------")
	t.Logf("CHILD B (%s):", childStkHash)
	t.Logf("  StatusDesc:                          %s (poisoned, never activated)", childFinal.StatusDesc)
	t.Logf("  StartHeight / EndHeight:             %d / %d", childFinal.StartHeight, childFinal.EndHeight)
	t.Log("-----------------------------------------------------------------")
	t.Logf("Net: 0 phantom voting power. Parent's UTXO spend on BTC is now reflected on Babylon.")
	t.Log("=================================================================")
}

// expectedDelState bundles the on-chain state we want to pin for a single BTC
// delegation at a given checkpoint.
type expectedDelState struct {
	Status        string // BTCDelegationStatus_*.String() — "ACTIVE", "VERIFIED", "UNBONDED", ...
	HasUnbondInfo bool   // whether DelegatorUnbondingInfo is set
}

// requireDelegationState queries the BTC delegation by staking-tx hash and
// asserts both StatusDesc and DelegatorUnbondingInfo presence match the
// expected state. label is included in failure messages so checkpoints in a
// long test are easy to identify.
func requireDelegationState(
	t *testing.T,
	n *tmanager.Node,
	label, stkHash string,
	exp expectedDelState,
) {
	t.Helper()
	d := n.QueryBTCDelegation(stkHash)
	require.NotNilf(t, d, "%s: delegation %s must exist", label, stkHash)
	require.Equalf(t, exp.Status, d.StatusDesc,
		"%s: status mismatch (delegation %s)", label, stkHash)
	hasUI := d.UndelegationResponse.DelegatorUnbondingInfoResponse != nil
	require.Equalf(t, exp.HasUnbondInfo, hasUI,
		"%s: DelegatorUnbondingInfo presence mismatch (delegation %s, got hasUI=%v)",
		label, stkHash, hasUI)
}

// TestStakeExpansionParentUnbondAtOneDeepWhenChildUnbonded_E2E pins the
// protocol property that justifies vigilante's `abortIfChildUnbonded` short-
// circuit:
//
//	When the child stake expansion is already UNBONDED on babylon (the
//	delegator submitted MsgBTCUndelegate for the child), the parent's
//	MsgBTCUndelegate via the expansion tx SUCCEEDS as soon as the expansion
//	tx is just 1-deep on Babylon's BTC light client — NO k-deep wait.
//
// Realistic timing (matches what vigilante observes):
//
//  1. Block H₀ on babylon: MsgBTCUndelegate(child, child-unbonding-tx)
//     lands. Child gets DelegatorUnbondingInfo set ⇒ status flips to
//     UNBONDED via IsUnbondedEarly.
//  2. One BTC block is mined containing the expansion staking tx and its
//     header is sent to babylon (BTC light client now has it 1-deep).
//  3. Block H₁ on babylon (H₁ > H₀): vigilante observes the parent's UTXO
//     spent and submits MsgBTCUndelegate(parent, expansion-staking-tx).
//     With the fix, this hits the default-switch case (since
//     child.IsUnbondedEarly() == true ⇒ shouldActivateStkExp = false) which
//     only requires a 1-deep merkle proof. tx succeeds, parent UNBONDED.
//
// Pre-fix: step 3 failed permanently with "already unbonded" because the
// stake-expansion branch routed through AddBTCDelegationInclusionProof on the
// poisoned child. Post-fix: succeeds at <k depth, no phantom voting power.
func TestStakeExpansionParentUnbondAtOneDeepWhenChildUnbonded_E2E(t *testing.T) {
	t.Parallel()

	tm := tmanager.NewTestManager(t)
	bbnCfg := tmanager.NewChainConfig(tm.TempDir, tmanager.CHAIN_ID_BABYLON)
	tm.Chains[tmanager.CHAIN_ID_BABYLON] = tmanager.NewChain(tm, bbnCfg)
	tm.Start()
	tm.ChainsWaitUntilHeight(3)

	n := tm.Chains[tmanager.CHAIN_ID_BABYLON].Nodes[0]

	wFP := n.CreateWallet("fp")
	wFP.VerifySentTx = true
	wStaker := n.CreateWallet("staker")
	wStaker.VerifySentTx = true

	fundCoins := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(100_000_000)))
	n.DefaultWallet().VerifySentTx = true
	n.SendCoins(wFP.Addr(), fundCoins)
	n.SendCoins(wStaker.Addr(), fundCoins)
	n.UpdateWalletAccSeqNumber(wFP.KeyName)
	n.UpdateWalletAccSeqNumber(wStaker.KeyName)

	fp := n.NewFpWithWallet(wFP)
	fpPK := fp.PublicKey.MustToBTCPK()

	stakingValue := int64(2 * 10e8)
	stakingTime := uint16(1000)
	stakerSK, _, err := datagen.GenRandomBTCKeyPair(n.Tm.R)
	require.NoError(t, err)
	parentResp, parentStkTx := n.CreateBtcDelegationWithSK(wStaker, stakerSK, fpPK, stakingValue, stakingTime)
	parentStkHash := parentStkTx.TxHash().String()
	requireDelegationState(t, n, "after parent ACTIVE", parentStkHash, expectedDelState{Status: "ACTIVE"})

	childResp, expansionMsg, fundingTx := n.CreateBtcStakeExpansionVerified(
		wStaker, stakerSK, fpPK, parentResp, parentStkTx,
		stakingValue, stakingTime, 100_000,
	)
	require.NotNil(t, childResp.StkExp, "child must carry stake-expansion metadata")

	bsParams := n.QueryBtcStakingParams()
	btcCfg := &chaincfg.SimNetParams

	childDel, err := tkeeper.ParseRespBTCDelToBTCDel(childResp)
	require.NoError(t, err)
	childStkTx, err := bbn.NewBTCTxFromBytes(childDel.StakingTx)
	require.NoError(t, err)
	childUnbondingTx, err := bbn.NewBTCTxFromBytes(childDel.BtcUndelegation.UnbondingTx)
	require.NoError(t, err)
	childStakingOutput := childStkTx.TxOut[childDel.StakingOutputIdx]
	childStkTxBz, err := bbn.SerializeBTCTx(childStkTx)
	require.NoError(t, err)
	childStkHash := childStkTx.TxHash().String()
	requireDelegationState(t, n, "after child VERIFIED [parent]", parentStkHash, expectedDelState{Status: "ACTIVE"})
	requireDelegationState(t, n, "after child VERIFIED [child]", childStkHash, expectedDelState{Status: "VERIFIED"})

	covSKs, _, _ := bstypes.DefaultCovenantCommittee()
	expansionWitnessedBz, _ := datagen.AddWitnessToStakeExpTx(
		t,
		parentStkTx.TxOut[datagen.StakingOutIdx],
		fundingTx.TxOut[0],
		stakerSK,
		covSKs,
		bsParams.CovenantQuorum,
		[]*btcec.PublicKey{fpPK},
		stakingTime,
		stakingValue,
		childStkTx,
		btcCfg,
	)
	childUnbondingTxBz, childUnbondingWitnessed := datagen.AddWitnessToUnbondingTx(
		t,
		childStakingOutput,
		stakerSK,
		covSKs,
		bsParams.CovenantQuorum,
		[]*btcec.PublicKey{fpPK},
		uint16(childResp.StakingTime),
		int64(childResp.TotalSat),
		childUnbondingTx,
		btcCfg,
	)

	// Stage child's unbonding tx on BTC (1-deep header is enough — the
	// poison path uses a 1-deep merkle proof check).
	tipBeforeChildUnbondResp, err := n.QueryTip()
	require.NoError(t, err)
	tipBeforeChildUnbond, err := tkeeper.ParseBTCHeaderInfoResponseToInfo(tipBeforeChildUnbondResp)
	require.NoError(t, err)
	childUnbondingBtcBlock := datagen.CreateBlockWithTransaction(
		n.Tm.R, tipBeforeChildUnbond.Header.ToBlockHeader(), childUnbondingWitnessed,
	)
	n.InsertHeader(&childUnbondingBtcBlock.HeaderBytes)
	childUnbondingInclusionProof := bstypes.NewInclusionProofFromSpvProof(childUnbondingBtcBlock.SpvProof)

	// Step 1: child gets unbonded early. Babylon block H₀.
	poisonMsg := &bstypes.MsgBTCUndelegate{
		Signer:                        wStaker.Address.String(),
		StakingTxHash:                 childStkHash,
		StakeSpendingTx:               childUnbondingTxBz,
		StakeSpendingTxInclusionProof: childUnbondingInclusionProof,
		FundingTransactions:           [][]byte{childStkTxBz},
	}
	n.SubmitBTCUndelegate(wStaker, poisonMsg)
	requireDelegationState(t, n, "after child unbonded [parent]", parentStkHash,
		expectedDelState{Status: "ACTIVE"})
	requireDelegationState(t, n, "after child unbonded [child]", childStkHash,
		expectedDelState{Status: "UNBONDED", HasUnbondInfo: true})

	// Step 2: one BTC block is mined containing the expansion staking tx;
	// its header reaches babylon. Crucially, NO further BTC blocks — the
	// expansion tx stays at depth=1 on Babylon's BTC light client.
	expansionBtcBlock := datagen.CreateBlockWithTransaction(
		n.Tm.R, childUnbondingBtcBlock.HeaderBytes.ToBlockHeader(), childStkTx,
	)
	expansionBlockHeight := tipBeforeChildUnbond.Height + 2
	n.InsertHeader(&expansionBtcBlock.HeaderBytes)
	expansionInclusionProof := bstypes.NewInclusionProofFromSpvProof(expansionBtcBlock.SpvProof)

	// Sanity: expansion tx must be LOWER than k-deep at this point. If a
	// regression re-introduces a k-deep gate on the default-switch path, the
	// next SubmitBTCUndelegate call will fail and pin the regression.
	tipAfterExpansionResp, err := n.QueryTip()
	require.NoError(t, err)
	tipAfterExpansion, err := tkeeper.ParseBTCHeaderInfoResponseToInfo(tipAfterExpansionResp)
	require.NoError(t, err)
	expansionDepth := tipAfterExpansion.Height - expansionBlockHeight + 1
	require.Less(t, expansionDepth, uint32(tmanager.BabylonBtcConfirmationPeriod),
		"sanity: expansion tx must be < k-deep when parent unbond is reported (depth=%d, k=%d)",
		expansionDepth, tmanager.BabylonBtcConfirmationPeriod)

	// Step 3: vigilante (or anyone) reports parent's unbonding using the
	// expansion staking tx as the spending tx. With the fix, this succeeds
	// even though the expansion tx is only 1-deep on babylon's BTC view,
	// because child.IsUnbondedEarly() ⇒ default-switch path with 1-deep
	// merkle check.
	parentStkTxBz, err := bbn.SerializeBTCTx(parentStkTx)
	require.NoError(t, err)
	parentUnbondMsg := &bstypes.MsgBTCUndelegate{
		Signer:                        wStaker.Address.String(),
		StakingTxHash:                 parentStkHash,
		StakeSpendingTx:               expansionWitnessedBz,
		StakeSpendingTxInclusionProof: expansionInclusionProof,
		FundingTransactions:           [][]byte{parentStkTxBz, expansionMsg.FundingTx},
	}
	n.SubmitBTCUndelegate(wStaker, parentUnbondMsg)

	// Final state: both UNBONDED, both carry DelegatorUnbondingInfo, no
	// k-deep wait was required for the parent.
	requireDelegationState(t, n, "after parent unbond [child]", childStkHash,
		expectedDelState{Status: "UNBONDED", HasUnbondInfo: true})
	requireDelegationState(t, n, "after parent unbond [parent]", parentStkHash,
		expectedDelState{Status: "UNBONDED", HasUnbondInfo: true})

	parentAfter := n.QueryBTCDelegation(parentStkHash)
	require.NotEmpty(t, parentAfter.UndelegationResponse.DelegatorUnbondingInfoResponse.SpendStakeTxHex,
		"default-switch case writes the full spending tx (the expansion), not empty bytes")

	childFinal := n.QueryBTCDelegation(childStkHash)
	require.Zero(t, childFinal.StartHeight,
		"child stays without an inclusion proof — poisoning happens before activation")
	require.Zero(t, childFinal.EndHeight,
		"child stays without an inclusion proof — poisoning happens before activation")

	t.Logf("Parent unbond accepted at depth=%d (< k=%d) once child was UNBONDED", expansionDepth, tmanager.BabylonBtcConfirmationPeriod)
}
