package prepare_test

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/prepare"
)

func TestPrepareProposalTxs(t *testing.T) {
	t.Run("happy path - can add transactions within limit", func(t *testing.T) {
		maxBytes := uint64(100)
		req := &abci.RequestPrepareProposal{
			MaxTxBytes: int64(maxBytes),
		}

		txs, err := prepare.NewPrepareProposalTxs(req)
		require.NoError(t, err)

		// Set checkpoint tx of size 20
		checkpointTx := make([]byte, 20)
		err = txs.SetOrReplaceCheckpointTx(checkpointTx)
		require.NoError(t, err)

		// Add other txs that fit within the remaining proto budget.
		otherTxs := [][]byte{
			make([]byte, 30), // tx1 (proto 32)
			make([]byte, 30), // tx2 (proto 32)
			make([]byte, 10), // tx3 (proto 12)
		}
		err = txs.ReplaceOtherTxs(otherTxs)
		require.NoError(t, err)

		// Verify all txs were added. UsedBytes is the protobuf Data.Txs size
		// (raw + per-tx framing): 22 + 32 + 32 + 12 = 98.
		allTxs := txs.GetTxsInOrder()
		require.Equal(t, 4, len(allTxs)) // checkpoint + 3 other txs
		require.Equal(t, uint64(98), txs.UsedBytes)
	})

	t.Run("partial addition - some transactions exceed limit", func(t *testing.T) {
		maxBytes := uint64(50)
		req := &abci.RequestPrepareProposal{
			MaxTxBytes: int64(maxBytes),
		}

		txs, err := prepare.NewPrepareProposalTxs(req)
		require.NoError(t, err)

		// Set checkpoint tx of size 20
		checkpointTx := make([]byte, 20)
		err = txs.SetOrReplaceCheckpointTx(checkpointTx)
		require.NoError(t, err)

		// Try to add other txs where some won't fit (proto sizes shown)
		otherTxs := [][]byte{
			make([]byte, 20), // tx1 (proto 22) - fits
			make([]byte, 20), // tx2 (proto 22) - won't fit
			make([]byte, 10), // tx3 (proto 12) - won't fit
		}
		err = txs.ReplaceOtherTxs(otherTxs)
		require.NoError(t, err)

		// Verify only fitting txs were added. Proto: checkpoint 22 + tx1 22 = 44;
		// tx2 would make 66 > 50, so it and tx3 are dropped.
		allTxs := txs.GetTxsInOrder()
		require.Equal(t, 2, len(allTxs)) // checkpoint + tx1
		require.Equal(t, uint64(44), txs.UsedBytes)
	})

	t.Run("full addition - transactions fill space exactly", func(t *testing.T) {
		maxBytes := uint64(100)
		req := &abci.RequestPrepareProposal{
			MaxTxBytes: int64(maxBytes),
		}

		txs, err := prepare.NewPrepareProposalTxs(req)
		require.NoError(t, err)

		// Set checkpoint tx of size 38 (proto 40)
		checkpointTx := make([]byte, 38)
		err = txs.SetOrReplaceCheckpointTx(checkpointTx)
		require.NoError(t, err)

		// Add other txs whose proto sizes exactly fill the remaining space:
		// 40 + 30 + 30 = 100.
		otherTxs := [][]byte{
			make([]byte, 28), // tx1 (proto 30)
			make([]byte, 28), // tx2 (proto 30)
		}
		err = txs.ReplaceOtherTxs(otherTxs)
		require.NoError(t, err)

		// Verify all txs were added and the proto budget is exactly filled.
		allTxs := txs.GetTxsInOrder()
		require.Equal(t, 3, len(allTxs)) // checkpoint + 2 other txs
		require.Equal(t, maxBytes, txs.UsedBytes)
	})

	t.Run("error - checkpoint tx exceeds limit", func(t *testing.T) {
		maxBytes := uint64(10)
		req := &abci.RequestPrepareProposal{
			MaxTxBytes: int64(maxBytes),
		}

		txs, err := prepare.NewPrepareProposalTxs(req)
		require.NoError(t, err)

		// Try to set checkpoint tx larger than max
		checkpointTx := make([]byte, 20)
		err = txs.SetOrReplaceCheckpointTx(checkpointTx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "exceeds max")
	})

	t.Run("error - negative MaxTxBytes", func(t *testing.T) {
		req := &abci.RequestPrepareProposal{
			MaxTxBytes: -1,
		}

		_, err := prepare.NewPrepareProposalTxs(req)
		require.Error(t, err)
		require.Contains(t, err.Error(), "must be positive")
	})

	// Regression for GHSA-692h-272j-rvgc: a set whose RAW byte sum fits MaxTxBytes
	// but whose protobuf Data.Txs size (raw + per-tx framing) does not. The old
	// raw-len accounting returned the whole set, and CometBFT's Txs.Validate
	// (which sums ComputeProtoSizeForTxs) then panicked the proposer building the
	// block. The proposal we return must never exceed MaxTxBytes in proto terms.
	t.Run("regression GHSA-692h-272j-rvgc - proto size never exceeds MaxTxBytes", func(t *testing.T) {
		maxBytes := uint64(100)
		req := &abci.RequestPrepareProposal{MaxTxBytes: int64(maxBytes)}

		txs, err := prepare.NewPrepareProposalTxs(req)
		require.NoError(t, err)

		checkpointTx := make([]byte, 8) // proto 10
		require.NoError(t, txs.SetOrReplaceCheckpointTx(checkpointTx))

		otherTxs := [][]byte{
			make([]byte, 22), // proto 24
			make([]byte, 22),
			make([]byte, 22),
			make([]byte, 22),
		}

		// Precondition that reproduces the bug: raw fits, proto does not.
		candidate := append([][]byte{checkpointTx}, otherTxs...)
		rawSum := 0
		for _, tx := range candidate {
			rawSum += len(tx)
		}
		require.LessOrEqual(t, uint64(rawSum), maxBytes,
			"raw sum must fit — this is what the buggy raw-len accounting saw")
		require.Greater(t, cmttypes.ComputeProtoSizeForTxs(cmttypes.ToTxs(candidate)), int64(maxBytes),
			"proto sum must exceed — this is what CometBFT's Txs.Validate rejects")

		require.NoError(t, txs.ReplaceOtherTxs(otherTxs))
		got := txs.GetTxsInOrder()

		// The fix must keep the returned proposal within MaxTxBytes in PROTO terms,
		// exactly the invariant CometBFT enforces, so the proposer never returns a
		// block it will then panic building.
		require.LessOrEqual(t, cmttypes.ComputeProtoSizeForTxs(cmttypes.ToTxs(got)), int64(maxBytes),
			"returned proposal must fit MaxTxBytes in proto terms")
		require.LessOrEqual(t, txs.UsedBytes, maxBytes)
		require.Less(t, len(got), len(candidate), "the overflowing tx must be dropped")
		require.Equal(t, checkpointTx, got[0], "the checkpoint tx must be kept")
	})
}
