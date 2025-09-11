package types

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type BTCLightClientHooks interface {
	AfterBTCRollBack(ctx context.Context, rollbackFrom, rollbackTo *BTCHeaderInfo) // Must be called after the chain is rolled back
	AfterBTCRollForward(ctx context.Context, headerInfo *BTCHeaderInfo)            // Must be called after the chain is rolled forward
	AfterBTCHeaderInserted(ctx context.Context, headerInfo *BTCHeaderInfo)         // Must be called after a header is inserted
}

type IncentiveKeeper interface {
	IndexRefundableMsg(ctx context.Context, msg sdk.Msg)
	IncRefundableMsgCount()
}
