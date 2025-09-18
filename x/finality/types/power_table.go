package types

import (
	fmt "fmt"
	"sort"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

func NewVotingPowerDistCache() *VotingPowerDistCache {
	return &VotingPowerDistCache{
		TotalVotingPower:  0,
		FinalityProviders: []*FinalityProviderDistInfo{},
	}
}

func NewVotingPowerDistCacheWithFinalityProviders(fps []*FinalityProviderDistInfo) *VotingPowerDistCache {
	cache := NewVotingPowerDistCache()
	for _, fp := range fps {
		cache.AddFinalityProviderDistInfo(fp)
	}

	return cache
}

func (dc *VotingPowerDistCache) Empty() bool {
	return len(dc.FinalityProviders) == 0
}

func (dc *VotingPowerDistCache) AddFinalityProviderDistInfo(v *FinalityProviderDistInfo) {
	dc.FinalityProviders = append(dc.FinalityProviders, v)
}

func (dc *VotingPowerDistCache) ActiveFpsByBtcPk() map[string]struct{} {
	activeFpByBtcPk := make(map[string]struct{}, 0)
	for idx, fp := range dc.FinalityProviders {
		canBeActive := idx < int(dc.NumActiveFps)
		if fp.FpStatus(canBeActive) == bstypes.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_ACTIVE {
			activeFpByBtcPk[fp.BtcPk.MarshalHex()] = struct{}{}
		}
	}
	return activeFpByBtcPk
}

func (dc *VotingPowerDistCache) FindNewActiveFinalityProviders(prevDc *VotingPowerDistCache) []*FinalityProviderDistInfo {
	activeFps := dc.GetActiveFinalityProviderSet()
	prevActiveFps := prevDc.GetActiveFinalityProviderSet()
	newActiveFps := make([]*FinalityProviderDistInfo, 0)

	for pk, fp := range activeFps {
		_, exists := prevActiveFps[pk]
		if !exists {
			newActiveFps = append(newActiveFps, fp)
		}
	}

	return newActiveFps
}

func (dc *VotingPowerDistCache) FindNewInactiveFinalityProviders(prevDc *VotingPowerDistCache) []*FinalityProviderDistInfo {
	inactiveFps := dc.GetInactiveFinalityProviderSet()
	prevInactiveFps := prevDc.GetInactiveFinalityProviderSet()
	newInactiveFps := make([]*FinalityProviderDistInfo, 0)

	for pk, fp := range inactiveFps {
		_, exists := prevInactiveFps[pk]
		if !exists {
			newInactiveFps = append(newInactiveFps, fp)
		}
	}

	// Also check for finality providers that were active before but now
	// are not present in the current cache due to full unbonding.
	currFps := make(map[string]struct{})
	for _, fp := range dc.FinalityProviders {
		currFps[fp.BtcPk.MarshalHex()] = struct{}{}
	}
	prevActivePfs := prevDc.GetActiveFinalityProviderSet()
	for pk, fp := range prevActivePfs {
		_, exists := currFps[pk]
		if !exists {
			// This FP was active before, but now it is not present in the current cache
			// due to full unbonding, so we add it to the new inactive FP list.
			newInactiveFps = append(newInactiveFps, fp)
		}
	}

	return newInactiveFps
}

// ApplyActiveFinalityProviders sorts all finality providers, counts the total voting
// power of top N finality providers, excluding those who don't have timestamped pub rand
// and records them in cache
func (dc *VotingPowerDistCache) ApplyActiveFinalityProviders(maxActiveFPs uint32) {
	// sort finality providers with timestamping considered
	SortFinalityProvidersWithZeroedVotingPower(dc.FinalityProviders)

	numActiveFPs := uint32(0)
	totalVotingPower := uint64(0)

	// finality providers are in the descending order of voting power
	// and timestamped ones come in the last
	for _, fp := range dc.FinalityProviders {
		if numActiveFPs == maxActiveFPs {
			break
		}
		if fp.TotalBondedSat == 0 {
			break
		}
		if !fp.IsTimestamped {
			break
		}
		if fp.IsJailed {
			break
		}
		if fp.IsSlashed {
			break
		}

		numActiveFPs++
		totalVotingPower += fp.TotalBondedSat
	}

	dc.TotalVotingPower = totalVotingPower
	dc.NumActiveFps = numActiveFPs
}

// GetActiveFinalityProviderSet returns a set of active finality providers
// keyed by the hex string of the finality provider's BTC public key
// i.e., top N of them in terms of voting power
func (dc *VotingPowerDistCache) GetActiveFinalityProviderSet() map[string]*FinalityProviderDistInfo {
	numActiveFPs := dc.NumActiveFps

	activeFps := make(map[string]*FinalityProviderDistInfo)

	for _, fp := range dc.FinalityProviders[:numActiveFPs] {
		activeFps[fp.BtcPk.MarshalHex()] = fp
	}

	return activeFps
}

// GetInactiveFinalityProviderSet returns a set of inactive finality providers
// keyed by the hex string of the finality provider's BTC public key
func (dc *VotingPowerDistCache) GetInactiveFinalityProviderSet() map[string]*FinalityProviderDistInfo {
	numActiveFPs := dc.NumActiveFps

	if len(dc.FinalityProviders) <= int(numActiveFPs) {
		return nil
	}

	inactiveFps := make(map[string]*FinalityProviderDistInfo)

	for _, fp := range dc.FinalityProviders[numActiveFPs:] {
		inactiveFps[fp.BtcPk.MarshalHex()] = fp
	}

	return inactiveFps
}

func (dc VotingPowerDistCache) Validate() error {
	// check fps are unique and total voting power is correct
	var (
		accVP uint64
		fpMap = make(map[string]struct{})
	)

	SortFinalityProvidersWithZeroedVotingPower(dc.FinalityProviders)
	numActiveFPs := uint32(0)

	for _, fp := range dc.FinalityProviders {
		if _, exists := fpMap[fp.BtcPk.MarshalHex()]; exists {
			return fmt.Errorf("invalid voting power distribution cache. Duplicate finality provider entry with BTC PK %s", fp.BtcPk.MarshalHex())
		}
		fpMap[fp.BtcPk.MarshalHex()] = struct{}{}

		if err := fp.Validate(); err != nil {
			return err
		}

		// take only into account active finality providers
		if !fp.IsTimestamped {
			continue
		}
		if fp.IsJailed {
			continue
		}
		if fp.IsSlashed {
			continue
		}

		accVP += fp.TotalBondedSat
		numActiveFPs++
	}

	if dc.TotalVotingPower != accVP {
		return fmt.Errorf("invalid voting power distribution cache. Provided TotalVotingPower %d is different than FPs accumulated voting power %d", dc.TotalVotingPower, accVP)
	}

	if dc.NumActiveFps != numActiveFPs {
		return fmt.Errorf("invalid voting power distribution cache. NumActiveFps %d is higher than active FPs count %d", dc.NumActiveFps, numActiveFPs)
	}

	return nil
}

// NewFinalityProviderDistInfo loads the FinalityProviderDistInfo based on the fp data.
// Note: The IsTimestamped property is always set to false, as it is not possible to determine
// the timestamp without the tip height.
func NewFinalityProviderDistInfo(fp *bstypes.FinalityProvider) *FinalityProviderDistInfo {
	return &FinalityProviderDistInfo{
		BtcPk:          fp.BtcPk,
		Addr:           sdk.MustAccAddressFromBech32(fp.Addr),
		Commission:     fp.Commission,
		TotalBondedSat: 0,
		IsJailed:       fp.Jailed,
		IsSlashed:      fp.IsSlashed(),
		IsTimestamped:  false,
	}
}

func (v *FinalityProviderDistInfo) GetAddress() sdk.AccAddress {
	return v.Addr
}

func (v *FinalityProviderDistInfo) ChangeDeltaSats(fpDeltaSats int64) {
	switch {
	case fpDeltaSats > 0:
		v.AddBondedSats(uint64(fpDeltaSats))
	case fpDeltaSats < 0:
		satsToRemove := abs(fpDeltaSats)
		v.RemoveBondedSats(uint64(satsToRemove))
	}
}

func (v *FinalityProviderDistInfo) AddBondedSats(sats uint64) {
	v.TotalBondedSat += sats
}

func (v *FinalityProviderDistInfo) RemoveBondedSats(sats uint64) {
	// safeguard against underflow in total bonded satoshis
	if v.TotalBondedSat < sats {
		sats = v.TotalBondedSat
	}
	v.TotalBondedSat -= sats
}

// GetBTCDelPortion returns the portion of a BTC delegation's voting power out of
// the finality provider's total voting power
func (v *FinalityProviderDistInfo) GetBTCDelPortion(totalSatDelegation uint64) sdkmath.LegacyDec {
	return sdkmath.LegacyNewDec(int64(totalSatDelegation)).QuoTruncate(sdkmath.LegacyNewDec(int64(v.TotalBondedSat)))
}

func (fpdi FinalityProviderDistInfo) Validate() error {
	if fpdi.BtcPk == nil {
		return fmt.Errorf("invalid fp dist info. empty finality provider BTC public key")
	}
	if fpdi.BtcPk.Size() != bbn.BIP340PubKeyLen {
		return fmt.Errorf("invalid fp dist info. finality provider BTC public key length: got %d, want %d", fpdi.BtcPk.Size(), bbn.BIP340PubKeyLen)
	}

	if fpdi.Addr == nil {
		return fmt.Errorf("invalid fp dist info. empty finality provider address")
	}

	if _, err := sdk.AccAddressFromBech32(sdk.AccAddress(fpdi.Addr).String()); err != nil {
		return fmt.Errorf("invalid bech32 address: %w", err)
	}

	if fpdi.Commission == nil {
		return fmt.Errorf("invalid fp dist info. commission is nil")
	}

	if fpdi.Commission.LT(sdkmath.LegacyZeroDec()) {
		return fmt.Errorf("invalid fp dist info. commission is negative")
	}

	if fpdi.Commission.GT(sdkmath.LegacyOneDec()) {
		return fmt.Errorf("invalid fp dist info. commission is greater than 1")
	}
	return nil
}

// SortFinalityProvidersWithZeroedVotingPower sorts the finality providers slice,
// from higher to lower voting power. In the following cases, the voting power
// is treated as zero:
// 1. IsTimestamped is false
// 2. IsJailed is true
// 3. IsSlashed is true
func SortFinalityProvidersWithZeroedVotingPower(fps []*FinalityProviderDistInfo) {
	sort.SliceStable(fps, func(i, j int) bool {
		iShouldBeZeroed := fps[i].IsJailed || !fps[i].IsTimestamped || fps[i].IsSlashed
		jShouldBeZeroed := fps[j].IsJailed || !fps[j].IsTimestamped || fps[j].IsSlashed

		if iShouldBeZeroed && !jShouldBeZeroed {
			return false
		}

		if !iShouldBeZeroed && jShouldBeZeroed {
			return true
		}

		iPkHex, jPkHex := fps[i].BtcPk.MarshalHex(), fps[j].BtcPk.MarshalHex()

		if iShouldBeZeroed && jShouldBeZeroed {
			// Both have zeroed voting power, compare BTC public keys
			return iPkHex < jPkHex
		}

		// both voting power the same, compare BTC public keys
		if fps[i].TotalBondedSat == fps[j].TotalBondedSat {
			return iPkHex < jPkHex
		}

		return fps[i].TotalBondedSat > fps[j].TotalBondedSat
	})
}

// abs returns the absolute value of a signed integer.
// There's a corner case: int64 minimum
// value (-9223372036854775808) cannot be negated.
// For satoshi values in Bitcoin context, this
// overflow scenario is extremely unlikely since it
// would represent an impossibly large amount of Bitcoin
func abs(val int64) int64 {
	if val < 0 {
		return -val
	}
	return val
}
