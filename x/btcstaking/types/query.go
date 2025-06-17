package types

import (
	"encoding/hex"
)

func delegatorUnbondingInfoToResponse(ui *DelegatorUnbondingInfo) *DelegatorUnbondingInfoResponse {
	var spendStakeTxHex = ""

	if len(ui.SpendStakeTx) > 0 {
		spendStakeTxHex = hex.EncodeToString(ui.SpendStakeTx)
	}

	return &DelegatorUnbondingInfoResponse{
		SpendStakeTxHex: spendStakeTxHex,
	}
}

// NewBTCDelegationResponse returns a new delegation response structure.
func NewBTCDelegationResponse(btcDel *BTCDelegation, status BTCDelegationStatus) (resp *BTCDelegationResponse) {
	resp = &BTCDelegationResponse{
		StakerAddr:           btcDel.StakerAddr,
		BtcPk:                btcDel.BtcPk,
		FpBtcPkList:          btcDel.FpBtcPkList,
		StakingTime:          btcDel.StakingTime,
		StartHeight:          btcDel.StartHeight,
		EndHeight:            btcDel.EndHeight,
		TotalSat:             btcDel.TotalSat,
		StakingTxHex:         hex.EncodeToString(btcDel.StakingTx),
		DelegatorSlashSigHex: btcDel.DelegatorSig.ToHexStr(),
		CovenantSigs:         btcDel.CovenantSigs,
		StakingOutputIdx:     btcDel.StakingOutputIdx,
		Active:               status == BTCDelegationStatus_ACTIVE,
		StatusDesc:           status.String(),
		UnbondingTime:        btcDel.UnbondingTime,
		UndelegationResponse: nil,
		ParamsVersion:        btcDel.ParamsVersion,
	}

	if btcDel.SlashingTx != nil {
		resp.SlashingTxHex = hex.EncodeToString(*btcDel.SlashingTx)
	}

	if btcDel.BtcUndelegation != nil {
		resp.UndelegationResponse = btcDel.BtcUndelegation.ToResponse()
	}

	if btcDel.IsStakeExpansion() {
		resp.PreviousStakingTxHashHex = btcDel.MustGetStakeExpansionTxHash().String()
	}

	return resp
}

// ToResponse parses an BTCUndelegation into BTCUndelegationResponse.
func (ud *BTCUndelegation) ToResponse() (resp *BTCUndelegationResponse) {
	resp = &BTCUndelegationResponse{
		UnbondingTxHex:           hex.EncodeToString(ud.UnbondingTx),
		CovenantUnbondingSigList: ud.CovenantUnbondingSigList,
		CovenantSlashingSigs:     ud.CovenantSlashingSigs,
	}

	if ud.DelegatorUnbondingInfo != nil {
		resp.DelegatorUnbondingInfoResponse = delegatorUnbondingInfoToResponse(ud.DelegatorUnbondingInfo)
	}
	if ud.SlashingTx != nil {
		resp.SlashingTxHex = ud.SlashingTx.ToHexStr()
	}
	if ud.DelegatorSlashingSig != nil {
		resp.DelegatorSlashingSigHex = ud.DelegatorSlashingSig.ToHexStr()
	}

	return resp
}

// NewFinalityProviderResponse creates a new finality provider response based on the finality provider
func NewFinalityProviderResponse(f *FinalityProvider, bbnBlockHeight uint64) *FinalityProviderResponse {
	return &FinalityProviderResponse{
		Description:          f.Description,
		Commission:           f.Commission,
		Addr:                 f.Addr,
		BtcPk:                f.BtcPk,
		Pop:                  f.Pop,
		SlashedBabylonHeight: f.SlashedBabylonHeight,
		SlashedBtcHeight:     f.SlashedBtcHeight,
		Jailed:               f.Jailed,
		Height:               bbnBlockHeight,
		HighestVotedHeight:   f.HighestVotedHeight,
		CommissionInfo:       f.CommissionInfo,
		ConsumerId:           f.ConsumerId,
	}
}
