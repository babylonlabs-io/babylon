package types

import (
	"fmt"
	"math"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/btcstaking"
	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/cometbft/cometbft/crypto/tmhash"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"gopkg.in/yaml.v2"
)

const (
	defaultMaxActiveFinalityProviders uint32 = 100
)

var _ paramtypes.ParamSet = (*Params)(nil)

// DefaultCovenantCommittee deterministically generates a covenant committee
// with 5 members and quorum size of 3
func DefaultCovenantCommittee() ([]*btcec.PrivateKey, []*btcec.PublicKey, uint32) {
	sks, pks := []*btcec.PrivateKey{}, []*btcec.PublicKey{}
	for i := uint8(0); i < 5; i++ {
		skBytes := tmhash.Sum([]byte{i})
		sk, pk := btcec.PrivKeyFromBytes(skBytes)
		sks = append(sks, sk)
		pks = append(pks, pk)
	}
	return sks, pks, 3
}

func defaultSlashingPkScript() []byte {
	// 20 bytes
	pkHash := []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	addr, err := btcutil.NewAddressPubKeyHash(pkHash, &chaincfg.SimNetParams)
	if err != nil {
		panic(err)
	}

	pkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		panic(err)
	}
	return pkScript
}

// ParamKeyTable the param key table for launch module
func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

// DefaultParams returns a default set of parameters
func DefaultParams() Params {
	_, pks, quorum := DefaultCovenantCommittee()
	return Params{
		CovenantPks:          bbn.NewBIP340PKsFromBTCPKs(pks),
		CovenantQuorum:       quorum,
		MinStakingValueSat:   1000,
		MaxStakingValueSat:   10 * 10e8,
		MinStakingTimeBlocks: 10,
		MaxStakingTimeBlocks: math.MaxUint16,
		SlashingPkScript:     defaultSlashingPkScript(),
		MinSlashingTxFeeSat:  1000,
		MinCommissionRate:    sdkmath.LegacyZeroDec(),
		// The Default slashing rate is 0.1 i.e., 10% of the total staked BTC will be burned.
		SlashingRate:               sdkmath.LegacyNewDecWithPrec(1, 1), // 1 * 10^{-1} = 0.1
		MaxActiveFinalityProviders: defaultMaxActiveFinalityProviders,
		// The default minimum unbonding time is 0, which effectively defaults to checkpoint
		// finalization timeout.
		MinUnbondingTimeBlocks: 0,
		UnbondingFeeSat:        1000,
	}
}

// ParamSetPairs get the params.ParamSet
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{}
}

func validateMinSlashingTxFeeSat(fee int64) error {
	if fee <= 0 {
		return fmt.Errorf("minimum slashing tx fee has to be positive")
	}
	return nil
}

func validateMinCommissionRate(rate sdkmath.LegacyDec) error {
	if rate.IsNil() {
		return fmt.Errorf("minimum commission rate cannot be nil")
	}

	if rate.IsNegative() {
		return fmt.Errorf("minimum commission rate cannot be negative")
	}

	if rate.GT(sdkmath.LegacyOneDec()) {
		return fmt.Errorf("minimum commission rate cannot be greater than 100%%")
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

// validateCovenantPks checks whether the covenants list contains any duplicates
func validateCovenantPks(covenantPks []bbn.BIP340PubKey) error {
	if ExistsDup(covenantPks) {
		return fmt.Errorf("duplicate covenant key")
	}
	return nil
}

func validateMinUnbondingTime(minUnbondingTimeBlocks uint32) error {
	if minUnbondingTimeBlocks > math.MaxUint16 {
		return fmt.Errorf("minimum unbonding time blocks cannot be greater than %d", math.MaxUint16)
	}

	return nil
}

func validateStakingAmout(minStakingAmt, maxStakingAmt int64) error {
	if minStakingAmt <= 0 {
		return fmt.Errorf("minimum staking amount has to be positive")
	}

	if maxStakingAmt <= 0 {
		return fmt.Errorf("maximum staking amount has to be positive")
	}

	if minStakingAmt > maxStakingAmt {
		return fmt.Errorf("minimum staking amount cannot be greater than maximum staking amount")
	}

	return nil
}

func validateStakingTime(minStakingTime, maxStakingTime uint32) error {
	if minStakingTime == 0 {
		return fmt.Errorf("minimum staking time has to be positive")
	}

	if minStakingTime > math.MaxUint16 {
		return fmt.Errorf("minimum staking time cannot be greater than %d", math.MaxUint16)
	}

	if maxStakingTime == 0 {
		return fmt.Errorf("maximum staking time has to be positive")
	}

	if maxStakingTime > math.MaxUint16 {
		return fmt.Errorf("maximum staking time cannot be greater than %d", math.MaxUint16)
	}

	if minStakingTime > maxStakingTime {
		return fmt.Errorf("minimum staking time cannot be greater than maximum staking time")
	}

	return nil
}

// Validate validates the set of params
func (p Params) Validate() error {
	if p.CovenantQuorum == 0 {
		return fmt.Errorf("covenant quorum size has to be positive")
	}
	if p.CovenantQuorum*2 <= uint32(len(p.CovenantPks)) {
		return fmt.Errorf("covenant quorum size has to be more than 1/2 of the covenant committee size")
	}

	if err := validateStakingAmout(p.MinStakingValueSat, p.MaxStakingValueSat); err != nil {
		return err
	}

	if err := validateStakingTime(p.MinStakingTimeBlocks, p.MaxStakingTimeBlocks); err != nil {
		return err
	}

	if err := validateCovenantPks(p.CovenantPks); err != nil {
		return err
	}
	if err := validateMinSlashingTxFeeSat(p.MinSlashingTxFeeSat); err != nil {
		return err
	}

	if err := validateMinCommissionRate(p.MinCommissionRate); err != nil {
		return err
	}

	if !btcstaking.IsRateValid(p.SlashingRate) {
		return btcstaking.ErrInvalidSlashingRate
	}

	if err := validateMaxActiveFinalityProviders(p.MaxActiveFinalityProviders); err != nil {
		return err
	}

	if err := validateMinUnbondingTime(p.MinUnbondingTimeBlocks); err != nil {
		return err
	}

	return nil
}

// String implements the Stringer interface.
func (p Params) String() string {
	out, _ := yaml.Marshal(p)
	return string(out)
}

func (p Params) HasCovenantPK(pk *bbn.BIP340PubKey) bool {
	for _, pk2 := range p.CovenantPks {
		if pk2.Equals(pk) {
			return true
		}
	}
	return false
}

func (p Params) CovenantPksHex() []string {
	covPksHex := make([]string, 0, len(p.CovenantPks))
	for _, pk := range p.CovenantPks {
		covPksHex = append(covPksHex, pk.MarshalHex())
	}
	return covPksHex
}

func (p Params) MustGetCovenantPks() []*btcec.PublicKey {
	covenantKeys, err := bbn.NewBTCPKsFromBIP340PKs(p.CovenantPks)

	if err != nil {
		panic(fmt.Errorf("failed to get covenant keys: %w", err))
	}

	return covenantKeys
}
