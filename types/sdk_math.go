package types

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// CoinsSafeMulInt multiply the amounts of coins by x
func CoinsSafeMulInt(coins sdk.Coins, x sdkmath.Int) (sdk.Coins, error) {
	if x.IsZero() {
		return nil, fmt.Errorf("%s: cannot multiply by zero", ErrInvalidAmount)
	}

	res := make(sdk.Coins, len(coins))
	for i, coin := range coins {
		amt, err := coin.Amount.SafeMul(x)
		if err != nil {
			return sdk.Coins{}, err
		}

		newCoin, err := SafeNewCoin(coin.Denom, amt)
		if err != nil {
			return sdk.Coins{}, err
		}
		res[i] = newCoin
	}

	return res, nil
}

func SafeNewCoin(denom string, amount sdkmath.Int) (sdk.Coin, error) {
	coin := sdk.Coin{
		Denom:  denom,
		Amount: amount,
	}

	if err := coin.Validate(); err != nil {
		return sdk.Coin{}, err
	}

	return coin, nil
}
