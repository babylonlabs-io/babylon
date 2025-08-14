package prepare_test

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
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

		// Add other txs that fit within remaining space (80 bytes)
		otherTxs := [][]byte{
			make([]byte, 30), // tx1
			make([]byte, 30), // tx2
			make([]byte, 10), // tx3
		}
		err = txs.ReplaceOtherTxs(otherTxs)
		require.NoError(t, err)

		// Verify all txs were added
		allTxs := txs.GetTxsInOrder()
		require.Equal(t, 4, len(allTxs)) // checkpoint + 3 other txs
		require.Equal(t, uint64(90), txs.UsedBytes)
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

		// Try to add other txs where some won't fit
		otherTxs := [][]byte{
			make([]byte, 20), // tx1 - fits
			make([]byte, 20), // tx2 - won't fit
			make([]byte, 10), // tx3 - won't fit
		}
		err = txs.ReplaceOtherTxs(otherTxs)
		require.NoError(t, err)

		// Verify only fitting txs were added
		allTxs := txs.GetTxsInOrder()
		require.Equal(t, 2, len(allTxs)) // checkpoint + tx1
		require.Equal(t, uint64(40), txs.UsedBytes)
	})

	t.Run("full addition - transactions fill space exactly", func(t *testing.T) {
		maxBytes := uint64(100)
		req := &abci.RequestPrepareProposal{
			MaxTxBytes: int64(maxBytes),
		}

		txs, err := prepare.NewPrepareProposalTxs(req)
		require.NoError(t, err)

		// Set checkpoint tx of size 40
		checkpointTx := make([]byte, 40)
		err = txs.SetOrReplaceCheckpointTx(checkpointTx)
		require.NoError(t, err)

		// Add other txs that exactly fill remaining space (60 bytes)
		otherTxs := [][]byte{
			make([]byte, 30), // tx1
			make([]byte, 30), // tx2
		}
		err = txs.ReplaceOtherTxs(otherTxs)
		require.NoError(t, err)

		// Verify all txs were added and space is exactly filled
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
}
