package types

import (
	"fmt"
)

const (
	DefaultEpochInterval uint64 = 10
)

const (
	DefaultMinAmount uint64 = 1
)

var DefaultEnqueueGasFees = EnqueueGasFees{
	Delegate:                  500,
	Undelegate:                400,
	BeginRedelegate:           600,
	CancelUnbondingDelegation: 300,
	EditValidator:             200,
	StakingUpdateParams:       100,
	CreateValidator:           800,
}

// NewParams creates a new Params instance
func NewParams(epochInterval uint64, enqueueGasFees EnqueueGasFees, minAmount uint64) Params {
	return Params{
		EpochInterval:  epochInterval,
		EnqueueGasFees: enqueueGasFees,
		MinAmount:      minAmount,
	}
}

// DefaultParams returns a default set of parameters
func DefaultParams() Params {
	return NewParams(DefaultEpochInterval, DefaultEnqueueGasFees, DefaultMinAmount)
}

// Validate validates the set of params
func (p Params) Validate() error {
	if err := validateEpochInterval(p.EpochInterval); err != nil {
		return err
	}

	if err := validateEnqueueGasFees(p.EnqueueGasFees); err != nil {
		return err
	}

	return nil
}

func validateEpochInterval(i interface{}) error {
	v, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v < 2 {
		return fmt.Errorf("epoch interval must be at least 2: %d", v)
	}

	return nil
}

func validateEnqueueGasFees(fees EnqueueGasFees) error {
	if fees.Delegate == 0 {
		return fmt.Errorf("delegate gas fee must be positive")
	}
	if fees.Undelegate == 0 {
		return fmt.Errorf("undelegate gas fee must be positive")
	}

	if fees.BeginRedelegate == 0 {
		return fmt.Errorf("begin redelegate gas fee must be positive")
	}

	if fees.CancelUnbondingDelegation == 0 {
		return fmt.Errorf("cancle unbonding delegation gas fee must be positive")
	}

	if fees.EditValidator == 0 {
		return fmt.Errorf("edit validator gas fee must be positive")
	}

	if fees.StakingUpdateParams == 0 {
		return fmt.Errorf("staking update params gas fee must be positive")
	}

	if fees.CreateValidator == 0 {
		return fmt.Errorf("create validator gas fee must be positive")
	}

	return nil
}
