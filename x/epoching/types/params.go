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

var DefaultExecuteGas = ExecuteGas{
	Delegate:                  130000, // estimated 117519 + 20%
	Undelegate:                88000,  // estimated 74000 + 20%
	BeginRedelegate:           220000, // estimated 184712 + 20%
	CancelUnbondingDelegation: 20500,  // estimated 17012 + 20%
	EditValidator:             61000,  // estimated 50783 + 20%
	CreateValidator:           60000,  // estimated 52000 + 20%
}

// NewParams creates a new Params instance
func NewParams(epochInterval uint64, executeGas ExecuteGas, minAmount uint64) Params {
	return Params{
		EpochInterval: epochInterval,
		ExecuteGas:    executeGas,
		MinAmount:     minAmount,
	}
}

// DefaultParams returns a default set of parameters
func DefaultParams() Params {
	return NewParams(DefaultEpochInterval, DefaultExecuteGas, DefaultMinAmount)
}

// Validate validates the set of params
func (p Params) Validate() error {
	if err := validateEpochInterval(p.EpochInterval); err != nil {
		return err
	}

	if err := validateExecuteGas(p.ExecuteGas); err != nil {
		return err
	}

	if err := validateMinAmount(p.MinAmount); err != nil {
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

func validateExecuteGas(gas ExecuteGas) error {
	if gas.Delegate == 0 {
		return fmt.Errorf("delegate gas fee must be positive")
	}
	if gas.Undelegate == 0 {
		return fmt.Errorf("undelegate gas fee must be positive")
	}

	if gas.BeginRedelegate == 0 {
		return fmt.Errorf("begin redelegate gas fee must be positive")
	}

	if gas.CancelUnbondingDelegation == 0 {
		return fmt.Errorf("cancel unbonding delegation gas fee must be positive")
	}

	if gas.EditValidator == 0 {
		return fmt.Errorf("edit validator gas fee must be positive")
	}

	if gas.CreateValidator == 0 {
		return fmt.Errorf("create validator gas fee must be positive")
	}

	return nil
}

func validateMinAmount(i interface{}) error {
	v, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v < 1 {
		return fmt.Errorf("min amount must be at least 1: %d", v)
	}
	return nil
}
