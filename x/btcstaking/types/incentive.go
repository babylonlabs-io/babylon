package types

import (
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func NewVotingPowerDistCache() *VotingPowerDistCache {
	return &VotingPowerDistCache{
		TotalVotingPower:  0,
		FinalityProviders: []*FinalityProviderDistInfo{},
	}
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

// ApplyActiveFinalityProviders sorts all finality providers, counts the total voting
// power of top N finality providers, excluding those who don't have timestamped pub rand
// and records them in cache
func (dc *VotingPowerDistCache) ApplyActiveFinalityProviders(maxActiveFPs uint32) {
	// sort finality providers with timestamping considered
	SortFinalityProvidersWithTimestamping(dc.FinalityProviders)

	numActiveFPs := uint32(0)

	// finality providers are in the descending order of voting power
	// and timestamped ones come in the last
	for _, fp := range dc.FinalityProviders {
		if numActiveFPs == maxActiveFPs {
			break
		}
		if fp.TotalVotingPower == 0 {
			break
		}
		if !fp.IsTimestamped {
			break
		}
		numActiveFPs++
	}

	totalVotingPower := uint64(0)

	for i := uint32(0); i < numActiveFPs; i++ {
		totalVotingPower += dc.FinalityProviders[i].TotalVotingPower
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

// FilterVotedDistCache filters out a voting power distribution cache
// with finality providers that have voted according to a map of given
// voters, and their total voted power.
func (dc *VotingPowerDistCache) FilterVotedDistCache(voterBTCPKs map[string]struct{}) *VotingPowerDistCache {
	activeFPs := dc.GetActiveFinalityProviderSet()
	var filteredFps []*FinalityProviderDistInfo
	totalVotingPower := uint64(0)
	for k, v := range activeFPs {
		if _, ok := voterBTCPKs[k]; ok {
			filteredFps = append(filteredFps, v)
			totalVotingPower += v.TotalVotingPower
		}
	}

	return &VotingPowerDistCache{
		FinalityProviders: filteredFps,
		TotalVotingPower:  totalVotingPower,
	}
}

// GetFinalityProviderPortion returns the portion of a finality provider's voting power out of the total voting power
func (dc *VotingPowerDistCache) GetFinalityProviderPortion(v *FinalityProviderDistInfo) sdkmath.LegacyDec {
	return sdkmath.LegacyNewDec(int64(v.TotalVotingPower)).QuoTruncate(sdkmath.LegacyNewDec(int64(dc.TotalVotingPower)))
}

func NewFinalityProviderDistInfo(fp *FinalityProvider) *FinalityProviderDistInfo {
	return &FinalityProviderDistInfo{
		BtcPk:            fp.BtcPk,
		Addr:             fp.Addr,
		Commission:       fp.Commission,
		TotalVotingPower: 0,
		BtcDels:          []*BTCDelDistInfo{},
	}
}

func (v *FinalityProviderDistInfo) GetAddress() sdk.AccAddress {
	return sdk.MustAccAddressFromBech32(v.Addr)
}

func (v *FinalityProviderDistInfo) AddBTCDel(btcDel *BTCDelegation) {
	btcDelDistInfo := &BTCDelDistInfo{
		BtcPk:         btcDel.BtcPk,
		StakerAddr:    btcDel.StakerAddr,
		StakingTxHash: btcDel.MustGetStakingTxHash().String(),
		VotingPower:   btcDel.TotalSat,
	}
	v.BtcDels = append(v.BtcDels, btcDelDistInfo)
	v.TotalVotingPower += btcDelDistInfo.VotingPower
}

func (v *FinalityProviderDistInfo) AddBTCDelDistInfo(d *BTCDelDistInfo) {
	v.BtcDels = append(v.BtcDels, d)
	v.TotalVotingPower += d.VotingPower
}

// GetBTCDelPortion returns the portion of a BTC delegation's voting power out of
// the finality provider's total voting power
func (v *FinalityProviderDistInfo) GetBTCDelPortion(d *BTCDelDistInfo) sdkmath.LegacyDec {
	return sdkmath.LegacyNewDec(int64(d.VotingPower)).QuoTruncate(sdkmath.LegacyNewDec(int64(v.TotalVotingPower)))
}

func (d *BTCDelDistInfo) GetAddress() sdk.AccAddress {
	return sdk.MustAccAddressFromBech32(d.StakerAddr)
}
