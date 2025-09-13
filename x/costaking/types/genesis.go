package types

import (
	"errors"
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/types"
)

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
	if err := gs.CurrentRewards.Validate(); err != nil {
		return fmt.Errorf("invalid current rewards: %w", err)
	}

	if err := types.ValidateEntries(gs.HistoricalRewards, func(e HistoricalRewardsEntry) uint64 {
		return e.Period
	}); err != nil {
		return fmt.Errorf("invalid historical rewards: %w", err)
	}

	if err := types.ValidateEntries(gs.CostakersRewardsTracker, func(e CostakerRewardsTrackerEntry) string {
		return e.CostakerAddress
	}); err != nil {
		return fmt.Errorf("invalid costakers rewards tracker: %w", err)
	}

	return gs.Params.Validate()
}

// Validate validates the CurrentRewardsEntry
func (cre CurrentRewardsEntry) Validate() error {
	if cre.Rewards == nil {
		return nil // empty current rewards is valid
	}
	return cre.Rewards.Validate()
}

// Validate validates the HistoricalRewardsEntry
func (hre HistoricalRewardsEntry) Validate() error {
	if hre.Rewards == nil {
		return fmt.Errorf("historical rewards at period %d is nil", hre.Period)
	}
	return hre.Rewards.Validate()
}

// Validate validates the CostakerRewardsTrackerEntry
func (crte CostakerRewardsTrackerEntry) Validate() error {
	if err := validateAddrStr(crte.CostakerAddress); err != nil {
		return fmt.Errorf("invalid costaker address: %w", err)
	}
	if crte.Tracker == nil {
		return fmt.Errorf("tracker for costaker address %s is nil", crte.CostakerAddress)
	}
	return crte.Tracker.Validate()
}

// validateAddrStr validates an address string
func validateAddrStr(addr string) error {
	if addr == "" {
		return errors.New("empty address")
	}
	if _, err := sdk.AccAddressFromBech32(addr); err != nil {
		return fmt.Errorf("invalid address: %s, error: %w", addr, err)
	}
	return nil
}

// SortData sorts slices to get a deterministic result on the tests
func SortData(gs *GenesisState) {
	sort.Slice(gs.HistoricalRewards, func(i, j int) bool {
		return gs.HistoricalRewards[i].Period < gs.HistoricalRewards[j].Period
	})

	sort.Slice(gs.CostakersRewardsTracker, func(i, j int) bool {
		return gs.CostakersRewardsTracker[i].CostakerAddress < gs.CostakersRewardsTracker[j].CostakerAddress
	})
}
