package e2e2

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	v43 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v4_3"
	"github.com/babylonlabs-io/babylon/v4/test/e2ev2/tmanager"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	tkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	blctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

// poisonedVictim bundles the on-chain identifiers of one parent/child pair we
// poisoned on the pre-upgrade binary. Kept across the upgrade boundary so we
// can re-query post-upgrade state for each victim.
type poisonedVictim struct {
	parentStkHash string
	childStkHash  string
	childStkTxHex string
}

// TestUpgradeV43RemediatesPhantomStakeExpansionParents drives the v4.3
// upgrade-handler's stake-expansion remediation end-to-end on a real Babylon
// chain.
//
// Flow:
//  1. Bootstrap a chain on the pre-upgrade binary (v4.2.x). This binary has
//     the GHSA-4rm2-cj74-f62h vulnerability: an attacker can poison a
//     stake-expansion child by submitting MsgBTCUndelegate(child,
//     child-unbonding-tx) before the child's inclusion proof is recorded,
//     which leaves the parent ACTIVE on babylon while its UTXO is gone on BTC
//     (phantom voting power).
//  2. Create N (≥2) ACTIVE parent delegations + N VERIFIED stake-expansion
//     children. Poison each child. Each parent stays ACTIVE; each child
//     becomes UNBONDED via IsUnbondedEarly.
//  3. Submit the v4.3 software-upgrade gov proposal, vote yes, halt at
//     upgrade height, swap container to the post-fix binary.
//  4. The v4.3 handler's RemediatePoisonedStakeExpansions runs during
//     InitChainer. Each poisoned parent is force-unbonded with the child's
//     staking tx as SpendStakeTx.
//  5. Verify post-upgrade: every parent UNBONDED with
//     DelegatorUnbondingInfoResponse.SpendStakeTxHex == child's staking tx
//     hex; every child stays UNBONDED.
func TestUpgradeV43RemediatesPhantomStakeExpansionParents(t *testing.T) {
	t.Parallel()

	tm := tmanager.NewTmWithUpgrade(t, 0, "", func(cfg *tmanager.ChainConfig) {
		cfg.EpochLength = 40
	})
	chainVal := tm.ChainValidator()
	n := tm.Chains[tmanager.CHAIN_ID_BABYLON].Nodes[0]

	tm.Start()
	chainVal.WaitUntilBlkHeight(3)

	wFP := n.CreateWallet("fp")
	wFP.VerifySentTx = true
	wStaker := n.CreateWallet("staker")
	wStaker.VerifySentTx = true

	fundCoins := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(500_000_000)))
	n.DefaultWallet().VerifySentTx = true
	n.SendCoins(wFP.Addr(), fundCoins)
	n.SendCoins(wStaker.Addr(), fundCoins)
	n.UpdateWalletAccSeqNumber(wFP.KeyName)
	n.UpdateWalletAccSeqNumber(wStaker.KeyName)

	fp := n.NewFpWithWallet(wFP)
	fpPK := fp.PublicKey.MustToBTCPK()

	bsParams := n.QueryBtcStakingParams()
	covSKs, _, _ := bstypes.DefaultCovenantCommittee()
	btcCfg := &chaincfg.SimNetParams

	const numVictims = 2
	stakingValue := int64(2 * 10e8)
	stakingTime := uint16(1000)

	victims := make([]poisonedVictim, 0, numVictims)

	for i := 0; i < numVictims; i++ {
		stakerSK, _, err := datagen.GenRandomBTCKeyPair(n.Tm.R)
		require.NoError(t, err)

		parentResp, parentStkTx := n.CreateBtcDelegationWithSK(wStaker, stakerSK, fpPK, stakingValue, stakingTime)
		parentStkHash := parentStkTx.TxHash().String()
		requireDelegationState(t, n, "victim parent ACTIVE", parentStkHash, expectedDelState{Status: "ACTIVE"})

		childResp, _, _ := n.CreateBtcStakeExpansionVerified(
			wStaker, stakerSK, fpPK, parentResp, parentStkTx,
			stakingValue, stakingTime, 100_000,
		)
		require.NotNil(t, childResp.StkExp, "child must carry stake-expansion metadata")

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
		requireDelegationState(t, n, "victim child VERIFIED", childStkHash, expectedDelState{Status: "VERIFIED"})

		// Stage child's unbonding tx on BTC at depth=1 — that's all the
		// pre-fix v4.2.x undelegation path requires (intent-based).
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
		tipResp, err := n.QueryTip()
		require.NoError(t, err)
		tip, err := tkeeper.ParseBTCHeaderInfoResponseToInfo(tipResp)
		require.NoError(t, err)
		unbondingBlock := datagen.CreateBlockWithTransaction(
			n.Tm.R, tip.Header.ToBlockHeader(), childUnbondingWitnessed,
		)
		n.InsertHeader(&unbondingBlock.HeaderBytes)
		unbondingInclusionProof := bstypes.NewInclusionProofFromSpvProof(unbondingBlock.SpvProof)

		// Poison: the v4.2.x binary accepts this and writes
		// DelegatorUnbondingInfo on the child even without the child's
		// inclusion proof being recorded.
		poisonMsg := &bstypes.MsgBTCUndelegate{
			Signer:                        wStaker.Address.String(),
			StakingTxHash:                 childStkHash,
			StakeSpendingTx:               childUnbondingTxBz,
			StakeSpendingTxInclusionProof: unbondingInclusionProof,
			FundingTransactions:           [][]byte{childStkTxBz},
		}
		n.SubmitBTCUndelegate(wStaker, poisonMsg)

		// Post-poison: parent stays ACTIVE (the deadlock — that's the
		// vulnerability), child UNBONDED via IsUnbondedEarly.
		requireDelegationState(t, n, "after poison [parent]", parentStkHash, expectedDelState{Status: "ACTIVE"})
		requireDelegationState(t, n, "after poison [child]", childStkHash, expectedDelState{Status: "UNBONDED", HasUnbondInfo: true})

		victims = append(victims, poisonedVictim{
			parentStkHash: parentStkHash,
			childStkHash:  childStkHash,
			childStkTxHex: hexEncode(childStkTxBz),
		})
	}

	// Sanity: at least 2 victims, all parents phantom-ACTIVE, all children
	// poisoned. This is the exact state the v4.3 handler must remediate.
	require.GreaterOrEqual(t, len(victims), 2,
		"test must create at least two phantom victims so the handler is exercised on a list, not a singleton")

	// =====================================================================
	// Faulty/expired victim: poisoned child whose parent has EXPIRED on
	// babylon BEFORE the upgrade runs. The v4.3 handler MUST skip this
	// parent (not call BtcUndelegate on it). We pick the minimum allowed stakingTime so we can advance
	// the BTC light client past parent.EndHeight - UnbondingTime cheaply.
	// =====================================================================
	expiredStakingTime := uint16(bsParams.MinStakingTimeBlocks)
	expiredStakerSK, _, err := datagen.GenRandomBTCKeyPair(n.Tm.R)
	require.NoError(t, err)

	expiredParentResp, expiredParentStkTx := n.CreateBtcDelegationWithSK(
		wStaker, expiredStakerSK, fpPK, stakingValue, expiredStakingTime,
	)
	expiredParentHash := expiredParentStkTx.TxHash().String()
	requireDelegationState(t, n, "expired-victim parent ACTIVE before expiry advance",
		expiredParentHash, expectedDelState{Status: "ACTIVE"})

	expiredChildResp, _, _ := n.CreateBtcStakeExpansionVerified(
		wStaker, expiredStakerSK, fpPK, expiredParentResp, expiredParentStkTx,
		stakingValue, expiredStakingTime, 100_000,
	)
	require.NotNil(t, expiredChildResp.StkExp)

	expiredChildDel, err := tkeeper.ParseRespBTCDelToBTCDel(expiredChildResp)
	require.NoError(t, err)
	expiredChildStkTx, err := bbn.NewBTCTxFromBytes(expiredChildDel.StakingTx)
	require.NoError(t, err)
	expiredChildUnbondingTx, err := bbn.NewBTCTxFromBytes(expiredChildDel.BtcUndelegation.UnbondingTx)
	require.NoError(t, err)
	expiredChildStakingOutput := expiredChildStkTx.TxOut[expiredChildDel.StakingOutputIdx]
	expiredChildStkTxBz, err := bbn.SerializeBTCTx(expiredChildStkTx)
	require.NoError(t, err)
	expiredChildStkHash := expiredChildStkTx.TxHash().String()

	expiredChildUnbondingTxBz, expiredChildUnbondingWitnessed := datagen.AddWitnessToUnbondingTx(
		t,
		expiredChildStakingOutput,
		expiredStakerSK,
		covSKs,
		bsParams.CovenantQuorum,
		[]*btcec.PublicKey{fpPK},
		uint16(expiredChildResp.StakingTime),
		int64(expiredChildResp.TotalSat),
		expiredChildUnbondingTx,
		btcCfg,
	)
	expiredTipResp, err := n.QueryTip()
	require.NoError(t, err)
	expiredTip, err := tkeeper.ParseBTCHeaderInfoResponseToInfo(expiredTipResp)
	require.NoError(t, err)
	expiredUnbondingBlock := datagen.CreateBlockWithTransaction(
		n.Tm.R, expiredTip.Header.ToBlockHeader(), expiredChildUnbondingWitnessed,
	)
	n.InsertHeader(&expiredUnbondingBlock.HeaderBytes)
	expiredUnbondingProof := bstypes.NewInclusionProofFromSpvProof(expiredUnbondingBlock.SpvProof)

	n.SubmitBTCUndelegate(wStaker, &bstypes.MsgBTCUndelegate{
		Signer:                        wStaker.Address.String(),
		StakingTxHash:                 expiredChildStkHash,
		StakeSpendingTx:               expiredChildUnbondingTxBz,
		StakeSpendingTxInclusionProof: expiredUnbondingProof,
		FundingTransactions:           [][]byte{expiredChildStkTxBz},
	})
	requireDelegationState(t, n, "after poison [expired-victim child]", expiredChildStkHash, expectedDelState{Status: "UNBONDED", HasUnbondInfo: true})

	// Advance Babylon's BTC light client past
	// expiredParent.EndHeight - UnbondingTime so the parent transitions to
	// EXPIRED status. Healthy victims (stakingTime=1000) stay ACTIVE because
	// their EndHeight is still well above the new tip.
	expiredParentBeforeExpiry := n.QueryBTCDelegation(expiredParentHash)
	require.Positive(t, expiredParentBeforeExpiry.EndHeight, "expired-victim parent must already have an inclusion proof (StartHeight/EndHeight set)")
	targetTipForExpiry := expiredParentBeforeExpiry.EndHeight - uint32(expiredParentBeforeExpiry.UnbondingTime) + 1
	advanceBtcTipUntil(t, n, targetTipForExpiry)

	requireDelegationState(t, n, "after expiry advance [expired-victim parent]", expiredParentHash, expectedDelState{Status: "EXPIRED"})
	for i, v := range victims {
		requireDelegationState(t, n, fmt.Sprintf("victim %d still ACTIVE after expiry advance", i), v.parentStkHash, expectedDelState{Status: "ACTIVE"})
	}

	// ---- submit v4.3 upgrade proposal, vote yes, halt, swap binary ----
	currHeight, err := chainVal.LatestBlockNumber()
	require.NoError(t, err)
	upgradeHeight := int64(currHeight) + 30

	govMsg := buildV43UpgradeProposal(t, chainVal.Wallet.WalletSender, upgradeHeight)
	tm.Upgrade(govMsg, func(_ []*tmanager.Node) {})

	// ---- post-upgrade: every poisoned parent must now be UNBONDED with the
	// child's staking tx as SpendStakeTx; every child stays UNBONDED ----
	for i, v := range victims {
		requireDelegationState(t, n, fmt.Sprintf("victim %d post-upgrade parent UNBONDED", i), v.parentStkHash, expectedDelState{Status: "UNBONDED", HasUnbondInfo: true})
		parentAfter := n.QueryBTCDelegation(v.parentStkHash)
		require.NotNil(t, parentAfter.UndelegationResponse.DelegatorUnbondingInfoResponse,
			"victim %d: parent must carry DelegatorUnbondingInfo from v4.3 handler", i)
		require.Equal(t, v.childStkTxHex,
			parentAfter.UndelegationResponse.DelegatorUnbondingInfoResponse.SpendStakeTxHex,
			"victim %d: SpendStakeTx must equal the child's staking tx hex", i)

		requireDelegationState(t, n, fmt.Sprintf("victim %d post-upgrade child stays UNBONDED", i), v.childStkHash, expectedDelState{Status: "UNBONDED", HasUnbondInfo: true})
	}

	// Expired-victim parent: handler MUST NOT have force-unbonded it. The
	// gate (IsBtcDelegationActive) skips non-ACTIVE parents to avoid a
	// redundant EventBTCDelegationStateUpdate{UNBONDED} on top of the prior
	// expiry-driven transition.
	requireDelegationState(t, n, "post-upgrade expired-victim parent stays EXPIRED",
		expiredParentHash, expectedDelState{Status: "EXPIRED"})
	expiredParentAfter := n.QueryBTCDelegation(expiredParentHash)
	require.Nil(t, expiredParentAfter.UndelegationResponse.DelegatorUnbondingInfoResponse,
		"EXPIRED parent must not be force-unbonded by the handler — Konrad/GAtom gate")

	requireDelegationState(t, n, "post-upgrade expired-victim child stays UNBONDED",
		expiredChildStkHash, expectedDelState{Status: "UNBONDED", HasUnbondInfo: true})

	// =====================================================================
	// Post-upgrade live-attack regression: prove the babylon-side fix is now
	// in effect — a NEW phantom-pattern attack created on the upgraded chain
	// can be remediated by anyone (vigilante) at 1-deep merkle, with NO
	// k-deep wait, the moment the child stake-expansion tx is included in
	// just one BTC block.
	//
	// This is the same property that
	// TestStakeExpansionParentUnbondAtOneDeepWhenChildUnbonded_E2E pins on a
	// fresh chain; here we confirm the post-upgrade chain (which started on
	// v4.2.5 and was upgraded to latest at runtime) honors the same path.
	// =====================================================================
	postUpgradeStakerSK, _, err := datagen.GenRandomBTCKeyPair(n.Tm.R)
	require.NoError(t, err)

	puParentResp, puParentStkTx := n.CreateBtcDelegationWithSK(
		wStaker, postUpgradeStakerSK, fpPK, stakingValue, stakingTime,
	)
	puParentHash := puParentStkTx.TxHash().String()
	requireDelegationState(t, n, "post-upgrade live-attack parent ACTIVE",
		puParentHash, expectedDelState{Status: "ACTIVE"})

	puChildResp, puExpansionMsg, puFundingTx := n.CreateBtcStakeExpansionVerified(
		wStaker, postUpgradeStakerSK, fpPK, puParentResp, puParentStkTx,
		stakingValue, stakingTime, 100_000,
	)
	require.NotNil(t, puChildResp.StkExp)

	puChildDel, err := tkeeper.ParseRespBTCDelToBTCDel(puChildResp)
	require.NoError(t, err)
	puChildStkTx, err := bbn.NewBTCTxFromBytes(puChildDel.StakingTx)
	require.NoError(t, err)
	puChildUnbondingTx, err := bbn.NewBTCTxFromBytes(puChildDel.BtcUndelegation.UnbondingTx)
	require.NoError(t, err)
	puChildStakingOutput := puChildStkTx.TxOut[puChildDel.StakingOutputIdx]
	puChildStkTxBz, err := bbn.SerializeBTCTx(puChildStkTx)
	require.NoError(t, err)
	puChildStkHash := puChildStkTx.TxHash().String()
	requireDelegationState(t, n, "post-upgrade live-attack child VERIFIED",
		puChildStkHash, expectedDelState{Status: "VERIFIED"})

	// Witness child's unbonding tx and stage it on BTC at depth=1.
	puChildUnbondingTxBz, puChildUnbondingWitnessed := datagen.AddWitnessToUnbondingTx(
		t,
		puChildStakingOutput,
		postUpgradeStakerSK,
		covSKs,
		bsParams.CovenantQuorum,
		[]*btcec.PublicKey{fpPK},
		uint16(puChildResp.StakingTime),
		int64(puChildResp.TotalSat),
		puChildUnbondingTx,
		btcCfg,
	)
	puTipBeforeUnbondingResp, err := n.QueryTip()
	require.NoError(t, err)
	puTipBeforeUnbonding, err := tkeeper.ParseBTCHeaderInfoResponseToInfo(puTipBeforeUnbondingResp)
	require.NoError(t, err)
	puUnbondingBlock := datagen.CreateBlockWithTransaction(
		n.Tm.R, puTipBeforeUnbonding.Header.ToBlockHeader(), puChildUnbondingWitnessed,
	)
	n.InsertHeader(&puUnbondingBlock.HeaderBytes)
	puUnbondingProof := bstypes.NewInclusionProofFromSpvProof(puUnbondingBlock.SpvProof)

	// Poison the child — same intent-based MsgBTCUndelegate as before.
	n.SubmitBTCUndelegate(wStaker, &bstypes.MsgBTCUndelegate{
		Signer:                        wStaker.Address.String(),
		StakingTxHash:                 puChildStkHash,
		StakeSpendingTx:               puChildUnbondingTxBz,
		StakeSpendingTxInclusionProof: puUnbondingProof,
		FundingTransactions:           [][]byte{puChildStkTxBz},
	})
	requireDelegationState(t, n, "post-upgrade live-attack [parent stays ACTIVE]",
		puParentHash, expectedDelState{Status: "ACTIVE"})
	requireDelegationState(t, n, "post-upgrade live-attack [child UNBONDED via poison]",
		puChildStkHash, expectedDelState{Status: "UNBONDED", HasUnbondInfo: true})

	// Stage child's STAKING tx on BTC at depth=1. Critically we do NOT
	// extend the BTC LC further — proving the parent unbond goes through at
	// 1-deep merkle, not k-deep. The babylon-side fix (commit 78b0ed2f)
	// makes this work because child.IsUnbondedEarly() == true ⇒
	// shouldActivateStkExp = false ⇒ default-switch path with 1-deep proof.
	puExpansionWitnessedBz, _ := datagen.AddWitnessToStakeExpTx(
		t,
		puParentStkTx.TxOut[datagen.StakingOutIdx],
		puFundingTx.TxOut[0],
		postUpgradeStakerSK,
		covSKs,
		bsParams.CovenantQuorum,
		[]*btcec.PublicKey{fpPK},
		stakingTime,
		stakingValue,
		puChildStkTx,
		btcCfg,
	)
	puTipBeforeExpansionResp, err := n.QueryTip()
	require.NoError(t, err)
	puTipBeforeExpansion, err := tkeeper.ParseBTCHeaderInfoResponseToInfo(puTipBeforeExpansionResp)
	require.NoError(t, err)
	puExpansionBlock := datagen.CreateBlockWithTransaction(
		n.Tm.R, puTipBeforeExpansion.Header.ToBlockHeader(), puChildStkTx,
	)
	puExpansionBlockHeight := puTipBeforeExpansion.Height + 1
	n.InsertHeader(&puExpansionBlock.HeaderBytes)
	puExpansionProof := bstypes.NewInclusionProofFromSpvProof(puExpansionBlock.SpvProof)

	// Sanity: expansion tx is at <k-deep — the very property we want the
	// post-upgrade chain to honor.
	puFinalTipResp, err := n.QueryTip()
	require.NoError(t, err)
	puFinalTip, err := tkeeper.ParseBTCHeaderInfoResponseToInfo(puFinalTipResp)
	require.NoError(t, err)
	puExpansionDepth := puFinalTip.Height - puExpansionBlockHeight + 1
	require.Less(t, puExpansionDepth, uint32(tmanager.BabylonBtcConfirmationPeriod),
		"sanity: expansion tx must be < k-deep to exercise the 1-deep merkle path (depth=%d, k=%d)",
		puExpansionDepth, tmanager.BabylonBtcConfirmationPeriod)

	puParentStkTxBz, err := bbn.SerializeBTCTx(puParentStkTx)
	require.NoError(t, err)

	// The vigilante (or any honest watcher) submits MsgBTCUndelegate for the
	// parent using the expansion staking tx as proof of spend. With the
	// post-upgrade binary running, this MUST succeed at 1-deep.
	n.SubmitBTCUndelegate(wStaker, &bstypes.MsgBTCUndelegate{
		Signer:                        wStaker.Address.String(),
		StakingTxHash:                 puParentHash,
		StakeSpendingTx:               puExpansionWitnessedBz,
		StakeSpendingTxInclusionProof: puExpansionProof,
		FundingTransactions:           [][]byte{puParentStkTxBz, puExpansionMsg.FundingTx},
	})

	requireDelegationState(t, n, "post-upgrade live-attack [parent UNBONDED via fix]",
		puParentHash, expectedDelState{Status: "UNBONDED", HasUnbondInfo: true})
	puParentAfter := n.QueryBTCDelegation(puParentHash)
	require.NotEmpty(t, puParentAfter.UndelegationResponse.DelegatorUnbondingInfoResponse.SpendStakeTxHex,
		"default-switch case writes the expansion tx as SpendStakeTx")
	requireDelegationState(t, n, "post-upgrade live-attack [child stays UNBONDED]",
		puChildStkHash, expectedDelState{Status: "UNBONDED", HasUnbondInfo: true})

	t.Logf("v4.3 upgrade handler force-unbonded %d phantom-parent victims, skipped 1 expired-parent victim, AND post-upgrade chain accepts vigilante's 1-deep parent-unbond at depth=%d (k=%d)", len(victims), puExpansionDepth, tmanager.BabylonBtcConfirmationPeriod)
}

