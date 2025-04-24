package keeper

import (
	"github.com/babylonlabs-io/babylon/v2/x/btcstaking/types"
)

var _ types.QueryServer = Keeper{}
