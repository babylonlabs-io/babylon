package types

import btcstaking "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"

// NewFinalityProviderResponse creates a new finality provider response for Consumer registered FPs.
// Note that slashing info, voting power and height are zero, as these FPs are not active here
func NewFinalityProviderResponse(f *btcstaking.FinalityProvider) *FinalityProviderResponse {
	return &FinalityProviderResponse{
		Addr:                 f.Addr,
		Description:          f.Description,
		Commission:           f.Commission,
		BtcPk:                f.BtcPk,
		Pop:                  f.Pop,
		SlashedBabylonHeight: f.SlashedBabylonHeight,
		SlashedBtcHeight:     f.SlashedBtcHeight,
		ConsumerId:           f.ConsumerId,
	}
}
