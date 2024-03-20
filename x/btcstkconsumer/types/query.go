package types

import btcstaking "github.com/babylonchain/babylon/x/btcstaking/types"

// NewFinalityProviderResponse creates a new finality provider response for CZ registered FPs.
// Note that slashing info, voting power and height are zero, as these FPs are not active here
func NewFinalityProviderResponse(f *btcstaking.FinalityProvider) *FinalityProviderResponse {
	return &FinalityProviderResponse{
		Description:          f.Description,
		Commission:           f.Commission,
		BabylonPk:            f.BabylonPk,
		BtcPk:                f.BtcPk,
		Pop:                  f.Pop,
		SlashedBabylonHeight: f.SlashedBabylonHeight,
		SlashedBtcHeight:     f.SlashedBtcHeight,
		ChainId:              f.ChainId,
	}
}
