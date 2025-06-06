package types

import (
	"context"

	lc "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
)

type BTCLightClientKeeper interface {
	GetTipInfo(ctx context.Context) *lc.BTCHeaderInfo
	GetBaseBTCHeader(ctx context.Context) *lc.BTCHeaderInfo
}
