package types

import (
	"errors"
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	bbntypes "github.com/babylonlabs-io/babylon/v3/types"
)

var (
	// it is needed to add decimal points when reducing the rewards amount
	// per sat to latter when giving out the rewards to the gauge, reduce
	// the decimal points back, currently 20 decimal points are being added
	// the sdkmath.Int holds a big int which support up to 2^256 integers
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

func NewFinalityProviderHistoricalRewards(cumulativeRewardsPerSat sdk.Coins) FinalityProviderHistoricalRewards {
	return FinalityProviderHistoricalRewards{
		CumulativeRewardsPerSat: cumulativeRewardsPerSat,
	}
}

// AddRewards adds the rewards to the FpCurrentRewards and apply the 20 decimal
// cases to increase precision for the time to calculate the rewards per active satoshi staked
func (f *FinalityProviderCurrentRewards) AddRewards(coinsToAdd sdk.Coins) error {
	coinsToAddWithDecimals, err := bbntypes.CoinsSafeMulInt(coinsToAdd, DecimalRewards)
	if err != nil {
		return err
	}

	var panicErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicErr = ErrInvalidAmount.Wrapf("math overflow: %v", r)
			}
		}()
		f.CurrentRewards = f.CurrentRewards.Add(coinsToAddWithDecimals...)
	}()
	return panicErr
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
