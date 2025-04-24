package keeper

import (
	"github.com/babylonlabs-io/babylon/v2/x/finality/types"
)

var _ types.QueryServer = Keeper{}
