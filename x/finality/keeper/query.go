package keeper

import (
	"github.com/babylonlabs-io/babylon/v3/x/finality/types"
)

var _ types.QueryServer = Keeper{}
