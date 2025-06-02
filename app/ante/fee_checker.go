package ante

import (
	"fmt"

	errors "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerror "github.com/cosmos/cosmos-sdk/types/errors"
)

const (
	// priorityScalingFactor is a scaling factor to convert the gas price to a priority.
	priorityScalingFactor = 1_000_000
)

// CheckTxFeeWithGlobalMinGasPrices implements the default fee logic, where the minimum price per
// unit of gas is fixed and set globally, and the tx priority is computed from the gas price.
// adapted from https://github.com/celestiaorg/celestia-app/pull/2985
func CheckTxFeeWithGlobalMinGasPrices(ctx sdk.Context, tx sdk.Tx) (sdk.Coins, int64, error) {
	feeTx, ok := tx.(sdk.FeeTx)
	if !ok {
		return nil, 0, errors.Wrap(sdkerror.ErrTxDecode, "Tx must be a FeeTx")
	}

	denom := appparams.DefaultBondDenom

	fee := feeTx.GetFee().AmountOf(denom)
	gas := feeTx.GetGas()

	// convert the global minimum gas price to a big.Int
	globalMinGasPrice, err := sdkmath.LegacyNewDecFromStr(fmt.Sprintf("%f", appparams.GlobalMinGasPrice))
	if err != nil {
		return nil, 0, errors.Wrap(err, "invalid GlobalMinGasPrice")
	}

	gasInt := sdkmath.NewIntFromUint64(gas)
	minFee := globalMinGasPrice.MulInt(gasInt).RoundInt()

	if !fee.GTE(minFee) {
		return nil, 0, errors.Wrapf(sdkerror.ErrInsufficientFee, "insufficient fees; got: %s required: %s", fee, minFee)
	}

	priority := getTxPriority(feeTx.GetFee(), int64(gas))
	return feeTx.GetFee(), priority, nil
}

// getTxPriority returns a naive tx priority based on the amount of the smallest denomination of the gas price
// provided in a transaction.
// NOTE: This implementation should not be used for txs with multiple coins.
func getTxPriority(fee sdk.Coins, gas int64) int64 {
	if gas == 0 {
		return 0
	}

	var priority int64
	for _, c := range fee {
		p := c.Amount.Mul(sdkmath.NewInt(priorityScalingFactor)).QuoRaw(gas)
		if !p.IsInt64() {
			continue
		}
		// take the lowest priority as the tx priority
		if priority == 0 || p.Int64() < priority {
			priority = p.Int64()
		}
	}

	return priority
}
