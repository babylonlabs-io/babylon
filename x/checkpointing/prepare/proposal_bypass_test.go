package prepare_test

import (
	"fmt"
	"testing"

	"cosmossdk.io/log"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/app"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/prepare"
	ckpttypes "github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
)

func setupProposalHandler(t *testing.T, tempApp *app.BabylonApp) *prepare.ProposalHandler {
	encCfg := appparams.DefaultEncodingConfig()
	ckpttypes.RegisterInterfaces(encCfg.InterfaceRegistry)
	mCfg := mempool.DefaultPriorityNonceMempoolConfig()
	mCfg.MaxTx = 0
	mem := mempool.NewPriorityMempool(mCfg)
	logger := log.NewTestLogger(t)
	db := dbm.NewMemDB()
	bApp := baseapp.NewBaseApp(t.Name(), logger, db, encCfg.TxConfig.TxDecoder(), baseapp.SetChainID("test"))

	ckptKeeper := tempApp.CheckpointingKeeper
	return prepare.NewProposalHandler(log.NewNopLogger(), ckptKeeper, mem, bApp, encCfg)
}

// TestVerifyVoteExtension_RejectsWrongEpoch covers the proposer-side epoch
// check. A validator constructs a properly signed vote extension over
// GetSignBytes(claimedEpoch, blockHash) for a claimedEpoch that differs
// from the epoch the proposer is building a checkpoint for.
// VerifyVoteExtension must reject it before the per-sig VerifyBLSSig step
// (which would otherwise verify the sig against the VE's own claimed
// epoch and pass).
func TestVerifyVoteExtension_RejectsWrongEpoch(t *testing.T) {
	const (
		expectedEpoch = uint64(7)
		claimedEpoch  = uint64(5)
	)

	tempApp := app.NewTmpBabylonApp()
	proposalHandler := setupProposalHandler(t, tempApp)
	ctx := setupSdkCtx(100)

	// One validator signs a VE over GetSignBytes(claimedEpoch, blockHash). The
	// signature itself is valid for that epoch — the only issue is that
	// claimedEpoch != expectedEpoch.
	val := genNTestValidators(t, 1)[0]
	bh := ckpttypes.BlockHash(make([]byte, ckpttypes.HashSize))
	ve := val.VoteExtension(&bh, claimedEpoch)
	veBytes, err := ve.Marshal()
	require.NoError(t, err)

	blsSig, err := proposalHandler.VerifyVoteExtension(ctx, expectedEpoch, veBytes, bh)
	require.Nil(t, blsSig)
	require.EqualError(t, err,
		fmt.Sprintf("vote extension epoch mismatch: expected %d, got %d", expectedEpoch, claimedEpoch))
}
