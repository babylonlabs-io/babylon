package ante

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	icacontrollertypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/controller/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
)

const (
	MaxMsgSize     = 500_000 // 500 KB
	MaxMemoSize    = 400_000 // 400 KB
	MaxAddressSize = 90      // 90 chars
)

// IBCMsgSizeDecorator checks that IBC messages size is within the accepted size
type IBCMsgSizeDecorator struct{}

func NewIBCMsgSizeDecorator() IBCMsgSizeDecorator {
	return IBCMsgSizeDecorator{}
}

// AnteHandle checks that IBC messages size is within the accepted size
func (IBCMsgSizeDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	for _, msg := range tx.GetMsgs() {
		var err error
		switch msg := msg.(type) {
		case *ibctransfertypes.MsgTransfer:
			err = validateIBCMsgTransfer(msg)
		// If one of the msgs is from ICA, limit it's size due to current spam potential.
		case *icacontrollertypes.MsgSendTx:
			err = validateICAMsgSendTx(msg)
		}
		if err != nil {
			return ctx, err
		}
	}
	return next(ctx, tx, simulate)
}

func validateIBCMsgTransfer(msg *ibctransfertypes.MsgTransfer) error {
	if msg.Size() > MaxMsgSize {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "msg size is too large. max_msg_size %d", MaxMsgSize)
	}

	if len([]byte(msg.Memo)) > MaxMemoSize {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "memo is too large. max_memo_size %d", MaxMemoSize)
	}

	if len(msg.Receiver) > MaxAddressSize {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "receiver address is too large. max_address_size %d", MaxAddressSize)
	}
	return nil
}

func validateICAMsgSendTx(msg *icacontrollertypes.MsgSendTx) error {
	if msg.PacketData.Size() > MaxMsgSize {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "packet data is too large. max_msg_size %d", MaxMsgSize)
	}

	if len([]byte(msg.Owner)) > MaxAddressSize {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "owner address is too large. max_address_size %d", MaxAddressSize)
	}
	return nil
}
