package types

import (
	"encoding/hex"

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

func NewInclusionProofEvent(
	stakingTxHash string,
	startHeight uint64,
	endHeight uint64,
) *EventBTCDelegationInclusionProofReceived {
	return &EventBTCDelegationInclusionProofReceived{
		StakingTxHash: stakingTxHash,
		StartHeight:   startHeight,
		EndHeight:     endHeight,
	}
}

func NewBtcDelCreationEvent(
	stakingTxHash string,
	btcDel *BTCDelegation,
) *EventBTCDelegationCreated {
	return &EventBTCDelegationCreated{
		StakingTxHash:             stakingTxHash,
		ParamsVersion:             btcDel.ParamsVersion,
		FinalityProviderBtcPksHex: btcDel.FinalityProviderKeys(),
		StakerBtcPkHex:            btcDel.BtcPk.MarshalHex(),
		StakingTime:               btcDel.StakingTime,
		StakingAmount:             btcDel.TotalSat,
		UnbondingTime:             btcDel.UnbondingTime,
		UnbondingTx:               hex.EncodeToString(btcDel.BtcUndelegation.UnbondingTx),
	}
}

func NewCovenantSignatureReceivedEvent(
	btcDel *BTCDelegation,
	covPK *bbn.BIP340PubKey,
	unbondingTxSig *bbn.BIP340Signature,
) *EventCovenantSignatureRecevied {
	return &EventCovenantSignatureRecevied{
		StakingTxHash:                 btcDel.MustGetStakingTxHash().String(),
		CovenantBtcPkHex:              covPK.MarshalHex(),
		CovenantUnbondingSignatureHex: unbondingTxSig.ToHexStr(),
	}
}
