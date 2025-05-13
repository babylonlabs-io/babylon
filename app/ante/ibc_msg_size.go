package ante

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	icacontrollertypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
)

const (
	MaxMsgSize     = 500_000 // 500 KB
	MaxMemoSize    = 400_000 // 400 KB
	MaxAddressSize = 65_000  // 65 KB
)

// IBCMsgSizeDecorator checks that IBC messages size is within the accepted size
type IBCMsgSizeDecorator struct{}

func NewIBCMsgSizeDecorator() IBCMsgSizeDecorator {
	return IBCMsgSizeDecorator{}
}

// AnteHandle checks that IBC messages size is within the accepted size
func (IBCMsgSizeDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	// Local mempool filter for improper ibc packets
	if ctx.IsCheckTx() {
		for _, msg := range tx.GetMsgs() {
			switch msg := msg.(type) {
			case *ibctransfertypes.MsgTransfer:
				if msg.Size() > MaxMsgSize {
					return ctx, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "msg size is too large. max_msg_size %d", MaxMsgSize)
				}

				if len([]byte(msg.Memo)) > MaxMemoSize {
					return ctx, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "memo is too large. max_memo_size %d", MaxMemoSize)
				}

				if len(msg.Receiver) > MaxAddressSize {
					return ctx, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "receiver address is too large. max_address_size %d", MaxAddressSize)
				}

			// If one of the msgs is from ICA, limit it's size due to current spam potential.
			case *icacontrollertypes.MsgSendTx:
				if msg.PacketData.Size() > MaxMsgSize {
					return ctx, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "packet data is too large. max_msg_size %d", MaxMsgSize)
				}

				if len([]byte(msg.Owner)) > MaxAddressSize {
					return ctx, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "owner address is too large. max_address_size %d", MaxAddressSize)
				}
			}
		}

	}

	return next(ctx, tx, simulate)
}
