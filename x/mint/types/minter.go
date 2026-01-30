package types

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const DefaultBondDenom = "ubbn"

// MaxMintedPerBlock defines the maximum amount (in ubbn, i.e. micro-Baby tokens) allowed to mint per block.
// On mainnet, the current rate is around 189 Baby tokens per block (≈189_000000 ubbn), so this cap of
// 600_000000 ubbn (600 Baby tokens) is approximately 3.17x the current rate.
var MaxMintedPerBlock = math.NewInt(600_000000)

// NewMinter returns a new Minter object.
func NewMinter(inflationRate math.LegacyDec, annualProvisions math.LegacyDec, bondDenom string) Minter {
	return Minter{
		InflationRate:    inflationRate,
		AnnualProvisions: annualProvisions,
		BondDenom:        bondDenom,
	}
}

// DefaultMinter returns a Minter object with default values.
func DefaultMinter() Minter {
	annualProvisions := math.LegacyNewDec(0)
	return NewMinter(InitialInflationRateAsDec(), annualProvisions, DefaultBondDenom)
}

// Validate returns an error if the minter is invalid.
func (m Minter) Validate() error {
	if m.InflationRate.IsNegative() {
		return fmt.Errorf("inflation rate %v should be positive", m.InflationRate.String())
	}
	if m.AnnualProvisions.IsNegative() {
		return fmt.Errorf("annual provisions %v should be positive", m.AnnualProvisions.String())
	}
	if strings.TrimSpace(m.BondDenom) == "" {
		return errors.New("bond denom cannot be empty")
	}
	if strings.ContainsAny(m.BondDenom, " \t\n\r") {
		return errors.New("bond denom cannot contain whitespace")
	}

	return nil
}

// CalculateInflationRate returns the inflation rate for the current year depending on
// the current block height in context. The inflation rate is expected to
// decrease every year according to the schedule specified in the README.
func (m Minter) CalculateInflationRate(ctx sdk.Context, genesis time.Time) math.LegacyDec {
	years := YearsSinceGenesis(genesis, ctx.BlockTime())
	inflationRate := InitialInflationRateAsDec().Mul(math.LegacyOneDec().Sub(DisinflationRateAsDec()).Power(uint64(years)))

	if inflationRate.LT(TargetInflationRateAsDec()) {
		return TargetInflationRateAsDec()
	}
	return inflationRate
}

// CalculateBlockProvision returns the total number of coins that should be
// minted due to inflation for the current block.
func (m Minter) CalculateBlockProvision(current time.Time, previous time.Time) (sdk.Coin, error) {
	if current.Before(previous) {
		return sdk.Coin{}, fmt.Errorf("current time %v cannot be before previous time %v", current, previous)
	}
	timeElapsed := current.Sub(previous).Nanoseconds()
	portionOfYear := math.LegacyNewDec(timeElapsed).Quo(math.LegacyNewDec(NanosecondsPerYear))
	blockProvision := m.AnnualProvisions.Mul(portionOfYear)

	blockProvisionInt := blockProvision.TruncateInt()
	amountToMint := math.MinInt(blockProvisionInt, MaxMintedPerBlock)

	return sdk.NewCoin(m.BondDenom, amountToMint), nil
}

// YearsSinceGenesis returns the number of years that have passed between
// genesis and current (rounded down).
func YearsSinceGenesis(genesis time.Time, current time.Time) (years int64) {
	if current.Before(genesis) {
		return 0
	}
	return current.Sub(genesis).Nanoseconds() / NanosecondsPerYear
}
