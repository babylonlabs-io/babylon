package types

import sdk "github.com/cosmos/cosmos-sdk/types"

// TODO: Rethink how to check whether an fp is a Babylon Genesis fp or not.
// Checking through the ChainID set by context is very brittle and will
// certainly lead to issues down the road.
func (fp *FinalityProvider) SecuresBabylonGenesis(ctx sdk.Context) bool {
	return fp.BsnId == ctx.ChainID()
}
