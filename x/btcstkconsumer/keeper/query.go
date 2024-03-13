package keeper

import (
	"github.com/babylonchain/babylon/x/btcstkconsumer/types"
)

var _ types.QueryServer = Keeper{}
