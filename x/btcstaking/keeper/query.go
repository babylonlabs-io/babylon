package keeper

import (
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

var _ types.QueryServer = Keeper{}
