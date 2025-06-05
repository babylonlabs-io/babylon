package types

import (
	"fmt"
	"time"

	bbntypes "github.com/babylonlabs-io/babylon/v2/types"
	bstypes "github.com/babylonlabs-io/babylon/v2/x/btcstaking/types"
)

// NewFinalityProviderSigningInfo creates a new FinalityProviderSigningInfo instance
func NewFinalityProviderSigningInfo(
	fpPk *bbntypes.BIP340PubKey, startHeight, missedBlocksCounter int64,
) FinalityProviderSigningInfo {
	return FinalityProviderSigningInfo{
		FpBtcPk:             fpPk,
		StartHeight:         startHeight,
		MissedBlocksCounter: missedBlocksCounter,
		JailedUntil:         time.Unix(0, 0).UTC(),
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

func (fpsi FinalityProviderSigningInfo) Validate() error {
	if fpsi.FpBtcPk == nil {
		return fmt.Errorf("invalid signing info. empty finality provider BTC public key")
	}
	if fpsi.FpBtcPk.Size() != bbntypes.BIP340PubKeyLen {
		return fmt.Errorf("invalid signing info. finality provider BTC public key length: got %d, want %d", fpsi.FpBtcPk.Size(), bbntypes.BIP340PubKeyLen)
	}
	if fpsi.StartHeight < 0 {
		return fmt.Errorf("invalid start height")
	}
	if fpsi.MissedBlocksCounter < 0 {
		return fmt.Errorf("invalid missed blocks counter")
	}
	return nil
}
