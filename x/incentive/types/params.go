package types

import (
	"fmt"

	"cosmossdk.io/math"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"gopkg.in/yaml.v2"
)

var _ paramtypes.ParamSet = (*Params)(nil)

// ParamKeyTable the param key table for launch module
func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

// DefaultParams returns a default set of parameters
func DefaultParams() Params {
	return Params{
		BtcStakingPortion: math.LegacyMustNewDecFromStr("0.181818182"),
		FpPortion:         math.LegacyMustNewDecFromStr("0.013636364"),
	}
}

// ParamSetPairs get the params.ParamSet
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{}
}

// TotalPortion calculates the sum of portions of all stakeholders
func (p *Params) TotalPortion() math.LegacyDec {
	sum := p.BtcStakingPortion.Add(p.FpPortion)
	return sum
}

// BTCStakingPortion calculates the sum of portions of all BTC staking stakeholders
func (p *Params) BTCStakingPortion() math.LegacyDec {
	return p.BtcStakingPortion
}

// Validate validates the set of params
func (p Params) Validate() error {
	if p.BtcStakingPortion.IsNil() {
		return fmt.Errorf("BtcStakingPortion should not be nil")
	}

	if p.BtcStakingPortion.IsNegative() {
		return fmt.Errorf("BtcStakingPortion should not be negative")
	}

	if p.FpPortion.IsNil() {
		return fmt.Errorf("FpPortion should not be nil")
	}

	if p.FpPortion.IsNegative() {
		return fmt.Errorf("FpPortion should not be negative")
	}

	// sum of all portions should be less than 1
	if p.TotalPortion().GTE(math.LegacyOneDec()) {
		return fmt.Errorf("sum of all portions should be less than 1")
	}

	return nil
}

// String implements the Stringer interface.
func (p Params) String() string {
	out, _ := yaml.Marshal(p)
	return string(out)
}
