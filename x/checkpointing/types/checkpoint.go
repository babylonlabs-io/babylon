package types

import "errors"

func (cm RawCheckpointWithMeta) Validate() error {
	if cm.Ckpt == nil {
		return errors.New("null checkpoint")
	}
	if err := cm.Ckpt.ValidateBasic(); err != nil {
		return err
	}

	if cm.Ckpt.BlsMultiSig != nil {
		if err := cm.Ckpt.BlsMultiSig.ValidateBasic(); err != nil {
			return err
		}
	}
	return nil
}
