package types

import (
	"errors"
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	// DecimalRewards is a precision multiplier used for reward calculations.
	// It adds 20 decimal places (10^20) to increase precision when calculating
	// rewards per satoshi. This prevents precision loss during division operations.
	// The value is later divided out when distributing final rewards to gauges.
	// Using sdkmath.Int which supports up to 2^256 integers for overflow safety.
	DecimalRewards, _ = sdkmath.NewIntFromString("100000000000000000000")
)

// NewEventBtcDelegationActivated returns a new EventPowerUpdate of type activated
func NewEventBtcDelegationActivated(fpAddr, btcDelAddr string, totalSat sdkmath.Int) *EventPowerUpdate {
	return &EventPowerUpdate{
		Ev: &EventPowerUpdate_BtcActivated{
			BtcActivated: &EventBTCDelegationActivated{
				FpAddr:     fpAddr,
				BtcDelAddr: btcDelAddr,
				TotalSat:   totalSat,
			},
		},
	}
}

// NewEventBtcDelegationUnboned returns a new EventPowerUpdate of type unbonded
func NewEventBtcDelegationUnboned(fpAddr, btcDelAddr string, totalSat sdkmath.Int) *EventPowerUpdate {
	return &EventPowerUpdate{
		Ev: &EventPowerUpdate_BtcUnbonded{
			BtcUnbonded: &EventBTCDelegationUnbonded{
				FpAddr:     fpAddr,
				BtcDelAddr: btcDelAddr,
				TotalSat:   totalSat,
			},
		},
	}
}

func NewBTCDelegationRewardsTracker(startPeriod uint64, totalSat sdkmath.Int) BTCDelegationRewardsTracker {
	return BTCDelegationRewardsTracker{
		StartPeriodCumulativeReward: startPeriod,
		TotalActiveSat:              totalSat,
	}
}

func NewFinalityProviderCurrentRewards(currentRewards sdk.Coins, period uint64, totalActiveSatFP sdkmath.Int) FinalityProviderCurrentRewards {
	return FinalityProviderCurrentRewards{
		CurrentRewards: currentRewards,
		Period:         period,
		TotalActiveSat: totalActiveSatFP,
	}
}

func NewFinalityProviderHistoricalRewards(cumulativeRewardsPerSat sdk.Coins, referenceCount uint32) FinalityProviderHistoricalRewards {
	return FinalityProviderHistoricalRewards{
		CumulativeRewardsPerSat: cumulativeRewardsPerSat,
		ReferenceCount:          referenceCount,
	}
}

func (f *FinalityProviderCurrentRewards) AddRewards(coinsToAdd sdk.Coins) {
	f.CurrentRewards = f.CurrentRewards.Add(coinsToAdd...)
}

func (f *FinalityProviderCurrentRewards) SubRewards(coinsToSubtract sdk.Coins) {
	f.CurrentRewards = f.CurrentRewards.Sub(coinsToSubtract...)
}

func (f *FinalityProviderCurrentRewards) AddTotalActiveSat(amt sdkmath.Int) {
	f.TotalActiveSat = f.TotalActiveSat.Add(amt)
}

func (f *FinalityProviderCurrentRewards) SubTotalActiveSat(amt sdkmath.Int) {
	f.TotalActiveSat = f.TotalActiveSat.Sub(amt)
}

// ToResponse converts FinalityProviderCurrentRewards to QueryFpCurrentRewardsResponse
// for gRPC query responses.
func (f *FinalityProviderCurrentRewards) ToResponse() *QueryFpCurrentRewardsResponse {
	return &QueryFpCurrentRewardsResponse{
		CurrentRewards: f.CurrentRewards,
		Period:         f.Period,
		TotalActiveSat: f.TotalActiveSat,
	}
}

func (f *FinalityProviderCurrentRewards) Validate() error {
	if !f.CurrentRewards.IsValid() {
		return fmt.Errorf("current rewards has invalid coins: %s", f.CurrentRewards.String())
	}
	if f.CurrentRewards.IsAnyNil() {
		return errors.New("current rewards has nil coins")
	}
	if f.CurrentRewards.Len() == 0 {
		return errors.New("current rewards has no coins")
	}

	if f.TotalActiveSat.IsNil() {
		return errors.New("current rewards has no total active satoshi delegated")
	}

	if f.TotalActiveSat.IsNegative() {
		return fmt.Errorf("current rewards has a negative total active satoshi delegated value: %s", f.TotalActiveSat.String())
	}

	//nolint:staticcheck
	if f.Period <= 0 {
		return fmt.Errorf("fp current rewards period must be positive")
	}

	return nil
}

func (f *BTCDelegationRewardsTracker) AddTotalActiveSat(amt sdkmath.Int) {
	f.TotalActiveSat = f.TotalActiveSat.Add(amt)
}

func (f *BTCDelegationRewardsTracker) SubTotalActiveSat(amt sdkmath.Int) {
	f.TotalActiveSat = f.TotalActiveSat.Sub(amt)
}

func (f *BTCDelegationRewardsTracker) Validate() error {
	if f.TotalActiveSat.IsNil() {
		return errors.New("btc delegation rewards tracker has nil total active sat")
	}

	if f.TotalActiveSat.IsNegative() {
		return fmt.Errorf("btc delegation rewards tracker has a negative total active satoshi delegated value: %s", f.TotalActiveSat.String())
	}
	return nil
}

func (hr *FinalityProviderHistoricalRewards) Validate() error {
	if !hr.CumulativeRewardsPerSat.IsValid() {
		return fmt.Errorf("cummulative rewards per sat has invalid coins: %s", hr.CumulativeRewardsPerSat.String())
	}
	if hr.CumulativeRewardsPerSat.IsAnyNil() {
		return errors.New("cummulative rewards per sat has nil coins")
	}
	if hr.CumulativeRewardsPerSat.Len() == 0 {
		return errors.New("cummulative rewards per sat has no coins")
	}
	return nil
}

func (evtPowerUpdt *EventsPowerUpdateAtHeight) Validate() error {
	for _, untypedEvt := range evtPowerUpdt.Events {
		if untypedEvt == nil {
			return errors.New("nil event in EventsPowerUpdateAtHeight")
		}
		if untypedEvt.Ev == nil {
			return errors.New("nil event type in EventsPowerUpdateAtHeight")
		}
		switch typedEvt := untypedEvt.Ev.(type) {
		case *EventPowerUpdate_BtcActivated:
			evt := typedEvt.BtcActivated
			if err := validateAddrStr(evt.FpAddr); err != nil {
				return fmt.Errorf("invalid event activated finality provider, error: %w", err)
			}
			if err := validateAddrStr(evt.BtcDelAddr); err != nil {
				return fmt.Errorf("invalid event activated btc delegator, error: %w", err)
			}
			if !evt.TotalSat.IsPositive() {
				return fmt.Errorf("invalid event activated total_sat: must be positive, got %s", evt.TotalSat.String())
			}
		case *EventPowerUpdate_BtcUnbonded:
			evt := typedEvt.BtcUnbonded
			if err := validateAddrStr(evt.FpAddr); err != nil {
				return fmt.Errorf("invalid event unbonded finality provider, error: %w", err)
			}
			if err := validateAddrStr(evt.BtcDelAddr); err != nil {
				return fmt.Errorf("invalid event unbonded btc delegator, error: %w", err)
			}
			if !evt.TotalSat.IsPositive() {
				return fmt.Errorf("invalid event activated total_sat: must be positive, got %s", evt.TotalSat.String())
			}
		}
	}

	return nil
}
