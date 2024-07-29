package keeper

import (
	"github.com/babylonlabs-io/babylon/x/finality/types"
)

var _ types.QueryServer = Keeper{}
