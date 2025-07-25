package types

import (
	"encoding/hex"
	"fmt"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"

	bbn "github.com/babylonlabs-io/babylon/v4/types"
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
	btcDel *BTCDelegation,
) *EventBTCDelegationCreated {
	e := &EventBTCDelegationCreated{
		StakingTxHex:              hex.EncodeToString(btcDel.StakingTx),
		StakingOutputIndex:        strconv.FormatUint(uint64(btcDel.StakingOutputIdx), 10),
		ParamsVersion:             strconv.FormatUint(uint64(btcDel.ParamsVersion), 10),
		FinalityProviderBtcPksHex: btcDel.FinalityProviderKeys(),
		StakerBtcPkHex:            btcDel.BtcPk.MarshalHex(),
		StakingTime:               strconv.FormatUint(uint64(btcDel.StakingTime), 10),
		UnbondingTime:             strconv.FormatUint(uint64(btcDel.UnbondingTime), 10),
		UnbondingTx:               hex.EncodeToString(btcDel.BtcUndelegation.UnbondingTx),
		NewState:                  BTCDelegationStatus_PENDING.String(),
		StakerAddr:                btcDel.StakerAddr,
	}
	if btcDel.IsStakeExpansion() {
		e.PreviousStakingTxHashHex = btcDel.MustGetStakeExpansionTxHash().String()
	}
	return e
}

func NewCovenantSignatureReceivedEvent(
	btcDel *BTCDelegation,
	covPK *bbn.BIP340PubKey,
	unbondingTxSig *bbn.BIP340Signature,
	stakeExpansionTxSig *bbn.BIP340Signature,
) *EventCovenantSignatureReceived {
	var stakeExpansionTxSigHex string

	if btcDel.IsStakeExpansion() {
		stakeExpansionTxSigHex = stakeExpansionTxSig.ToHexStr()
	}

	return &EventCovenantSignatureReceived{
		StakingTxHash:                      btcDel.MustGetStakingTxHash().String(),
		CovenantBtcPkHex:                   covPK.MarshalHex(),
		CovenantUnbondingSignatureHex:      unbondingTxSig.ToHexStr(),
		CovenantStakeExpansionSignatureHex: stakeExpansionTxSigHex,
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
	startHeight uint32,
) *EventBTCDelgationUnbondedEarly {
	return &EventBTCDelgationUnbondedEarly{
		StakingTxHash: stakingTxHash,
		StartHeight:   strconv.FormatUint(uint64(startHeight), 10),
		NewState:      BTCDelegationStatus_UNBONDED.String(),
	}
}

func NewExpiredDelegationEvent(
	stakingTxHash string,
) *EventBTCDelegationExpired {
	return &EventBTCDelegationExpired{
		StakingTxHash: stakingTxHash,
		NewState:      BTCDelegationStatus_EXPIRED.String(),
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

func NewUnexpectedUnbondingTxEvent(
	stakingTxHash, spendStakeTxHash, spendStakeTxHeaderHash string,
	spendStakeTxBlockIndex uint32,
) *EventUnexpectedUnbondingTx {
	return &EventUnexpectedUnbondingTx{
		StakingTxHash:          stakingTxHash,
		SpendStakeTxHash:       spendStakeTxHash,
		SpendStakeTxHeaderHash: spendStakeTxHeaderHash,
		SpendStakeTxBlockIndex: spendStakeTxBlockIndex,
	}
}

func NewStakeExpansionActivatedEvent(
	previousStakingTxHash, stakeExpansionTxHash, stakeExpansionTxHeaderHash string,
	stakeExpansionTxBlockIndex uint32,
) *EventStakeExpansionActivated {
	return &EventStakeExpansionActivated{
		PreviousStakingTxHash:      previousStakingTxHash,
		StakeExpansionTxHash:       stakeExpansionTxHash,
		StakeExpansionTxHeaderHash: stakeExpansionTxHeaderHash,
		StakeExpansionTxBlockIndex: stakeExpansionTxBlockIndex,
	}
}

// EmitUnexpectedUnbondingTxEvent emits events for an unexpected unbonding tx
func EmitUnexpectedUnbondingTxEvent(
	sdkCtx sdk.Context,
	stakingTxHash, spendStakeTxHash, spendStakeTxHeaderHash string,
	spendStakeTxBlockIndex uint32,
) {
	ev := NewUnexpectedUnbondingTxEvent(stakingTxHash, spendStakeTxHash, spendStakeTxHeaderHash, spendStakeTxBlockIndex)
	if err := sdkCtx.EventManager().EmitTypedEvent(ev); err != nil {
		panic(fmt.Errorf("failed to emit event the unexpected unbonding tx event: %w", err))
	}
}

// EmitStakeExpansionActivatedEvent emits events for a stake expansion activation
func EmitStakeExpansionActivatedEvent(
	sdkCtx sdk.Context,
	previousStakingTxHash, stakeExpansionTxHash, stakeExpansionTxHeaderHash string,
	stakeExpansionTxBlockIndex uint32,
) {
	ev := NewStakeExpansionActivatedEvent(previousStakingTxHash, stakeExpansionTxHash, stakeExpansionTxHeaderHash, stakeExpansionTxBlockIndex)
	if err := sdkCtx.EventManager().EmitTypedEvent(ev); err != nil {
		panic(fmt.Errorf("failed to emit stake expansion activated event: %w", err))
	}
}

// EmitEarlyUnbondedEvent emits events for an early unbonded BTC delegation
func EmitEarlyUnbondedEvent(sdkCtx sdk.Context, stakingTxHash string, inclusionHeight uint32) {
	ev := NewDelegationUnbondedEarlyEvent(stakingTxHash, inclusionHeight)
	if err := sdkCtx.EventManager().EmitTypedEvent(ev); err != nil {
		panic(fmt.Errorf("failed to emit event the early unbonded BTC delegation: %w", err))
	}
}

// EmitExpiredDelegationEvent emits events for an expired delegation
func EmitExpiredDelegationEvent(sdkCtx sdk.Context, stakingTxHash string) {
	ev := NewExpiredDelegationEvent(stakingTxHash)
	if err := sdkCtx.EventManager().EmitTypedEvent(ev); err != nil {
		panic(fmt.Errorf("failed to emit event the expired BTC delegation: %w", err))
	}
}

func EmitSlashedFPEvent(sdkCtx sdk.Context, fpBTCPK *bbn.BIP340PubKey) {
	statusChangeEvent := NewFinalityProviderStatusChangeEvent(fpBTCPK, FinalityProviderStatus_FINALITY_PROVIDER_STATUS_SLASHED)
	if err := sdkCtx.EventManager().EmitTypedEvent(statusChangeEvent); err != nil {
		panic(fmt.Errorf(
			"failed to emit FinalityProviderStatusChangeEvent with status %s: %w",
			FinalityProviderStatus_FINALITY_PROVIDER_STATUS_SLASHED.String(), err))
	}
}

func EmitJailedFPEvent(sdkCtx sdk.Context, fpBTCPK *bbn.BIP340PubKey) {
	statusChangeEvent := NewFinalityProviderStatusChangeEvent(fpBTCPK, FinalityProviderStatus_FINALITY_PROVIDER_STATUS_JAILED)
	if err := sdkCtx.EventManager().EmitTypedEvent(statusChangeEvent); err != nil {
		panic(fmt.Errorf(
			"failed to emit FinalityProviderStatusChangeEvent with status %s: %w",
			FinalityProviderStatus_FINALITY_PROVIDER_STATUS_JAILED.String(), err))
	}
}
