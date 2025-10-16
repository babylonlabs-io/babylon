package types

import (
	"errors"
	"fmt"
	"strings"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func NewGauge(coins ...sdk.Coin) *Gauge {
	return &Gauge{
		Coins: coins,
	}
}

func (g *Gauge) GetCoinsPortion(portion math.LegacyDec) sdk.Coins {
	return GetCoinsPortion(g.Coins, portion)
}

func (g *Gauge) Validate() error {
	if !g.Coins.IsValid() {
		return fmt.Errorf("gauge has invalid coins: %s", g.Coins.String())
	}
	if g.Coins.IsAnyNil() {
		return errors.New("gauge has nil coins")
	}
	if g.Coins.Len() == 0 {
		return errors.New("gauge has no coins")
	}
	return nil
}

func NewRewardGauge(coins ...sdk.Coin) *RewardGauge {
	return &RewardGauge{
		Coins:          coins,
		WithdrawnCoins: sdk.NewCoins(),
	}
}

// GetWithdrawableCoins returns withdrawable coins in this reward gauge
func (rg *RewardGauge) GetWithdrawableCoins() sdk.Coins {
	return rg.Coins.Sub(rg.WithdrawnCoins...)
}

// SetFullyWithdrawn makes the reward gauge to have no withdrawable coins
// typically called after the stakeholder withdraws its reward
func (rg *RewardGauge) SetFullyWithdrawn() {
	rg.WithdrawnCoins = sdk.NewCoins(rg.Coins...)
}

// IsFullyWithdrawn returns whether the reward gauge has nothing to withdraw
func (rg *RewardGauge) IsFullyWithdrawn() bool {
	return rg.Coins.Equal(rg.WithdrawnCoins)
}

func (rg *RewardGauge) Add(coins sdk.Coins) {
	rg.Coins = rg.Coins.Add(coins...)
}

func (rg *RewardGauge) Validate() error {
	if !rg.Coins.IsValid() {
		return fmt.Errorf("reward gauge has invalid or negative coins: %s", rg.Coins.String())
	}
	if !rg.WithdrawnCoins.IsValid() {
		return fmt.Errorf("reward gauge has invalid or negative withdrawn coins: %s", rg.WithdrawnCoins.String())
	}
	if rg.WithdrawnCoins.IsAnyGT(rg.Coins) {
		return fmt.Errorf("withdrawn coins (%s) cannot exceed total coins (%s)", rg.WithdrawnCoins.String(), rg.Coins.String())
	}
	if rg.Coins.Len() == 0 && rg.WithdrawnCoins.Len() == 0 {
		return errors.New("reward gauge has no coins")
	}
	// Ensure WithdrawnCoins only contains denominations that exist in Coins
	for _, wc := range rg.WithdrawnCoins {
		if !rg.Coins.AmountOf(wc.Denom).IsPositive() {
			return fmt.Errorf("withdrawn coin denomination (%s) does not exist in reward coins", wc.Denom)
		}
	}
	return nil
}

func GetCoinsPortion(coinsInt sdk.Coins, portion math.LegacyDec) sdk.Coins {
	// coins with decimal value
	coins := sdk.NewDecCoinsFromCoins(coinsInt...)
	// portion of coins with decimal values
	portionCoins := coins.MulDecTruncate(portion)
	// truncate back
	// TODO: how to deal with changes?
	portionCoinsInt, _ := portionCoins.TruncateDecimal()
	return portionCoinsInt
}

func GetAllStakeholderTypes() []StakeholderType {
	return []StakeholderType{FINALITY_PROVIDER, BTC_STAKER, COSTAKER}
}

func NewStakeHolderType(stBytes []byte) (StakeholderType, error) {
	if len(stBytes) != 1 {
		return FINALITY_PROVIDER, fmt.Errorf("invalid format for stBytes")
	}
	switch stBytes[0] {
	case byte(FINALITY_PROVIDER):
		return FINALITY_PROVIDER, nil
	case byte(BTC_STAKER):
		return BTC_STAKER, nil
	case byte(COSTAKER):
		return COSTAKER, nil
	default:
		return FINALITY_PROVIDER, fmt.Errorf("invalid stBytes")
	}
}

func NewStakeHolderTypeFromString(stStr string) (StakeholderType, error) {
	// Convert to uppercase for case-insensitive matching
	stStr = strings.ToUpper(stStr)
	if value, ok := StakeholderType_value[stStr]; ok {
		return StakeholderType(value), nil
	}
	return FINALITY_PROVIDER, fmt.Errorf("invalid stStr: %s", stStr)
}

func (st StakeholderType) Bytes() []byte {
	return []byte{byte(st)}
}

func (st StakeholderType) Validate() error {
	switch st {
	case FINALITY_PROVIDER, BTC_STAKER, COSTAKER:
		return nil
	default:
		return fmt.Errorf("invalid stakeholder type: %d", st)
	}
}
