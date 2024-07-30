package keeper

import (
	"github.com/babylonlabs-io/babylon/x/btcstkconsumer/types"
)

var _ types.QueryServer = Keeper{}
