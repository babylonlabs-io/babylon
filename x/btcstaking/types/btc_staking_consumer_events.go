package types

import "fmt"

func CreateNewFinalityProviderEvent(fp *FinalityProvider) *BTCStakingConsumerEvent {
	event := &BTCStakingConsumerEvent{
		Event: &BTCStakingConsumerEvent_NewFp{
			NewFp: &NewFinalityProvider{
				Description: fp.Description,
				Commission:  fp.Commission.String(),
				Addr:        fp.Addr,
				BtcPkHex:    fp.BtcPk.MarshalHex(),
				Pop:         fp.Pop,
				ConsumerId:  fp.ConsumerId,
			},
		},
	}

	return event
}

func CreateActiveBTCDelegationEvent(activeDel *BTCDelegation) (*BTCStakingConsumerEvent, error) {
	fpBtcPkHexList := make([]string, len(activeDel.FpBtcPkList))
	for i, fpBtcPk := range activeDel.FpBtcPkList {
		fpBtcPkHexList[i] = fpBtcPk.MarshalHex()
	}

	slashingTxBytes, err := activeDel.SlashingTx.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal SlashingTx: %w", err)
	}

	delegatorSlashingSigBytes, err := activeDel.DelegatorSig.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal DelegatorSig: %w", err)
	}

	if activeDel.BtcUndelegation.DelegatorUnbondingSig != nil {
		return nil, fmt.Errorf("unexpected DelegatorUnbondingSig in active delegation")
	}

	unbondingSlashingTxBytes, err := activeDel.BtcUndelegation.SlashingTx.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal BtcUndelegation.SlashingTx: %w", err)
	}

	delegatorUnbondingSlashingSigBytes, err := activeDel.BtcUndelegation.DelegatorSlashingSig.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal BtcUndelegation.DelegatorSlashingSig: %w", err)
	}

	event := &BTCStakingConsumerEvent{
		Event: &BTCStakingConsumerEvent_ActiveDel{
			ActiveDel: &ActiveBTCDelegation{
				BtcPkHex:             activeDel.BtcPk.MarshalHex(),
				FpBtcPkList:          fpBtcPkHexList,
				StartHeight:          activeDel.StartHeight,
				EndHeight:            activeDel.EndHeight,
				TotalSat:             activeDel.TotalSat,
				StakingTx:            activeDel.StakingTx,
				SlashingTx:           slashingTxBytes,
				DelegatorSlashingSig: delegatorSlashingSigBytes,
				CovenantSigs:         activeDel.CovenantSigs,
				StakingOutputIdx:     activeDel.StakingOutputIdx,
				UnbondingTime:        activeDel.UnbondingTime,
				UndelegationInfo: &BTCUndelegationInfo{
					UnbondingTx:              activeDel.BtcUndelegation.UnbondingTx,
					SlashingTx:               unbondingSlashingTxBytes,
					DelegatorSlashingSig:     delegatorUnbondingSlashingSigBytes,
					CovenantUnbondingSigList: activeDel.BtcUndelegation.CovenantUnbondingSigList,
					CovenantSlashingSigs:     activeDel.BtcUndelegation.CovenantSlashingSigs,
				},
				ParamsVersion: activeDel.ParamsVersion,
			},
		},
	}

	return event, nil
}

func CreateUnbondedBTCDelegationEvent(unbondedDel *BTCDelegation) (*BTCStakingConsumerEvent, error) {
	if unbondedDel.BtcUndelegation.DelegatorUnbondingSig == nil {
		return nil, fmt.Errorf("missing DelegatorUnbondingSig in unbonded delegation")
	}

	unbondingTxSigBytes, err := unbondedDel.BtcUndelegation.DelegatorUnbondingSig.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal DelegatorUnbondingSig: %w", err)
	}

	event := &BTCStakingConsumerEvent{
		Event: &BTCStakingConsumerEvent_UnbondedDel{
			UnbondedDel: &UnbondedBTCDelegation{
				StakingTxHash:  unbondedDel.MustGetStakingTxHash().String(),
				UnbondingTxSig: unbondingTxSigBytes,
			},
		},
	}

	return event, nil
}
