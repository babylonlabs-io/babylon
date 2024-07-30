package types

import (
	bbn "github.com/babylonlabs-io/babylon/types"
)

func NewEventPowerDistUpdateWithBTCDel(ev *EventBTCDelegationStateUpdate) *EventPowerDistUpdate {
	return &EventPowerDistUpdate{
		Ev: &EventPowerDistUpdate_BtcDelStateUpdate{
			BtcDelStateUpdate: ev,
		},
	}
}

func NewEventPowerDistUpdateWithSlashedFP(fpBTCPK *bbn.BIP340PubKey) *EventPowerDistUpdate {
	return &EventPowerDistUpdate{
		Ev: &EventPowerDistUpdate_SlashedFp{
			SlashedFp: &EventPowerDistUpdate_EventSlashedFinalityProvider{
				Pk: fpBTCPK,
			},
		},
	}
}
