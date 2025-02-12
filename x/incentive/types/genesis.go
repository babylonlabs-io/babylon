package types

import (
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

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

func (bse BTCStakingGaugeEntry) Validate() error {
	if bse.Gauge == nil {
		return fmt.Errorf("BTC staking gauge at height %d has nil Gauge", bse.Height)
	}
	if err := bse.Gauge.Validate(); err != nil {
		return fmt.Errorf("invalid BTC staking gauge at height %d: %w", bse.Height, err)
	}
	return nil
}

func (rge RewardGaugeEntry) Validate() error {
	if rge.Address == "" {
		return fmt.Errorf("reward gauge entry has empty address")
	}

	if _, err := sdk.AccAddressFromBech32(rge.Address); err != nil {
		return fmt.Errorf("invalid address: %s, error: %w", rge.Address, err)
	}

	if err := rge.StakeholderType.Validate(); err != nil {
		return fmt.Errorf("invalid stakeholder type for address %s: %w", rge.Address, err)
	}

	if rge.RewardGauge == nil {
		return fmt.Errorf("reward gauge for address %s is nil", rge.Address)
	}

	if err := rge.RewardGauge.Validate(); err != nil {
		return fmt.Errorf("invalid reward gauge for address %s: %w", rge.Address, err)
	}
	return nil
}

func validateBTCStakingGauges(entries []BTCStakingGaugeEntry) error {
	heightMap := make(map[uint64]bool) // To check for duplicate heights
	for _, entry := range entries {
		if _, exists := heightMap[entry.Height]; exists {
			return fmt.Errorf("duplicate BTC staking gauge for height: %d", entry.Height)
		}
		heightMap[entry.Height] = true

		if err := entry.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func validateRewardGauges(entries []RewardGaugeEntry) error {
	addressMap := make(map[string]bool) // To check for duplicate addresses
	for _, entry := range entries {
		if _, exists := addressMap[entry.Address]; exists {
			return fmt.Errorf("duplicate reward gauge for address: %s", entry.Address)
		}
		addressMap[entry.Address] = true

		if err := entry.Validate(); err != nil {
			return err
		}
	}
	return nil
}
