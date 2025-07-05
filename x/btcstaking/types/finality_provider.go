package types

import sdk "github.com/cosmos/cosmos-sdk/types"

func (fp *FinalityProvider) SecuresBabylonGenesis(ctx sdk.Context) bool {
	return fp.BsnId == ctx.ChainID()
}
