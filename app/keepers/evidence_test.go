package keepers_test

import (
	"testing"
	"time"

	"cosmossdk.io/core/comet"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	testhelper "github.com/babylonlabs-io/babylon/v4/testutil/helper"
)

type evValidator struct {
	addr  []byte
	power int64
}

func (v evValidator) Address() []byte { return v.addr }
func (v evValidator) Power() int64    { return v.power }

type evDuplicateVote struct {
	val    evValidator
	height int64
	time   time.Time
}

func (e evDuplicateVote) Type() comet.MisbehaviorType { return comet.DuplicateVote }
func (e evDuplicateVote) Validator() comet.Validator  { return e.val }
func (e evDuplicateVote) Height() int64               { return e.height }
func (e evDuplicateVote) Time() time.Time             { return e.time }
func (e evDuplicateVote) TotalVotingPower() int64     { return e.val.power }

type evList []comet.Evidence

func (l evList) Len() int                 { return len(l) }
func (l evList) Get(i int) comet.Evidence { return l[i] }

type evBlockInfo struct{ ev evList }

func (b evBlockInfo) GetEvidence() comet.EvidenceList { return b.ev }
func (b evBlockInfo) GetValidatorsHash() []byte       { return nil }
func (b evBlockInfo) GetProposerAddress() []byte      { return nil }
func (b evBlockInfo) GetLastCommit() comet.CommitInfo { return nil }

// TestEvidenceForDeletedValidator is the regression test for
// GHSA-24xq-83wm-x4g5: checkpoint finalization can delete an unbonded
// validator while CometBFT still accepts equivocation evidence against it.
// Such evidence must be skipped, not fail FinalizeBlock on every node.
func TestEvidenceForDeletedValidator(t *testing.T) {
	h := testhelper.NewHelper(t)
	app, ctx := h.App, h.Ctx
	sk := app.StakingKeeper

	vals, err := sk.GetAllValidators(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, vals)
	val := vals[0]

	opAddr, err := sk.ValidatorAddressCodec().StringToBytes(val.GetOperator())
	require.NoError(t, err)
	consAddr, err := val.GetConsAddr()
	require.NoError(t, err)
	consPubKey, err := val.ConsPubKey()
	require.NoError(t, err)

	height, blockTime := ctx.BlockHeight(), ctx.BlockTime()
	require.NoError(t, app.SlashingKeeper.AddPubkey(ctx, consPubKey))
	require.NoError(t, app.SlashingKeeper.SetValidatorSigningInfo(ctx, consAddr,
		slashingtypes.NewValidatorSigningInfo(consAddr, height, 0, time.Unix(0, 0), false, 0)))

	evCtx := ctx.WithBlockHeight(height + 1).WithCometInfo(evBlockInfo{ev: evList{
		evDuplicateVote{val: evValidator{addr: consAddr, power: 1}, height: height, time: blockTime},
	}})

	// Terminal state UnbondAllMatureValidators reaches for a fully undelegated
	// validator, followed by the same RemoveValidator call.
	val.Status = stakingtypes.Unbonded
	val.Tokens = math.ZeroInt()
	val.DelegatorShares = math.LegacyZeroDec()
	require.NoError(t, sk.SetValidator(evCtx, val))
	require.NoError(t, sk.RemoveValidator(evCtx, opAddr))
	_, err = sk.GetValidatorByConsAddr(evCtx, sdk.ConsAddress(consAddr))
	require.ErrorIs(t, err, stakingtypes.ErrNoValidatorFound)

	require.NoError(t, app.EvidenceKeeper.BeginBlocker(evCtx))
}
