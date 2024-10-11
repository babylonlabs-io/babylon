package types

import (
	"encoding/hex"
	"strconv"

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
		BtcPkHex:        fp.BtcPk.MarshalHex(),
		Addr:            fp.Addr,
		Commission:      fp.Commission.String(),
		Moniker:         fp.Description.Moniker,
		Identity:        fp.Description.Identity,
		Website:         fp.Description.Website,
		SecurityContact: fp.Description.SecurityContact,
		Details:         fp.Description.Details,
	}
}

func NewEventFinalityProviderEdited(fp *FinalityProvider) *EventFinalityProviderEdited {
	return &EventFinalityProviderEdited{
		BtcPkHex:        fp.BtcPk.MarshalHex(),
		Commission:      fp.Commission.String(),
		Moniker:         fp.Description.Moniker,
		Identity:        fp.Description.Identity,
		Website:         fp.Description.Website,
		SecurityContact: fp.Description.SecurityContact,
		Details:         fp.Description.Details,
	}
}

func NewInclusionProofEvent(
	stakingTxHash string,
	startHeight uint32,
	endHeight uint32,
	state BTCDelegationStatus,
) *EventBTCDelegationInclusionProofReceived {
	return &EventBTCDelegationInclusionProofReceived{
		StakingTxHash: stakingTxHash,
		StartHeight:   strconv.FormatUint(uint64(startHeight), 10),
		EndHeight:     strconv.FormatUint(uint64(endHeight), 10),
		NewState:      state.String(),
	}
}

func NewBtcDelCreationEvent(
	stakingTxHash string,
	btcDel *BTCDelegation,
) *EventBTCDelegationCreated {
	return &EventBTCDelegationCreated{
		StakingTxHash:             stakingTxHash,
		ParamsVersion:             strconv.FormatUint(uint64(btcDel.ParamsVersion), 10),
		FinalityProviderBtcPksHex: btcDel.FinalityProviderKeys(),
		StakerBtcPkHex:            btcDel.BtcPk.MarshalHex(),
		StakingTime:               strconv.FormatUint(uint64(btcDel.StakingTime), 10),
		StakingAmount:             strconv.FormatUint(btcDel.TotalSat, 10),
		UnbondingTime:             strconv.FormatUint(uint64(btcDel.UnbondingTime), 10),
		UnbondingTx:               hex.EncodeToString(btcDel.BtcUndelegation.UnbondingTx),
		NewState:                  BTCDelegationStatus_PENDING.String(),
	}
}

func NewCovenantSignatureReceivedEvent(
	btcDel *BTCDelegation,
	covPK *bbn.BIP340PubKey,
	unbondingTxSig *bbn.BIP340Signature,
) *EventCovenantSignatureReceived {
	return &EventCovenantSignatureReceived{
		StakingTxHash:                 btcDel.MustGetStakingTxHash().String(),
		CovenantBtcPkHex:              covPK.MarshalHex(),
		CovenantUnbondingSignatureHex: unbondingTxSig.ToHexStr(),
	}
}

func NewCovenantQuorumReachedEvent(
	btcDel *BTCDelegation,
	state BTCDelegationStatus,
) *EventCovenantQuorumReached {
	return &EventCovenantQuorumReached{
		StakingTxHash: btcDel.MustGetStakingTxHash().String(),
		NewState:      state.String(),
	}
}

func NewDelegationUnbondedEarlyEvent(
	stakingTxHash string,
) *EventBTCDelgationUnbondedEarly {
	return &EventBTCDelgationUnbondedEarly{
		StakingTxHash: stakingTxHash,
		NewState:      BTCDelegationStatus_UNBONDED.String(),
	}
}

func NewExpiredDelegationEvent(
	stakingTxHash string,
) *EventBTCDelegationExpired {
	return &EventBTCDelegationExpired{
		StakingTxHash: stakingTxHash,
		NewState:      BTCDelegationStatus_UNBONDED.String(),
	}
}

func NewFinalityProviderStatusChangeEvent(
	fpPk *bbn.BIP340PubKey,
	status FinalityProviderStatus,
) *EventFinalityProviderStatusChange {
	return &EventFinalityProviderStatusChange{
		BtcPk:    fpPk.MarshalHex(),
		NewState: status.String(),
	}
}
