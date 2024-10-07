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

func NewEventPowerDistUpdateWithJailedFP(fpBTCPK *bbn.BIP340PubKey) *EventPowerDistUpdate {
	return &EventPowerDistUpdate{
		Ev: &EventPowerDistUpdate_JailedFp{
			JailedFp: &EventPowerDistUpdate_EventJailedFinalityProvider{
				Pk: fpBTCPK,
			},
		},
	}
}

func NewEventPowerDistUpdateWithUnjailedFP(fpBTCPK *bbn.BIP340PubKey) *EventPowerDistUpdate {
	return &EventPowerDistUpdate{
		Ev: &EventPowerDistUpdate_UnjailedFp{
			UnjailedFp: &EventPowerDistUpdate_EventUnjailedFinalityProvider{
				Pk: fpBTCPK,
			},
		},
	}
}

func NewEventFinalityProviderCreated(fp *FinalityProvider) *EventFinalityProviderCreated {
	return &EventFinalityProviderCreated{
		BtcPk:       fp.BtcPk,
		Addr:        fp.Addr,
		Commission:  fp.Commission,
		Description: fp.Description,
	}
}

func NewEventFinalityProviderEdited(fp *FinalityProvider) *EventFinalityProviderEdited {
	return &EventFinalityProviderEdited{
		BtcPk:       fp.BtcPk,
		Commission:  fp.Commission,
		Description: fp.Description,
	}
}
