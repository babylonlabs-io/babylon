package types

import (
	"errors"
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:            DefaultParams(),
		BtcStakingGauges:  []BTCStakingGaugeEntry{},
		RewardGauges:      []RewardGaugeEntry{},
		WithdrawAddresses: []WithdrawAddressEntry{},
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

	if err := validateWithdrawAddresses(gs.WithdrawAddresses); err != nil {
		return fmt.Errorf("invalid withdraw addresses: %w", err)
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
	if err := validateAddrStr(rge.Address); err != nil {
		return err
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

func (wa WithdrawAddressEntry) Validate() error {
	if err := validateAddrStr(wa.DelegatorAddress); err != nil {
		return fmt.Errorf("invalid delegator, error: %w", err)
	}
	if err := validateAddrStr(wa.WithdrawAddress); err != nil {
		return fmt.Errorf("invalid withdrawer, error: %w", err)
	}
	return nil
}

func validateWithdrawAddresses(entries []WithdrawAddressEntry) error {
	addrMap := make(map[string]bool) // check for duplicate entries
	for _, e := range entries {
		if _, exists := addrMap[e.DelegatorAddress]; exists {
			return fmt.Errorf("duplicate delegator address: %s", e.DelegatorAddress)
		}
		addrMap[e.DelegatorAddress] = true
		err := e.Validate()
		if err != nil {
			return err
		}
	}
	return nil
}

func validateAddrStr(addr string) error {
	if addr == "" {
		return errors.New("empty address")
	}
	if _, err := sdk.AccAddressFromBech32(addr); err != nil {
		return fmt.Errorf("invalid address: %s, error: %w", addr, err)
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
	addressTypeMap := make(map[string]map[StakeholderType]bool) // Map of address -> map of types

	for _, entry := range entries {
		if _, exists := addressTypeMap[entry.Address]; !exists {
			addressTypeMap[entry.Address] = make(map[StakeholderType]bool)
		}

		if _, exists := addressTypeMap[entry.Address][entry.StakeholderType]; exists {
			return fmt.Errorf("duplicate reward gauge for address: %s and type: %s", entry.Address, entry.StakeholderType)
		}

		addressTypeMap[entry.Address][entry.StakeholderType] = true

		if err := entry.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Helper function to sort gauges and addresses to get a deterministic
// result on the tests
func SortData(gs *GenesisState) {
	sort.Slice(gs.RewardGauges, func(i, j int) bool {
		if gs.RewardGauges[i].StakeholderType != gs.RewardGauges[j].StakeholderType {
			return gs.RewardGauges[i].StakeholderType < gs.RewardGauges[j].StakeholderType
		}
		return gs.RewardGauges[i].Address < gs.RewardGauges[j].Address
	})

	sort.Slice(gs.BtcStakingGauges, func(i, j int) bool {
		return gs.BtcStakingGauges[i].Height < gs.BtcStakingGauges[j].Height
	})

	sort.Slice(gs.WithdrawAddresses, func(i, j int) bool {
		return gs.WithdrawAddresses[i].DelegatorAddress < gs.WithdrawAddresses[j].DelegatorAddress
	})
}
