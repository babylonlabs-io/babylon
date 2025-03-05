package ante

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

type ValidateInternalMsgDecorator struct {
}

func NewValidateInternalMsgDecorator() ValidateInternalMsgDecorator {
	return ValidateInternalMsgDecorator{}
}

func (vd ValidateInternalMsgDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	msgs := tx.GetMsgs()
	if InternalInjectedMsg(msgs) {
		if len(msgs) > 1 {
			return ctx, fmt.Errorf("%w: internal msg must be the only msg in tx", sdkerrors.ErrInvalidRequest)
		}

		if ctx.IsCheckTx() {
			return ctx, fmt.Errorf("%w: internal msg not allowed in CheckTx", sdkerrors.ErrInvalidRequest)
		}

		if ctx.IsReCheckTx() {
			return ctx, fmt.Errorf("%w: internal msg not allowed in ReCheckTx", sdkerrors.ErrInvalidRequest)
		}

		if simulate {
			return ctx, fmt.Errorf("%w: internal msg not allowed in simulation", sdkerrors.ErrInvalidRequest)
		}

		switch ctx.ExecMode() {
		case sdk.ExecModePrepareProposal:
			return ctx, fmt.Errorf("%w: internal msg not allowed in prepare proposal", sdkerrors.ErrInvalidRequest)
		case sdk.ExecModeProcessProposal:
			return ctx, fmt.Errorf("%w: internal msg not allowed in process proposal", sdkerrors.ErrInvalidRequest)
		case sdk.ExecModeReCheck:
			return ctx, fmt.Errorf("%w: internal msg not allowed in recheck mode", sdkerrors.ErrInvalidRequest)
		case sdk.ExecModeVoteExtension:
			return ctx, fmt.Errorf("%w: internal msg not allowed in vote extension mode", sdkerrors.ErrInvalidRequest)
		case sdk.ExecModeVerifyVoteExtension:
			return ctx, fmt.Errorf("%w: internal msg not allowed in verify vote extension mode", sdkerrors.ErrInvalidRequest)
		case sdk.ExecModeFinalize:
			return ctx, fmt.Errorf("%w: internal msg not allowed in finalize mode", sdkerrors.ErrInvalidRequest)
		}
	}

	return next(ctx, tx, simulate)
}
