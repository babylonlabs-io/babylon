package types

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// CoinsSafeMulInt multiplies the amounts of coins by x. Returns an error
// if anything fails during the multiplication or coin validation.
// Ex.: CoinsSafeMulInt(100utest, sdkmath.NewInt(5)) = 500utest
func CoinsSafeMulInt(coins sdk.Coins, x sdkmath.Int) (sdk.Coins, error) {
	if x.IsZero() {
		return nil, fmt.Errorf("%w: cannot multiply coins by zero", ErrInvalidAmount)
	}

	res := make(sdk.Coins, len(coins))
	for i, coin := range coins {
		amt, err := coin.Amount.SafeMul(x)
		if err != nil {
			return sdk.Coins{}, fmt.Errorf(
				"%w: unable to multiply coins %s%s by %s: %w",
				ErrInvalidAmount, coin.Amount.String(), coin.Denom, x.String(), err,
			)
		}

		newCoin, err := SafeNewCoin(coin.Denom, amt)
		if err != nil {
			return sdk.Coins{}, fmt.Errorf("unable to create new coin %s%s: %w", amt.String(), coin.Denom, err)
		}
		res[i] = newCoin
	}

	return res, nil
}

// SafeNewCoin safely validates the coin created instead of panicking.
// Returns an error if the coin denomination or amount is invalid.
func SafeNewCoin(denom string, amount sdkmath.Int) (sdk.Coin, error) {
	coin := sdk.Coin{
		Denom:  denom,
		Amount: amount,
	}

	if err := coin.Validate(); err != nil {
		return sdk.Coin{}, fmt.Errorf("unable to validate new coin %s%s: %w", amount.String(), denom, err)
	}

	return coin, nil
}