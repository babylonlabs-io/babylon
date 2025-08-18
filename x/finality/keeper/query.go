package keeper

import (
	"github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

var _ types.QueryServer = Keeper{}
