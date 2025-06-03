package types

import (
	"fmt"
	"math"
	"sort"

	sdkmath "cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/mempool"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/cometbft/cometbft/crypto/tmhash"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"gopkg.in/yaml.v2"

	"github.com/babylonlabs-io/babylon/v4/btcstaking"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
)

const (
	// TODO: need to determine a proper default value
	defaultDelegationCreationBaseGasFee = 1000
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
		MinStakingValueSat:   10000,
		MaxStakingValueSat:   10 * 10e8,
		MinStakingTimeBlocks: 400, // this should be larger than minUnbonding
		MaxStakingTimeBlocks: math.MaxUint16,
		SlashingPkScript:     defaultSlashingPkScript(),
		MinSlashingTxFeeSat:  1000,
		MinCommissionRate:    sdkmath.LegacyZeroDec(),
		// The Default slashing rate is 0.1 i.e., 10% of the total staked BTC will be burned.
		SlashingRate: sdkmath.LegacyNewDecWithPrec(1, 1), // 1 * 10^{-1} = 0.1
		// unbonding time should be always larger than the checkpoint finalization timeout
		UnbondingTimeBlocks:          200,
		UnbondingFeeSat:              1000,
		DelegationCreationBaseGasFee: defaultDelegationCreationBaseGasFee,
		// The default allow list expiration height is 0, which effectively disables the allow list.
		// Allow list can only be enabled by upgrade
		AllowListExpirationHeight: 0,
		BtcActivationHeight:       0,
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

// validateCovenantPks checks whether the covenants list contains any duplicates
func validateCovenantPks(covenantPks []bbn.BIP340PubKey) error {
	duplicate, err := ExistsDup(covenantPks)
	if err != nil {
		return err
	}
	if duplicate {
		return fmt.Errorf("duplicate covenant key")
	}
	return nil
}

func validateUnbondingTime(unbondingTimeBlocks uint32) error {
	if unbondingTimeBlocks > math.MaxUint16 {
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

func validateNoDustSlashingOutput(p *Params) error {
	// OP_RETURN scripts are allowed to be dust by BTC standard rules
	if len(p.SlashingPkScript) > 0 && p.SlashingPkScript[0] == txscript.OP_RETURN {
		return nil
	}

	slashingRateFloat64, err := p.SlashingRate.Float64()
	if err != nil {
		return fmt.Errorf("error converting slashing rate to float64: %w", err)
	}

	minUnbondingOutputValue := p.MinStakingValueSat - p.UnbondingFeeSat

	minSlashingAmount := btcutil.Amount(minUnbondingOutputValue).MulF64(slashingRateFloat64)

	minSlashingOutput := wire.NewTxOut(int64(minSlashingAmount), p.SlashingPkScript)

	if mempool.IsDust(minSlashingOutput, mempool.DefaultMinRelayTxFee) {
		return fmt.Errorf("invalid parameters configuration. Minimum slashing output is dust")
	}

	return nil
}

// Validate validates the set of params
func (p Params) Validate() error {
	if p.CovenantQuorum == 0 {
		return fmt.Errorf("covenant quorum size has to be positive")
	}
	if int(p.CovenantQuorum)*2 <= len(p.CovenantPks) {
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

	if !btcstaking.IsSlashingRateValid(p.SlashingRate) {
		return btcstaking.ErrInvalidSlashingRate
	}

	if err := validateNoDustSlashingOutput(&p); err != nil {
		return err
	}

	if err := validateUnbondingTime(p.UnbondingTimeBlocks); err != nil {
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

func NewHeightVersionPair(
	startHeight uint64,
	version uint32,
) *HeightVersionPair {
	return &HeightVersionPair{
		StartHeight: startHeight,
		Version:     version,
	}
}

func NewHeightToVersionMap() *HeightToVersionMap {
	return &HeightToVersionMap{
		Pairs: []*HeightVersionPair{},
	}
}

func (m *HeightToVersionMap) GetLastPair() *HeightVersionPair {
	if m.IsEmpty() {
		return nil
	}
	return m.Pairs[len(m.Pairs)-1]
}

// AddNewPair adds a new pair to the map it preserves the following invariants:
// 1. pairs are sorted by start height in ascending order
// 2. versions are strictly increasing by increments of 1
// If newPair breaks any of the invariants, it returns an error
func (m *HeightToVersionMap) AddNewPair(startHeight uint64, version uint32) error {
	return m.AddPair(&HeightVersionPair{
		StartHeight: startHeight,
		Version:     version,
	})
}

// AddPair adds a new pair to the map it preserves the following invariants:
// 1. pairs are sorted by start height in ascending order
// 2. versions are strictly increasing by increments of 1
// If newPair breaks any of the invariants, it returns an error
func (m *HeightToVersionMap) AddPair(newPair *HeightVersionPair) error {
	if len(m.Pairs) == 0 && newPair.Version != 0 {
		return fmt.Errorf("version must be 0 for the first pair")
	}

	if len(m.Pairs) == 0 && newPair.Version == 0 {
		m.Pairs = append(m.Pairs, newPair)
		return nil
	}

	// we already checked `m.Pairs` is not empty, so this won't be nil
	lastPair := m.GetLastPair()

	if newPair.StartHeight <= lastPair.StartHeight {
		return fmt.Errorf("pairs must be sorted by start height in ascending order, got %d <= %d",
			newPair.StartHeight, lastPair.StartHeight)
	}

	if newPair.Version != lastPair.Version+1 {
		return fmt.Errorf("versions must be strictly increasing, got %d != %d + 1",
			newPair.Version, lastPair.Version)
	}

	m.Pairs = append(m.Pairs, newPair)
	return nil
}

func NewHeightToVersionMapFromPairs(pairs []*HeightVersionPair) (*HeightToVersionMap, error) {
	if len(pairs) == 0 {
		return nil, fmt.Errorf("can't construct HeightToVersionMap from empty list of HeightVersionPair")
	}

	heightToVersionMap := NewHeightToVersionMap()

	for _, pair := range pairs {
		if err := heightToVersionMap.AddPair(pair); err != nil {
			return nil, err
		}
	}

	return heightToVersionMap, nil
}

func (m *HeightToVersionMap) IsEmpty() bool {
	return len(m.Pairs) == 0
}

func (m *HeightToVersionMap) GetVersionForHeight(height uint64) (uint32, error) {
	if m.IsEmpty() {
		return 0, fmt.Errorf("height to version map is empty")
	}

	// Binary search to find the applicable version of the parameters
	idx := sort.Search(len(m.Pairs), func(i int) bool {
		return m.Pairs[i].StartHeight > height
	}) - 1

	if idx < 0 {
		return 0, fmt.Errorf("no parameters found for block height %d", height)
	}

	return m.Pairs[idx].Version, nil
}

func (m *HeightToVersionMap) Validate() error {
	if len(m.Pairs) == 0 {
		return fmt.Errorf("height to version map is empty")
	}

	if len(m.Pairs) == 1 {
		if m.Pairs[0].Version != 0 {
			return fmt.Errorf("version must be 0 for the first pair")
		}
		return nil
	}

	for i, pair := range m.Pairs {
		if i == 0 {
			continue
		}

		if pair.StartHeight <= m.Pairs[i-1].StartHeight {
			return fmt.Errorf("pairs must be sorted by start height in ascending order, got %d <= %d",
				pair.StartHeight, m.Pairs[i-1].StartHeight)
		}

		if pair.Version != m.Pairs[i-1].Version+1 {
			return fmt.Errorf("versions must be strictly increasing, got %d != %d + 1",
				pair.Version, m.Pairs[i-1].Version)
		}
	}

	return nil
}
