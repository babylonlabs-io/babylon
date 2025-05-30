package types

import errorsmod "cosmossdk.io/errors"

func (p BTCSpvProof) Validate() error {
	if len(p.BtcTransaction) == 0 {
		return errorsmod.Wrapf(ErrInvalidBTCSpvProof, "btc_transaction must not be empty")
	}

	if len(p.MerkleNodes)%32 != 0 {
		return errorsmod.Wrapf(ErrInvalidBTCSpvProof, "merkle_nodes length must be divisible by 32")
	}

	if p.ConfirmingBtcHeader == nil {
		return errorsmod.Wrapf(ErrInvalidBTCSpvProof, "confirming_btc_header must not be nil")
	}

	if len(*p.ConfirmingBtcHeader) != 80 {
		return errorsmod.Wrapf(ErrInvalidBTCSpvProof, "confirming_btc_header must be exactly 80 bytes")
	}
	return nil
}
