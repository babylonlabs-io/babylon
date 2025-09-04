package types

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:                  DefaultParams(),
		CurrentRewards:          CurrentRewardsEntry{},
		HistoricalRewards:       []HistoricalRewardsEntry{},
		CostakersRewardsTracker: []CostakerRewardsTrackerEntry{},
	}
}

func (gs GenesisState) Validate() error {
	return nil
}
