package keeper

import (
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

var _ types.QueryServer = Keeper{}
