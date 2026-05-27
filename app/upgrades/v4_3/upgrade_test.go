package v4_3_test

import (
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	v4_3 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v4_3"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	testutilkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btcctypes "github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/types"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	btcstkkeeper "github.com/babylonlabs-io/babylon/v4/x/btcstaking/keeper"
	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

// setup builds a minimal BTC staking keeper with mocked BTC light client and
// checkpoint keepers, and seeds the params so AddBTCDelegation works.
//
// The mocked BTC light client tip is set so that delegations created via
// genDelegation evaluate as ACTIVE (StartHeight=10, EndHeight≈210,
// UnbondingTime=101 ⇒ ACTIVE iff tip < EndHeight - UnbondingTime ≈ 109).
func setup(t *testing.T) (sdk.Context, btcstkkeeper.Keeper, *gomock.Controller) {
	return setupWithBtcTip(t, 100)
}

// setupWithBtcTip is like setup but lets the caller pin the BTC tip height,
// so individual tests can force genDelegation parents into EXPIRED status by
// passing a tip past their EndHeight.
func setupWithBtcTip(t *testing.T, btcTipHeight uint32) (sdk.Context, btcstkkeeper.Keeper, *gomock.Controller) {
	ctrl := gomock.NewController(t)

	btclc := btcstktypes.NewMockBTCLightClientKeeper(ctrl)
	btclc.EXPECT().GetTipInfo(gomock.Any()).Return(&btclctypes.BTCHeaderInfo{Height: btcTipHeight}).AnyTimes()

	btcc := btcstktypes.NewMockBtcCheckpointKeeper(ctrl)
	btcc.EXPECT().GetParams(gomock.Any()).Return(btcctypes.DefaultParams()).AnyTimes()

	k, ctx := testutilkeeper.BTCStakingKeeper(t, btclc, btcc, nil)
	return ctx, *k, ctrl
}

// genDelegation produces a fully-formed BTCDelegation with a valid serialized
// staking tx. Its HasInclusionProof() is true (StartHeight, EndHeight > 0)
// and BtcUndelegation has no DelegatorUnbondingInfo.
func genDelegation(t *testing.T, r *rand.Rand) *btcstktypes.BTCDelegation {
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	fp, err := datagen.GenRandomFinalityProvider(r)
	require.NoError(t, err)

	startHeight := uint32(10)
	endHeight := startHeight + btcctypes.DefaultParams().CheckpointFinalizationTimeout + 100
	stakingTime := endHeight - startHeight

	slashingAddr, err := datagen.GenRandomBTCAddress(r, &chaincfg.RegressionNetParams)
	require.NoError(t, err)
	slashingPkScript, err := txscript.PayToAddrScript(slashingAddr)
	require.NoError(t, err)

	covSKs, covPKs, covQ := datagen.GenCovenantCommittee(r)
	del, err := datagen.GenRandomBTCDelegation(
		r, t, &chaincfg.RegressionNetParams,
		[]bbn.BIP340PubKey{*fp.BtcPk},
		delSK, covSKs, covPKs, covQ,
		slashingPkScript,
		stakingTime, startHeight, endHeight, 50000,
		math.LegacyNewDecWithPrec(15, 2),
		uint16(101),
	)
	require.NoError(t, err)
	del.StakerAddr = datagen.GenRandomAccount().GetAddress().String()
	return del
}

// markStakeExpansionOf turns child into a stake-expansion delegation of parent.
func markStakeExpansionOf(child, parent *btcstktypes.BTCDelegation) {
	parentHash := parent.MustGetStakingTxHash()
	child.StkExp = &btcstktypes.StakeExpansion{
		PreviousStakingTxHash: parentHash[:],
		// OtherFundingTxOut intentionally left empty — the remediation handler
		// does not parse it.
	}
}

// poison clears child's inclusion proof and sets DelegatorUnbondingInfo,
// matching the GHSA-4rm2-cj74-f62h attack pattern: child was unbonded early
// before its inclusion proof was added.
func poison(child *btcstktypes.BTCDelegation, spendTx []byte) {
	child.StartHeight = 0
	child.EndHeight = 0
	child.BtcUndelegation.DelegatorUnbondingInfo = &btcstktypes.DelegatorUnbondingInfo{
		SpendStakeTx: spendTx,
	}
}

func TestRemediate_RemediatesPoisonedParent(t *testing.T) {
	ctx, k, ctrl := setup(t)
	defer ctrl.Finish()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	parent := genDelegation(t, r)
	child := genDelegation(t, r)
	markStakeExpansionOf(child, parent)
	poison(child, child.StakingTx)

	require.NoError(t, k.AddBTCDelegation(ctx, parent))
	require.NoError(t, k.AddBTCDelegation(ctx, child))

	// Pre-condition: parent has no DelegatorUnbondingInfo yet.
	parentHash := parent.MustGetStakingTxHash().String()
	pre, err := k.GetBTCDelegation(ctx, parentHash)
	require.NoError(t, err)
	require.Nil(t, pre.BtcUndelegation.DelegatorUnbondingInfo)

	remediated, err := v4_3.RemediatePoisonedStakeExpansions(ctx, k)
	require.NoError(t, err)
	require.Equal(t, []string{parentHash}, remediated)

	// Post-condition: parent is unbonded with the child's staking tx as proof.
	post, err := k.GetBTCDelegation(ctx, parentHash)
	require.NoError(t, err)
	require.NotNil(t, post.BtcUndelegation.DelegatorUnbondingInfo)
	require.Equal(t, child.StakingTx, post.BtcUndelegation.DelegatorUnbondingInfo.SpendStakeTx)

	// Audit event must be emitted exactly once for this pair.
	requireRemediationEvent(t, ctx, parentHash, child.MustGetStakingTxHash().String())
}

func TestRemediate_SkipsHealthyDelegations(t *testing.T) {
	ctx, k, ctrl := setup(t)
	defer ctrl.Finish()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Case A: plain delegation, not stake expansion.
	plain := genDelegation(t, r)
	require.NoError(t, k.AddBTCDelegation(ctx, plain))

	// Case B: stake-expansion child WITH inclusion proof (legitimately activated).
	parentB := genDelegation(t, r)
	childB := genDelegation(t, r)
	markStakeExpansionOf(childB, parentB)
	// keep child's StartHeight/EndHeight nonzero — has inclusion proof.
	require.NoError(t, k.AddBTCDelegation(ctx, parentB))
	require.NoError(t, k.AddBTCDelegation(ctx, childB))

	// Case C: stake-expansion child without inclusion proof, but NOT
	// unbonded early (no DelegatorUnbondingInfo). Just a pending verified
	// expansion — not poisoned.
	parentC := genDelegation(t, r)
	childC := genDelegation(t, r)
	markStakeExpansionOf(childC, parentC)
	childC.StartHeight = 0
	childC.EndHeight = 0
	require.NoError(t, k.AddBTCDelegation(ctx, parentC))
	require.NoError(t, k.AddBTCDelegation(ctx, childC))

	remediated, err := v4_3.RemediatePoisonedStakeExpansions(ctx, k)
	require.NoError(t, err)
	require.Empty(t, remediated)

	// All parents must remain non-unbonded.
	for _, p := range []*btcstktypes.BTCDelegation{parentB, parentC} {
		got, err := k.GetBTCDelegation(ctx, p.MustGetStakingTxHash().String())
		require.NoError(t, err)
		require.Nil(t, got.BtcUndelegation.DelegatorUnbondingInfo)
	}
}

func TestRemediate_Idempotent(t *testing.T) {
	ctx, k, ctrl := setup(t)
	defer ctrl.Finish()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	parent := genDelegation(t, r)
	child := genDelegation(t, r)
	markStakeExpansionOf(child, parent)
	poison(child, child.StakingTx)
	require.NoError(t, k.AddBTCDelegation(ctx, parent))
	require.NoError(t, k.AddBTCDelegation(ctx, child))

	first, err := v4_3.RemediatePoisonedStakeExpansions(ctx, k)
	require.NoError(t, err)
	require.Len(t, first, 1)

	// Second run sees parent already unbonded and reports nothing.
	second, err := v4_3.RemediatePoisonedStakeExpansions(ctx, k)
	require.NoError(t, err)
	require.Empty(t, second)
}

func TestRemediate_MultipleVictims(t *testing.T) {
	ctx, k, ctrl := setup(t)
	defer ctrl.Finish()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	const n = 5
	parents := make([]*btcstktypes.BTCDelegation, n)
	for i := 0; i < n; i++ {
		parent := genDelegation(t, r)
		child := genDelegation(t, r)
		markStakeExpansionOf(child, parent)
		poison(child, child.StakingTx)
		require.NoError(t, k.AddBTCDelegation(ctx, parent))
		require.NoError(t, k.AddBTCDelegation(ctx, child))
		parents[i] = parent
	}

	remediated, err := v4_3.RemediatePoisonedStakeExpansions(ctx, k)
	require.NoError(t, err)
	require.Len(t, remediated, n)

	for _, p := range parents {
		got, err := k.GetBTCDelegation(ctx, p.MustGetStakingTxHash().String())
		require.NoError(t, err)
		require.NotNil(t, got.BtcUndelegation.DelegatorUnbondingInfo)
	}
}

func TestRemediate_SkipsAlreadyUnbondedParent(t *testing.T) {
	// Models the case where parent unbonding succeeded through the patched
	// BTCUndelegate path (commit 78b0ed2f) before the upgrade ran.
	ctx, k, ctrl := setup(t)
	defer ctrl.Finish()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	parent := genDelegation(t, r)
	child := genDelegation(t, r)
	markStakeExpansionOf(child, parent)
	poison(child, child.StakingTx)

	// Pre-mark parent as legitimately unbonded with some other spend tx.
	otherSpend := []byte{0xde, 0xad, 0xbe, 0xef}
	parent.BtcUndelegation.DelegatorUnbondingInfo = &btcstktypes.DelegatorUnbondingInfo{
		SpendStakeTx: otherSpend,
	}

	require.NoError(t, k.AddBTCDelegation(ctx, parent))
	require.NoError(t, k.AddBTCDelegation(ctx, child))

	remediated, err := v4_3.RemediatePoisonedStakeExpansions(ctx, k)
	require.NoError(t, err)
	require.Empty(t, remediated)

	got, err := k.GetBTCDelegation(ctx, parent.MustGetStakingTxHash().String())
	require.NoError(t, err)
	require.Equal(t, otherSpend, got.BtcUndelegation.DelegatorUnbondingInfo.SpendStakeTx,
		"existing DelegatorUnbondingInfo must not be overwritten")
}

// TestRemediate_SkipsExpiredParent guards against double-counting voting power
// when a poisoned child's parent has already transitioned to a non-ACTIVE
// status (typically EXPIRED via timelock at parent.EndHeight). For such a
// parent, the prior expiry transition already drove voting power to zero;
// force-unbonding now would enqueue a redundant
// EventBTCDelegationStateUpdate{UNBONDED} on top.
//
// We pin the gate by mocking the BTC light-client tip far past the parent's
// EndHeight; IsBtcDelegationActive then evaluates the parent as EXPIRED. The
// handler must skip the parent (no remediation, no DelegatorUnbondingInfo
// write).
func TestRemediate_SkipsExpiredParent(t *testing.T) {
	// btcTipHeight far past parent.EndHeight ⇒ status is EXPIRED.
	// genDelegation builds parents with EndHeight ≈ 210 and UnbondingTime 101,
	// so any tip at or above 109 expires the parent.
	const expiredTip uint32 = 100_000
	ctx, k, ctrl := setupWithBtcTip(t, expiredTip)
	defer ctrl.Finish()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	parent := genDelegation(t, r)
	child := genDelegation(t, r)
	markStakeExpansionOf(child, parent)
	poison(child, child.StakingTx)

	require.NoError(t, k.AddBTCDelegation(ctx, parent))
	require.NoError(t, k.AddBTCDelegation(ctx, child))

	remediated, err := v4_3.RemediatePoisonedStakeExpansions(ctx, k)
	require.NoError(t, err)
	require.Empty(t, remediated, "EXPIRED parent must be skipped — no force-unbond")

	// Parent's BtcUndelegation must still have no DelegatorUnbondingInfo: the
	// handler did NOT touch it. (Expiry alone does not set this field.)
	got, err := k.GetBTCDelegation(ctx, parent.MustGetStakingTxHash().String())
	require.NoError(t, err)
	require.Nil(t, got.BtcUndelegation.DelegatorUnbondingInfo,
		"EXPIRED parent must not be force-unbonded by the handler")
}

func TestRemediate_SkipsChildWithMissingParent(t *testing.T) {
	// Models the case where a poisoned child references a parent that is no
	// longer in the store (e.g., pruned by a future feature). Should be a
	// silent no-op rather than an error.
	ctx, k, ctrl := setup(t)
	defer ctrl.Finish()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Build a fake "parent" purely to seed the child's PreviousStakingTxHash,
	// but never persist it.
	ghostParent := genDelegation(t, r)
	child := genDelegation(t, r)
	markStakeExpansionOf(child, ghostParent)
	poison(child, child.StakingTx)
	require.NoError(t, k.AddBTCDelegation(ctx, child))

	remediated, err := v4_3.RemediatePoisonedStakeExpansions(ctx, k)
	require.NoError(t, err)
	require.Empty(t, remediated)
}

func TestRemediate_DedupesSameParent(t *testing.T) {
	// Defense-in-depth: if two poisoned children somehow share a parent,
	// the parent must be unbonded at most once and only one
	// EventPowerDistUpdate must be queued.
	ctx, k, ctrl := setup(t)
	defer ctrl.Finish()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	parent := genDelegation(t, r)
	child1 := genDelegation(t, r)
	child2 := genDelegation(t, r)
	markStakeExpansionOf(child1, parent)
	markStakeExpansionOf(child2, parent)
	poison(child1, child1.StakingTx)
	poison(child2, child2.StakingTx)

	require.NoError(t, k.AddBTCDelegation(ctx, parent))
	require.NoError(t, k.AddBTCDelegation(ctx, child1))
	require.NoError(t, k.AddBTCDelegation(ctx, child2))

	remediated, err := v4_3.RemediatePoisonedStakeExpansions(ctx, k)
	require.NoError(t, err)
	require.Len(t, remediated, 1, "parent must appear exactly once even with two poisoned children")

	unbondedEvents := unbondedPowerDistEventsFor(ctx, k, parent.MustGetStakingTxHash().String())
	require.Len(t, unbondedEvents, 1, "exactly one UNBONDED power-dist event must be queued for the parent")
}

func TestRemediate_QueuesUnbondedPowerDistEvent(t *testing.T) {
	// Asserts that BtcUndelegate's power-dist event flow fires. Without this
	// the next BeginBlock would not drop the phantom power.
	ctx, k, ctrl := setup(t)
	defer ctrl.Finish()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	parent := genDelegation(t, r)
	child := genDelegation(t, r)
	markStakeExpansionOf(child, parent)
	poison(child, child.StakingTx)
	require.NoError(t, k.AddBTCDelegation(ctx, parent))
	require.NoError(t, k.AddBTCDelegation(ctx, child))

	_, err := v4_3.RemediatePoisonedStakeExpansions(ctx, k)
	require.NoError(t, err)

	parentHash := parent.MustGetStakingTxHash().String()
	unbondedEvents := unbondedPowerDistEventsFor(ctx, k, parentHash)
	require.Len(t, unbondedEvents, 1)
}

func unbondedPowerDistEventsFor(ctx sdk.Context, k btcstkkeeper.Keeper, stakingTxHash string) []*btcstktypes.EventPowerDistUpdate {
	// Tip is mocked at height 100; scan a generous range.
	all := k.GetAllPowerDistUpdateEvents(ctx, 0, 200)
	var out []*btcstktypes.EventPowerDistUpdate
	for _, ev := range all {
		btcEv := ev.GetBtcDelStateUpdate()
		if btcEv == nil {
			continue
		}
		if btcEv.NewState != btcstktypes.BTCDelegationStatus_UNBONDED {
			continue
		}
		if btcEv.StakingTxHash != stakingTxHash {
			continue
		}
		out = append(out, ev)
	}
	return out
}

func requireRemediationEvent(t *testing.T, ctx sdk.Context, parentHash, childHash string) {
	t.Helper()
	for _, ev := range ctx.EventManager().Events() {
		if ev.Type != v4_3.EventTypePhantomStakeExpansionRemediated {
			continue
		}
		var gotParent, gotChild string
		for _, a := range ev.Attributes {
			switch a.Key {
			case v4_3.AttrParentStakingTxHash:
				gotParent = a.Value
			case v4_3.AttrChildStakingTxHash:
				gotChild = a.Value
			}
		}
		if gotParent == parentHash && gotChild == childHash {
			return
		}
	}
	t.Fatalf("expected remediation event for parent=%s child=%s not found", parentHash, childHash)
}
