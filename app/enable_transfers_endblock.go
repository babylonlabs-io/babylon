package app

import (
	"fmt"
	ibctransferkeeper "github.com/cosmos/ibc-go/v8/modules/apps/transfer/keeper"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/bank/keeper"
)

// EnableTransfersHeight TODO: Set this to the height for mainnet once it gets closer to launch.
const EnableTransfersHeight = 10

type EnableTransfersEndBlock struct {
	bankKeeper     keeper.Keeper
	TransferKeeper ibctransferkeeper.Keeper
	targetHeight   int64
}

func NewTransferEndBlocker(
	bankKeeper keeper.Keeper,
	transferKeeper ibctransferkeeper.Keeper,
	targetHeight int64,
) *EnableTransfersEndBlock {
	return &EnableTransfersEndBlock{
		bankKeeper:     bankKeeper,
		TransferKeeper: transferKeeper,
		targetHeight:   targetHeight,
	}
}

func (h *EnableTransfersEndBlock) EndBlocker(ctx sdk.Context) error {
	if ctx.BlockHeight() == h.targetHeight {
		// Log that we're executing the custom logic
		ctx.Logger().Info(fmt.Sprintf("Executing custom EndBlocker logic at height %d", ctx.BlockHeight()))

		bankParams := h.bankKeeper.GetParams(ctx)
		bankParams.DefaultSendEnabled = true

		err := h.bankKeeper.SetParams(ctx, bankParams)
		if err != nil {
			return fmt.Errorf(fmt.Sprintf("Did not update bank params at height %d", ctx.BlockHeight()))
		}

		transferParams := h.TransferKeeper.GetParams(ctx)
		transferParams.SendEnabled = true
		transferParams.ReceiveEnabled = true

		h.TransferKeeper.SetParams(ctx, transferParams)

		ctx.Logger().Info(fmt.Sprintf("Updated transfer and bank parameters at height %d", ctx.BlockHeight()))
	}

	return nil
}
