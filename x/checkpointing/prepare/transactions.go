package prepare

import (
	"errors"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
)

// PrepareProposalTxs is used as an intermediary storage for transactions when creating
// a proposal for `PrepareProposal`.
type PrepareProposalTxs struct {
	// Transactions.
	CheckpointTx []byte
	OtherTxs     [][]byte

	// Bytes.
	// In general, there's no need to check for int64 overflow given that it would require
	// exabytes of memory to hit the max int64 value in bytes.
	MaxBytes  uint64
	UsedBytes uint64
}

// NewPrepareProposalTxs returns a new `PrepareProposalTxs` given the request.
func NewPrepareProposalTxs(
	req *abci.RequestPrepareProposal,
) (PrepareProposalTxs, error) {
	if req.MaxTxBytes <= 0 {
		return PrepareProposalTxs{}, errors.New("MaxTxBytes must be positive")
	}

	return PrepareProposalTxs{
		MaxBytes:  1008600, // uint64(req.MaxTxBytes),
		UsedBytes: 0,
	}, nil
}

// SetOrReplaceCheckpointTx sets the tx used for checkpoint. If the checkpoint tx already exists,
// replace it
func (t *PrepareProposalTxs) SetOrReplaceCheckpointTx(tx []byte) error {
	oldBytes := uint64(len(t.CheckpointTx))
	newBytes := uint64(len(tx))
	if err := t.updateUsedBytes(oldBytes, newBytes); err != nil {
		return err
	}
	t.CheckpointTx = tx
	return nil
}

// ReplaceOtherTxs replaces other txs with the given txs (existing ones are cleared)
func (t *PrepareProposalTxs) ReplaceOtherTxs(allTxs [][]byte) error {
	t.OtherTxs = make([][]byte, 0, len(allTxs))
	bytesToAdd := uint64(0)
	for _, tx := range allTxs {
		txSize := uint64(len(tx))
		if t.UsedBytes+bytesToAdd+txSize > t.MaxBytes {
			break
		}

		bytesToAdd += txSize
		t.OtherTxs = append(t.OtherTxs, tx)
	}

	if err := t.updateUsedBytes(0, bytesToAdd); err != nil {
		return err
	}

	return nil
}

// updateUsedBytes updates the used bytes field. This returns an error if the num used bytes
// exceeds the max byte limit.
func (t *PrepareProposalTxs) updateUsedBytes(
	bytesToRemove uint64,
	bytesToAdd uint64,
) error {
	if t.UsedBytes < bytesToRemove {
		return errors.New("result cannot be negative")
	}

	finalBytes := t.UsedBytes - bytesToRemove + bytesToAdd
	if finalBytes > t.MaxBytes {
		return fmt.Errorf("exceeds max: max=%d, used=%d, adding=%d", t.MaxBytes, t.UsedBytes, bytesToAdd)
	}

	t.UsedBytes = finalBytes
	return nil
}

// GetTxsInOrder returns a list of txs in an order that the `ProcessProposal` expects.
func (t *PrepareProposalTxs) GetTxsInOrder() [][]byte {
	txsToReturn := make([][]byte, 0, 1+len(t.OtherTxs))

	// 1. Checkpoint tx
	if len(t.CheckpointTx) > 0 {
		txsToReturn = append(txsToReturn, t.CheckpointTx)
	}

	// 2. "Other" txs
	if len(t.OtherTxs) > 0 {
		txsToReturn = append(txsToReturn, t.OtherTxs...)
	}

	return txsToReturn
}
