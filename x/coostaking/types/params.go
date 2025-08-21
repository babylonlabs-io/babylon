package types

import (
	fmt "fmt"

	"cosmossdk.io/math"
)

var (
	// DefaultCoostakingPortion defines how much of the fee_collector
	// balances will go to coostakers. Reminder that incentives gets
	// his portion first, than coostaking than the rest goes to distribution.
	// Goal: 3% to BTC stakers, 3% to BABY stakers, 2% Coostakers.
	// Since incentives get it first it should get 37% of total fee collector
	// balance and coostaking should get 40%
	DefaultCoostakingPortion = math.LegacyMustNewDecFromStr("0.4")
	// DefaultScoreRatioBtcByBaby defines the min number of baby staked to
	// make one BTC count as score. Each BTC staked should have at least 5k
	// BABY staked. Tranlating that into sats and ubbn the ratio should be
	// (5k * conversion baby to ubbn / conversion BTC to sats)
	// 5_000 * 1_000_000 ubbn / 100_000_000 sats= 50 ubbn per sat
	DefaultScoreRatioBtcByBaby = math.NewInt(50)
)

// DefaultParams returns a default set of parameters
func DefaultParams() Params {
	return Params{
		CoostakingPortion:   DefaultCoostakingPortion,
		ScoreRatioBtcByBaby: DefaultScoreRatioBtcByBaby,
	}
}

// Validate validates the set of params
func (p Params) Validate() error {
	if p.CoostakingPortion.IsNil() {
		return fmt.Errorf("CoostakingPortion should not be nil")
	}

	if p.CoostakingPortion.GTE(math.LegacyOneDec()) {
		return fmt.Errorf("coostaking portion should be less or equal 1")
	}

	if p.ScoreRatioBtcByBaby.IsNil() {
		return fmt.Errorf("ScoreRatioBtcByBaby should not be nil")
	}

	if p.ScoreRatioBtcByBaby.LT(math.OneInt()) {
		return fmt.Errorf("score ratio of btc to baby should be higher or equal 1")
	}

	return nil
}
