package types

import (
	"fmt"

	"github.com/babylonlabs-io/babylon/v2/crypto/bls12381"
)

func (cm RawCheckpointWithMeta) Validate() error {
	if cm.Ckpt == nil {
		return ErrNilCkpt
	}
	if err := cm.Ckpt.ValidateBasic(); err != nil {
		return err
	}
	if !isValidCheckpointStatus(cm.Status) {
		return ErrInvalidCkptStatus.Wrapf("got %d", cm.Status)
	}
	if cm.BlsAggrPk == nil {
		return ErrNilBlsAggrPk
	}
	if cm.BlsAggrPk.Size() != bls12381.PubKeySize {
		return fmt.Errorf("invalid size of BlsAggrPk: got %d, expected %d", cm.BlsAggrPk.Size(), bls12381.PubKeySize)
	}
	if cm.Lifecycle != nil {
		for _, lc := range cm.Lifecycle {
			if err := lc.Validate(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (csu *CheckpointStateUpdate) Validate() error {
	if !isValidCheckpointStatus(csu.State) {
		return fmt.Errorf("%w: %d", ErrInvalidCkptStatus, csu.State)
	}

	if csu.BlockHeight == 0 {
		return ErrZeroBlockHeight
	}

	if csu.BlockTime == nil {
		return ErrNilBlockTime
	}

	return nil
}

func isValidCheckpointStatus(status CheckpointStatus) bool {
	switch status {
	case Accumulating, Sealed, Submitted, Confirmed, Finalized:
		return true
	default:
		return false
	}
}