// advanceBtcTipUntil drives Babylon's BTC light client tip up to at least
// targetHeight by submitting batched MsgInsertHeaders txs. Used to push a
// short-lived parent delegation into EXPIRED status before the upgrade runs.
//
// MsgInsertHeaders gas cost grows with BTC LC depth in the v4.2.5 binary
// (we observed ~300K per tx around depth 100+ with even a single header),
// so we use SubmitMsgsWithGas with a 50M-gas budget — comfortably within
// the chain's 300M block gas limit and large enough to absorb depth-driven
// cost growth across the full advance. Batching ~25 headers per tx keeps
// the round-trip count low (~12 batches for 300 headers).
func advanceBtcTipUntil(t *testing.T, n *tmanager.Node, targetHeight uint32) {
	const (
		batchSize = 25
		// Chain caps per-tx gas at 10M; pick that ceiling. Empirically
		// MsgInsertHeaders costs ~300K base + tens of K per header, so 10M
		// is plenty even as BTC LC depth grows.
		batchGasLim = uint64(10_000_000)
	)

	wallet := n.Wallet("node-key")
	require.NotNil(t, wallet, "node-key wallet must exist")

	for {
		tipResp, err := n.QueryTip()
		require.NoError(t, err)
		if tipResp.Height >= targetHeight {
			return
		}

		need := int(targetHeight - tipResp.Height)
		if need > batchSize {
			need = batchSize
		}

		tip, err := tkeeper.ParseBTCHeaderInfoResponseToInfo(tipResp)
		require.NoError(t, err)

		headers := make([]bbn.BTCHeaderBytes, 0, need)
		parent := tip
		for i := 0; i < need; i++ {
			child := datagen.GenRandomValidBTCHeaderInfoWithParent(n.Tm.R, *parent)
			headers = append(headers, *child.Header)
			parent = child
		}

		_, tx := wallet.SubmitMsgsWithGas(batchGasLim, &blctypes.MsgInsertHeaders{
			Signer:  wallet.Address.String(),
			Headers: headers,
		})
		require.NotNil(t, tx, "MsgInsertHeaders tx must not be nil")

		n.WaitUntilBtcHeight(tipResp.Height + uint32(need))
	}
}

// buildV43UpgradeProposal constructs the standard MsgSoftwareUpgrade gov
// proposal targeting the v4.3 plan name at the given height.
func buildV43UpgradeProposal(t *testing.T, valWallet *tmanager.WalletSender, upgradeHeight int64) *govtypes.MsgSubmitProposal {
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

	return &govtypes.MsgSubmitProposal{
		Messages:       []*types.Any{anyMsg},
		InitialDeposit: []sdk.Coin{sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1_000_000))},
		Proposer:       valWallet.Address.String(),
		Metadata:       "",
		Title:          v43.UpgradeName,
		Summary:        "v4.3 upgrade — remediate poisoned stake-expansion parents (GHSA-4rm2-cj74-f62h)",
		Expedited:      false,
	}
}

func hexEncode(b []byte) string {
	const hexChars = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, x := range b {
		out[i*2] = hexChars[x>>4]
		out[i*2+1] = hexChars[x&0x0f]
	}
	return string(out)
}
