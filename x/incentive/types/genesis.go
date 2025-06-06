package types

import (
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"sort"

	"github.com/cometbft/cometbft/crypto/tmhash"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v3/types"
)

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:                                DefaultParams(),
		BtcStakingGauges:                      []BTCStakingGaugeEntry{},
		RewardGauges:                          []RewardGaugeEntry{},
		WithdrawAddresses:                     []WithdrawAddressEntry{},
		RefundableMsgHashes:                   []string{},
		FinalityProvidersCurrentRewards:       []FinalityProviderCurrentRewardsEntry{},
		FinalityProvidersHistoricalRewards:    []FinalityProviderHistoricalRewardsEntry{},
		BtcDelegationRewardsTrackers:          []BTCDelegationRewardsTrackerEntry{},
		BtcDelegatorsToFps:                    []BTCDelegatorToFpEntry{},
		EventRewardTracker:                    []EventsPowerUpdateAtHeightEntry{},
		LastProcessedHeightEventRewardTracker: 0,
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

	if err := validateMsgHashes(gs.RefundableMsgHashes); err != nil {
		return fmt.Errorf("invalid msg hashes: %w", err)
	}

	if err := validateFPCurrentRewards(gs.FinalityProvidersCurrentRewards); err != nil {
		return fmt.Errorf("invalid finality providers current rewards: %w", err)
	}

	// Create a map of FP addresses to their current periods for validation
	fpCurrentPeriods := make(map[string]uint64)
	for _, fpcr := range gs.FinalityProvidersCurrentRewards {
		fpCurrentPeriods[fpcr.Address] = fpcr.Rewards.Period
	}

	if err := validateFPHistoricalRewards(gs.FinalityProvidersHistoricalRewards, fpCurrentPeriods); err != nil {
		return fmt.Errorf("invalid finality providers historical rewards: %w", err)
	}

	if err := validateEvtPowerUpdateEntries(gs.EventRewardTracker); err != nil {
		return fmt.Errorf("invalid events from reward tracker: %w", err)
	}

	btcRewardsAddrMap, err := validateBTCDelegationsRewardsTrackers(gs.BtcDelegationRewardsTrackers, fpCurrentPeriods)
	if err != nil {
		return fmt.Errorf("invalid BTC delegations rewards trackers: %w", err)
	}

	addrMap, err := validateBTCDelegatorsToFps(gs.BtcDelegatorsToFps)
	if err != nil {
		return fmt.Errorf("invalid BTC delegators to finality providers: %w", err)
	}

	// btcRewardsAddrMap and addrMap should be equal, considering these entries
	// should have the same fp_address <-> del_address relationship
	if equal := reflect.DeepEqual(btcRewardsAddrMap, addrMap); !equal {
		return fmt.Errorf("BTC delegators to finality providers map is not equal to the btc delegations data: %w", err)
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

func (fpcr FinalityProviderCurrentRewardsEntry) Validate() error {
	if err := validateAddrStr(fpcr.Address); err != nil {
		return fmt.Errorf("invalid finality provider, error: %w", err)
	}
	if fpcr.Rewards == nil {
		return fmt.Errorf("current rewards for address %s is nil", fpcr.Address)
	}
	if err := fpcr.Rewards.Validate(); err != nil {
		return fmt.Errorf("invalid current rewards for address %s: %w", fpcr.Address, err)
	}
	return nil
}

func (fphr FinalityProviderHistoricalRewardsEntry) Validate() error {
	if err := validateAddrStr(fphr.Address); err != nil {
		return fmt.Errorf("invalid finality provider, error: %w", err)
	}
	if fphr.Rewards == nil {
		return fmt.Errorf("historical rewards for address %s is nil", fphr.Address)
	}
	if err := fphr.Rewards.Validate(); err != nil {
		return fmt.Errorf("invalid historical rewards for address %s: %w", fphr.Address, err)
	}
	return nil
}

func (bdt BTCDelegationRewardsTrackerEntry) Validate() error {
	if err := validateAddrStr(bdt.FinalityProviderAddress); err != nil {
		return fmt.Errorf("invalid finality provider, error: %w", err)
	}
	if err := validateAddrStr(bdt.DelegatorAddress); err != nil {
		return fmt.Errorf("invalid delegator, error: %w", err)
	}
	if bdt.Tracker == nil {
		return fmt.Errorf("tracker for fp address %s and delegator address %s is nil", bdt.FinalityProviderAddress, bdt.DelegatorAddress)
	}
	if err := bdt.Tracker.Validate(); err != nil {
		return fmt.Errorf("invalid tracker for finality provider address %s and delegator address %s: %w", bdt.FinalityProviderAddress, bdt.DelegatorAddress, err)
	}
	return nil
}

func (bdt BTCDelegatorToFpEntry) Validate() error {
	if err := validateAddrStr(bdt.FinalityProviderAddress); err != nil {
		return fmt.Errorf("invalid finality provider, error: %w", err)
	}
	if err := validateAddrStr(bdt.DelegatorAddress); err != nil {
		return fmt.Errorf("invalid delegator, error: %w", err)
	}
	return nil
}

func (evtPowedUpdEntry EventsPowerUpdateAtHeightEntry) Validate() error {
	return evtPowedUpdEntry.Events.Validate()
}

func validateEvtPowerUpdateEntries(entries []EventsPowerUpdateAtHeightEntry) error {
	return types.ValidateEntries(entries, func(e EventsPowerUpdateAtHeightEntry) uint64 {
		return e.Height
	})
}

func validateWithdrawAddresses(entries []WithdrawAddressEntry) error {
	return types.ValidateEntries(entries, func(e WithdrawAddressEntry) string {
		return e.DelegatorAddress
	})
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
	return types.ValidateEntries(entries, func(e BTCStakingGaugeEntry) uint64 {
		return e.Height
	})
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

func validateMsgHashes(hashes []string) error {
	hashMap := make(map[string]bool) // To check for duplicate hashes
	for _, hash := range hashes {
		if hash == "" {
			return errors.New("empty hash")
		}
		bz, err := hex.DecodeString(hash)
		if err != nil {
			return fmt.Errorf("error decoding msg hash: %w", err)
		}
		if len(bz) != tmhash.Size {
			return fmt.Errorf("hash size should be %d characters: %s", tmhash.Size, hash)
		}
		if _, exists := hashMap[hash]; exists {
			return fmt.Errorf("duplicate hash: %s", hash)
		}
		hashMap[hash] = true
	}
	return nil
}

func validateFPCurrentRewards(entries []FinalityProviderCurrentRewardsEntry) error {
	return types.ValidateEntries(entries, func(e FinalityProviderCurrentRewardsEntry) string {
		return e.Address
	})
}

func validateFPHistoricalRewards(entries []FinalityProviderHistoricalRewardsEntry, fpCurrentPeriods map[string]uint64) error {
	addressPeriodMap := make(map[string]map[uint64]bool) // Map of FpAddr -> map of period

	for _, entry := range entries {
		if _, exists := addressPeriodMap[entry.Address]; !exists {
			addressPeriodMap[entry.Address] = make(map[uint64]bool)
		}

		if _, exists := addressPeriodMap[entry.Address][entry.Period]; exists {
			return fmt.Errorf("duplicate historical rewards for address: %s and period: %d", entry.Address, entry.Period)
		}

		addressPeriodMap[entry.Address][entry.Period] = true

		if err := entry.Validate(); err != nil {
			return err
		}
	}

	// Validate that each FP with current rewards has all historical periods from 0 to currentPeriod-1
	for fpAddr, currentPeriod := range fpCurrentPeriods {
		if currentPeriod > 0 {
			periods, exists := addressPeriodMap[fpAddr]
			if !exists {
				return fmt.Errorf("finality provider %s has current rewards with period %d but no historical rewards", fpAddr, currentPeriod)
			}

			// Check for all periods from 0 to currentPeriod-1
			for period := uint64(0); period < currentPeriod; period++ {
				if !periods[period] {
					return fmt.Errorf("finality provider %s is missing historical rewards for period %d (current period is %d)", fpAddr, period, currentPeriod)
				}
			}
		}
	}

	// Check that no FP has historical rewards beyond their current period
	for fpAddr, periods := range addressPeriodMap {
		currentPeriod, exists := fpCurrentPeriods[fpAddr]
		if !exists {
			return fmt.Errorf("finality provider %s has historical rewards but no current rewards", fpAddr)
		}

		for period := range periods {
			if period >= currentPeriod {
				return fmt.Errorf("finality provider %s has historical rewards for period %d which is >= current period %d", fpAddr, period, currentPeriod)
			}
		}
	}
	return nil
}

func validateBTCDelegationsRewardsTrackers(entries []BTCDelegationRewardsTrackerEntry, fpCurrentPeriods map[string]uint64) (map[string]map[string]bool, error) {
	addressAddressMap := make(map[string]map[string]bool) // Map of FpAddr -> map of delAddr

	for _, entry := range entries {
		if _, exists := addressAddressMap[entry.FinalityProviderAddress]; !exists {
			addressAddressMap[entry.FinalityProviderAddress] = make(map[string]bool)
		}

		if _, exists := addressAddressMap[entry.FinalityProviderAddress][entry.DelegatorAddress]; exists {
			return nil, fmt.Errorf("duplicate btc delegation rewards tracker for finality provider: %s and delegator: %s", entry.FinalityProviderAddress, entry.DelegatorAddress)
		}

		addressAddressMap[entry.FinalityProviderAddress][entry.DelegatorAddress] = true

		if err := entry.Validate(); err != nil {
			return nil, err
		}

		// Validate that the tracker's StartPeriodCumulativeReward is less than the FP's current period
		currentPeriod, exists := fpCurrentPeriods[entry.FinalityProviderAddress]
		if !exists {
			return nil, fmt.Errorf("delegation tracker for finality provider %s exists but FP has no current rewards", entry.FinalityProviderAddress)
		}

		if entry.Tracker.StartPeriodCumulativeReward >= currentPeriod {
			return nil, fmt.Errorf("delegation tracker for FP %s and delegator %s has StartPeriodCumulativeReward %d >= FP's current period %d",
				entry.FinalityProviderAddress, entry.DelegatorAddress, entry.Tracker.StartPeriodCumulativeReward, currentPeriod)
		}
	}
	return addressAddressMap, nil
}

func validateBTCDelegatorsToFps(entries []BTCDelegatorToFpEntry) (map[string]map[string]bool, error) {
	// Map of FpAddr -> map of delAddr, keep the fpAddr as key
	// to then compare with the map resulting from validateBTCDelegationsRewardsTrackers
	addressAddressMap := make(map[string]map[string]bool)

	for _, entry := range entries {
		if _, exists := addressAddressMap[entry.FinalityProviderAddress]; !exists {
			addressAddressMap[entry.FinalityProviderAddress] = make(map[string]bool)
		}

		if _, exists := addressAddressMap[entry.FinalityProviderAddress][entry.DelegatorAddress]; exists {
			return nil, fmt.Errorf("duplicate entry with finality provider: %s and delegator: %s", entry.FinalityProviderAddress, entry.DelegatorAddress)
		}

		addressAddressMap[entry.FinalityProviderAddress][entry.DelegatorAddress] = true

		if err := entry.Validate(); err != nil {
			return nil, err
		}
	}
	return addressAddressMap, nil
}

// Helper function to sort slices to get a deterministic
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

	sort.Slice(gs.RefundableMsgHashes, func(i, j int) bool {
		return gs.RefundableMsgHashes[i] < gs.RefundableMsgHashes[j]
	})

	sort.Slice(gs.FinalityProvidersCurrentRewards, func(i, j int) bool {
		return gs.FinalityProvidersCurrentRewards[i].Address < gs.FinalityProvidersCurrentRewards[j].Address
	})

	sort.Slice(gs.FinalityProvidersHistoricalRewards, func(i, j int) bool {
		if gs.FinalityProvidersHistoricalRewards[i].Address == gs.FinalityProvidersHistoricalRewards[j].Address {
			return gs.FinalityProvidersHistoricalRewards[i].Period < gs.FinalityProvidersHistoricalRewards[j].Period
		}
		return gs.FinalityProvidersHistoricalRewards[i].Address < gs.FinalityProvidersHistoricalRewards[j].Address
	})

	sort.Slice(gs.BtcDelegationRewardsTrackers, func(i, j int) bool {
		if gs.BtcDelegationRewardsTrackers[i].FinalityProviderAddress == gs.BtcDelegationRewardsTrackers[j].FinalityProviderAddress {
			return gs.BtcDelegationRewardsTrackers[i].DelegatorAddress < gs.BtcDelegationRewardsTrackers[j].DelegatorAddress
		}
		return gs.BtcDelegationRewardsTrackers[i].FinalityProviderAddress < gs.BtcDelegationRewardsTrackers[j].FinalityProviderAddress
	})

	sort.Slice(gs.BtcDelegatorsToFps, func(i, j int) bool {
		if gs.BtcDelegatorsToFps[i].DelegatorAddress == gs.BtcDelegatorsToFps[j].DelegatorAddress {
			return gs.BtcDelegatorsToFps[i].FinalityProviderAddress < gs.BtcDelegatorsToFps[j].FinalityProviderAddress
		}
		return gs.BtcDelegatorsToFps[i].DelegatorAddress < gs.BtcDelegatorsToFps[j].DelegatorAddress
	})

	sort.Slice(gs.EventRewardTracker, func(i, j int) bool {
		return gs.EventRewardTracker[i].Height < gs.EventRewardTracker[j].Height
	})
}
