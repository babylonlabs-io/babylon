package types

import (
	"time"

	bbntypes "github.com/babylonlabs-io/babylon/types"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
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

func (si *FinalityProviderSigningInfo) IsJailingPeriodPassed(curBlockTime time.Time) (bool, error) {
	if si.JailedUntil.IsZero() {
		return false, bstypes.ErrFpNotJailed
	}

	return si.JailedUntil.Before(curBlockTime), nil
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
