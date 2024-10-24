package types

import (
	"sort"

	sdkmath "cosmossdk.io/math"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func NewVotingPowerDistCache() *VotingPowerDistCache {
	return &VotingPowerDistCache{
		TotalBondedSat:    0,
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

	return newInactiveFps
}

// ApplyActiveFinalityProviders sorts all finality providers, counts the total voting
// power of top N finality providers, excluding those who don't have timestamped pub rand
// and records them in cache
func (dc *VotingPowerDistCache) ApplyActiveFinalityProviders(maxActiveFPs uint32) {
	// sort finality providers with timestamping considered
	SortFinalityProvidersWithZeroedVotingPower(dc.FinalityProviders)

	numActiveFPs := uint32(0)

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
		numActiveFPs++
	}

	TotalBondedSat := uint64(0)

	for i := uint32(0); i < numActiveFPs; i++ {
		TotalBondedSat += dc.FinalityProviders[i].TotalBondedSat
	}

	dc.TotalBondedSat = TotalBondedSat
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
// i.e., not within top N of them in terms of voting power and not slashed or jailed
func (dc *VotingPowerDistCache) GetInactiveFinalityProviderSet() map[string]*FinalityProviderDistInfo {
	numActiveFPs := dc.NumActiveFps

	if len(dc.FinalityProviders) <= int(numActiveFPs) {
		return nil
	}

	inactiveFps := make(map[string]*FinalityProviderDistInfo)

	for _, fp := range dc.FinalityProviders[numActiveFPs:] {
		if !fp.IsSlashed && !fp.IsJailed {
			inactiveFps[fp.BtcPk.MarshalHex()] = fp
		}
	}

	return inactiveFps
}

// FilterVotedDistCache filters out a voting power distribution cache
// with finality providers that have voted according to a map of given
// voters, and their total voted power.
func (dc *VotingPowerDistCache) FilterVotedDistCache(voterBTCPKs map[string]struct{}) *VotingPowerDistCache {
	activeFPs := dc.GetActiveFinalityProviderSet()
	var filteredFps []*FinalityProviderDistInfo
	TotalBondedSat := uint64(0)
	for k, v := range activeFPs {
		if _, ok := voterBTCPKs[k]; ok {
			filteredFps = append(filteredFps, v)
			TotalBondedSat += v.TotalBondedSat
		}
	}

	return &VotingPowerDistCache{
		FinalityProviders: filteredFps,
		TotalBondedSat:    TotalBondedSat,
	}
}

// GetFinalityProviderPortion returns the portion of a finality provider's voting power out of the total voting power
func (dc *VotingPowerDistCache) GetFinalityProviderPortion(v *FinalityProviderDistInfo) sdkmath.LegacyDec {
	return sdkmath.LegacyNewDec(int64(v.TotalBondedSat)).QuoTruncate(sdkmath.LegacyNewDec(int64(dc.TotalBondedSat)))
}

func NewFinalityProviderDistInfo(fp *bstypes.FinalityProvider) *FinalityProviderDistInfo {
	return &FinalityProviderDistInfo{
		BtcPk:          fp.BtcPk,
		Addr:           fp.Addr,
		Commission:     fp.Commission,
		TotalBondedSat: 0,
		BtcDels:        []*BTCDelDistInfo{},
	}
}

func (v *FinalityProviderDistInfo) GetAddress() sdk.AccAddress {
	return sdk.MustAccAddressFromBech32(v.Addr)
}

func (v *FinalityProviderDistInfo) AddBTCDel(btcDel *bstypes.BTCDelegation) {
	btcDelDistInfo := &BTCDelDistInfo{
		BtcPk:         btcDel.BtcPk,
		StakerAddr:    btcDel.StakerAddr,
		StakingTxHash: btcDel.MustGetStakingTxHash().String(),
		TotalSat:      btcDel.TotalSat,
	}
	v.BtcDels = append(v.BtcDels, btcDelDistInfo)
	v.TotalBondedSat += btcDelDistInfo.TotalSat
}

func (v *FinalityProviderDistInfo) AddBTCDelDistInfo(d *BTCDelDistInfo) {
	v.BtcDels = append(v.BtcDels, d)
	v.TotalBondedSat += d.TotalSat
}

// GetBTCDelPortion returns the portion of a BTC delegation's voting power out of
// the finality provider's total voting power
func (v *FinalityProviderDistInfo) GetBTCDelPortion(d *BTCDelDistInfo) sdkmath.LegacyDec {
	return sdkmath.LegacyNewDec(int64(d.TotalSat)).QuoTruncate(sdkmath.LegacyNewDec(int64(v.TotalBondedSat)))
}

func (d *BTCDelDistInfo) GetAddress() sdk.AccAddress {
	return sdk.MustAccAddressFromBech32(d.StakerAddr)
}

// SortFinalityProvidersWithZeroedVotingPower sorts the finality providers slice,
// from higher to lower voting power. In the following cases, the voting power
// is treated as zero:
// 1. IsTimestamped is false
// 2. IsJailed is true
func SortFinalityProvidersWithZeroedVotingPower(fps []*FinalityProviderDistInfo) {
	sort.SliceStable(fps, func(i, j int) bool {
		iShouldBeZeroed := fps[i].IsJailed || !fps[i].IsTimestamped
		jShouldBeZeroed := fps[j].IsJailed || !fps[j].IsTimestamped

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
