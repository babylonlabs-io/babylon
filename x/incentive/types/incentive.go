package types

import (
	"fmt"

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
	return []StakeholderType{FINALITY_PROVIDER, BTC_DELEGATION}
}

func NewStakeHolderType(stBytes []byte) (StakeholderType, error) {
	if len(stBytes) != 1 {
		return FINALITY_PROVIDER, fmt.Errorf("invalid format for stBytes")
	}
	switch stBytes[0] {
	case byte(FINALITY_PROVIDER):
		return FINALITY_PROVIDER, nil
	case byte(BTC_DELEGATION):
		return BTC_DELEGATION, nil
	default:
		return FINALITY_PROVIDER, fmt.Errorf("invalid stBytes")
	}
}

func NewStakeHolderTypeFromString(stStr string) (StakeholderType, error) {
	switch stStr {
	case "finality_provider":
		return FINALITY_PROVIDER, nil
	case "btc_delegation":
		return BTC_DELEGATION, nil
	default:
		return FINALITY_PROVIDER, fmt.Errorf("invalid stStr")
	}
}

func (st StakeholderType) Bytes() []byte {
	return []byte{byte(st)}
}

func (st StakeholderType) String() string {
	if st == FINALITY_PROVIDER {
		return "finality_provider"
	} else if st == BTC_DELEGATION {
		return "btc_delegation"
	}
	panic("invalid stakeholder type")
}
