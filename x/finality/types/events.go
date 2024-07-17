package types

import "github.com/babylonchain/babylon/types"

func NewEventSlashedFinalityProvider(evidence *Evidence) *EventSlashedFinalityProvider {
	return &EventSlashedFinalityProvider{
		Evidence: evidence,
	}
}

func NewEventSluggishFinalityProviderDetected(fpPk *types.BIP340PubKey) *EventSluggishFinalityProviderDetected {
	return &EventSluggishFinalityProviderDetected{PublicKey: fpPk.MarshalHex()}
}

func NewEventSluggishFinalityProviderReverted(fpPk *types.BIP340PubKey) *EventSluggishFinalityProviderReverted {
	return &EventSluggishFinalityProviderReverted{PublicKey: fpPk.MarshalHex()}
}
