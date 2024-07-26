package types

import (
	bbntypes "github.com/babylonlabs-io/babylon/types"
)

// NewFinalityProviderSigningInfo creates a new FinalityProviderSigningInfo instance
func NewFinalityProviderSigningInfo(
	fpPk *bbntypes.BIP340PubKey, startHeight, missedBlocksCounter int64,
) FinalityProviderSigningInfo {
	return FinalityProviderSigningInfo{
		FpBtcPk:             fpPk,
		StartHeight:         startHeight,
		MissedBlocksCounter: missedBlocksCounter,
	}
}

func (si *FinalityProviderSigningInfo) IncrementMissedBlocksCounter() {
	si.MissedBlocksCounter++
}

func (si *FinalityProviderSigningInfo) DecrementMissedBlocksCounter() {
	si.MissedBlocksCounter--
}

func (si *FinalityProviderSigningInfo) ResetMissedBlocksCounter() {
	si.MissedBlocksCounter = 0
}
