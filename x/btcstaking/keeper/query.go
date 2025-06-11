package keeper

import (
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
)

var _ types.QueryServer = Keeper{}
