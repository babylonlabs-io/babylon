package types

import (
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
)

var (
	// DefaultCostakingPortion defines how much of the fee_collector
	// balances will go to costakers. Reminder that incentives gets
	// his portion first, than costaking than the rest goes to distribution.
	DefaultCostakingPortion = math.LegacyMustNewDecFromStr("0.531073446")
	// DefaultScoreRatioBtcByBaby defines the min number of baby staked to
	// make one BTC count as score. Each BTC staked should have at least 20k
	// BABY staked. Tranlating that into sats and ubbn the ratio should be
	// (20k * conversion baby to ubbn / conversion BTC to sats)
	// 20_000 * 1_000_000 ubbn / 100_000_000 sats= 200 ubbn per sat
	DefaultScoreRatioBtcByBaby = math.NewInt(200)
	// DefaultValidatorsPortion defines how much of the fee_collector
	// remaining balances will go directly to baby validators
	DefaultValidatorsPortion = math.LegacyMustNewDecFromStr("0.016949153")
)

// DefaultParams returns a default set of parameters
func DefaultParams() Params {
	return Params{
		CostakingPortion:    DefaultCostakingPortion,
		ScoreRatioBtcByBaby: DefaultScoreRatioBtcByBaby,
		ValidatorsPortion:   DefaultValidatorsPortion,
	}
}

// Validate validates the set of params
func (p Params) Validate() error {
	if err := validatePercentage(p.CostakingPortion); err != nil {
		return errorsmod.Wrap(err, "invalid costaking portion")
	}
	if err := validatePercentage(p.ValidatorsPortion); err != nil {
		return errorsmod.Wrap(err, "invalid validators portion")
	}

	if err := validatePercentage(p.CostakingPortion.Add(p.ValidatorsPortion)); err != nil {
		return errorsmod.Wrapf(err, "invalid total portion; costaking (%s) + validators (%s)", p.CostakingPortion, p.ValidatorsPortion)
	}

	if p.ScoreRatioBtcByBaby.IsNil() {
		return ErrInvalidScoreRatioBtcByBaby
	}

	if p.ScoreRatioBtcByBaby.LT(math.OneInt()) {
		return ErrScoreRatioTooLow
	}

	return nil
}

func validatePercentage(percentage math.LegacyDec) error {
	if percentage.IsNil() {
		return ErrInvalidPercentage
	}

	if percentage.GTE(math.LegacyOneDec()) {
		return ErrPercentageTooHigh
	}

	if percentage.LT(math.LegacyZeroDec()) {
		return ErrInvalidPercentage.Wrap("lower than zero")
	}

	return nil
}
