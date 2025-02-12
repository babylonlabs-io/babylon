package types

import "fmt"

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:           DefaultParams(),
		BtcStakingGauges: nil,
		RewardGauges:     nil,
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	if err := validateBTCStakingGauges(gs.BtcStakingGauges); err != nil {
		return fmt.Errorf("invalid BTC staking gauges: %w", err)
	}

	if err := validateRewardGauges(gs.RewardGauges); err != nil {
		return fmt.Errorf("invalid reward gauges: %w", err)
	}

	return gs.Params.Validate()
}

func validateBTCStakingGauges(entries []BTCStakingGaugeEntry) error {
	heightMap := make(map[uint64]bool) // To check for duplicate heights
	for _, entry := range entries {
		if entry.Height == 0 {
			return fmt.Errorf("BTC staking gauge has invalid height: %d", entry.Height)
		}
		if _, exists := heightMap[entry.Height]; exists {
			return fmt.Errorf("duplicate BTC staking gauge for height: %d", entry.Height)
		}
		heightMap[entry.Height] = true

		if entry.Gauge == nil {
			return fmt.Errorf("BTC staking gauge at height %d has nil Gauge", entry.Height)
		}

		if err := entry.Gauge.Validate(); err != nil {
			return fmt.Errorf("invalid BTC staking gauge at height %d: %w", entry.Height, err)
		}
	}
	return nil
}

func validateRewardGauges(entries []RewardGaugeEntry) error {
	addressMap := make(map[string]bool) // To check for duplicate addresses
	for _, entry := range entries {
		if entry.Address == "" {
			return fmt.Errorf("reward gauge entry has empty address")
		}
		if _, exists := addressMap[entry.Address]; exists {
			return fmt.Errorf("duplicate reward gauge for address: %s", entry.Address)
		}
		addressMap[entry.Address] = true

		if err := entry.StakeholderType.Validate(); err != nil {
			return fmt.Errorf("invalid stakeholder type for address %s: %w", entry.Address, err)
		}

		if entry.RewardGauge == nil {
			return fmt.Errorf("reward gauge for address %s is nil", entry.Address)
		}

		if err := entry.RewardGauge.Validate(); err != nil {
			return fmt.Errorf("invalid reward gauge for address %s: %w", entry.Address, err)
		}
	}
	return nil
}
