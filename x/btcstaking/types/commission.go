package types

import (
	"cosmossdk.io/math"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// NewCommissionRates returns an initialized validator commission rates.
func NewCommissionRates(rate, maxRate, maxChangeRate math.LegacyDec) CommissionRates {
	return CommissionRates{
		Rate:          rate,
		MaxRate:       maxRate,
		MaxChangeRate: maxChangeRate,
	}
}

// Validate performs basic sanity validation checks of initial commission
// parameters. If validation fails, an error is returned.
func (cr CommissionRates) Validate() error {
	switch {
	case cr == (CommissionRates{}):
		// empty commission rates
		return ErrEmptyCommissionRates
	case cr.MaxRate.IsNegative():
		// max rate cannot be negative
		return stktypes.ErrCommissionNegative

	case cr.MaxRate.GT(math.LegacyOneDec()):
		// max rate cannot be greater than 1
		return stktypes.ErrCommissionHuge

	case cr.Rate.IsNegative():
		// rate cannot be negative
		return stktypes.ErrCommissionNegative

	case cr.Rate.GT(cr.MaxRate):
		// rate cannot be greater than the max rate
		return ErrCommissionGTMaxRate

	case cr.MaxChangeRate.IsNegative():
		// change rate cannot be negative
		return stktypes.ErrCommissionChangeRateNegative

	case cr.MaxChangeRate.GT(cr.MaxRate):
		// change rate cannot be greater than the max rate
		return stktypes.ErrCommissionChangeRateGTMaxRate
	}

	return nil
}
