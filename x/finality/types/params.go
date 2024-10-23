package types

import (
	"fmt"
	"time"

	"cosmossdk.io/math"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"gopkg.in/yaml.v2"
)

// Default parameter namespace
const (
	DefaultMaxActiveFinalityProviders = uint32(100)
	DefaultSignedBlocksWindow         = int64(100)
	DefaultMinPubRand                 = 100
	DefaultFinalitySigTimeout         = 3
	DefaultJailDuration               = 24 * 60 * 60 * 1 * time.Second // 1 day
	// For mainnet considering we want 48 hours
	// at a block time of 10s that would be 17280 blocks
	// considering the upgrade for Phase-2 will happen at block
	// 220, the mainnet activation height for btcstaking should
	// be 17280 + 220 = 17500.
	// For now it is set to 1 to avoid breaking dependencies.
	DefaultFinalityActivationHeight = 1
)

var (
	DefaultMinSignedPerWindow = math.LegacyNewDecWithPrec(5, 1)
)

var _ paramtypes.ParamSet = (*Params)(nil)

// DefaultParams returns a default set of parameters
func DefaultParams() Params {
	return Params{
		MaxActiveFinalityProviders: DefaultMaxActiveFinalityProviders,
		FinalitySigTimeout:         DefaultFinalitySigTimeout,
		SignedBlocksWindow:         DefaultSignedBlocksWindow,
		MinSignedPerWindow:         DefaultMinSignedPerWindow,
		MinPubRand:                 DefaultMinPubRand,
		JailDuration:               DefaultJailDuration,
		FinalityActivationHeight:   DefaultFinalityActivationHeight,
	}
}

// ParamSetPairs get the params.ParamSet
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{}
}

func validateMinPubRand(minPubRand uint64) error {
	if minPubRand == 0 {
		return fmt.Errorf("min Pub Rand cannot be 0")
	}
	return nil
}

// String implements the Stringer interface.
func (p Params) String() string {
	out, _ := yaml.Marshal(p)
	return string(out)
}

// Validate validates the params
// finality activation height can be any value, even 0.
func (p Params) Validate() error {
	if err := validateMaxActiveFinalityProviders(p.MaxActiveFinalityProviders); err != nil {
		return err
	}

	if err := validateSignedBlocksWindow(p.SignedBlocksWindow); err != nil {
		return err
	}

	if err := validateFinalitySigTimeout(p.FinalitySigTimeout); err != nil {
		return err
	}

	if err := validateMinSignedPerWindow(p.MinSignedPerWindow); err != nil {
		return err
	}

	if err := validateMinPubRand(p.MinPubRand); err != nil {
		return err
	}

	return nil
}

// validateMaxActiveFinalityProviders checks if the maximum number of
// active finality providers is at least the default value
func validateMaxActiveFinalityProviders(maxActiveFinalityProviders uint32) error {
	if maxActiveFinalityProviders == 0 {
		return fmt.Errorf("max finality providers must be positive")
	}
	return nil
}

func validateSignedBlocksWindow(i interface{}) error {
	v, ok := i.(int64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v <= 0 {
		return fmt.Errorf("signed blocks window must be positive: %d", v)
	}

	return nil
}

func validateFinalitySigTimeout(i interface{}) error {
	v, ok := i.(int64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v <= 0 {
		return fmt.Errorf("finality vote delay must be positive: %d", v)
	}

	return nil
}

func validateMinSignedPerWindow(i interface{}) error {
	v, ok := i.(math.LegacyDec)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v.IsNil() {
		return fmt.Errorf("min signed per window cannot be nil: %s", v)
	}
	if v.IsNegative() {
		return fmt.Errorf("min signed per window cannot be negative: %s", v)
	}
	if v.GT(math.LegacyOneDec()) {
		return fmt.Errorf("min signed per window too large: %s", v)
	}

	return nil
}

// MinSignedPerWindowInt returns min signed per window as an integer (vs the decimal in the param)
func (p *Params) MinSignedPerWindowInt() int64 {
	signedBlocksWindow := p.SignedBlocksWindow
	minSignedPerWindow := p.MinSignedPerWindow

	// NOTE: RoundInt64 will never panic as minSignedPerWindow is
	//       less than 1.
	return minSignedPerWindow.MulInt64(signedBlocksWindow).RoundInt64()
}
